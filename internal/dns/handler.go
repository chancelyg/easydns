package dns

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/proxy"

	"easydns/internal/config"
	"easydns/pkg/util"
)

// Handler DNS请求处理器，负责转发和缓存DNS查询
type Handler struct {
	config         *config.Config // 全局配置
	connPool       sync.Map       // DNS客户端连接池 (map[string]*dns.Client)
	querySemaphore chan struct{}  // 并发查询信号量
	evictionTimers sync.Map       // 缓存删除定时器跟踪 (key -> *time.Timer)
	evictionLock   sync.Mutex     // 定时器映射操作锁
	primaryDialer  proxy.Dialer   // 主DNS SOCKS5代理拨号器
	filteredDialer proxy.Dialer   // 过滤DNS SOCKS5代理拨号器
}

// NewHandler 创建DNS请求处理器
func NewHandler(cfg *config.Config) *Handler {
	h := &Handler{
		config:         cfg,
		querySemaphore: make(chan struct{}, cfg.DNS.MaxConcurrentQueries),
	}

	// 初始化SOCKS5代理客户端
	h.initProxyDialers()

	return h
}

// initProxyDialers 初始化SOCKS5代理拨号器
func (h *Handler) initProxyDialers() {
	// 初始化primary_servers的代理
	if h.config.DNS.PrimaryProxy != "" {
		if dialer, err := h.createSOCKS5Dialer(h.config.DNS.PrimaryProxy); err != nil {
			logrus.WithFields(logrus.Fields{
				"proxy": h.config.DNS.PrimaryProxy,
				"error": err,
			}).Error("Failed to create SOCKS5 dialer for primary servers")
		} else {
			h.primaryDialer = dialer
			logrus.WithField("proxy", h.config.DNS.PrimaryProxy).Info("SOCKS5 proxy enabled for primary servers")
		}
	}

	// 初始化filtered_servers的代理
	if h.config.DNS.FilterProxy != "" {
		if dialer, err := h.createSOCKS5Dialer(h.config.DNS.FilterProxy); err != nil {
			logrus.WithFields(logrus.Fields{
				"proxy": h.config.DNS.FilterProxy,
				"error": err,
			}).Error("Failed to create SOCKS5 dialer for filtered servers")
		} else {
			h.filteredDialer = dialer
			logrus.WithField("proxy", h.config.DNS.FilterProxy).Info("SOCKS5 proxy enabled for filtered servers")
		}
	}
}

// createSOCKS5Dialer 创建SOCKS5代理拨号器
func (h *Handler) createSOCKS5Dialer(proxyURL string) (proxy.Dialer, error) {
	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy URL: %w", err)
	}

	if u.Scheme != "socks5" {
		return nil, fmt.Errorf("unsupported proxy scheme: %s, only socks5 is supported", u.Scheme)
	}

	// 创建SOCKS5代理拨号器
	dialer, err := proxy.SOCKS5("tcp", u.Host, nil, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("failed to create SOCKS5 proxy: %w", err)
	}

	return dialer, nil
}

