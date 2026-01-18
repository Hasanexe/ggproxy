package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

// -----------------------------------------------------
// Setup
// -----------------------------------------------------

// Logging
var (
	cfg     *Config
	logChan chan string
)

const logChanBufferSize = 1024

// -----------------------------------------------------
// Main
// -----------------------------------------------------

func main() {
	configPath := flag.String("config", "/etc/ggproxy.conf", "Path to ggproxy config file")
	flag.Parse()

	// Load config
	var err error
	cfg, err = loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Setup async logging (stdout only; journald can capture stdout via systemd)
	logChan = make(chan string, logChanBufferSize)
	go func() {
		for msg := range logChan {
			timestamp := time.Now().Format("02.01.2006 15:04:05")
			fmt.Fprintln(os.Stdout, timestamp+" "+msg)
		}
	}()

	// Initialize buffer pool
	initBufferPool()

	// Parse allowed CIDRs
	var networks []*net.IPNet
	for _, cidrStr := range cfg.AllowedIPs {
		ip, ipNet, e := net.ParseCIDR(cidrStr)
		if e != nil {
			logChan <- fmt.Sprintf("Invalid CIDR %q (skipped): %v", cidrStr, e)
			continue
		}
		// skip IPv6
		if ip.To4() == nil {
			logChan <- fmt.Sprintf("Skipping IPv6 CIDR %q", cidrStr)
			continue
		}
		networks = append(networks, ipNet)
	}

	// ListenConfig with global keep-alive
	lc := &net.ListenConfig{
		KeepAlive: 15 * time.Second,
	}

	addr := fmt.Sprintf("0.0.0.0:%d", cfg.Port)
	ln, err := lc.Listen(context.Background(), "tcp", addr)
	if err != nil {
		logChan <- fmt.Sprintf("Failed to listen on %s: %v", addr, err)
		os.Exit(1)
	}
	defer ln.Close()

	modeStr := "HTTP"
	if cfg.isSocks {
		modeStr = "SOCKS"
	}
	logChan <- fmt.Sprintf("%s: listening on %s", modeStr, addr)

	// Accept loop
	for {
		conn, err := ln.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				os.Exit(0)
			}
			if !cfg.isLogOff {
				logChan <- fmt.Sprintf("%s: Accept error: %v", modeStr, err)
			}
			continue
		}

		go handleConnection(conn, networks)
	}
}

// handleConnection handles incoming connections
func handleConnection(c net.Conn, networks []*net.IPNet) {
	defer c.Close()

	remoteAddr, ok := c.RemoteAddr().(*net.TCPAddr)
	if !ok {
		if !cfg.isLogOff {
			modeStr := "HTTP"
			if cfg.isSocks {
				modeStr = "SOCKS"
			}
			logChan <- fmt.Sprintf("%s: Could not parse remote address: %v", modeStr, c.RemoteAddr())
		}
		return
	}

	if !isAllowed(remoteAddr.IP, networks) {
		if !cfg.isLogOff {
			modeStr := "HTTP"
			if cfg.isSocks {
				modeStr = "SOCKS"
			}
			logChan <- fmt.Sprintf("%s: Denying client %s (not in allowed ranges)", modeStr, remoteAddr.IP)
		}
		return
	}

	c.SetReadDeadline(time.Now().Add(10 * time.Second))

	if cfg.isSocks {
		if cfg.isDebug {
			handleSocksDebug(c)
		} else {
			handleSocks(c)
		}
	} else {
		if cfg.isDebug {
			handleHTTPDebug(c)
		} else {
			handleHTTP(c)
		}
	}
}
