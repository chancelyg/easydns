package config

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"easydns/internal/cache"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

const Version = "1.0.1"

// Config 存储程序配置
type Config struct {
	Server ServerConfig `yaml:"server"` // 服务相关配置
	DNS    DNSConfig    `yaml:"dns"`    // DNS相关配置
	Cache  CacheConfig  `yaml:"cache"`  // 缓存相关配置
	Paths  PathsConfig  `yaml:"paths"`  // 路径相关配置

	// Runtime fields
	DomainList      map[string]struct{}
	CachePrimaryDNS *cache.DNSCache
	CacheMinorDNS   *cache.DNSCache
	HostsMap        map[string][]string
}

type ServerConfig struct {
	Port    int    `yaml:"port"`     // 监听端口
	UDPSize uint16 `yaml:"udp_size"` // UDP包大小
	IPV4    bool   `yaml:"ipv4"`     // 是否支持IPv4
	IPV6    bool   `yaml:"ipv6"`     // 是否支持IPv6
}

type DNSConfig struct {
	PrimaryServers       []string `yaml:"primary_servers"`         // 主DNS服务器列表
	FilteredServers      []string `yaml:"filtered_servers"`        // 过滤DNS服务器列表
	PrimaryProxy         string   `yaml:"primary_proxy,omitempty"` // 主DNS代理地址
	FilterProxy          string   `yaml:"filter_proxy,omitempty"`  // 过滤DNS代理地址
	MaxConcurrentQueries int      `yaml:"max_concurrent_queries"`  // 最大并发上游查询数
}

type CacheConfig struct {
	Limit int `yaml:"limit"` // 缓存条目限制
}

type PathsConfig struct {
	FilteredServerList  string   `yaml:"filtered_server_list"`  // 过滤服务器列表路径（单个本地文件）
	FilteredServerLists []string `yaml:"filtered_server_lists"` // 过滤服务器列表源（支持本地路径或HTTPS URL）
	Hosts               string   `yaml:"hosts"`                 // hosts文件路径
}

// LoadFromFile loads configuration from a YAML file
func LoadFromFile(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Set default values if not specified
	if len(cfg.DNS.PrimaryServers) == 0 {
		cfg.DNS.PrimaryServers = []string{"114.114.114.114:53"}
	}
	if len(cfg.DNS.FilteredServers) == 0 {
		cfg.DNS.FilteredServers = []string{"8.8.8.8:53"}
	}
	if cfg.DNS.MaxConcurrentQueries == 0 {
		cfg.DNS.MaxConcurrentQueries = 50 // 默认最大50个并发查询
	}

	// 验证和标准化DNS服务器格式
	if err := cfg.validateAndNormalizeDNSServers(); err != nil {
		return nil, err
	}

	if cfg.Cache.Limit == 0 {
		cfg.Cache.Limit = 4096
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 53
	}
	if cfg.Server.UDPSize == 0 {
		cfg.Server.UDPSize = 512
	}
	if cfg.Paths.FilteredServerList == "" {
		cfg.Paths.FilteredServerList = "filtered_server_list.txt"
	}
	if cfg.Paths.Hosts == "" {
		cfg.Paths.Hosts = "/etc/hosts"
	}

	return cfg, nil
}

// Initialize 初始化配置
func (c *Config) Initialize() error {
	var err error
	c.CachePrimaryDNS, err = cache.New(c.Cache.Limit)
	if err != nil {
		return err
	}

	c.CacheMinorDNS, err = cache.New(c.Cache.Limit)
	if err != nil {
		return err
	}

	c.DomainList = c.loadDomainList()
	c.HostsMap = parseHostsFile(c.Paths.Hosts)
	return nil
}

// loadDomainList 加载域名列表，支持多个源（本地文件或HTTPS URL）
func (c *Config) loadDomainList() map[string]struct{} {
	domainList := make(map[string]struct{})

	// 优先使用新的多源配置
	if len(c.Paths.FilteredServerLists) > 0 {
		for _, source := range c.Paths.FilteredServerLists {
			domains := c.loadDomainFromSource(source)
			for domain := range domains {
				domainList[domain] = struct{}{}
			}
		}
		return domainList
	}

	// 兼容旧的单文件配置
	if c.Paths.FilteredServerList != "" {
		domains := c.loadDomainFromSource(c.Paths.FilteredServerList)
		for domain := range domains {
			domainList[domain] = struct{}{}
		}
	}

	return domainList
}

