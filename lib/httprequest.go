package lib

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/sasha-s/go-deadlock"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type HttpClient struct {
	jsonStr         []byte
	jsonData        map[string]interface{}
	postData        map[string]string
	fullCookies     map[string][]*http.Cookie
	headers         map[string]string
	client          *http.Client
	jar             *cookiejar.Jar
	cloudBypassIps  []string
	cpOneIPIndex    int
	cloudBypassTime time.Time

	curSessionId string
	mu           *deadlock.Mutex
	logger       *logrus.Logger
}

type CleanResponse struct {
	Content string
	Cookies map[string]string
}

// NewHttpClient 构造函数
func NewHttpClient() *HttpClient {
	jar, _ := cookiejar.New(nil)

	var timeoutSet time.Duration = 30 * time.Second
	timeout := viper.GetInt("request.timeout")
	if timeout > 0 {
		timeoutSet = time.Duration(timeout) * time.Second
	}

	client := &http.Client{
		Timeout: timeoutSet,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Jar: jar,
	}

	// 支持代理配置
	if viper.GetBool("proxy.proxyenable") {
		proxyStr := viper.GetString("proxy.proxyaddr")
		LoadLogger().Infof("配置代理: %s", proxyStr)
		proxyURL, err := url.Parse(proxyStr)
		if err == nil {
			client.Transport = &http.Transport{
				Proxy:           http.ProxyURL(proxyURL),
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
		}
	}

	return &HttpClient{
		fullCookies: map[string][]*http.Cookie{},
		headers:     make(map[string]string),
		client:      client,
		jar:         jar,
		mu:          new(deadlock.Mutex),
		logger:      LoadLogger(),
	}
}

// SetPostData 设置表单数据
func (h *HttpClient) SetPostData(data map[string]string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.postData = maps.Clone(data)
}

// SetPostData 设置表单数据
func (h *HttpClient) GetPostData() map[string]string {
	h.mu.Lock()
	defer h.mu.Unlock()

	return maps.Clone(h.postData)
}

// SetJSONData 设置 JSON 数据
func (h *HttpClient) SetJSONData(data map[string]interface{}) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.jsonData = maps.Clone(data)
}

// SetJSONData 设置 JSON 数据
func (h *HttpClient) GetJSONData() map[string]interface{} {
	h.mu.Lock()
	defer h.mu.Unlock()

	return maps.Clone(h.jsonData)
}

// SetJSONData 设置 JSON 数据
func (h *HttpClient) SetJSONStr(data []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.jsonStr = data
}

// SetJSONData 设置 JSON 数据
func (h *HttpClient) GetJSONStr() []byte {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.jsonStr
}

// SetHeaders 设置请求头
func (h *HttpClient) SetHeaders(headers map[string]string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for k, v := range headers {
		h.headers[k] = v
	}
}

// GetHeaders 获取当前请求头
func (h *HttpClient) GetHeaders() map[string]string {
	h.mu.Lock()
	defer h.mu.Unlock()

	return maps.Clone(h.headers)
}

func (h *HttpClient) SetFullCookies(weburl string, cookies []*http.Cookie) {
	u, err := url.Parse(weburl)
	if err != nil {
		h.logger.Errorf("url.parse err:%s", err)
	}

	var hostname string
	// 循环取 cookie
	for _, cookie := range cookies {
		hostname = cookie.Domain
		if hostname == "" {
			hostname = u.Hostname()
		}

		if len(h.fullCookies[hostname]) == 0 {
			h.fullCookies[hostname] = []*http.Cookie{}
		}

		// 追加到全局变量
		h.fullCookies[hostname] = append(h.fullCookies[hostname], cookie)
	}
}

