package lib

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"maps"
	"math"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// 下载进度信息
type DownloadProgress struct {
	urlmark    string
	Total      int64
	Downloaded int64 // 使用 atomic 操作
	Speed      int64
	StartTime  time.Time
	mu         sync.RWMutex // 保护非原子字段
	lastUpdate time.Time
	lastBytes  int64

	logger *logrus.Logger
}

// 线程安全的进度更新
func (p *DownloadProgress) AddDownloaded(bytes int64) {
	atomic.AddInt64(&p.Downloaded, bytes)
}

// 获取当前下载量
func (p *DownloadProgress) GetDownloaded() int64 {
	return atomic.LoadInt64(&p.Downloaded)
}

// 显示下载进度
func (p *DownloadProgress) Update() {
	now := time.Now()
	downloaded := p.GetDownloaded()

	p.mu.Lock()
	defer p.mu.Unlock()

	// 计算速度（每秒字节数）
	if now.Sub(p.lastUpdate) >= time.Second {
		elapsed := now.Sub(p.lastUpdate).Seconds()
		if elapsed > 0 {
			p.Speed = int64(float64(downloaded-p.lastBytes) / elapsed)
		}
		p.lastUpdate = now
		p.lastBytes = downloaded
	}

	// 显示进度
	percentage := float64(downloaded) / float64(p.Total) * 100
	speedMB := float64(p.Speed) / 1024 / 1024
	totalMB := float64(p.Total) / 1024 / 1024
	downloadedMB := float64(downloaded) / 1024 / 1024

	p.logger.Infof("当前下载进度: %.2f%% (%.2f MB / %.2f MB) 速度: %.2f MB/s, [%s]",
		percentage, downloadedMB, totalMB, speedMB, p.urlmark)
}

// 分块信息
type ChunkInfo struct {
	Index    int
	Start    int64
	End      int64
	Filename string
	Size     int64
}

// 进度读取器（用于单线程下载）
type ProgressReader struct {
	Reader   io.Reader
	Progress *DownloadProgress
	mu       sync.Mutex
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.Reader.Read(p)
	if n > 0 {
		pr.Progress.AddDownloaded(int64(n))

		// 每秒更新一次进度显示
		pr.mu.Lock()
		now := time.Now()
		if now.Sub(pr.Progress.lastUpdate) >= time.Second {
			pr.Progress.Update()
		}
		pr.mu.Unlock()
	}
	return n, err
}

type BigfileDownloader struct {
	maxRetries      int
	chunkSize       int64
	bufferSize      int
	timeout         time.Duration
	mu              *sync.RWMutex
	rotatingProxies []string
	headers         map[string]string
	logger          *logrus.Logger
	client          *http.Client
}

// 创建新的下载器
func NewBigfileDownloader() *BigfileDownloader {
	return &BigfileDownloader{
		maxRetries: 3,
		chunkSize:  1 * 1024 * 1024, // 默认1MB每块
		bufferSize: 64 * 1024,       // 64KB缓冲区
		timeout:    60 * time.Second,
		mu:         &sync.RWMutex{},
		headers:    map[string]string{},
		logger:     LoadLogger(),
	}
}

// SetHeaders 设置请求头
func (d *BigfileDownloader) SetHeaders(headers map[string]string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	maps.Copy(d.headers, headers)
}

// SetRotatingProxies 设置轮询代理
func (d *BigfileDownloader) SetRotatingProxies(proxies []string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.rotatingProxies = proxies
}

func (d *BigfileDownloader) makeHttpClient() *http.Client {
	if d.client != nil {
		return d.client
	}

	jar, _ := cookiejar.New(nil)

	var timeoutSet time.Duration = d.timeout
	timeout := viper.GetInt("request.timeout")
	if timeout > 0 {
		timeoutSet = time.Duration(timeout) * time.Second
	}

	d.client = &http.Client{
		Timeout: timeoutSet,
		Jar:     jar,
	}

	// 支持代理配置
	clientTransport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	if viper.GetBool("proxy.proxyenable") {
		proxyStr := viper.GetString("proxy.proxyaddr")
		// LoadLogger().Infof("配置代理: %s", proxyStr)
		proxyURL, err := url.Parse(proxyStr)
		if err == nil {
			clientTransport.Proxy = http.ProxyURL(proxyURL)
		}
	}
	d.client.Transport = clientTransport
	d.logger.Infof("http client set timeout: %02ds", d.client.Timeout/time.Second)

	return d.client
}

