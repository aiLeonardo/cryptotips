package lib

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
)

type TgInfo struct {
	TgToken string
	ChatID  string

	MsgInterval int
	MsgContents []string
	SentMsgAt   time.Time
}

var tginfo *TgInfo
var tginfoOnce sync.Once // 控制只初始化一次

// readToken 读取 Telegram 信息
func loadTgtoken() bool {
	tginfoOnce.Do(func() {
		tginfo = &TgInfo{

			TgToken: viper.GetString("tginfo.tg_token"),
			ChatID:  viper.GetString("tginfo.chat_id"),

			MsgInterval: viper.GetInt("tginfo.msg_interval"),
			MsgContents: []string{},
			SentMsgAt:   time.Now(),
		}
	})

	return true
}

// NoticeWhenError 发送 Telegram 通知
func NoticeWhenError(taskName string, message string) {
	loadTgtoken()

	// 消息暂存
	tempMsg := fmt.Sprintf("%s: %s", taskName, message)
	tginfo.MsgContents = append(tginfo.MsgContents, tempMsg)

	// 每隔 1 秒, 发一次(tginfo.MsgInterval 秒)
	secondsSince := time.Since(tginfo.SentMsgAt)
	if secondsSince < time.Second*time.Duration(tginfo.MsgInterval) {
		return
	}

	tginfo.SentMsgAt = time.Now() // 时间重置
	msgContents := strings.Join(tginfo.MsgContents, "\n")
	tginfo.MsgContents = []string{} //内容重置

	// fmt.Println("message: ", taskName, message)
	apiUrl := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", tginfo.TgToken)

	payload := map[string]string{
		"chat_id": tginfo.ChatID,
		"text":    msgContents,
	}

	postData(apiUrl, payload)
}

// postData 执行 POST 请求
func postData(api string, data map[string]string) []byte {
	bodyBytes, err := json.Marshal(data)
	if err != nil {
		fmt.Println("JSON 编码失败:", err)
		return nil
	}

	proxyFunc := http.ProxyFromEnvironment
	if viper.GetBool("proxy.proxyenable") {
		proxyAddr := viper.GetString("proxy.proxyaddr")
		if proxyAddr != "" {
			parsedProxy, _ := url.Parse(proxyAddr)
			proxyFunc = http.ProxyURL(parsedProxy)
		}
	}

	client := &http.Client{
		Timeout: 300 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			Proxy:           proxyFunc,
		},
	}

	req, err := http.NewRequest("POST", api, bytes.NewReader(bodyBytes))
	if err != nil {
		fmt.Println("创建请求失败:", err)
		return nil
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("请求失败:", err)
		return nil
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("读取响应失败:", err)
		return nil
	}
	// fmt.Println("api: ", api)
	// fmt.Println("bodyBytes: ", string(bodyBytes))
	// fmt.Println("respData: ", string(respData))

	return respData
}
