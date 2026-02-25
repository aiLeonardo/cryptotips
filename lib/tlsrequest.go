package lib

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	profiles "github.com/bogdanfinn/tls-client/profiles"
	"github.com/sasha-s/go-deadlock"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type tlsLogger struct {
	logger *logrus.Logger
}

func newTlsLogger(l *logrus.Logger) *tlsLogger {
	return &tlsLogger{
		logger: l,
	}
}

func (l *tlsLogger) Debug(format string, args ...any) {
	l.logger.Debugf(format, args...)
}
func (l *tlsLogger) Info(format string, args ...any) {
	l.logger.Infof(format, args...)
}
func (l *tlsLogger) Warn(format string, args ...any) {
	l.logger.Warnf(format, args...)
}
func (l *tlsLogger) Error(format string, args ...any) {
	l.logger.Errorf(format, args...)
}

type TlsClient struct {
	jsonStr     []byte
	jsonData    map[string]interface{}
	postData    map[string]string
	fullCookies map[string][]*fhttp.Cookie
	referer     string
	client      tls_client.HttpClient
	jar         tls_client.CookieJar

	mu        *deadlock.Mutex
	logger    *logrus.Logger
	tlsLogger *tlsLogger
	headers   fhttp.Header
}

// NewTlsClient 构造函数
func NewTlsClient() *TlsClient {
	var timeoutSet int = 30
	timeout := viper.GetInt("request.timeout")
	if timeout > 0 {
		timeoutSet = timeout
	}

	// 初始化 options - 使用最新稳定版 Chrome 131 指纹
	cookieJar := tls_client.NewCookieJar()
	var options = []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(timeoutSet),
		tls_client.WithClientProfile(profiles.Chrome_131), // 更新到 Chrome 131
		// tls_client.WithNotFollowRedirects(),
		tls_client.WithCookieJar(cookieJar), // create cookieJar instance and pass it as argument
	}

	if viper.GetBool("proxy.proxyenable") {
		proxyUrl := viper.GetString("proxy.proxyaddr")
		if len(proxyUrl) > 0 {
			options = append(options, tls_client.WithProxyUrl(proxyUrl))
		}
	}

	var loggerObj = LoadLogger()
	var tlsLoggerObj = newTlsLogger(loggerObj)
	client, err := tls_client.NewHttpClient(tlsLoggerObj, options...)
	if err != nil {
		loggerObj.Errorf("tls net client:%s", err)
		return nil
	}

	return &TlsClient{
		fullCookies: make(map[string][]*fhttp.Cookie),
		client:      client,
		jar:         cookieJar,
		mu:          new(deadlock.Mutex),
		logger:      loggerObj,
		tlsLogger:   tlsLoggerObj,
		headers:     make(fhttp.Header),
	}
}

// TlsClientWithProxy 构造函数
func (h *TlsClient) ResetClientWithProxy(proxyUrl, cookieUrl string, cookiesMap map[string]string) error {
	if len(proxyUrl) < 9 {
		return errors.New("empty proxyUrl")
	}

	var timeoutSet int = 30
	timeout := viper.GetInt("request.timeout")
	if timeout > 0 {
		timeoutSet = timeout
	}

	cookieJar := h.makeCookiesByMap(cookieUrl, cookiesMap)
	// 初始化 options - 使用最新稳定版 Chrome 131 指纹
	var options = []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(timeoutSet),
		tls_client.WithClientProfile(profiles.Chrome_131), // 更新到 Chrome 131
		// tls_client.WithNotFollowRedirects(),
		tls_client.WithCookieJar(cookieJar), // create cookieJar instance and pass it as argument
		tls_client.WithProxyUrl(proxyUrl),
	}

	client, err := tls_client.NewHttpClient(h.tlsLogger, options...)
	if err != nil {
		return fmt.Errorf("new tls client: %w", err)
	}

	h.jar = cookieJar
	h.client = client

	return nil
}

// SetPostData 设置表单数据
func (h *TlsClient) SetPostData(data map[string]string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.postData = maps.Clone(data)
}

