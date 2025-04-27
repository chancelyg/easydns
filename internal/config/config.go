package config

import (
	"bufio"
	"os"
	"strings"

	"easydns/internal/cache"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

const Version = "1.0.1"

// Config 存储程序配置
type Config struct {
	Server ServerConfig `yaml:"server"`
	DNS    DNSConfig    `yaml:"dns"`
	Cache  CacheConfig  `yaml:"cache"`
	Paths  PathsConfig  `yaml:"paths"`

	// Runtime fields
	DomainList      map[string]struct{}
	CachePrimaryDNS *cache.DNSCache
	CacheMinorDNS   *cache.DNSCache
	HostsMap        map[string][]string
}

type ServerConfig struct {
	Port    int    `yaml:"port"`
	UDPSize uint16 `yaml:"udp_size"`
	IPV4    bool   `yaml:"ipv4"`
	IPV6    bool   `yaml:"ipv6"`
}

type DNSConfig struct {
	PrimaryServers  []string `yaml:"primary_servers"`
	FilteredServers []string `yaml:"filtered_servers"`
}

type CacheConfig struct {
	Limit int `yaml:"limit"`
}

type PathsConfig struct {
	FilteredServerList string `yaml:"filtered_server_list"`
	Hosts              string `yaml:"hosts"`
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
		cfg.DNS.PrimaryServers = []string{"8.8.8.8:53"}
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

	c.DomainList = loadDomainFile(c.Paths.FilteredServerList)
	c.HostsMap = parseHostsFile(c.Paths.Hosts)
	return nil
}

func loadDomainFile(filename string) map[string]struct{} {
	domainList := make(map[string]struct{})
	file, err := os.Open(filename)
	if err != nil {
		logrus.WithFields(logrus.Fields{"err": err, "filename": filename}).Fatal("Failed to open file")
		return domainList
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		domainList[strings.TrimSpace(scanner.Text())] = struct{}{}
	}

	if err := scanner.Err(); err != nil {
		logrus.WithFields(logrus.Fields{"err": err, "filename": filename}).Fatal("Error reading domain list file")
	}

	return domainList
}

func parseHostsFile(filePath string) map[string][]string {
	hosts := make(map[string][]string)
	file, err := os.Open(filePath)
	if err != nil {
		logrus.WithFields(logrus.Fields{"err": err, "filePath": filePath}).Error("can't open file")
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
		logrus.WithFields(logrus.Fields{"err": err, "filePath": filePath}).Error("can't load file")
	}

	return hosts
}
