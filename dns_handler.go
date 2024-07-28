package main

import (
	"net"
	"strings"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/miekg/dns"
	"github.com/sirupsen/logrus"
)

func handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	clientIP, _, _ := net.SplitHostPort(w.RemoteAddr().String())
	requestedDomain := strings.TrimSuffix(r.Question[0].Name, ".")
	requestType := dns.TypeToString[r.Question[0].Qtype]

	// 检查 /etc/hosts
	if ips, exists := config.HostsMap[requestedDomain]; exists {
		response := new(dns.Msg)
		response.SetReply(r)
		for _, ip := range ips {
			if net.ParseIP(ip).To4() != nil && r.Question[0].Qtype == dns.TypeA {
				response.Answer = append(response.Answer, &dns.A{
					Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 3600},
					A:   net.ParseIP(ip),
				})
			} else if net.ParseIP(ip).To16() != nil && r.Question[0].Qtype == dns.TypeAAAA {
				response.Answer = append(response.Answer, &dns.AAAA{
					Hdr:  dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 3600},
					AAAA: net.ParseIP(ip),
				})
			}
		}
		if len(response.Answer) > 0 {
			logrus.Infof("Client IP: %s, Domain: %s, Upstream DNS: /etc/hosts, Type: %s, IPs: %v", clientIP, requestedDomain, requestType, ips)
			w.WriteMsg(response)
			return
		}
	}

	var upstream string
	var cache *lru.Cache
	var cacheDuration time.Duration

	ltdDoamin := getTLD(requestedDomain)
	if _, exists := config.DomainList[ltdDoamin]; exists {
		upstream = config.UpstreamDomesticDNS
		cache = config.CacheDomestic
		cacheDuration = 5 * time.Minute
	} else {
		upstream = config.UpstreamOverseasDNS
		cache = config.CacheOverseas
		cacheDuration = 6 * time.Hour
	}

	if cachedResponse, found := cache.Get(requestedDomain); found {
		cachedMsg := cachedResponse.(*dns.Msg)
		ips := extractIPAddresses(cachedMsg)
		logrus.Infof("Client IP: %s, Domain: %s, Upstream DNS: %s, Type: %s, Cache Duration: %s, IPs: %v", clientIP, requestedDomain, upstream, requestType, cacheDuration.String(), ips)
		cachedMsg.Id = r.Id
		w.WriteMsg(cachedMsg)
		return
	}

	response, err := forwardDNSQuery(r, upstream)
	if err != nil {
		dns.HandleFailed(w, r)
		logrus.Errorf("Client %s requested %s, failed to get response from %s", clientIP, requestedDomain, upstream)
		return
	}

	time.AfterFunc(cacheDuration, func() {
		cache.Remove(requestedDomain)
	})

	ips := extractIPAddresses(response)
	logrus.Infof("Client IP: %s, Domain: %s, Upstream DNS: %s, Type: %s, Cache Duration: %s, IPs: %v", clientIP, requestedDomain, upstream, requestType, cacheDuration.String(), ips)
	if len(ips) > 0 {
		cache.Add(requestedDomain, response)
	}
	w.WriteMsg(response)
}

func forwardDNSQuery(query *dns.Msg, server string) (*dns.Msg, error) {
	client := new(dns.Client)
	response, _, err := client.Exchange(query, server)
	if err != nil {
		logrus.Errorf("Failed to forward query to %s: %s", server, err)
		return nil, err
	}
	return response, nil
}