// SetPostData 设置表单数据
func (h *TlsClient) GetPostData() map[string]string {
	h.mu.Lock()
	defer h.mu.Unlock()

	return maps.Clone(h.postData)
}

// SetJSONData 设置 JSON 数据
func (h *TlsClient) SetJSONData(data map[string]interface{}) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.jsonData = maps.Clone(data)
}

// SetJSONData 设置 JSON 数据
func (h *TlsClient) GetJSONData() map[string]interface{} {
	h.mu.Lock()
	defer h.mu.Unlock()

	return maps.Clone(h.jsonData)
}

// SetJSONData 设置 JSON 数据
func (h *TlsClient) SetJSONStr(data []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.jsonStr = data
}

// SetJSONData 设置 JSON 数据
func (h *TlsClient) GetJSONStr() []byte {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.jsonStr
}

// SetHeaders 设置请求头
func (h *TlsClient) SetReferer(referer string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.referer = referer
}

// 获取当前请求头
func (h *TlsClient) DefaultHeaders() fhttp.Header {
	h.mu.Lock()
	defer h.mu.Unlock()

	var headers = fhttp.Header{
		"Accept":                    {"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7"},
		"Accept-Encoding":           {"gzip, deflate, br, zstd"},
		"Accept-Language":           {"zh-CN,zh;q=0.9,en;q=0.8"},
		"Cache-Control":             {"no-cache"},
		"Pragma":                    {"no-cache"},
		"Priority":                  {"u=0, i"},
		"Referer":                   {h.referer},
		"Sec-Ch-Ua":                 {`"Google Chrome";v="131", "Chromium";v="131", "Not_A Brand";v="24"`},
		"Sec-Ch-Ua-Mobile":          {"?0"},
		"Sec-Ch-Ua-Platform":        {`"Windows"`},
		"Sec-Fetch-Dest":            {"document"},
		"Sec-Fetch-Mode":            {"navigate"},
		"Sec-Fetch-Site":            {"same-origin"},
		"Sec-Fetch-User":            {"?1"},
		"Upgrade-Insecure-Requests": {"1"},
		"User-Agent":                {"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"},

		fhttp.HeaderOrderKey: {
			"Accept",
			"Accept-Encoding",
			"Accept-Language",
			"Cache-Control",
			"Pragma",
			"Priority",
			"Referer",
			"Sec-Ch-Ua",
			"Sec-Ch-Ua-Mobile",
			"Sec-Ch-Ua-Platform",
			"Sec-Fetch-Dest",
			"Sec-Fetch-Mode",
			"Sec-Fetch-Site",
			"Sec-Fetch-User",
			"Upgrade-Insecure-Requests",
			"User-Agent",
		},
	}

	return headers
}

func (h *TlsClient) GetHeaders() fhttp.Header {
	h.mu.Lock()
	headers := maps.Clone(h.headers)
	h.mu.Unlock()

	if len(headers) > 0 {
		return headers
	}

	return h.DefaultHeaders()
}

