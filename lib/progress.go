package lib

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

type CrawlProgress struct {
	logger *logrus.Logger
}

// NewCrawlProgress 创建 CrawlProgress 实例
func NewCrawlProgress() *CrawlProgress {
	return &CrawlProgress{
		logger: LoadLogger(),
	}
}

// getProgressFilePath 返回存储进度文件路径
func (cp *CrawlProgress) getProgressFilePath(filename string) string {
	execPath, _ := os.Executable()
	baseDir := filepath.Dir(execPath)
	baseName := filepath.Base(filename)
	localFile := filepath.Join(baseDir, "logs", baseName+".pgs")

	return localFile
}

// allCrawlProgress 读取所有任务的进度
func (cp *CrawlProgress) allCrawlProgress(filename string) map[string]string {
	localFile := cp.getProgressFilePath(filename)
	if _, err := os.Stat(localFile); os.IsNotExist(err) {
		cp.logger.Errorf("IP列表所在文件不存在 %s", filename)
		return nil
	}

	data, err := os.ReadFile(localFile)
	if err != nil {
		cp.logger.Errorf("无法读取IP列表所在文件 %s", filename)
		return nil
	}

	var progressInfo map[string]string
	if err := json.Unmarshal(data, &progressInfo); err != nil {
		cp.logger.Errorf("json unmarshal error: %v", err)
		return nil
	}

	return progressInfo
}

// loadCrawlProgress 加载某个任务的进度
func (cp *CrawlProgress) loadCrawlProgress(filename, taskname string) string {
	progressInfo := cp.allCrawlProgress(filename)
	if progressInfo == nil {
		cp.logger.Errorf("无法读取代理IP")
		return ""
	}

	progress, ok := progressInfo[taskname]
	if ok {
		return progress
	}

	return ""
}

// HasProgressExists 判断任务是否有记录进度
func (cp *CrawlProgress) HasProgressExists(filename, taskname string) bool {
	progress := cp.loadCrawlProgress(filename, taskname)
	return len(progress) > 0
}

// IsEqualProgress 判断当前进度是否与保存的一致
func (cp *CrawlProgress) IsEqualProgress(filename, taskname, currProgress string) bool {
	saved := cp.loadCrawlProgress(filename, taskname)
	if len(saved) == 0 || len(currProgress) == 0 {
		return false
	}
	return saved == currProgress
}

// SetCrawlProgress 设置任务进度
func (cp *CrawlProgress) SetCrawlProgress(filename, taskname, progress string) bool {
	progressInfo := cp.allCrawlProgress(filename)
	if progressInfo == nil {
		progressInfo = make(map[string]string)
	}

	progressInfo[taskname] = progress
	localFile := cp.getProgressFilePath(filename)

	// 确保 logs 目录存在
	dir := filepath.Dir(localFile)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		cp.logger.Errorf("mkdir error: %v, %s", err, dir)
		return false
	}

	data, err := json.MarshalIndent(progressInfo, "", "  ")
	if err != nil {
		cp.logger.Errorf("json marshal error: %v", err)
		return false
	}

	if err := os.WriteFile(localFile, data, 0644); err != nil {
		cp.logger.Errorf("write file error: %v, file: %s", err, localFile)
		return false
	}

	return true
}
