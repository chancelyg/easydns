package main

import (
	"fmt"
	"net"
	"strings"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/miekg/dns"
	log "github.com/sirupsen/logrus"
)

func handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	clientIP, _, _ := net.SplitHostPort(w.RemoteAddr().String())
	requestedDomain := strings.TrimSuffix(r.Question[0].Name, ".")
	requestType := dns.TypeToString[r.Question[0].Qtype]

	// 不允许解析 IPV6
	if !config.IPV6 && requestType == "AAAA" {
		nxdomainResponse := new(dns.Msg)
		nxdomainResponse.SetReply(r)
		nxdomainResponse.Rcode = dns.RcodeNotImplemented
		w.WriteMsg(nxdomainResponse)
		return
	}

	// 不允许解析 IPV4
	if !config.IPV4 && requestType == "A" {
		nxdomainResponse := new(dns.Msg)
		nxdomainResponse.SetReply(r)
		nxdomainResponse.Rcode = dns.RcodeNotImplemented
		w.WriteMsg(nxdomainResponse)
		return
	}

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
			log.WithFields(log.Fields{"clientIP": clientIP, "requestedDomain": requestedDomain, "requestType": requestType, "ips": ips}).Info(("query success by hosts file"))
			w.WriteMsg(response)
			return
		}
	}

	var upstream string
	var cache *lru.Cache
	var cacheDuration time.Duration

	ltdDoamin := ExtractDomain(requestedDomain)
	if _, exists := config.DomainList[ltdDoamin]; exists {
		upstream = config.MinorDNS
		cache = config.CacheMinorDNS
		cacheDuration = 6 * time.Hour
	} else {
		upstream = config.PrimaryDNS
		cache = config.CachePrimaryDNS
		cacheDuration = 5 * time.Minute
	}

	cacheID := fmt.Sprintf("%s-%s", requestType, requestedDomain)
	if cachedResponse, found := cache.Get(cacheID); found {
		cachedMsg := cachedResponse.(*dns.Msg)
		ips := extractIPAddresses(cachedMsg)
		log.WithFields(log.Fields{"clientIP": clientIP,
			"cacheID":         cacheID,
			"requestedDomain": requestedDomain,
			"requestType":     requestType,
			"ips":             ips,
			"upstream":        upstream,
			"cacheDuration":   cacheDuration.String()}).Info(("query success by cache"))
		cachedMsg.Id = r.Id
		w.WriteMsg(cachedMsg)
		return
	}

	response, err := forwardDNSQuery(r, upstream)
	if err != nil {
		dns.HandleFailed(w, r)
		log.WithFields(log.Fields{"clientIP": clientIP, "requestedDomain": requestedDomain, "upstream": upstream}).Error("failed to get response")
		return
	}

	time.AfterFunc(cacheDuration, func() {
		cache.Remove(cacheID)
	})

	ips := extractIPAddresses(response)
	log.WithFields(log.Fields{"clientIP": clientIP,
		"requestedDomain": requestedDomain,
		"requestType":     requestType,
		"ips":             ips,
		"upstream":        upstream,
		"cacheDuration":   cacheDuration.String()}).Info(("query success by dns server"))
	if len(ips) > 0 {
		cache.Add(cacheID, response)
	}
	w.WriteMsg(response)
}

func forwardDNSQuery(query *dns.Msg, server string) (*dns.Msg, error) {
	client := &dns.Client{
		UDPSize: uint16(config.UDPSize), // 增加缓冲区大小
	}
	response, _, err := client.Exchange(query, server)
	if err != nil {
		log.WithFields(log.Fields{"server": server, "err": err}).Error("Failed to forward query")
		return nil, err
	}
	return response, nil
}