// SetHeaders 设置请求头
func (h *TlsClient) SetHeaders(headers map[string]string) {
	if len(headers) == 0 {
		return
	}
	var headerOrderKey = []string{
		"Accept",
		"Accept-Language",
		"Cache-Control",
		"Pragma",
		"Priority",
		"Referer",
		"Sec-Fetch-Dest",
		"Sec-Fetch-Mode",
		"Sec-Fetch-Site",
		"User-Agent",
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	h.headers = fhttp.Header{
		fhttp.HeaderOrderKey: headerOrderKey,
	}

	for _, k := range headerOrderKey {
		v, ok := headers[k]
		if ok {
			h.headers[k] = []string{v}
		}
	}
}

// 保存 cookies gocolly jar
func (h *TlsClient) makeCookiesByMap(cookieUrl string, cookieMaps map[string]string) tls_client.CookieJar {
	cookieJar := tls_client.NewCookieJar()
	// 有值的情况下再 reset
	if len(cookieMaps) == 0 {
		return cookieJar
	}

	u, err := url.Parse(cookieUrl)
	if err != nil {
		h.logger.Errorf("parse cookieUrl error: %v, %s", err, cookieUrl)
		return cookieJar
	}

	var httpCookies = make([]*fhttp.Cookie, 0)
	for k, v := range cookieMaps {
		httpCookies = append(httpCookies, &fhttp.Cookie{Name: k, Value: v})
	}

	cookieJar.SetCookies(u, httpCookies)
	return cookieJar
}

// SetHttpCookie 设置请求 Cookies（基于 CookieJar）
func (h *TlsClient) SetHttpCookie(weburl string, cookies []*fhttp.Cookie) {
	h.mu.Lock()
	defer h.mu.Unlock()

	u, err := url.Parse(weburl)
	if err != nil {
		h.logger.Errorf("parse weburl error: %v, %s", err, weburl)
		return
	}

	h.logger.Infoln("setting cookies")
	h.jar.SetCookies(u, cookies)
}

// SetCookies 设置请求 Cookies（基于 CookieJar）
func (h *TlsClient) SetCookies(weburl string, cookies map[string]string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	u, err := url.Parse(weburl)
	if err != nil {
		h.logger.Errorf("parse weburl error: %v, %s", err, weburl)
		return
	}
	var cs = []*fhttp.Cookie{}
	for k, v := range cookies {
		cs = append(cs, &fhttp.Cookie{Name: k, Value: v})
	}

	h.logger.Infof("setting cookies: %v", cookies)
	h.jar.SetCookies(u, cs)
}

// AttachCookieMaps 设置请求 Cookies（基于 CookieJar）
func (h *TlsClient) AttachCookieMaps(weburl string, cookies map[string]string) {
	originCookieMap := h.DumpCookies(weburl)
	maps.Copy(originCookieMap, cookies)

	h.mu.Lock()
	defer h.mu.Unlock()

	u, err := url.Parse(weburl)
	if err != nil {
		h.logger.Errorf("parse weburl error: %v, %s", err, weburl)
		return
	}

	var cs = []*fhttp.Cookie{}
	for k, v := range originCookieMap {
		cs = append(cs, &fhttp.Cookie{Name: k, Value: v})
	}

	h.logger.Infof("setting cookies: %v", originCookieMap)
	h.jar.SetCookies(u, cs)
}

// GetCookies 获取当前 URL 的 cookies（从 CookieJar）
func (h *TlsClient) GetCookies(weburl string) []*fhttp.Cookie {
	h.mu.Lock()
	defer h.mu.Unlock()

	u, err := url.Parse(weburl)
	if err != nil {
		return nil
	}
	return h.jar.Cookies(u)
}

// GetCookies 获取当前 URL 的 cookies（从 CookieJar）
func (h *TlsClient) GetCookiesTxt(weburl string) string {
	h.mu.Lock()
	defer h.mu.Unlock()

	u, err := url.Parse(weburl)
	if err != nil {
		h.logger.Errorf("parse weburl error: %v, %s", err, weburl)
		return ""
	}
	cookies := h.jar.Cookies(u)

	result := ""
	for _, c := range cookies {
		result = c.Name + "=" + c.Value + ";" + result
	}

	return result
}

func (h *TlsClient) DumpCookies(weburl string) map[string]string {
	h.mu.Lock()
	defer h.mu.Unlock()

	u, err := url.Parse(weburl)
	if err != nil {
		h.logger.Errorf("parse weburl error: %v, %s", err, weburl)
		return nil
	}
	cookies := h.jar.Cookies(u)
	result := make(map[string]string)
	for _, c := range cookies {
		result[c.Name] = c.Value
	}
	return result
}

func (h *TlsClient) CleanAllCookies() bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.jar = tls_client.NewCookieJar()
	h.client.SetCookieJar(h.jar)

	return true
}

