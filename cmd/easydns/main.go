package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/miekg/dns"
	"github.com/sirupsen/logrus"

	"easydns/internal/config"
	dnsserver "easydns/internal/dns"
)

func init() {
	bytesWriter := &bytes.Buffer{}
	stdoutWriter := os.Stdout
	logrus.SetFormatter(&logrus.TextFormatter{
		TimestampFormat: "2006-01-02T15:04:05Z",
		FullTimestamp:   true,
	})
	logrus.SetOutput(io.MultiWriter(bytesWriter, stdoutWriter))
	logrus.SetLevel(logrus.InfoLevel)
}

func main() {
	configPath := flag.String("c", "config.yaml", "path to config file")
	flagH := flag.Bool("h", false, "show help")
	flagVersion := flag.Bool("V", false, "show version")

	flag.Parse()

	if *flagH {
		flag.Usage()
		os.Exit(0)
	}

	if *flagVersion {
		fmt.Printf("easydns %s\n", config.Version)
		os.Exit(0)
	}

	cfg, err := config.LoadFromFile(*configPath)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to load configuration")
	}

	if err := cfg.Initialize(); err != nil {
		logrus.WithError(err).Fatal("Failed to initialize configuration")
	}

	handler := dnsserver.NewHandler(cfg)
	dns.HandleFunc(".", handler.HandleDNSRequest)

	// Start UDP server
	go func() {
		server := &dns.Server{
			Addr: fmt.Sprintf(":%d", cfg.Server.Port),
			Net:  "udp",
		}
		if err := server.ListenAndServe(); err != nil {
			logrus.WithError(err).Fatal("Failed to start UDP server")
		}
	}()

	// Start TCP server
	tcpServer := &dns.Server{
		Addr: fmt.Sprintf(":%d", cfg.Server.Port),
		Net:  "tcp",
	}

	logrus.WithFields(logrus.Fields{
		"port":            cfg.Server.Port,
		"PrimaryServers":  cfg.DNS.PrimaryServers,
		"FilteredServers": cfg.DNS.FilteredServers,
	}).Info("Server started")

	if err := tcpServer.ListenAndServe(); err != nil {
		logrus.WithError(err).Fatal("Failed to start TCP server")
	}
}
