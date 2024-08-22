package main

import (
	"bufio"
	"os"
	"strings"

	lru "github.com/hashicorp/golang-lru"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	PrimaryDNS      string
	MinorDNS        string
	CacheLimit      int
	DomainFilePath  string
	HostsFilePath   string
	Port            int
	DomainList      map[string]struct{}
	CachePrimaryDNS *lru.Cache
	CacheMinorDNS   *lru.Cache
	HostsMap        map[string][]string
	IPV4            bool
	IPV6            bool
	UDPSize         uint16
}

var config *Config

func (c *Config) Initialize() {
	var err error
	c.CachePrimaryDNS, err = InitializeCache(c.CacheLimit)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Fatal("Failed to create primary dns cache")
	}

	c.CacheMinorDNS, err = InitializeCache(c.CacheLimit)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Fatal("Failed to create overseas cache")
	}

	c.DomainList = loadDomainFile(c.DomainFilePath)
	c.HostsMap = parseHostsFile(c.HostsFilePath)
}

func loadDomainFile(filename string) map[string]struct{} {
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