// HandleDNSRequest 处理DNS请求入口
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
		Remove(interface{})
	}
	const (
		minCacheDuration = 60 * time.Second // 最小缓存时间1分钟
		maxFilteredTTL   = 6 * time.Hour    // 受过滤域名最大缓存时间
		maxPrimaryTTL    = 30 * time.Minute // 普通域名最大缓存时间
		dnsQueryTimeout  = 3 * time.Second  // DNS查询超时时间
	)

	// Add EDNS0 support
	opt := r.IsEdns0()
	if opt == nil {
		opt = new(dns.OPT)
		opt.Hdr.Name = "."
		opt.Hdr.Rrtype = dns.TypeOPT
		opt.SetUDPSize(h.config.Server.UDPSize)
		r.Extra = append(r.Extra, opt)
	}

	ltdDomain := util.ExtractDomain(requestedDomain)
	useFiltered := false
	if _, exists := h.config.DomainList[ltdDomain]; exists {
		upstream = h.config.DNS.FilteredServers
		cache = h.config.CacheMinorDNS
		useFiltered = true
	} else {
		upstream = h.config.DNS.PrimaryServers
		cache = h.config.CachePrimaryDNS
	}

	cacheID := fmt.Sprintf("%s-%s", requestType, requestedDomain)
	if cachedResponse, found := cache.Get(cacheID); found {
		cachedMsg := cachedResponse.(*dns.Msg)
		// Check if cache TTL is still valid
		if !isCacheExpired(cachedMsg) {
			ips := util.ExtractIPAddresses(cachedMsg)
			logrus.WithFields(logrus.Fields{
				"clientIP":        clientIP,
				"cacheID":         cacheID,
				"requestedDomain": requestedDomain,
				"requestType":     requestType,
				"ips":             ips,
				"upstream":        upstream,
			}).Info("query success by cache")
			cachedMsg.Id = r.Id
			w.WriteMsg(cachedMsg)
			return
		}
	}

	// 并发查询所有上游DNS服务器，增加超时控制
	type dnsResponse struct {
		msg    *dns.Msg
		server string
	}

	ctx, cancel := context.WithTimeout(context.Background(), dnsQueryTimeout)
	defer cancel()

	responses := make(chan dnsResponse, len(upstream))
	var wg sync.WaitGroup

	// 为了减少IPv6查询慢导致的整体延迟，优先处理A记录（IPv4）的响应
	var ipv4Response *dns.Msg
	var ipv4Server string
	var ipv4Mutex sync.Mutex

	for _, server := range upstream {
		wg.Add(1)
		go func(srv string) {
			defer wg.Done()

			// 获取信号量，限制并发数
			select {
			case h.querySemaphore <- struct{}{}:
				defer func() { <-h.querySemaphore }()
			case <-ctx.Done():
				return
			}

			response, err := h.forwardDNSQuery(ctx, r, srv, useFiltered)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"server": srv,
					"error":  err,
					"domain": requestedDomain,
				}).Warn("DNS query failed")
				return
			}

			// 如果是A记录查询且有结果，优先保存
			if requestType == "A" && response != nil && len(response.Answer) > 0 {
				ipv4Mutex.Lock()
				if ipv4Response == nil {
					ipv4Response = response
					ipv4Server = srv
				}
				ipv4Mutex.Unlock()
			}

			select {
			case responses <- dnsResponse{msg: response, server: srv}:
			case <-ctx.Done():
				return
			}
		}(server)
	}

	// 等待所有查询完成或超时
	go func() {
		wg.Wait()
		close(responses)
	}()

	// 收集结果
	var bestResponse *dns.Msg
	var responseServer string

	for resp := range responses {
		if resp.msg != nil && len(resp.msg.Answer) > 0 {
			bestResponse = resp.msg
			responseServer = resp.server
			break
		}
	}

	// 如果没有找到结果但有IPv4结果，使用IPv4结果
	if bestResponse == nil && ipv4Response != nil {
		bestResponse = ipv4Response
		responseServer = ipv4Server
	}

	// 处理响应
	if bestResponse == nil {
		dns.HandleFailed(w, r)
		logrus.WithFields(logrus.Fields{
			"clientIP":        clientIP,
			"requestedDomain": requestedDomain,
			"upstream":        upstream,
		}).Error("all dns servers failed to respond")
		return
	}

	logrus.WithFields(logrus.Fields{
		"clientIP":        clientIP,
		"requestedDomain": requestedDomain,
		"requestType":     requestType,
		"ips":             util.ExtractIPAddresses(bestResponse),
		"upstream":        upstream,
		"responseServer":  responseServer,
	}).Info("query dns record success")

	// 根据响应确定缓存时间
	ttl := getMinTTL(bestResponse)
	cacheDuration := time.Duration(ttl) * time.Second

	// 确保缓存时间在合理范围内
	if cacheDuration < minCacheDuration {
		cacheDuration = minCacheDuration
	}

	maxTTL := maxPrimaryTTL
	if _, exists := h.config.DomainList[ltdDomain]; exists {
		maxTTL = maxFilteredTTL
	}

	if cacheDuration > maxTTL {
		cacheDuration = maxTTL
	}

	ips := util.ExtractIPAddresses(bestResponse)
	if len(ips) > 0 {
		cache.Add(cacheID, bestResponse)
		// 先添加缓存，再调度删除，避免竞态条件
		h.scheduleCacheEviction(cache, cacheID, cacheDuration)
	}
	w.WriteMsg(bestResponse)
}

// sendNotImplemented 返回未实现响应
func (h *Handler) sendNotImplemented(w dns.ResponseWriter, r *dns.Msg) {
	nxdomainResponse := new(dns.Msg)
	nxdomainResponse.SetReply(r)
	nxdomainResponse.Rcode = dns.RcodeNotImplemented
	w.WriteMsg(nxdomainResponse)
}

// handleHostsResponse 处理hosts文件中的DNS响应
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

