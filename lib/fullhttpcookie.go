package lib

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type RichHttpCookie struct {
	logger *logrus.Logger
}

func NewFullHttpcookie() *RichHttpCookie {
	return &RichHttpCookie{
		logger: LoadLogger(),
	}
}

func (h *RichHttpCookie) CookiesFromResponse(resp *http.Response) []*http.Cookie {
	var cookies []*http.Cookie

	for _, setCookieHeader := range resp.Header["Set-Cookie"] {
		h.logger.Infof("CookiesFromResponse Header Set-Cookie: %s", setCookieHeader)
		// 创建 http.Cookie 结构体
		cookie := &http.Cookie{}

		// 简单解析
		parts := strings.Split(setCookieHeader, ";")

		// 解析 name=value
		if len(parts) > 0 {
			nameValue := strings.SplitN(strings.TrimSpace(parts[0]), "=", 2)
			if len(nameValue) == 2 {
				cookie.Name = nameValue[0]
				cookie.Value = nameValue[1]
			}
		}

		// 解析属性
		for i := 1; i < len(parts); i++ {
			attr := strings.TrimSpace(parts[i])
			if strings.Contains(attr, "=") {
				kv := strings.SplitN(attr, "=", 2)
				key := strings.ToLower(kv[0])
				value := kv[1]

				// 填入 http.Cookie 结构体
				switch key {
				case "domain":
					cookie.Domain = value
				case "path":
					cookie.Path = value
				case "expires":
					// 尝试解析过期时间，支持多种格式
					if expTime, err := h.parseExpires(value); err == nil {
						cookie.Expires = expTime
					}
				case "max-age":
					if maxAge, err := strconv.Atoi(value); err == nil {
						cookie.MaxAge = maxAge
					}
				case "samesite":
					switch strings.ToLower(value) {
					case "strict":
						cookie.SameSite = http.SameSiteStrictMode
					case "lax":
						cookie.SameSite = http.SameSiteLaxMode
					case "none":
						cookie.SameSite = http.SameSiteNoneMode
					default:
						cookie.SameSite = http.SameSiteDefaultMode
					}
				}
			} else {
				key := strings.ToLower(attr)

				// 处理布尔型属性
				switch key {
				case "secure":
					cookie.Secure = true
				case "httponly":
					cookie.HttpOnly = true
				}
			}
		}

		cookies = append(cookies, cookie)
	}

	return cookies
}

// 解析多种格式的过期时间
func (h *RichHttpCookie) parseExpires(expires string) (time.Time, error) {
	// 常见的时间格式
	formats := []string{
		"Mon, 02-Jan-2006 15:04:05 MST",
		"Mon, 02 Jan 2006 15:04:05 MST",
		"Monday, 02-Jan-06 15:04:05 MST",
		"Mon Jan  2 15:04:05 2006",
		"Mon, 02 Jan 06 15:04:05 MST",
		"02-Jan-2006 15:04:05 MST",
		"02 Jan 2006 15:04:05 MST",
		time.RFC1123,
		time.RFC1123Z,
		time.RFC822,
		time.RFC822Z,
		time.RFC850,
		time.RFC3339,
	}

	for _, format := range formats {
		if t, err := time.Parse(format, expires); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("无法解析时间格式: %s", expires)
}