// 支持断点续传的下载（推荐用于大文件）
func (d *BigfileDownloader) downloadWithResume(resLink, filename string) error {
	d.logger.Infof("开始下载（支持断点续传）: %s", resLink)
	// 判断是否需要下载
	if d.fileExists(filename) {
		d.logger.Infof("文件已经存在,跳过:%s", filename)
		return nil
	}

	// 检测上级目录
	dirSave := path.Dir(filename)
	if !d.dirExists(dirSave) {
		err := os.MkdirAll(dirSave, os.ModePerm)
		if err != nil {
			return fmt.Errorf("when mkdir:%s", err)
		}
	}

	// 检查文件是否已存在
	var startPos int64 = 0
	if info, err := os.Stat(filename); err == nil {
		startPos = info.Size()
		d.logger.Infof("发现已存在文件，从 %d 字节处继续下载", startPos)
	}

	// 创建 HTTP 请求
	req, err := http.NewRequest("GET", resLink, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}
	// 添加 Headers
	for k, v := range d.headers {
		req.Header.Set(k, v)
	}

	// 设置 Range 头实现断点续传
	if startPos > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", startPos))
	}

	client := d.makeHttpClient()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %s", err)
	}
	defer resp.Body.Close()

	// 检查状态码
	var statusOkList = []int{http.StatusPartialContent, http.StatusOK}
	if !slices.Contains(statusOkList, resp.StatusCode) {
		return fmt.Errorf("HTTP 状态错误:%s, 状态码: %s", filename, resp.Status)
	}
	d.logger.Infof("resp stauts: %s(%03dkb)", resp.Status, resp.ContentLength/1024)

	// 获取总文件大小
	var totalSize int64
	if resp.StatusCode == http.StatusPartialContent {
		// 从 Content-Range 头获取总大小
		contentRange := resp.Header.Get("Content-Range")
		if contentRange != "" {
			var start, end int64
			n, parseErr := fmt.Sscanf(contentRange, "bytes %d-%d/%d", &start, &end, &totalSize)
			if parseErr != nil || n != 3 {
				totalSize = startPos + resp.ContentLength
			}
		} else {
			totalSize = startPos + resp.ContentLength
		}
	} else {
		totalSize = resp.ContentLength
	}

	if totalSize <= 0 {
		return fmt.Errorf("无法获取文件大小")
	}

	// 打开文件进行追加写入
	var file *os.File
	if startPos > 0 {
		file, err = os.OpenFile(filename, os.O_WRONLY|os.O_APPEND, 0644)
	} else {
		file, err = os.Create(filename)
	}
	if err != nil {
		return fmt.Errorf("打开文件失败: %v", err)
	}
	defer file.Close()

	// 初始化进度
	progress := &DownloadProgress{
		urlmark:    resLink,
		Total:      totalSize,
		Downloaded: startPos,
		StartTime:  time.Now(),
		lastUpdate: time.Now(),
		lastBytes:  startPos,
		logger:     d.logger,
	}

	// 创建进度监控器
	progressReader := &ProgressReader{
		Reader:   resp.Body,
		Progress: progress,
	}

	// 复制数据
	_, err = io.Copy(file, progressReader)
	if err != nil {
		return fmt.Errorf("下载失败: %s", err)
	}

	d.logger.Infof("下载完成! 总耗时: %.2fs", float64(time.Since(progress.StartTime))/float64(time.Second))
	return nil
}

