package main

import (
	"bufio"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

func parseHostsFile(filePath string) map[string][]string {
	hosts := make(map[string][]string)
	file, err := os.Open(filePath)
	if err != nil {
		logrus.Errorf("Error opening /etc/hosts file: %v", err)
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
		logrus.Errorf("Error reading /etc/hosts file: %v", err)
	}

	return hosts
}
