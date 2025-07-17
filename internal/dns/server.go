// Package dns provides DNS handling functionality including server parsing and TLS support.
// This package supports multiple DNS server formats and protocols including UDP, TCP, and TLS (DNS over TLS).
package dns

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// DNSServerInfo 包含DNS服务器的详细信息
// 支持解析多种DNS服务器格式，包括传统UDP/TCP和现代TLS协议
type DNSServerInfo struct {
	Protocol string // DNS协议类型: "udp", "tcp", "tls" (DNS over TLS)
	Host     string // 服务器主机地址（IP或域名）
	Port     int    // 服务器端口号
	Address  string // 完整的网络地址，用于建立连接
}

// ParseDNSServer 解析DNS服务器配置，支持多种格式：
//
// 支持的格式：
//   - "8.8.8.8"           -> UDP协议，默认端口53
//   - "8.8.8.8:53"        -> UDP协议，指定端口53
//   - "tcp://8.8.8.8"     -> TCP协议，默认端口53
//   - "tcp://8.8.8.8:53"  -> TCP协议，指定端口53
//   - "tls://8.8.8.8"     -> TLS协议，默认端口853 (DNS over TLS)
//   - "tls://8.8.8.8:853" -> TLS协议，指定端口853
//
// 返回值：
//   - *DNSServerInfo: 解析后的服务器信息
//   - error: 解析过程中的错误
func ParseDNSServer(server string) (*DNSServerInfo, error) {
	server = strings.TrimSpace(server)
	if server == "" {
		return nil, fmt.Errorf("empty server string")
	}

	// 初始化默认值
	info := &DNSServerInfo{
		Protocol: "udp", // 默认使用UDP协议
		Port:     53,    // 默认DNS端口
	}

	// 检查并解析协议前缀
	if strings.HasPrefix(server, "tls://") {
		info.Protocol = "tls"
		info.Port = 853 // DNS over TLS 标准端口
		server = strings.TrimPrefix(server, "tls://")
	} else if strings.HasPrefix(server, "tcp://") {
		info.Protocol = "tcp"
		server = strings.TrimPrefix(server, "tcp://")
	} else if strings.HasPrefix(server, "udp://") {
		info.Protocol = "udp"
		server = strings.TrimPrefix(server, "udp://")
	}

	// 解析主机和端口部分
	if strings.Contains(server, ":") {
		// 格式: host:port
		host, portStr, err := net.SplitHostPort(server)
		if err != nil {
			return nil, fmt.Errorf("invalid server format: %w", err)
		}

		// 验证端口号
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("invalid port number: %w", err)
		}

		if port < 1 || port > 65535 {
			return nil, fmt.Errorf("port number out of range: %d", port)
		}

		info.Host = host
		info.Port = port
	} else {
		// 格式: host (使用默认端口)
		info.Host = server
	}

	// 验证主机地址格式
	if net.ParseIP(info.Host) == nil {
		// 如果不是IP地址，验证是否为有效域名
		if !isValidDomainName(info.Host) {
			return nil, fmt.Errorf("invalid host address: %s", info.Host)
		}
	}

	// 构建完整的网络地址
	info.Address = net.JoinHostPort(info.Host, strconv.Itoa(info.Port))

	return info, nil
}

// isValidDomainName 验证域名格式是否有效
//
// 执行基本的域名格式验证，包括：
//   - 长度检查（0-253字符）
//   - 字符集验证（字母、数字、点号、连字符）
//
// 参数：
//   - domain: 待验证的域名字符串
//
// 返回值：
//   - bool: 如果域名格式有效返回true，否则返回false
func isValidDomainName(domain string) bool {
	// RFC 1035规定域名最大长度为253字符
	if len(domain) == 0 || len(domain) > 253 {
		return false
	}

	// 验证字符集：只允许字母、数字、点号和连字符
	for _, char := range domain {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '.' || char == '-') {
			return false
		}
	}

	return true
}

// String 返回DNS服务器信息的可读字符串表示
//
// 根据协议类型和端口号生成相应的字符串格式：
//   - UDP + 默认端口53: 只返回主机地址
//   - TLS + 默认端口853: 返回 "tls://host" 格式
//   - 其他情况: 返回 "protocol://host:port" 格式
//
// 返回值：
//   - string: 格式化的服务器地址字符串
func (s *DNSServerInfo) String() string {
	// 简化显示：UDP协议默认端口时只显示主机地址
	if s.Protocol == "udp" && s.Port == 53 {
		return s.Host
	}
	// 简化显示：TLS协议默认端口时省略端口号
	if s.Protocol == "tls" && s.Port == 853 {
		return fmt.Sprintf("tls://%s", s.Host)
	}
	// UDP协议非默认端口时不显示协议前缀
	if s.Protocol == "udp" {
		return s.Address
	}
	// 其他情况显示完整格式
	return fmt.Sprintf("%s://%s", s.Protocol, s.Address)
}