// forwardDNSQuery 转发DNS查询，支持UDP、TCP、TLS和SOCKS5代理
func (h *Handler) forwardDNSQuery(ctx context.Context, query *dns.Msg, server string, useFiltered bool) (*dns.Msg, error) {
	// 解析服务器信息
	serverInfo, err := ParseDNSServer(server)
	if err != nil {
		return nil, fmt.Errorf("failed to parse server %s: %w", server, err)
	}

	// 获取或创建客户端 (使用 sync.Map 实现并发安全)
	var client *dns.Client
	if c, exists := h.connPool.Load(server); exists {
		client = c.(*dns.Client)
	} else {
		client = h.createDNSClient(serverInfo, useFiltered)
		h.connPool.Store(server, client)
	}

	// 使用上下文控制超时
	type exchangeResult struct {
		msg *dns.Msg
		rtt time.Duration
		err error
	}

	resultChan := make(chan exchangeResult, 1)

	go func() {
		var msg *dns.Msg
		var rtt time.Duration
		var err error

		// 根据协议和代理情况选择连接方式
		if serverInfo.Protocol == "tls" {
			msg, rtt, err = h.exchangeWithTLS(query, serverInfo, useFiltered)
		} else if (useFiltered && h.filteredDialer != nil) || (!useFiltered && h.primaryDialer != nil) {
			msg, rtt, err = h.exchangeWithProxy(query, serverInfo.Address, useFiltered)
		} else {
			msg, rtt, err = client.Exchange(query, serverInfo.Address)
		}

		resultChan <- exchangeResult{msg: msg, rtt: rtt, err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultChan:
		return result.msg, result.err
	}
}

// createDNSClient 根据服务器信息和代理设置创建适当的DNS客户端
//
// 该方法会：
//  1. 根据协议类型（UDP/TCP/TLS）设置相应的网络类型
//  2. 为TLS连接配置TLS证书验证
//  3. 根据需要配置SOCKS5代理拨号器
//  4. 设置合适的超时时间和UDP包大小
//
// 参数：
//   - serverInfo: 解析后的DNS服务器信息
//   - useFiltered: 是否使用过滤DNS服务器的代理设置
//
// 返回值：
//   - *dns.Client: 配置好的DNS客户端
func (h *Handler) createDNSClient(serverInfo *DNSServerInfo, useFiltered bool) *dns.Client {
	client := &dns.Client{
		UDPSize: h.config.Server.UDPSize, // 使用配置中的UDP包大小
		Timeout: 2 * time.Second,         // 设置2秒超时
	}

	// 根据协议类型配置网络连接参数
	switch serverInfo.Protocol {
	case "tls":
		client.Net = "tcp-tls" // 使用TLS over TCP
		client.TLSConfig = &tls.Config{
			ServerName: serverInfo.Host, // 设置SNI服务器名称用于证书验证
		}
	case "tcp":
		client.Net = "tcp" // 使用标准TCP
	default:
		client.Net = "udp" // 默认使用UDP
	}

	// 配置SOCKS5代理（如果需要且可用）
	var dialer proxy.Dialer
	if useFiltered && h.filteredDialer != nil {
		dialer = h.filteredDialer // 使用过滤DNS的代理
	} else if !useFiltered && h.primaryDialer != nil {
		dialer = h.primaryDialer // 使用主DNS的代理
	}

	// 为非TLS连接配置代理拨号器
	if dialer != nil && serverInfo.Protocol != "tls" {
		client.Dialer = &net.Dialer{
			Timeout: 2 * time.Second,
		}
		// SOCKS5代理通常需要TCP连接
		if client.Net == "udp" {
			client.Net = "tcp"
		}
	}

	return client
}

// exchangeWithTLS 通过TLS连接进行DNS查询（DNS over TLS）
//
// 该方法实现了DNS over TLS (DoT) 协议，支持：
//  1. 直接TLS连接到DNS服务器
//  2. 通过SOCKS5代理建立TLS连接
//  3. 正确的TLS握手和证书验证
//  4. 连接超时和错误处理
//
// 参数：
//   - query: 要发送的DNS查询消息
//   - serverInfo: DNS服务器信息（包含主机名用于TLS验证）
//   - useFiltered: 是否使用过滤DNS的代理设置
//
// 返回值：
//   - *dns.Msg: DNS响应消息
//   - time.Duration: 查询耗时
//   - error: 连接或查询过程中的错误
func (h *Handler) exchangeWithTLS(query *dns.Msg, serverInfo *DNSServerInfo, useFiltered bool) (*dns.Msg, time.Duration, error) {
	start := time.Now()

	// 声明连接变量
	var conn net.Conn
	var err error

	// 根据代理设置选择连接方式
	if (useFiltered && h.filteredDialer != nil) || (!useFiltered && h.primaryDialer != nil) {
		// 场景1：通过SOCKS5代理建立TLS连接
		var dialer proxy.Dialer
		if useFiltered {
			dialer = h.filteredDialer
		} else {
			dialer = h.primaryDialer
		}

		// 先通过代理建立TCP连接
		conn, err = dialer.Dial("tcp", serverInfo.Address)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to dial through proxy: %w", err)
		}

		// 在代理连接基础上建立TLS连接
		tlsConn := tls.Client(conn, &tls.Config{
			ServerName: serverInfo.Host, // 重要：设置SNI用于证书验证
		})
		err = tlsConn.Handshake()
		if err != nil {
			conn.Close()
			return nil, 0, fmt.Errorf("TLS handshake failed: %w", err)
		}
		conn = tlsConn
	} else {
		// 场景2：直接建立TLS连接到DNS服务器
		conn, err = tls.Dial("tcp", serverInfo.Address, &tls.Config{
			ServerName: serverInfo.Host, // 设置SNI服务器名称
		})
		if err != nil {
			return nil, 0, fmt.Errorf("failed to establish TLS connection: %w", err)
		}
	}
	defer conn.Close()

	// 设置连接超时时间
	conn.SetDeadline(time.Now().Add(2 * time.Second))

	// 创建DNS over TLS连接
	dnsConn := &dns.Conn{Conn: conn}
	defer dnsConn.Close()

	// 发送DNS查询
	err = dnsConn.WriteMsg(query)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to write DNS query: %w", err)
	}

	// 读取DNS响应
	response, err := dnsConn.ReadMsg()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read DNS response: %w", err)
	}

	rtt := time.Since(start)
	return response, rtt, nil
}

