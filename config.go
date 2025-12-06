package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Config holds all configuration options
type Config struct {
	Port              int
	isSocks           bool
	isDebug           bool
	isLogOff          bool
	LogFile           string
	AllowedIPs        []string
	IdleTimeout       time.Duration
	BufferSize        int
	LogChanBufferSize int
}

// loadConfig loads configuration from the specified file path
func loadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Default config with mode=http, port=3128
	cfg := &Config{
		isSocks:           false,                     //proxy_mode   = http
		isDebug:           false,                     //log_level    = debug
		isLogOff:          false,                     //log_level    = off || none
		Port:              3128,                      //port             = 3128
		AllowedIPs:        []string{},                //allowed_ip   = 0.0.0.0/0 (cidr)
		LogFile:           "/var/log/mikroproxy.log", //log_file
		IdleTimeout:       30 * time.Second,          //idle_timeout
		BufferSize:        32 * 1024,                 //buffer_size
		LogChanBufferSize: 1000,                      //log_buffer_size
	}

	var content strings.Builder
	buf := make([]byte, 1024)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			content.Write(buf[:n])
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
	}

	lines := strings.Split(content.String(), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "proxy_mode":
			cfg.isSocks = strings.HasPrefix(strings.ToLower(val), "socks")
		case "port":
			var p int
			fmt.Sscanf(val, "%d", &p)
			if p > 0 && p < 65536 {
				cfg.Port = p
			}
		case "log_file":
			cfg.LogFile = val
		case "log_level":
			logLevel := strings.ToLower(val)
			cfg.isDebug = logLevel == "debug"
			cfg.isLogOff = logLevel == "off" || logLevel == "none"
		case "allowed_ip":
			cfg.AllowedIPs = append(cfg.AllowedIPs, val)
		case "idle_timeout":
			dur, err := time.ParseDuration(val)
			if err != nil {
				return nil, fmt.Errorf("invalid idle_timeout: %v", err)
			}
			if dur <= 0 {
				return nil, fmt.Errorf("idle_timeout must be > 0")
			}
			cfg.IdleTimeout = dur
		case "buffer_size":
			var size int
			if _, err := fmt.Sscanf(val, "%d", &size); err != nil {
				return nil, fmt.Errorf("invalid buffer_size: %v", err)
			}
			if size <= 0 {
				return nil, fmt.Errorf("buffer_size must be > 0")
			}
			cfg.BufferSize = size
		case "log_buffer_size":
			var size int
			if _, err := fmt.Sscanf(val, "%d", &size); err != nil {
				return nil, fmt.Errorf("invalid log_buffer_size: %v", err)
			}
			if size <= 0 {
				return nil, fmt.Errorf("log_buffer_size must be > 0")
			}
			cfg.LogChanBufferSize = size
		}
	}
	return cfg, nil
}
