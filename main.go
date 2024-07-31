package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/miekg/dns"
	log "github.com/sirupsen/logrus"
)

const CONST_VERSION = "V24.07.27"

func init() {
	bytesWriter := &bytes.Buffer{}
	stdoutWriter := os.Stdout
	log.SetFormatter(&log.TextFormatter{
		TimestampFormat: "2006-01-02T15:04:05Z",
		FullTimestamp:   true})
	log.SetOutput(io.MultiWriter(bytesWriter, stdoutWriter))
	log.SetLevel(log.InfoLevel)
}

func main() {
	flagH := flag.Bool("h", false, "--help")
	flagD := flag.String("d", "114.114.114.114:53", "domestic's DNS server")
	flagO := flag.String("o", "8.8.8.8:53", "overseas' DNS server")
	flagF := flag.String("f", "domestic-domain.txt", "collection of domestic domain names")
	flagL := flag.Int("l", 4096, "cache limit")
	printVersion := flag.Bool("V", false, "Show version")
	port := flag.Int("p", 53, "service listen port")

	flagHosts := flag.String("hosts", "/etc/hosts", "path to hosts file")
	flagIPV4 := flag.Bool("ipv4", true, "enable IPV4 resolution(default true)")
	flagIPV6 := flag.Bool("ipv6", false, "enable IPV6 resolution(default false)")
	flagUPDSize := flag.Uint("udpsize", 512, "enable IPV6 resolution(default false)")

	if *flagH {
		flag.Usage()
		return
	}

	if *printVersion {
		fmt.Println(CONST_VERSION)
		return
	}

	flag.Parse()

	config = &Config{
		UpstreamDomesticDNS: *flagD,
		UpstreamOverseasDNS: *flagO,
		CacheLimit:          *flagL,
		DomesticFilePath:    *flagF,
		HostsFilePath:       *flagHosts,
		Port:                *port,
		IPV4:                *flagIPV4,
		IPV6:                *flagIPV6,
		UDPSize:             uint16(*flagUPDSize),
	}

	config.Initialize()

	dns.HandleFunc(".", handleDNSRequest)

	server := &dns.Server{Addr: fmt.Sprintf(":%d", config.Port), Net: "udp"}
	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.WithFields(log.Fields{"err": err}).Fatal("Failed to start server")
		}
	}()
	serverTCP := &dns.Server{Addr: fmt.Sprintf(":%d", config.Port), Net: "tcp"}
	log.WithFields(log.Fields{
		"config.Port":                config.Port,
		"config.UpstreamDomesticDNS": config.UpstreamDomesticDNS,
		"config.UpstreamOverseasDNS": config.UpstreamOverseasDNS,
		"config.CacheLimit":          config.CacheLimit,
		"config.DomesticFilePath":    config.DomesticFilePath,
		"config.HostsFilePath":       config.HostsFilePath,
		"config.IPV4":                config.IPV4,
		"config.IPV6":                config.IPV6,
	}).Info("string listen")
	if err := serverTCP.ListenAndServe(); err != nil {
		log.WithFields(log.Fields{"err": err}).Fatal("Failed to start server")
	}
}
