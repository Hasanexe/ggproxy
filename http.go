package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"
)

// handleHTTPDebug handles HTTP proxy requests with debug logging
func handleHTTPDebug(client net.Conn) {
	defer client.Close()
	logChan <- fmt.Sprintf("%s: New connection", "HTTP")

	// Set idle timeout
	client.SetDeadline(time.Now().Add(cfg.IdleTimeout))
	defer client.SetDeadline(time.Time{})

	reader := bufio.NewReader(client)

	line, err := reader.ReadString('\n')
	if err != nil {
		logChan <- fmt.Sprintf("HTTP: read error from %s: %v", client.RemoteAddr(), err)
		return
	}
	line = strings.TrimRight(line, "\r\n")
	logChan <- fmt.Sprintf("HTTP: request line from %s: %q", client.RemoteAddr(), line)

	parts := strings.SplitN(strings.TrimSpace(line), " ", 3)
	if len(parts) < 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		io.WriteString(client, "HTTP/1.1 400 Bad Request\r\n\r\n")
		logChan <- fmt.Sprintf("HTTP: malformed request from %s => 400", client.RemoteAddr())
		return
	}
	method, requestURI, version := parts[0], parts[1], parts[2]

	if strings.ToUpper(method) == "CONNECT" {
		logChan <- fmt.Sprintf("HTTP: CONNECT request => tunnel for %s", client.RemoteAddr())
		handleHTTPConnectDebug(client, reader, requestURI, version)
		return
	}

	// Normal forward-proxy for HTTP method=GET/POST/PUT/DELETE...
	logChan <- fmt.Sprintf("HTTP: forward proxy for method=%s from %s, URI=%s", method, client.RemoteAddr(), requestURI)

	hostPort, newFirstLine, e := parseHostPortFromAbsoluteURI(method, requestURI, version)
	if e != nil {
		io.WriteString(client, "HTTP/1.1 400 Bad Request\r\n\r\n")
		logChan <- fmt.Sprintf("HTTP: parseHostPort error for %s: %v", client.RemoteAddr(), e)
		return
	}

	remote, err := net.Dial("tcp", hostPort)
	if err != nil {
		logChan <- fmt.Sprintf("HTTP: dial fail %s => %v", hostPort, err)
		io.WriteString(client, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
		return
	}
	defer remote.Close()
	remote.SetDeadline(time.Now().Add(cfg.IdleTimeout))
	defer remote.SetDeadline(time.Time{})

	// If we want to rewrite only the first line to remove the absolute URL
	firstLine := line
	if newFirstLine != "" {
		firstLine = newFirstLine
	}
	remote.Write([]byte(firstLine + "\r\n"))

	var wg sync.WaitGroup
	wg.Add(2)

	// Client -> Remote
	go func() {
		defer wg.Done()
		copyWithPool(remote, reader)
		if tcpConn, ok := remote.(*net.TCPConn); ok {
			tcpConn.CloseWrite()
		}
	}()

	// Remote -> Client
	go func() {
		defer wg.Done()
		copyWithPool(client, remote)
		if tcpConn, ok := client.(*net.TCPConn); ok {
			tcpConn.CloseWrite()
		}
	}()

	wg.Wait()
	logChan <- fmt.Sprintf("HTTP: forward done for %s", client.RemoteAddr())
}

// handleHTTPConnectDebug handles HTTP CONNECT tunneling with debug logging
func handleHTTPConnectDebug(client net.Conn, reader *bufio.Reader, hostPort, httpVersion string) {
	logChan <- fmt.Sprintf("HTTP: Attempting to tunnel to %s for %s", hostPort, client.RemoteAddr())

	// Read headers until end of headers (blank line)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			logChan <- fmt.Sprintf("HTTP: error reading headers from %s: %v", client.RemoteAddr(), err)
			io.WriteString(client, httpVersion+" 500 Internal Server Error\r\n\r\n")
			return
		}
		// Check for end of headers (blank line)
		if line == "\r\n" || line == "\n" {
			break
		}
	}

	remote, err := net.Dial("tcp", hostPort)
	if err != nil {
		logChan <- fmt.Sprintf("HTTP: Failed to connect to %s for %s: %v", hostPort, client.RemoteAddr(), err)
		io.WriteString(client, httpVersion+" 502 Bad Gateway\r\n\r\n")
		return
	}
	// Send 200 response
	io.WriteString(client, httpVersion+" 200 Connection Established\r\n\r\n")

	logChan <- fmt.Sprintf("HTTP: tunnel established %s <-> %s", client.RemoteAddr(), hostPort)

	defer remote.Close()
	remote.SetDeadline(time.Now().Add(cfg.IdleTimeout))
	defer remote.SetDeadline(time.Time{})

	var wg sync.WaitGroup
	wg.Add(2)

	// Client -> Remote
	go func() {
		defer wg.Done()
		copyWithPool(remote, reader)
		if tcpConn, ok := remote.(*net.TCPConn); ok {
			tcpConn.CloseWrite()
		}
	}()

	// Remote -> Client
	go func() {
		defer wg.Done()
		copyWithPool(client, remote)
		if tcpConn, ok := client.(*net.TCPConn); ok {
			tcpConn.CloseWrite()
		}
	}()

	wg.Wait()
	logChan <- fmt.Sprintf("HTTP: tunnel closed %s <-> %s", client.RemoteAddr(), hostPort)
}