// getMinTTL 获取DNS响应的最小TTL
func getMinTTL(msg *dns.Msg) uint32 {
	if msg == nil || len(msg.Answer) == 0 {
		return 0
	}

	minTTL := msg.Answer[0].Header().Ttl
	for _, answer := range msg.Answer {
		if answer.Header().Ttl < minTTL {
			minTTL = answer.Header().Ttl
		}
	}
	return minTTL
}

// isCacheExpired 判断DNS缓存是否过期
func isCacheExpired(msg *dns.Msg) bool {
	if msg == nil || len(msg.Answer) == 0 {
		return true
	}

	for _, answer := range msg.Answer {
		if answer.Header().Ttl == 0 {
			return true
		}
	}
	return false
}

// exchangeWithProxy 通过SOCKS5代理进行DNS查询
func (h *Handler) exchangeWithProxy(query *dns.Msg, server string, useFiltered bool) (*dns.Msg, time.Duration, error) {
	var dialer proxy.Dialer
	if useFiltered {
		dialer = h.filteredDialer
	} else {
		dialer = h.primaryDialer
	}

	if dialer == nil {
		return nil, 0, fmt.Errorf("no proxy dialer available")
	}

	start := time.Now()

	// 通过代理连接到DNS服务器
	conn, err := dialer.Dial("tcp", server)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to dial through proxy: %w", err)
	}
	defer conn.Close()

	// 设置连接超时
	conn.SetDeadline(time.Now().Add(2 * time.Second))

	// 创建DNS连接
	dnsConn := &dns.Conn{Conn: conn}
	defer dnsConn.Close()

	// 发送查询
	err = dnsConn.WriteMsg(query)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to write DNS query: %w", err)
	}

	// 读取响应
	response, err := dnsConn.ReadMsg()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read DNS response: %w", err)
	}

	rtt := time.Since(start)
	return response, rtt, nil
}

// scheduleCacheEviction 调度缓存条目在指定时间后删除
func (h *Handler) scheduleCacheEviction(cache interface{ Remove(interface{}) }, cacheID string, delay time.Duration) {
	h.evictionLock.Lock()
	defer h.evictionLock.Unlock()

	// 取消已存在的同名定时器（如果存在）
	if existing, ok := h.evictionTimers.Load(cacheID); ok {
		if t, ok := existing.(*time.Timer); ok {
			t.Stop()
		}
	}

	// 创建新的定时器
	timer := time.AfterFunc(delay, func() {
		h.evictionLock.Lock()
		h.evictionTimers.Delete(cacheID)
		h.evictionLock.Unlock()

		if c, ok := cache.(interface{ Remove(interface{}) }); ok {
			c.Remove(cacheID)
			logrus.WithFields(logrus.Fields{"cacheID": cacheID}).Debug("cache evicted via timer")
		}
	})

	h.evictionTimers.Store(cacheID, timer)
}

// Stop 停止所有定时器，用于优雅关闭
func (h *Handler) Stop() {
	h.evictionLock.Lock()
	defer h.evictionLock.Unlock()

	h.evictionTimers.Range(func(key, value interface{}) bool {
		if t, ok := value.(*time.Timer); ok {
			t.Stop()
		}
		h.evictionTimers.Delete(key)
		return true
	})
}
