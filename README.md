# EasyDNS

一个用 Go 编写的简易高性能 DNS 代理服务器，支持**基于域名列表的 DNS 分流、缓存、本地 hosts**，并支持多种 DNS 协议（UDP、TCP、TLS）和 SOCKS5 代理转发。

## 特性

- 🚀 **智能 DNS 分流**：基于配置的域名列表智能选择不同的 DNS 服务器
- 📦 **多协议支持**：支持传统 UDP/TCP 协议和现代 TLS 加密协议（DNS over TLS）
- 🔧 **灵活的服务器格式**：支持多种 DNS 服务器配置格式，自动端口补全
- 💾 **高效缓存机制**：LRU 缓存机制显著提升查询性能
- 🌐 **代理支持**：支持 SOCKS5 代理转发，可为不同 DNS 服务器组配置独立代理
- 📋 **本地 hosts 支持**：支持本地 hosts 文件解析
- 🔗 **IPv4/IPv6 支持**：可配置的 IPv4 和 IPv6 协议支持
- 📊 **详细日志记录**：完整的查询日志和性能统计

### 分流

- 当查询域名匹配 `filtered_server_list` 文件中的记录时，使用 `filtered_servers` 进行解析
- 其他域名使用 `primary_servers` 进行解析
- 支持为不同服务器组配置独立的 SOCKS5 代理

## 使用

下载仓库 Releases 中适合的架构二进制文件，运行方法如下：

```bash
easydns -c config.yaml
```

`config.yaml` 根据需要自行修改，示例文件位于本仓库的 `.config.yaml`。

### 配置说明

配置文件使用 YAML 格式，默认为 `config.yaml`。配置项按功能模块划分，例如：

```yaml
server:
  port: 53 # DNS服务监听端口
  udp_size: 1492 # UDP包最大大小
  ipv4: true # 启用IPv4支持
  ipv6: true # 启用IPv6支持

dns:
  primary_servers: # 主DNS服务器列表（用于一般域名查询）
    - "114.114.114.114" # 自动补全为 114.114.114.114:53
    - "119.29.29.29:53" # 标准UDP格式
    - "tcp://1.1.1.1" # TCP协议，自动补全为 tcp://1.1.1.1:53

  filtered_servers: # 过滤DNS服务器列表（用于特定域名查询）
    - "tls://8.8.8.8" # DNS over TLS，自动补全为 tls://8.8.8.8:853
    - "tls://1.1.1.1:853" # DNS over TLS，指定端口

  primary_proxy:
  filter_proxy:

cache:
  limit: 4096 # DNS缓存条目数

paths:
  filtered_server_list: "/path/to/filtered_servers.txt"
  hosts: "/etc/hosts" # 系统hosts文件路径
```

### 网络代理

对于被屏蔽的 dns 服务器，可以为其配置 SOCKS5 代理，例如：

```yaml
dns:
  primary_servers:
    - "114.114.114.114"
    - "119.29.29.29"
  filtered_servers:
    - "tls://8.8.8.8"
    - "tls://1.1.1.1"

  # 为主DNS服务器配置代理（可选）
  primary_proxy: "socks5://proxy1.example.com:1080"

  # 为过滤DNS服务器配置代理（可选）
  filter_proxy: "socks5://proxy2.example.com:1081"
```

## 域名列表格式

域名列表文件（由 `paths.filtered_server_list` 指定）用于控制 DNS 分流，每行一个域名：

```
github.com
google.com
youtube.com
```

**匹配规则：**

- 精确匹配：`github.com` 只匹配 `github.com`
- 子域名不自动匹配：需要单独添加 `api.github.com`

### 配置示例场景

#### 场景 1：国内外 DNS 分流

```yaml
dns:
  primary_servers: # 国内DNS（用于一般域名）
    - "114.114.114.114"
    - "119.29.29.29"

  filtered_servers: # 国外DNS（用于国外域名）
    - "tls://8.8.8.8" # 使用TLS加密
    - "tls://1.1.1.1"
```

#### 场景 2：隐私优先配置

```yaml
dns:
  primary_servers:
    - "tls://1.1.1.1" # Cloudflare DNS over TLS
    - "tls://8.8.8.8" # Google DNS over TLS

  filtered_servers:
    - "tls://9.9.9.9" # Quad9 DNS over TLS
```

#### 场景 3：混合协议配置

```yaml
dns:
  primary_servers:
    - "114.114.114.114" # 国内UDP，速度快
    - "tcp://119.29.29.29" # 国内TCP，稳定性好

  filtered_servers:
    - "tls://8.8.8.8" # 国外TLS，隐私保护
    - "udp://1.1.1.1:53" # 国外UDP，备用
```

## 项目结构

```
├── cmd/easydns/       # 主程序入口
├── internal/          # 内部包
│   ├── cache/        # DNS缓存实现
│   ├── config/       # 配置管理
│   ├── dns/         # DNS处理器
│   └── hosts/       # hosts文件解析
├── pkg/              # 公共包
│   └── util/        # 工具函数
└── scripts/          # 辅助脚本
```

## 常见问题

### Q: 如何测试 DNS over TLS 是否工作正常？

```bash
# 使用dig测试（Linux/macOS）
dig @127.0.0.1 +tcp +tls-ca=/etc/ssl/certs/ca-certificates.crt google.com

# 使用nslookup测试（Windows）
nslookup google.com 127.0.0.1
```

### Q: 代理连接失败怎么办？

1. 检查代理服务器是否正常运行
2. 验证代理地址格式：`socks5://host:port`
3. 确认防火墙没有阻止连接
4. 查看日志获取详细错误信息

### Q: TLS 连接失败的可能原因？

1. **证书验证失败**: 检查系统时间是否正确
2. **SNI 不匹配**: 确保 DNS 服务器支持 TLS
3. **网络问题**: 检查 853 端口是否被阻止
4. **代理问题**: TLS through proxy 需要代理支持 CONNECT 方法

## 贡献指南

欢迎提交 Issue 和 Pull Request！

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

## 许可证

此项目采用 MIT 许可证。详见 [LICENSE](LICENSE) 文件。