// 多线程分块下载（适合超大文件如20GB+）
func (d *BigfileDownloader) DownloadMultithread(resLink, filename string, concurrency int) error {
	d.logger.Infof("开始多线程下载（%d 个线程）: %s", concurrency, resLink)

	// 判断是否需要下载
	if d.fileExists(filename) {
		d.logger.Infof("文件已经存在,跳过:%s", filename)
		return nil
	}

	// 获取文件信息
	client := d.makeHttpClient()
	resp, err := client.Head(resLink)
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %v", err)
	}
	defer func() {
		if resp.Body != nil {
			resp.Body.Close()
		}
	}()

	// 大文件下载， 低于 0.5kb的，直接忽略
	fileSize := resp.ContentLength
	if fileSize < 512 {
		return fmt.Errorf("无法获取文件大小")
	}

	// 检查是否支持 Range 请求
	if resp.Header.Get("Accept-Ranges") != "bytes" {
		d.logger.Errorf("Header未找到Accept-Ranges,文件大小：%.2fmb", float64(fileSize)/1024/1024)
	}

	d.logger.Infof("当前资源大小: %.2fmb", float64(fileSize)/1024/1024)

	// 计算总块数
	var chunks = int(math.Ceil(float64(fileSize) / float64(d.chunkSize)))
	d.logger.Infof("文件被拆成%d个子文件[%s][%s]", chunks, filename, resLink)

	// 创建分块信息
	var chunkChanInfo chan ChunkInfo = make(chan ChunkInfo, chunks)
	var chunkList = make([]ChunkInfo, 0)
	for i := 0; i < chunks; i++ {
		start := int64(i) * d.chunkSize
		end := start + d.chunkSize - 1

		// 最后一块可能大小不同
		if i == chunks-1 {
			end = fileSize - 1
		}

		tmpChunk := ChunkInfo{
			Index:    1 + i,
			Start:    start,
			End:      end,
			Filename: fmt.Sprintf("%s.part%03d", filename, 1+i),
			Size:     end - start + 1,
		}

		chunkChanInfo <- tmpChunk
		chunkList = append(chunkList, tmpChunk)
	}
	close(chunkChanInfo)

	// 进度跟踪
	progress := &DownloadProgress{
		urlmark:    resLink,
		Total:      fileSize,
		StartTime:  time.Now(),
		lastUpdate: time.Now(),
		logger:     LoadLogger(),
	}

	// 下载所有分块
	err = d.downloadAllChunks(resLink, chunkChanInfo, progress, concurrency)
	if err != nil {
		return err
	}

	// 合并文件
	err = d.mergeFiles(filename, chunkList)
	if err != nil {
		d.removeFile(filename)
		return fmt.Errorf("合并文件失败: %v", err)
	}

	// 清理临时文件
	defer d.cleanupTempFiles(chunkList)

	// 验证下载
	if err := d.verifyDownload(filename, fileSize); err != nil {
		d.removeFile(filename)
		return fmt.Errorf("下载后的文件大小验证失败: %s[%s]", err, resLink)
	}

	d.logger.Infof("多线程下载完成! 总耗时: %v", time.Since(progress.StartTime))
	return nil
}

// 下载所有分块
func (d *BigfileDownloader) downloadAllChunks(resLink string, chunkChanInfo <-chan ChunkInfo, progress *DownloadProgress, concurrency int) error {
	var wg sync.WaitGroup
	var lastErr error
	var mu sync.Mutex

	// 启动多个协程下载
	for range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for chunk := range chunkChanInfo {
				err := d.downloadChunkWithRetry(resLink, chunk, progress)
				if err != nil {
					mu.Lock()
					lastErr = fmt.Errorf("分块 %s[%d-%d] 下载失败: %v", chunk.Filename, chunk.Start, chunk.End, err)
					mu.Unlock()
				}
			}
		}()
	}

	// 等待所有下载完成
	wg.Wait()
	if lastErr != nil {
		return lastErr
	}

	return nil
}

// 下载单个分块到独立文件
func (d *BigfileDownloader) downloadChunkWithRetry(resLink string, chunk ChunkInfo, progress *DownloadProgress) error {
	// 带重试的下载
	var lastErr error
	for attempt := 0; attempt <= d.maxRetries; attempt++ {
		if attempt > 0 {
			waitTime := time.Duration(attempt) * time.Second
			time.Sleep(waitTime)
		}

		err := d.downloadChunkOnce(resLink, chunk, progress)
		if err == nil {
			return nil
		}

		lastErr = err
		d.logger.Errorf("downloadChunkOnce error: %s", lastErr)
	}

	return fmt.Errorf("重试 %d 次后仍然失败: %v", d.maxRetries, lastErr)
}

