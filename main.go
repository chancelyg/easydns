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
	flagP := flag.String("p", "114.114.114.114:53", "primary dns server")
	flagM := flag.String("m", "8.8.8.8:53", "minor dns server")
	flagD := flag.String("d", "domain.txt", "domain list file path")
	flagL := flag.Int("l", 4096, "cache limit")

	flagPort := flag.Int("port", 53, "service listen port")
	flagHosts := flag.String("hosts", "/etc/hosts", "path to hosts file")
	flagIPV4 := flag.Bool("ipv4", true, "enable IPV4 resolution(default true)")
	flagIPV6 := flag.Bool("ipv6", false, "enable IPV6 resolution(default false)")
	flagUPDSize := flag.Uint("udpsize", 512, "enable IPV6 resolution(default false)")
	flagVersion := flag.Bool("version", false, "Show version")

	flag.Parse()

	if *flagH {
		flag.Usage()
		os.Exit(0)
	}

	if *flagVersion {
		fmt.Println("easydns", CONST_VERSION)
		os.Exit(0)
	}

	config = &Config{
		PrimaryDNS:     *flagP,
		MinorDNS:       *flagM,
		CacheLimit:     *flagL,
		DomainFilePath: *flagD,
		HostsFilePath:  *flagHosts,
		Port:           *flagPort,
		IPV4:           *flagIPV4,
		IPV6:           *flagIPV6,
		UDPSize:        uint16(*flagUPDSize),
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
		"config.Port":           config.Port,
		"config.PrimaryDNS":     config.PrimaryDNS,
		"config.MinorDNS":       config.MinorDNS,
		"config.CacheLimit":     config.CacheLimit,
		"config.DomainFilePath": config.DomainFilePath,
		"config.HostsFilePath":  config.HostsFilePath,
		"config.IPV4":           config.IPV4,
		"config.IPV6":           config.IPV6,
	}).Info("string listen")
	if err := serverTCP.ListenAndServe(); err != nil {
		log.WithFields(log.Fields{"err": err}).Fatal("Failed to start server")
	}
}