// loadDomainFromSource 从单个源加载域名（支持本地路径或HTTPS URL）
func (c *Config) loadDomainFromSource(source string) map[string]struct{} {
	if strings.HasPrefix(source, "https://") || strings.HasPrefix(source, "http://") {
		return c.loadDomainFromURL(source)
	}
	return loadDomainFile(source)
}

// loadDomainFromURL 从HTTPS URL下载并解析域名列表
func (c *Config) loadDomainFromURL(url string) map[string]struct{} {
	domainList := make(map[string]struct{})

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		logrus.WithFields(logrus.Fields{"url": url, "error": err}).Warn("Failed to download domain list from URL")
		return domainList
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logrus.WithFields(logrus.Fields{"url": url, "status": resp.StatusCode}).Warn("Failed to download domain list: non-200 status")
		return domainList
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logrus.WithFields(logrus.Fields{"url": url, "error": err}).Warn("Failed to read domain list body")
		return domainList
	}

	content := string(body)

	// 检测是否为 base64 编码（GFWList 格式）
	decoded, err := base64.StdEncoding.DecodeString(content)
	if err == nil && len(decoded) < len(content) {
		content = string(decoded)
	}

	// 解析域名列表（每行一个域名，# 开头为注释）
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		// 跳过 GFWList 的特殊标记
		if strings.HasPrefix(line, "[") || strings.HasPrefix(line, "||") || strings.HasPrefix(line, ".") {
			continue
		}
		// 处理 ||domain.com 格式（AdGuard 格式）
		if strings.HasPrefix(line, "||") {
			line = strings.TrimPrefix(line, "||")
			line = strings.Split(line, "^")[0]
		}
		// 处理 domain.com^ 格式
		line = strings.Split(line, "^")[0]
		// 处理 *  通配符
		line = strings.Trim(line, "*")
		if line != "" && c.isValidDomainName(line) {
			domainList[line] = struct{}{}
		}
	}

	logrus.WithFields(logrus.Fields{"url": url, "count": len(domainList)}).Info("Downloaded domain list from URL")
	return domainList
}

func loadDomainFile(filename string) map[string]struct{} {
	domainList := make(map[string]struct{})
	file, err := os.Open(filename)
	if err != nil {
		logrus.WithFields(logrus.Fields{"err": err, "filename": filename}).Warn("Failed to open domain list file, continuing without it")
		return domainList
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		domainList[strings.TrimSpace(scanner.Text())] = struct{}{}
	}

	if err := scanner.Err(); err != nil {
		logrus.WithFields(logrus.Fields{"err": err, "filename": filename}).Warn("Error reading domain list file, continuing with loaded entries")
	}

	return domainList
}

func parseHostsFile(filePath string) map[string][]string {
	hosts := make(map[string][]string)
	file, err := os.Open(filePath)
	if err != nil {
		logrus.WithFields(logrus.Fields{"err": err, "filePath": filePath}).Warn("Failed to open hosts file, continuing without it")
		return hosts
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		ip := fields[0]
		for _, domain := range fields[1:] {
			hosts[domain] = append(hosts[domain], ip)
		}
	}

	if err := scanner.Err(); err != nil {
		logrus.WithFields(logrus.Fields{"err": err, "filePath": filePath}).Warn("Error reading hosts file, continuing with loaded entries")
	}

	return hosts
}

// validateAndNormalizeDNSServers 验证并标准化DNS服务器配置
//
// 该方法会：
//  1. 解析并验证所有primary_servers和filtered_servers中的DNS服务器地址
//  2. 将简化格式（如"8.8.8.8"）标准化为完整格式（如"8.8.8.8:53"）
//  3. 支持多种协议前缀：udp://、tcp://、tls://
//  4. 验证主机地址和端口号的有效性
//
// 返回值：
//   - error: 如果发现无效的DNS服务器配置则返回错误，成功时返回nil
func (c *Config) validateAndNormalizeDNSServers() error {
	// 验证主DNS服务器
	for i, server := range c.DNS.PrimaryServers {
		normalized, err := c.parseDNSServer(server)
		if err != nil {
			return fmt.Errorf("invalid primary DNS server '%s': %w", server, err)
		}
		c.DNS.PrimaryServers[i] = normalized
	}

	// 验证过滤DNS服务器
	for i, server := range c.DNS.FilteredServers {
		normalized, err := c.parseDNSServer(server)
		if err != nil {
			return fmt.Errorf("invalid filtered DNS server '%s': %w", server, err)
		}
		c.DNS.FilteredServers[i] = normalized
	}

	return nil
}