// 单次下载分块
func (d *BigfileDownloader) downloadChunkOnce(resLink string, chunk ChunkInfo, progress *DownloadProgress) error {
	// 判断是否需要下载
	if d.fileExists(chunk.Filename) {
		if err := d.verifyDownload(chunk.Filename, chunk.Size); err != nil {
			d.removeFile(chunk.Filename)
			return fmt.Errorf("被中断的下载文件: %s[%s]", err, resLink)
		}

		d.logger.Infof("文件已经存在,跳过:%s [%s]", chunk.Filename, resLink)
		return nil
	}

	// 检测上级目录
	dirSave := path.Dir(chunk.Filename)
	if !d.dirExists(dirSave) {
		err := os.MkdirAll(dirSave, os.ModePerm)
		if err != nil {
			return fmt.Errorf("when mkdir:%s", err)
		}
	}

	req, err := http.NewRequest("GET", resLink, nil)
	if err != nil {
		return fmt.Errorf("new request:%s", err)
	}

	// 设置 Range 头
	d.logger.Infof("download range: [%d-%d], %s", chunk.Start, chunk.End, chunk.Filename)

	// 添加 Headers
	for k, v := range d.headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", chunk.Start, chunk.End))

	client := d.makeHttpClient()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http request error:%s, filename: %s [%s]", err, chunk.Filename, resLink)
	}
	defer resp.Body.Close()

	// resp.StatusCode != http.StatusPartialContent
	var statusOkList = []int{http.StatusPartialContent, http.StatusOK}
	if !slices.Contains(statusOkList, resp.StatusCode) {
		return fmt.Errorf("分块请求失败%s, 状态码: %s [%s]", chunk.Filename, resp.Status, resLink)
	}
	var contentLength int64 = resp.ContentLength

	// 创建分块文件
	file, err := os.Create(chunk.Filename)
	if err != nil {
		return fmt.Errorf("when create file:%s", err)
	}

	// 创建缓冲区
	buffer := make([]byte, d.bufferSize)
	var downloaded int64
	for downloaded < contentLength {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			_, writeErr := file.Write(buffer[:n])
			if writeErr != nil {
				file.Close()
				d.removeFile(chunk.Filename)
				return writeErr
			}

			downloaded += int64(n)
			progress.AddDownloaded(int64(n))
		}

		if err != nil {
			if err == io.EOF {
				break
			}

			file.Close()
			d.removeFile(chunk.Filename)
			return fmt.Errorf("when read resp body:%s [%s]", err, resLink)
		}
	}
	progress.Update()

	// 验证分块大小
	if downloaded < chunk.Size {
		file.Close()
		d.removeFile(chunk.Filename)
		return fmt.Errorf("分块 %d 大小不匹配: 期望值:%d, ContentLength:%d, Range: %d-%d, 已下载: %d [%s]",
			chunk.Index, chunk.Size, contentLength, chunk.Start, chunk.End, downloaded, chunk.Filename)
	}
	file.Close()

	return nil
}

// 合并所有分块文件
func (d *BigfileDownloader) mergeFiles(filename string, chunkInfos []ChunkInfo) error {
	d.logger.Infof("开始合并文件...")

	// 检测上级目录
	dirSave := path.Dir(filename)
	if !d.dirExists(dirSave) {
		err := os.MkdirAll(dirSave, os.ModePerm)
		if err != nil {
			return fmt.Errorf("when mkdir:%s", err)
		}
	}

	// 创建最终文件
	outFile, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer outFile.Close()

	// 按顺序合并所有分块
	for _, chunk := range chunkInfos {
		d.logger.Infof("正在合并分块 %s/%d", chunk.Filename, len(chunkInfos))
		if !d.fileExists(chunk.Filename) {
			return fmt.Errorf("文件不存在: %s", chunk.Filename)
		}

		inFile, err := os.Open(chunk.Filename)
		if err != nil {
			return fmt.Errorf("打开分块文件 %s 失败: %v", chunk.Filename, err)
		}

		_, err = io.Copy(outFile, inFile)
		inFile.Close() // 立即关闭
		if err != nil {
			return fmt.Errorf("合并分块 %s 失败: %v", chunk.Filename, err)
		}
	}

	d.logger.Infof("文件合并完成:%s", filename)
	return nil
}

// 清理临时文件
func (d *BigfileDownloader) cleanupTempFiles(chunkInfos []ChunkInfo) {
	for _, chunk := range chunkInfos {
		err := os.Remove(chunk.Filename)
		if err != nil {
			d.logger.Errorf("remove file error:%s", err)
		}
	}
}

