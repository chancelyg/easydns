package main

import (
	"bufio"
	"os"
	"strings"

	lru "github.com/hashicorp/golang-lru"
	"github.com/sirupsen/logrus"
)

type Config struct {
	UpstreamDomesticDNS string
	UpstreamOverseasDNS string
	CacheLimit          int
	DomesticFilePath    string
	HostsFilePath       string
	Port                int
	DomainList          map[string]struct{}
	CacheDomestic       *lru.Cache
	CacheOverseas       *lru.Cache
	HostsMap            map[string][]string
}

var config *Config

func (c *Config) Initialize() {
	var err error
	c.CacheDomestic, err = InitializeCache(c.CacheLimit)
	if err != nil {
		logrus.Fatalf("Failed to create domestic cache: %v", err)
	}

	c.CacheOverseas, err = InitializeCache(c.CacheLimit)
	if err != nil {
		logrus.Fatalf("Failed to create overseas cache: %v", err)
	}

	logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
	logrus.SetLevel(logrus.InfoLevel)

	c.DomainList = loadDomesticDomains(c.DomesticFilePath)
	c.HostsMap = parseHostsFile(c.HostsFilePath)
}

func loadDomesticDomains(filename string) map[string]struct{} {
	domainList := make(map[string]struct{})
	file, err := os.Open(filename)
	if err != nil {
		logrus.Fatalf("Failed to open %s file: %v", filename, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		domainList[strings.TrimSpace(scanner.Text())] = struct{}{}
	}

	if err := scanner.Err(); err != nil {
		logrus.Fatalf("Error reading domain list file: %v", err)
	}

	return domainList
}
