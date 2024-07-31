package main

import (
	"bufio"
	"os"
	"strings"

	lru "github.com/hashicorp/golang-lru"
	log "github.com/sirupsen/logrus"
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
	IPV4                bool
	IPV6                bool
	UDPSize             uint16
}

var config *Config

func (c *Config) Initialize() {
	var err error
	c.CacheDomestic, err = InitializeCache(c.CacheLimit)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Fatal("Failed to create domestic cache")
	}

	c.CacheOverseas, err = InitializeCache(c.CacheLimit)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Fatal("Failed to create overseas cache")
	}

	c.DomainList = loadDomesticDomains(c.DomesticFilePath)
	c.HostsMap = parseHostsFile(c.HostsFilePath)
}

func loadDomesticDomains(filename string) map[string]struct{} {
	domainList := make(map[string]struct{})
	file, err := os.Open(filename)
	if err != nil {
		log.WithFields(log.Fields{"err": err, "filename": filename}).Fatal("Failed to open file")
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		domainList[strings.TrimSpace(scanner.Text())] = struct{}{}
	}

	if err := scanner.Err(); err != nil {
		log.WithFields(log.Fields{"err": err, "filename": filename}).Fatal("Error reading domain list file")
	}

	return domainList
}
