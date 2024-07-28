package main

import (
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