// 清理临时文件
func (d *BigfileDownloader) removeFile(filename string) error {
	err := os.Remove(filename)
	if err != nil {
		d.logger.Errorf("remove file error:%s", err)
	}

	return err
}

// 获取文件大小
func (d *BigfileDownloader) getFileSize(resLink string) (int64, error) {
	client := d.makeHttpClient()
	resp, err := client.Head(resLink)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HTTP 状态错误: %s", resp.Status)
	}

	contentLength := resp.Header.Get("Content-Length")
	if contentLength == "" {
		return 0, fmt.Errorf("服务器未提供文件大小信息")
	}

	size, err := strconv.ParseInt(contentLength, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("解析文件大小失败: %v", err)
	}

	return size, nil
}

// 验证下载完整性
func (d *BigfileDownloader) verifyDownload(filename string, expectedSize int64) error {
	info, err := os.Stat(filename)
	if err != nil {
		return fmt.Errorf("无法获取文件信息: %v", err)
	}

	if info.Size() < expectedSize {
		return fmt.Errorf("总文件大小不匹配: 期望 %d 字节，实际 %d 字节",
			expectedSize, info.Size())
	}

	d.logger.Infof("文件下载完整性验证通过")
	return nil
}

// 高级下载函数，包含重试机制
func (d *BigfileDownloader) DownloadWithRetry(resLink, filename string, maxRetries int) error {
	// 判断是否需要下载
	if d.fileExists(filename) {
		d.logger.Infof("文件已经存在,跳过:%s", filename)
		return nil
	}

	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		d.logger.Infof("尝试下载 (第 %d/%d 次)...", attempt, maxRetries)

		err := d.downloadWithResume(resLink, filename)
		if err == nil {
			return nil // 成功
		}

		lastErr = err
		d.logger.Errorf("下载失败: %v", err)

		if attempt < maxRetries {
			waitTime := time.Duration(attempt) * 3 * time.Second
			d.logger.Infof("等待 %v 后重试...", waitTime)
			time.Sleep(waitTime)
		}
	}

	return fmt.Errorf("重试 %d 次后仍然失败: %v", maxRetries, lastErr)
}

// 智能下载函数，自动选择下载方式
func (d *BigfileDownloader) SmartDownload(resLink, filename string) error {
	resLink = strings.TrimSpace(resLink)
	if resLink == "" {
		d.logger.Errorf("resLink 为空:%s", resLink)
		return nil
	}

	// 判断是否需要下载
	if d.fileExists(filename) {
		d.logger.Infof("文件已经存在,跳过:%s", filename)
		return nil
	}

	// 获取文件大小
	fileSize, err := d.getFileSize(resLink)
	if err != nil {
		return fmt.Errorf("获取文件大小失败: %v", err)
	}

	// 正常文件大小,不会小于9字节
	if fileSize < 9 {
		return fmt.Errorf("获取文件大小失败: 0")
	}

	d.logger.Infof("文件大小: %.2f MB", float64(fileSize)/1024/1024)

	// 根据文件大小智能选择下载方式
	const (
		singleThreadThreshold = 2 * 1024 * 1024  // 2MB
		multiThreadThreshold  = 10 * 1024 * 1024 // 10MB
	)

	if fileSize < singleThreadThreshold {
		d.logger.Infof("使用单线程下载...")
		return d.downloadWithResume(resLink, filename)
	} else if fileSize < multiThreadThreshold {
		d.logger.Infof("使用4线程下载...")
		return d.DownloadMultithread(resLink, filename, 2)
	}

	d.logger.Infof("使用4线程下载...")
	return d.DownloadMultithread(resLink, filename, 4)
}

func (d *BigfileDownloader) fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func (d *BigfileDownloader) dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// 测试函数
func TestDownload() {
	down := NewBigfileDownloader()

	// 示例使用
	resLink := "https://example.com/large-file.zip" // 替换为实际的下载链接
	filename := "large-file.zip"

	// 智能下载
	err := down.SmartDownload(resLink, filename)
	if err != nil {
		log.Fatal("下载失败:", err)
	}

	fmt.Println("下载并验证成功!")
}
