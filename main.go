package main

import (
	"flag"
	"fmt"

	"github.com/miekg/dns"
	"github.com/sirupsen/logrus"
)

const CONST_VERSION = "V24.07.27"

func main() {
	flagH := flag.Bool("h", false, "--help")
	flagD := flag.String("d", "114.114.114.114:53", "domestic's DNS server")
	flagO := flag.String("o", "8.8.8.8:53", "overseas' DNS server")
	flagF := flag.String("f", "domestic-domain.txt", "collection of domestic domain names")
	flagL := flag.Int("l", 4096, "cache limit")
	flagHosts := flag.String("hosts", "/etc/hosts", "path to hosts file")
	printVersion := flag.Bool("V", false, "Show version")
	port := flag.Int("p", 53, "service listen port")

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
	}

	config.Initialize()

	dns.HandleFunc(".", handleDNSRequest)

	server := &dns.Server{Addr: fmt.Sprintf(":%d", config.Port), Net: "udp"}
	go func() {
		if err := server.ListenAndServe(); err != nil {
			logrus.Fatalf("Failed to start server: %s", err)
		}
	}()
	serverTCP := &dns.Server{Addr: fmt.Sprintf(":%d", config.Port), Net: "tcp"}
	logrus.Infof("Listening on port %d (tcp/udp), domestic server=%s, overseas server=%s, cache limit=%d", config.Port, config.UpstreamDomesticDNS, config.UpstreamOverseasDNS, config.CacheLimit)
	if err := serverTCP.ListenAndServe(); err != nil {
		logrus.Fatalf("Failed to start server: %s", err)
	}
}
