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

func getTLD(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return domain // 如果域名中没有点，返回原始域名
	}
	// 返回最后两个部分作为顶级域名
	return parts[len(parts)-2] + "." + parts[len(parts)-1]
}
