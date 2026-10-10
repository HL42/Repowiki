package util

import (
	"regexp"
	"strings"
)

var (
	shopIDPattern  = regexp.MustCompile(`^SHOP[0-9]{6,12}$`)
	orderIDPattern = regexp.MustCompile(`^ORD[0-9]{14}$`)
	phonePattern   = regexp.MustCompile(`^1[3-9][0-9]{9}$`)
)

// IsValidShopID 校验门店号格式，要求 SHOP 前缀 + 6~12 位数字
func IsValidShopID(shopID string) bool {
	return shopIDPattern.MatchString(shopID)
}

// IsValidOrderID 校验订单号格式，要求 ORD + 8 位日期 + 6 位序号
func IsValidOrderID(orderID string) bool {
	return orderIDPattern.MatchString(orderID)
}

// IsValidPhone 校验中国大陆手机号
func IsValidPhone(phone string) bool {
	return phonePattern.MatchString(phone)
}

// Truncate 按字符数截断字符串，超出部分丢弃。
// 用 []rune 而非按字节切分，避免把中文截成乱码。
func Truncate(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes])
}

// MaskPhone 手机号脱敏，保留前 3 位与后 4 位，用于日志输出
func MaskPhone(phone string) string {
	if len(phone) < 7 {
		return "***"
	}
	return phone[:3] + "****" + phone[len(phone)-4:]
}

// NormalizeRemark 清洗用户备注：去掉首尾空白、折叠连续空白
func NormalizeRemark(remark string) string {
	return strings.Join(strings.Fields(remark), " ")
}
