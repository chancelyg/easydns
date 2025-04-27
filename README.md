# EasyDNS

一个用 Go 编写的高性能 DNS 代理服务器，支持基于域名列表的 DNS 分流、缓存和本地 hosts。

## 特性

- 基于配置的 DNS 分流（例如国内外采用不同的 DNS 查询）
- LRU 缓存机制提升查询性能
- 支持 UDP 和 TCP 协议
- 支持本地 hosts 文件
- IPv4/IPv6 配置支持
- 详细的日志记录

分流策略：
* 当域名符合配置文件中的 `filtered_server_list` 中的记录时，将返回 `FilteredServerList` 的解析结果
* 其他域名一律走 `primary_servers` 的解析结果

## 安装

下载仓库 Releases 中适合的架构二进制文件，运行方法如下：

```
easydns -c config.yaml
```

`config.yaml` 根据需要自行修改，示例文件位于本仓库的 `.config.yaml` 中

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

## 配置说明

配置文件使用 YAML 格式，默认为 `config.yaml`。配置项按功能模块划分：

```yaml
server:
  port: 53            # 服务监听端口
  udp_size: 512       # UDP包大小
  ipv4: true         # 是否启用IPv4
  ipv6: false        # 是否启用IPv6

dns:
  primary_servers:    # 主DNS服务器列表
    - "114.114.114.114:53"
    - "1.1.1.1:53"
    - "8.8.4.4:53"
  filtered_servers: # 备用DNS服务器
    - "1.1.1.1:53"
    - "8.8.8.8:53"

cache:
  limit: 4096        # 缓存条目限制

paths:
  filtered_server_list: "filtered_servers.txt"  # 域名列表文件路径
  hosts: "/etc/hosts"      # hosts文件路径
```

## 域名列表格式

域名列表文件（由 paths.filtered_server_list 指定）的格式为每行一个域名：

```
github.com
google.com
```

## 开发相关

### 依赖
- Go 1.18+
- github.com/miekg/dns: DNS库
- github.com/hashicorp/golang-lru: LRU缓存实现
- gopkg.in/yaml.v3: YAML配置解析

### 构建

```bash
goreleaser --snapshot --clean
```

## 许可证

此项目采用 MIT 许可证。详见 [LICENSE](LICENSE) 文件。