// handleHTTP handles HTTP proxy requests without debug logging
func handleHTTP(client net.Conn) {
	defer client.Close()

	// Set idle timeout
	client.SetDeadline(time.Now().Add(cfg.IdleTimeout))
	defer client.SetDeadline(time.Time{})

	reader := bufio.NewReader(client)

	line, err := reader.ReadString('\n')
	if err != nil {
		if !cfg.isLogOff {
			logChan <- fmt.Sprintf("HTTP: read error from %s: %v", client.RemoteAddr(), err)
		}
		return
	}
	line = strings.TrimRight(line, "\r\n")

	parts := strings.SplitN(strings.TrimSpace(line), " ", 3)
	if len(parts) < 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		io.WriteString(client, "HTTP/1.1 400 Bad Request\r\n\r\n")
		return
	}
	method, requestURI, version := parts[0], parts[1], parts[2]

	if strings.ToUpper(method) == "CONNECT" {
		handleHTTPConnect(client, reader, requestURI, version)
		return
	}

	hostPort, newFirstLine, e := parseHostPortFromAbsoluteURI(method, requestURI, version)
	if e != nil {
		io.WriteString(client, "HTTP/1.1 400 Bad Request\r\n\r\n")
		return
	}

	remote, err := net.Dial("tcp", hostPort)
	if err != nil {
		io.WriteString(client, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
		return
	}
	defer remote.Close()
	remote.SetDeadline(time.Now().Add(cfg.IdleTimeout))
	defer remote.SetDeadline(time.Time{})

	// If we want to rewrite only the first line to remove the absolute URL
	firstLine := line
	if newFirstLine != "" {
		firstLine = newFirstLine
	}
	remote.Write([]byte(firstLine + "\r\n"))

	var wg sync.WaitGroup
	wg.Add(2)

	// Client -> Remote
	go func() {
		defer wg.Done()
		copyWithPool(remote, reader)
		if tcpConn, ok := remote.(*net.TCPConn); ok {
			tcpConn.CloseWrite()
		}
	}()

	// Remote -> Client
	go func() {
		defer wg.Done()
		copyWithPool(client, remote)
		if tcpConn, ok := client.(*net.TCPConn); ok {
			tcpConn.CloseWrite()
		}
	}()

	wg.Wait()

	if !cfg.isLogOff {
		logChan <- fmt.Sprintf("HTTP: forward done for %s", client.RemoteAddr())
	}
}

// handleHTTPConnect handles HTTP CONNECT tunneling without debug logging
func handleHTTPConnect(client net.Conn, reader *bufio.Reader, hostPort, httpVersion string) {
	// Read headers until end of headers (blank line)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			io.WriteString(client, httpVersion+" 500 Internal Server Error\r\n\r\n")
			return
		}
		// Check for end of headers (blank line)
		if line == "\r\n" || line == "\n" {
			break
		}
	}

	remote, err := net.Dial("tcp", hostPort)
	if err != nil {
		io.WriteString(client, httpVersion+" 502 Bad Gateway\r\n\r\n")
		return
	}
	// Send 200 response
	io.WriteString(client, httpVersion+" 200 Connection Established\r\n\r\n")

	if !cfg.isLogOff {
		logChan <- fmt.Sprintf("HTTP: tunnel established %s <-> %s", client.RemoteAddr(), hostPort)
	}

	defer remote.Close()
	remote.SetDeadline(time.Now().Add(cfg.IdleTimeout))
	defer remote.SetDeadline(time.Time{})

	var wg sync.WaitGroup
	wg.Add(2)

	// Client -> Remote
	go func() {
		defer wg.Done()
		copyWithPool(remote, reader)
		if tcpConn, ok := remote.(*net.TCPConn); ok {
			tcpConn.CloseWrite()
		}
	}()

	// Remote -> Client
	go func() {
		defer wg.Done()
		copyWithPool(client, remote)
		if tcpConn, ok := client.(*net.TCPConn); ok {
			tcpConn.CloseWrite()
		}
	}()

	wg.Wait()
}

// parseHostPortFromAbsoluteURI parses host and port from absolute URI
func parseHostPortFromAbsoluteURI(method, requestURI, httpVersion string) (hostPort, newFirstLine string, err error) {
	u, e := url.Parse(requestURI)
	if e != nil {
		return "", "", fmt.Errorf("url parse error: %v", e)
	}
	host := u.Hostname()
	port := u.Port()
	scheme := strings.ToLower(u.Scheme)
	if port == "" {
		if scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	hostPort = net.JoinHostPort(host, port)

	// If you want minimal rewriting so the server sees "GET /path HTTP/1.1" instead of absolute
	// If you want total pass-thru, set newFirstLine = "" so caller uses the original line
	newFirstLine = fmt.Sprintf("%s %s %s", method, u.RequestURI(), httpVersion)

	return hostPort, newFirstLine, nil
}
