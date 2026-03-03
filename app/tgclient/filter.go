package tgclient

import "strings"

var spamKeywords = []string{
	"airdrop", "注册送", "免费领", "返佣", "拉群", "加v", "加微", "vx:", "vx：",
	"推广", "广告", "商务合作", "邀请码", "返利", "稳赚", "稳赚不赔", "带单", "喊单", "现货带单",
	"spot signal", "join our", "telegram group", "pump", "guaranteed profit", "referral", "promo code",
}

func shouldFilter(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	if t == "" {
		return false
	}

	// Rule 1: short promotion-like snippets.
	if len([]rune(t)) < 28 && (strings.Contains(t, "http://") || strings.Contains(t, "https://") || strings.Contains(t, "t.me/")) {
		return true
	}
	// Rule 2: heavy link density on short messages.
	if len([]rune(t)) < 120 {
		if strings.Count(t, "http")+strings.Count(t, "t.me/") >= 2 {
			return true
		}
	}
	// Rule 3: promotion keywords.
	for _, kw := range spamKeywords {
		if strings.Contains(t, kw) {
			return true
		}
	}
	return false
}