// SetHttpCookie 设置请求 Cookies（基于 CookieJar）
func (h *HttpClient) SetHttpCookie(weburl string, cookies []*http.Cookie) {
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
func (h *HttpClient) SetCookies(weburl string, cookies map[string]string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	u, err := url.Parse(weburl)
	if err != nil {
		h.logger.Errorf("parse weburl error: %v, %s", err, weburl)
		return
	}
	var cs = []*http.Cookie{}
	for k, v := range cookies {
		cs = append(cs, &http.Cookie{Name: k, Value: v})
	}

	h.logger.Infof("setting cookies: %v", cookies)
	h.jar.SetCookies(u, cs)
}

// AttachCookieMaps 设置请求 Cookies（基于 CookieJar）
func (h *HttpClient) AttachCookieMaps(weburl string, cookies map[string]string) {
	originCookieMap := h.DumpCookies(weburl)
	maps.Copy(originCookieMap, cookies)

	h.mu.Lock()
	defer h.mu.Unlock()

	u, err := url.Parse(weburl)
	if err != nil {
		h.logger.Errorf("parse weburl error: %v, %s", err, weburl)
		return
	}

	var cs = []*http.Cookie{}
	for k, v := range originCookieMap {
		cs = append(cs, &http.Cookie{Name: k, Value: v})
	}

	h.logger.Infof("setting cookies: %v", originCookieMap)
	h.jar.SetCookies(u, cs)
}

// GetCookies 获取当前 URL 的 cookies（从 CookieJar）
func (h *HttpClient) GetCookies(weburl string) []*http.Cookie {
	h.mu.Lock()
	defer h.mu.Unlock()

	u, err := url.Parse(weburl)
	if err != nil {
		return nil
	}
	return h.jar.Cookies(u)
}

// GetCookies 获取当前 URL 的 cookies（从 CookieJar）
func (h *HttpClient) GetCookiesTxt(weburl string) string {
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

func (h *HttpClient) DumpCookies(weburl string) map[string]string {
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

func (h *HttpClient) CleanAllCookies() bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.jar, _ = cookiejar.New(nil)
	h.client.Jar = h.jar

	return true
}

// 保存 cookies 到文件
func (h *HttpClient) SaveCookiesToFile(weburl string, cookiefile string) error {
	u, err := url.Parse(weburl)
	if err != nil {
		return err
	}

	originCookies := h.jar.Cookies(u)
	hostname := u.Hostname()

	allCookies := map[string][]*http.Cookie{}
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
func (h *HttpClient) LoadCookies(cookiefile string) error {
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

	var allCookies map[string][]*http.Cookie
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

func (h *HttpClient) ParseResponseBody(weburl string, resp *http.Response) (*CleanResponse, error) {
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
func (h *HttpClient) DoGetRequest(weburl string) (*http.Response, error) {
	var err error
	var resp *http.Response

	for retry := range 3 {
		resp, err = h.DoHttpRequest(weburl, "GET")
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

// DoGetRequest 执行请求
func (h *HttpClient) DoPostRequest(weburl string) (*http.Response, error) {
	var err error
	var resp *http.Response

	for retry := range 3 {
		resp, err = h.DoHttpRequest(weburl, "POST")
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
func (h *HttpClient) DoHttpRequest(weburl, method string) (*http.Response, error) {
	httpHeaders := h.GetHeaders()
	postData := h.GetPostData()
	jsonData := h.GetJSONData()
	jsonStr := h.GetJSONStr()
	h.logger.Infof("request to: %s, postData len: %d, jsonData len: %d, headers len: %d", weburl, len(postData), len(jsonData), len(httpHeaders))

	var req *http.Request
	var err error

	// 根据数据类型构建请求
	if postData != nil {
		form := url.Values{}
		for k, v := range postData {
			form.Set(k, v)
		}
		req, err = http.NewRequest("POST", weburl, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else if jsonData != nil {
		jsonBody, _ := json.Marshal(jsonData)
		req, err = http.NewRequest("POST", weburl, bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json, text/javascript, */*; q=0.01")
	} else if jsonStr != nil {
		req, err = http.NewRequest("POST", weburl, bytes.NewBuffer(jsonStr))
		req.Header.Set("Content-Type", "application/json, text/javascript, */*; q=0.01")
	} else {
		method = strings.ToUpper(method)
		if !slices.Contains([]string{"GET", "POST"}, method) {
			return nil, fmt.Errorf("not supported method: %s", method)
		}

		req, err = http.NewRequest(method, weburl, nil)
	}

	if err != nil {
		h.logger.Errorf("request create error: %v, %s", err, weburl)
		return nil, err
	}

	// 添加 Headers
	for k, v := range httpHeaders {
		req.Header.Set(k, v)
	}

	// 执行请求
	resp, err := h.client.Do(req)
	if err != nil {
		h.logger.Errorf("do request error: %s, resp: %+v", err, resp)
		return nil, err
	}
	// h.SetFullCookies(originUrl, resp.Cookies()) // 记录下cookies值

	if resp.StatusCode == http.StatusOK {
		return resp, nil
	}

	// 错误释放 body
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	return resp, fmt.Errorf("failed StatusCode %d", resp.StatusCode)
}

// 获取响应体（字节）
func (h *HttpClient) GetBodyAsBytes(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		h.logger.Errorf("invaild resp body: %v", err)
		return nil, err
	}
	return data, nil
}

// 获取响应体（字符串）
func (h *HttpClient) GetBodyAsString(resp *http.Response) (string, error) {
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		h.logger.Errorf("invaild resp body: %v", err)
		return "", err
	}
	return string(data), nil
}

// 安全释放 response body
func (h *HttpClient) CloseResponse(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

// SaveContentsToFile 保存内容到本地文件
func (h *HttpClient) SaveContentsToFile(contents []byte, filename string) bool {
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

func (h *HttpClient) ResolveURL(baseURL, refRUL string) string {
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
func (h *HttpClient) IsStatusOK(resp *http.Response) bool {
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

// 判断是否是重定向 3xx
func (h *HttpClient) IsRedirect(resp *http.Response) bool {
	return resp.StatusCode >= 300 && resp.StatusCode < 400
}

// 判断是否是客户端错误 4xx
func (h *HttpClient) IsClientError(resp *http.Response) bool {
	return resp.StatusCode >= 400 && resp.StatusCode < 500
}

// 判断是否是服务端错误 5xx
func (h *HttpClient) IsServerError(resp *http.Response) bool {
	return resp.StatusCode >= 500
}
