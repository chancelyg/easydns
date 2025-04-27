package dns

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/sirupsen/logrus"

	"easydns/internal/config"
	"easydns/pkg/util"
)

type Handler struct {
	config *config.Config
}

func NewHandler(cfg *config.Config) *Handler {
	return &Handler{
		config: cfg,
	}
}

func (h *Handler) HandleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	clientIP, _, _ := net.SplitHostPort(w.RemoteAddr().String())
	requestedDomain := strings.TrimSuffix(r.Question[0].Name, ".")
	requestType := dns.TypeToString[r.Question[0].Qtype]

	// 不允许解析 IPV6
	if !h.config.Server.IPV6 && requestType == "AAAA" {
		h.sendNotImplemented(w, r)
		return
	}

	// 不允许解析 IPV4
	if !h.config.Server.IPV4 && requestType == "A" {
		h.sendNotImplemented(w, r)
		return
	}

	// 检查 /etc/hosts
	if ips, exists := h.config.HostsMap[requestedDomain]; exists {
		if h.handleHostsResponse(w, r, ips) {
			logrus.WithFields(logrus.Fields{
				"clientIP":        clientIP,
				"requestedDomain": requestedDomain,
				"requestType":     requestType,
				"ips":             ips,
			}).Info("query success by hosts file")
			return
		}
	}

	var upstream []string
	var cache interface {
		Get(interface{}) (interface{}, bool)
		Add(interface{}, interface{}) bool
	}
	var cacheDuration time.Duration

	ltdDomain := util.ExtractDomain(requestedDomain)
	if _, exists := h.config.DomainList[ltdDomain]; exists {
		upstream = h.config.DNS.FilteredServers
		cache = h.config.CacheMinorDNS
		cacheDuration = 6 * time.Hour
	} else {
		upstream = h.config.DNS.PrimaryServers
		cache = h.config.CachePrimaryDNS
		cacheDuration = 5 * time.Minute
	}

	cacheID := fmt.Sprintf("%s-%s", requestType, requestedDomain)
	// if cachedResponse, found := cache.Get(cacheID); found {
	// 	cachedMsg := cachedResponse.(*dns.Msg)
	// 	ips := util.ExtractIPAddresses(cachedMsg)
	// 	logrus.WithFields(logrus.Fields{
	// 		"clientIP":        clientIP,
	// 		"cacheID":         cacheID,
	// 		"requestedDomain": requestedDomain,
	// 		"requestType":     requestType,
	// 		"ips":             ips,
	// 		"upstream":        upstream,
	// 		"cacheDuration":   cacheDuration.String(),
	// 	}).Info("query success by cache")
	// 	cachedMsg.Id = r.Id
	// 	w.WriteMsg(cachedMsg)
	// 	return
	// }

	// 并发查询所有上游DNS服务器
	type dnsResponse struct {
		msg    *dns.Msg
		server string
	}
	responses := make(chan dnsResponse, len(upstream))
	errors := make(chan error, len(upstream))
	done := make(chan struct{})

	for _, server := range upstream {
		go func(srv string) {
			response, err := h.forwardDNSQuery(r, srv)
			if err != nil {
				select {
				case errors <- err:
				case <-done:
				}
				return
			}
			select {
			case responses <- dnsResponse{msg: response, server: srv}:
			case <-done:
			}
		}(server)
	}

	// 获取第一个成功的响应
	var response *dns.Msg
	errorCount := 0
	for {
		select {
		case resp := <-responses:
			logrus.WithFields(logrus.Fields{
				"clientIP":        clientIP,
				"requestedDomain": requestedDomain,
				"requestType":     requestType,
				"ips":             util.ExtractIPAddresses(resp.msg),
				"upstream":        upstream,
				"responseServer":  resp.server,
				"cacheDuration":   cacheDuration.String(),
			}).Info("query dns record success")
			if resp.msg != nil && len(resp.msg.Answer) > 0 {
				close(done) // 通知其他 goroutine 退出
				response = resp.msg
				goto handleResponse
			}
		case <-errors:
			errorCount++
			if errorCount == len(upstream) {
				close(done)
				goto handleResponse
			}
		}
	}

handleResponse:
	if response == nil {
		dns.HandleFailed(w, r)
		logrus.WithFields(logrus.Fields{
			"clientIP":        clientIP,
			"requestedDomain": requestedDomain,
			"upstream":        upstream,
		}).Error("all dns servers failed to respond")
		return
	}

	time.AfterFunc(cacheDuration, func() {
		if c, ok := cache.(interface{ Remove(interface{}) }); ok {
			c.Remove(cacheID)
		}
	})

	ips := util.ExtractIPAddresses(response)
	if len(ips) > 0 {
		cache.Add(cacheID, response)
	}
	w.WriteMsg(response)
}

func (h *Handler) sendNotImplemented(w dns.ResponseWriter, r *dns.Msg) {
	nxdomainResponse := new(dns.Msg)
	nxdomainResponse.SetReply(r)
	nxdomainResponse.Rcode = dns.RcodeNotImplemented
	w.WriteMsg(nxdomainResponse)
}

func (h *Handler) handleHostsResponse(w dns.ResponseWriter, r *dns.Msg, ips []string) bool {
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
		w.WriteMsg(response)
		return true
	}
	return false
}

func (h *Handler) forwardDNSQuery(query *dns.Msg, server string) (*dns.Msg, error) {
	client := &dns.Client{
		UDPSize: h.config.Server.UDPSize,
	}
	response, _, err := client.Exchange(query, server)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"server": server,
			"err":    err,
		}).Error("Failed to forward query")
		return nil, err
	}
	return response, nil
}