// 保存 cookies 到文件
func (h *TlsClient) SaveCookiesToFile(weburl string, cookiefile string) error {
	u, err := url.Parse(weburl)
	if err != nil {
		return err
	}

	originCookies := h.jar.Cookies(u)
	hostname := u.Hostname()

	allCookies := map[string][]*fhttp.Cookie{}
	for _, cookie := range originCookies {
		if len(cookie.Domain) > 1 {
			hostname = cookie.Domain
		}

		// h.logger.Infof("saveCookies Cookie:%s: %s=%s", hostname, cookie.Name, cookie.Value)
		allCookies[hostname] = append(allCookies[hostname], cookie)
	}

	data, err := json.MarshalIndent(allCookies, "", "  ")
	if err != nil {
		h.logger.Errorf("json MarshalIndent error: %v", err)
		return err
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	return os.WriteFile(cookiefile, data, os.ModePerm)
}

// 从文件加载 cookies
func (h *TlsClient) LoadCookies(cookiefile string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	data, err := os.ReadFile(cookiefile)
	if err != nil {
		h.logger.Errorf("cookie file load error: %v, %s", err, cookiefile)
		return err
	}
	if len(data) < 9 {
		h.logger.Errorf("empty cookie file: %s", cookiefile)
		return errors.New("empty cookie file")
	}

	var allCookies map[string][]*fhttp.Cookie
	err = json.Unmarshal(data, &allCookies)
	if err != nil {
		h.logger.Errorf("cookies json Unmarshal error: %v", err)
		return err
	}

	for domain, cookies := range allCookies {
		u := &url.URL{Scheme: "http", Host: domain}
		h.jar.SetCookies(u, cookies)
	}
	return nil
}

func (h *TlsClient) ParseResponseBody(weburl string, resp *fhttp.Response) (*CleanResponse, error) {
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	content := string(bodyBytes)

	// 清洗内容
	content = strings.ReplaceAll(content, `\"`, `"`)
	content = strings.ReplaceAll(content, `\r`, "")
	content = strings.ReplaceAll(content, "&#13;", "")
	content = strings.ReplaceAll(content, `\n`, "\n")
	content = strings.ReplaceAll(content, `\t`, " ")
	content = strings.Trim(content, `"`)

	// 提取 cookies
	cookieMap := make(map[string]string)
	u, err := url.Parse(weburl)
	if err == nil {
		for _, c := range h.jar.Cookies(u) {
			cookieMap[c.Name] = c.Value
		}
	}

	return &CleanResponse{
		Content: content,
		Cookies: cookieMap,
	}, nil
}

// DoGetRequest 执行请求
func (h *TlsClient) DoGetRequest(weburl string) (*fhttp.Response, error) {
	var err error
	var resp *fhttp.Response

	for retry := range 3 {
		resp, err = h.DoHttpRequest(weburl, fhttp.MethodGet)
		if err != nil {
			if retry == 0 {
				h.logger.Errorf("do http get error: %v, %s", err, weburl)
			} else {
				h.logger.Errorf("retry do http get error: %v, %s", err, weburl)
			}

			// 失败后短暂休眠
			time.Sleep(time.Duration(retry+1) * time.Second)
			continue
		}

		return resp, nil
	}

	return resp, err
}

// DoPostRequest 执行请求
func (h *TlsClient) DoPostRequest(weburl string) (*fhttp.Response, error) {
	var err error
	var resp *fhttp.Response

	for retry := range 3 {
		resp, err = h.DoHttpRequest(weburl, fhttp.MethodPost)
		if err != nil {
			if retry == 0 {
				h.logger.Errorf("do http post error: %v, %s", err, weburl)
			} else {
				h.logger.Errorf("retry do http post error: %v, %s", err, weburl)
			}

			continue
		}

		return resp, nil
	}

	return resp, err
}

// DoHttpPost 执行请求
func (h *TlsClient) DoHttpRequest(originUrl, method string) (*fhttp.Response, error) {
	method = strings.ToUpper(method)
	if !slices.Contains([]string{"GET", "POST"}, method) {
		return nil, fmt.Errorf("not supported method: %s", method)
	}

	httpHeaders := h.GetHeaders()
	postData := h.GetPostData()
	jsonData := h.GetJSONData()
	jsonStr := h.GetJSONStr()
	// h.logger.Infof("request to: %s, postData: %v, jsonData: %v, headers: %v", weburl, postData, jsonData, httpHeaders)

	var req *fhttp.Request
	var err error

	// 根据数据类型构建请求
	if postData != nil {
		form := url.Values{}
		for k, v := range postData {
			form.Set(k, v)
		}

		req, err = fhttp.NewRequest(method, originUrl, strings.NewReader(form.Encode()))
		req.Header = httpHeaders
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else if jsonData != nil {
		jsonBody, _ := json.Marshal(jsonData)

		req, err = fhttp.NewRequest(method, originUrl, bytes.NewBuffer(jsonBody))
		req.Header = httpHeaders
		req.Header.Set("Content-Type", "application/json, text/javascript, */*; q=0.01")
	} else if jsonStr != nil {
		req, err = fhttp.NewRequest(method, originUrl, bytes.NewBuffer(jsonStr))
		req.Header = httpHeaders
		req.Header.Set("Content-Type", "application/json, text/javascript, */*; q=0.01")
	} else {
		req, err = fhttp.NewRequest(method, originUrl, nil)
		req.Header = httpHeaders
	}

	if err != nil {
		h.logger.Errorf("request create error: %v, %s", err, originUrl)
		return nil, err
	}

	// 执行请求
	resp, err := h.client.Do(req)
	if err != nil {
		//h.logger.Errorf("do request error: %s, resp: %+v", err, resp)
		return nil, err
	}

	switch resp.StatusCode {
	case fhttp.StatusOK, fhttp.StatusPartialContent, fhttp.StatusMovedPermanently, fhttp.StatusFound:
		return resp, nil
	}

	// 错误释放 body
	respString, err := h.GetBodyAsString(resp)
	if err != nil {
		h.logger.Errorf("response read error: %s, resp: %+v", err, resp)
	} else {
		h.logger.Errorf("response contents: %s", respString)
	}

	return resp, fmt.Errorf("StatusCode %d", resp.StatusCode)
}

// 获取响应体（字节）
func (h *TlsClient) GetBodyAsBytes(resp *fhttp.Response) ([]byte, error) {
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		h.logger.Errorf("invaild resp body: %v", err)
		return nil, err
	}
	return data, nil
}

// 获取响应体（字符串）
func (h *TlsClient) GetBodyAsString(resp *fhttp.Response) (string, error) {
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		h.logger.Errorf("invaild resp body: %v", err)
		return "", err
	}
	return string(data), nil
}