// parseDNSServer 解析单个DNS服务器配置字符串
//
// 支持的输入格式：
//   - "8.8.8.8"               -> "8.8.8.8:53" (UDP协议，默认端口)
//   - "8.8.8.8:53"            -> "8.8.8.8:53" (UDP协议，指定端口)
//   - "tcp://8.8.8.8"         -> "tcp://8.8.8.8:53" (TCP协议，默认端口)
//   - "tcp://8.8.8.8:53"      -> "tcp://8.8.8.8:53" (TCP协议，指定端口)
//   - "tls://8.8.8.8"         -> "tls://8.8.8.8:853" (TLS协议，默认端口)
//   - "tls://8.8.8.8:853"     -> "tls://8.8.8.8:853" (TLS协议，指定端口)
//
// 参数：
//   - server: 待解析的DNS服务器配置字符串
//
// 返回值：
//   - string: 标准化后的DNS服务器地址
//   - error: 解析过程中遇到的错误
func (c *Config) parseDNSServer(server string) (string, error) {
	server = strings.TrimSpace(server)
	if server == "" {
		return "", fmt.Errorf("empty server string")
	}

	protocol := "udp" // 默认协议为UDP
	defaultPort := 53 // 默认端口为53

	// 解析协议前缀，确定协议类型和默认端口
	if strings.HasPrefix(server, "tls://") {
		protocol = "tls"
		defaultPort = 853 // DNS over TLS标准端口
		server = strings.TrimPrefix(server, "tls://")
	} else if strings.HasPrefix(server, "tcp://") {
		protocol = "tcp"
		server = strings.TrimPrefix(server, "tcp://")
	} else if strings.HasPrefix(server, "udp://") {
		protocol = "udp"
		server = strings.TrimPrefix(server, "udp://")
	}

	var host string
	var port int

	// 解析主机和端口
	if strings.Contains(server, ":") {
		h, portStr, err := net.SplitHostPort(server)
		if err != nil {
			return "", fmt.Errorf("invalid server format: %w", err)
		}

		p, err := strconv.Atoi(portStr)
		if err != nil {
			return "", fmt.Errorf("invalid port number: %w", err)
		}

		if p < 1 || p > 65535 {
			return "", fmt.Errorf("port number out of range: %d", p)
		}

		host = h
		port = p
	} else {
		host = server
		port = defaultPort
	}

	// 验证主机地址
	if net.ParseIP(host) == nil {
		// 如果不是IP地址，验证是否为有效域名
		if !c.isValidDomainName(host) {
			return "", fmt.Errorf("invalid host address: %s", host)
		}
	}

	// 构建标准化地址
	address := net.JoinHostPort(host, strconv.Itoa(port))

	// 根据协议返回相应格式
	switch protocol {
	case "tls":
		return fmt.Sprintf("tls://%s", address), nil
	case "tcp":
		return fmt.Sprintf("tcp://%s", address), nil
	default:
		return address, nil
	}
}

// isValidDomainName 验证域名格式的有效性
//
// 执行基本的域名格式验证，包括：
//   - 长度限制：域名总长度不能超过253字符（RFC 1035规定）
//   - 字符集验证：只允许字母、数字、点号和连字符
//
// 注意：这是一个简化的验证函数，不包括完整的RFC域名规范检查
//
// 参数：
//   - domain: 待验证的域名字符串
//
// 返回值：
//   - bool: 域名格式有效返回true，否则返回false
func (c *Config) isValidDomainName(domain string) bool {
	if len(domain) == 0 || len(domain) > 253 {
		return false
	}

	labels := strings.Split(domain, ".")
	for _, label := range labels {
		if len(label) == 0 {
			return false
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, char := range label {
			if !((char >= 'a' && char <= 'z') ||
				(char >= 'A' && char <= 'Z') ||
				(char >= '0' && char <= '9') ||
				char == '-') {
				return false
			}
		}
	}

	return true
}
