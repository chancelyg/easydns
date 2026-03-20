package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	udpServer := &dns.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Net:          "udp",
		ReadTimeout:  0,
		WriteTimeout: 0,
	}

	tcpServer := &dns.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Net:          "tcp",
		ReadTimeout:  0,
		WriteTimeout: 0,
	}

	// 启动 UDP server
	go func() {
		logrus.Info("Starting UDP server")
		if err := udpServer.ListenAndServe(); err != nil {
			logrus.WithError(err).Error("UDP server error")
		}
	}()

	// 启动 TCP server
	go func() {
		logrus.Info("Starting TCP server")
		if err := tcpServer.ListenAndServe(); err != nil {
			logrus.WithError(err).Error("TCP server error")
		}
	}()

	// 启动 HTTP 健康检查服务器
	healthPort := cfg.Server.Port + 10000 // 默认 15353
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`{"primary_servers":%d,"filtered_servers":%d,"cache_primary_size":%d,"cache_minor_size":%d}`,
			len(cfg.DNS.PrimaryServers), len(cfg.DNS.FilteredServers),
			cfg.CachePrimaryDNS.Len(), cfg.CacheMinorDNS.Len())))
	})
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", healthPort),
		Handler: mux,
	}
	go func() {
		logrus.Infof("Starting health check server on port %d", healthPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Error("Health check server error")
		}
	}()

	logrus.WithFields(logrus.Fields{
		"port":            cfg.Server.Port,
		"health_port":     healthPort,
		"PrimaryServers":  cfg.DNS.PrimaryServers,
		"FilteredServers": cfg.DNS.FilteredServers,
	}).Info("Server started")

	// 等待信号实现优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logrus.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 停止 Handler 的定时器
	handler.Stop()

	// 关闭 HTTP 健康检查服务器
	httpServer.Shutdown(context.Background())

	// 关闭 UDP server
	udpNet := udpServer.PacketConn
	if udpNet != nil {
		udpNet.Close()
	}

	// 关闭 TCP server
	if tcpListener, ok := tcpServer.Listener.(net.Listener); ok {
		tcpListener.Close()
	}

	<-ctx.Done()
	logrus.Info("Server shutdown complete")
}
