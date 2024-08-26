package main

import (
	"strings"

	"github.com/miekg/dns"
)

func extractIPAddresses(msg *dns.Msg) []string {
	var ips []string
	for _, ans := range msg.Answer {
		switch record := ans.(type) {
		case *dns.A:
			ips = append(ips, record.A.String())
		case *dns.AAAA:
			ips = append(ips, record.AAAA.String())
		}
	}
	return ips
}

// ExtractDomain 提取完整域名
func ExtractDomain(domain string) string {
	// 将域名按 "." 分割
	parts := strings.Split(domain, ".")

	// 如果域名部分少于 2，直接返回原域名
	if len(parts) < 2 {
		return domain
	}

	// 获取最后两个部分
	lastTwoParts := parts[len(parts)-2:] // 例如 ["google", "com"]

	// 如果是国家/地区顶级域名（ccTLD），则还需要加上倒数第三部分
	if len(parts) > 2 && len(parts[len(parts)-1]) == 2 {
		// 例如 "google.com.hk"，需要提取 "google.com.hk"
		lastTwoParts = append(parts[len(parts)-3:len(parts)-1], parts[len(parts)-1])
	}

	// 组合成完整域名
	return strings.Join(lastTwoParts, ".")
}