// 安全释放 response body
func (h *TlsClient) CloseResponse(resp *fhttp.Response) {
	if resp != nil && resp.Body != nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

// SaveContentsToFile 保存内容到本地文件
func (h *TlsClient) SaveContentsToFile(contents []byte, filename string) bool {
	localDir := filepath.Dir(filename)

	// 如果目录不存在，则创建
	if _, err := os.Stat(localDir); os.IsNotExist(err) {
		err := os.MkdirAll(localDir, os.ModePerm)
		if err != nil {
			h.logger.Errorf("localDir mkdir error: %v, %s", err, localDir)
			return false
		}
	}

	// 写入文件
	if err := os.WriteFile(filename, contents, 0644); err != nil {
		h.logger.Errorf("save contents to file error: %v, %s", err, filename)
		return false
	}

	return true
}

func (h *TlsClient) ResolveURL(baseURL, refRUL string) string {
	r, err := url.Parse(refRUL)
	if err != nil {
		h.logger.Errorf("inviald refRUL: %s", refRUL)
		return refRUL
	}
	if r.IsAbs() {
		return r.String()
	}
	b, _ := url.Parse(baseURL)
	return b.ResolveReference(r).String()
}

// 判断响应是否是 2xx 状态码
func (h *TlsClient) IsStatusOK(resp *fhttp.Response) bool {
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

// 判断是否是重定向 3xx
func (h *TlsClient) IsRedirect(resp *fhttp.Response) bool {
	return resp.StatusCode >= 300 && resp.StatusCode < 400
}

// 判断是否是客户端错误 4xx
func (h *TlsClient) IsClientError(resp *fhttp.Response) bool {
	return resp.StatusCode >= 400 && resp.StatusCode < 500
}

// 判断是否是服务端错误 5xx
func (h *TlsClient) IsServerError(resp *fhttp.Response) bool {
	return resp.StatusCode >= 500
}
