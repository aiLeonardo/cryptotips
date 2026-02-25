package lib

import (
	"crypto/tls"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"

	"github.com/aiLeonardo/cryptotips/lib/api2captcha"

	"github.com/spf13/viper"
)

type my2captcha struct {
	apiKey     string
	numberic   int
	reqTimeout time.Duration
	cookiejar  *cookiejar.Jar
}

func NewMy2captcha() *my2captcha {
	cookiejar, _ := cookiejar.New(nil)
	return &my2captcha{
		apiKey:     viper.GetString("2captcha.apiKey"),
		numberic:   1,
		reqTimeout: 30 * time.Second,
		cookiejar:  cookiejar,
	}
}

func (c *my2captcha) SetNumberic(numberic int) {
	c.numberic = numberic
}

func (c *my2captcha) Clientinit() *api2captcha.Client {
	httpClient := &http.Client{
		Timeout: c.reqTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Jar: c.cookiejar,
	}

	// 支持代理配置
	if viper.GetBool("proxy.proxyenable") {
		proxyStr := viper.GetString("proxy.proxyaddr")
		proxyURL, err := url.Parse(proxyStr)
		if err == nil {
			httpClient.Transport = &http.Transport{
				Proxy:           http.ProxyURL(proxyURL),
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
		}
	}

	client := api2captcha.NewClientExt(c.apiKey, httpClient)
	// client.Callback = "https://your.site/result-receiver"
	// client.DefaultTimeout = 120
	// client.RecaptchaTimeout = 600
	// client.PollingInterval = 100

	return client
}

func (c *my2captcha) NormalCaptcha(imgFilepath string) (string, string, error) {
	cap := api2captcha.Normal{
		File:     imgFilepath,
		Numberic: c.numberic,
		Lang:     "en",
	}

	client := c.Clientinit()
	token, captchaId, err := client.Solve(cap.ToRequest())
	// fmt.Printf("token: %s, captchaId: %s, err: %s", token, captchaId, err)

	return token, captchaId, err
}

func (c *my2captcha) TurnstileCaptcha(sitekey, weburl, webUA string) (string, string, error) {
	cap := api2captcha.CloudflareTurnstile{
		SiteKey:   sitekey,
		Url:       weburl,
		UserAgent: webUA,
	}

	client := c.Clientinit()
	token, captchaId, err := client.Solve(cap.ToRequest())
	// fmt.Printf("token: %s, captchaId: %s, err: %s", token, captchaId, err)

	return token, captchaId, err
}

func (c *my2captcha) TencentCaptcha(appId, weburl string) (string, string, error) {
	cap := api2captcha.Tencent{
		AppId: appId,
		Url:   weburl,
	}

	client := c.Clientinit()
	token, captchaId, err := client.Solve(cap.ToRequest())

	return token, captchaId, err
}

func (c *my2captcha) GetBalance() (float64, error) {
	client := c.Clientinit()
	balance, err := client.GetBalance()

	return balance, err
}

func (c *my2captcha) StatusReport(captchaId string, correctly bool) error {
	client := c.Clientinit()
	err := client.Report(captchaId, correctly)

	return err
}
