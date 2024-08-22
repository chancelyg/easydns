# 1. DNS 代理服务器

这是一个用 Go 编写的简单 DNS 代理服务器，可以根据域名列表将 DNS 请求分别转发到不同的上游 DNS 服务器，并缓存 DNS 响应以提高性能

## 1.1. 功能特性

- 根据域名列表将 DNS 请求分别转发到主副 DNS 服务器（国内或国外的上游 DNS 服务器）
- 使用 LRU 缓存机制缓存 DNS 响应
- 支持 UDP 和 TCP 协议
- 提供详细的 DNS 解析日志记录

## 1.2. 使用方法

下载 Releases 中的二进制文件，直接执行

```bash
./easydns -p 114.114.114.114:53 -m 8.8.8.8:53 -d domain.txt
```

参数解析：
* **-p**：Primary DNS 服务器，若域名不在指定集合内，将由该DNS服务器进行解析
* **-m**：Minor DNS 服务器，若域名在指定集合内，将从该DNS服务器进行解析
* **-d**：指定域名集合，每行一个域名

## 1.3. 命令行参数

参数说明：

* `-p`：Primary DNS 服务器（默认值为 `114.114.114.114:53`）
* `-o`：Minor DNS 服务器（默认值为 `8.8.8.8:53`）
* `-d`：域名列表文件的路径（默认值为 `domain.txt`）
* `-p`：服务监听端口（默认值为 `53`）
* `-l`：缓存条数限制（默认值为 `4096`）
* `-hosts`：本地hosts的位置（默认 `/etc/hosts`）
* `-udpsize`：指定udp包大小（默认 `512`）
* `-h`：显示帮助信息
* `-V`：显示版本信息

对于域名列表文件，应是每行一个域名，如下：

```bash
github.com
google.com
```

另外，如果文件中的域名是顶级域名，则其域名下所有二级域名均会被视为需要分流至 Minor DNS 服务器

## 1.4. 日志

程序会将运行时的各种信息输出到控制台，包括请求的客户端 IP、请求的域名、使用的上游 DNS 服务器、请求类型、缓存状态等


## 1.5. 依赖

该项目依赖以下 Go 库：

- `github.com/hashicorp/golang-lru`
- `github.com/miekg/dns`
- `github.com/sirupsen/logrus`

你可以使用 `go mod` 来管理这些依赖：

```bash
go mod tidy
```

## 1.6. 许可证

此项目使用 MIT 许可证，详细信息请参阅 LICENSE 文件