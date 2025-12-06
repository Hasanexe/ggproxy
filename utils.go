package main

import (
	"io"
	"net"
	"sync"
)

// Buffer pool for efficient memory management
var bufPool sync.Pool

// initBufferPool initializes the buffer pool
func initBufferPool() {
	bufPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, cfg.BufferSize)
		},
	}
}

// copyWithPool copies data between connections using pooled buffers
func copyWithPool(dst io.Writer, src io.Reader) {
	buf, ok := bufPool.Get().([]byte)
	if !ok {
		buf = make([]byte, cfg.BufferSize) // Fallback to direct buffer set instead of sync.pool
	}
	defer bufPool.Put(buf) // Return to pool when done

	_, _ = io.CopyBuffer(dst, src, buf)
}

// isAllowed checks if an IP address is allowed based on the configured networks
func isAllowed(ip net.IP, networks []*net.IPNet) bool {
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	for _, n := range networks {
		if n.Contains(ip4) {
			return true
		}
	}
	return false
}
