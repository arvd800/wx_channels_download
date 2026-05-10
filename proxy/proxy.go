// Package proxy implements an HTTP/HTTPS proxy server that intercepts
// WeChat Channels (视频号) media requests to extract download URLs.
package proxy

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
)

// MediaInfo holds extracted information about a captured media stream.
type MediaInfo struct {
	URL      string
	Headers  map[string]string
	FileKey  string
	DecryKey string
}

// Proxy represents the intercepting proxy server.
type Proxy struct {
	addr     string
	mu       sync.Mutex
	mediaMap map[string]*MediaInfo
	onMedia  func(*MediaInfo)
}

// New creates a new Proxy instance listening on the given address.
func New(addr string, onMedia func(*MediaInfo)) *Proxy {
	return &Proxy{
		addr:     addr,
		mediaMap: make(map[string]*MediaInfo),
		onMedia:  onMedia,
	}
}

// Start starts the proxy server and blocks until an error occurs.
func (p *Proxy) Start() error {
	server := &http.Server{
		Addr:    p.addr,
		Handler: p,
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec // required for MITM proxy
		},
	}
	log.Printf("[proxy] listening on %s", p.addr)
	return server.ListenAndServe()
}

// ServeHTTP handles incoming proxy requests.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		p.handleTunnel(w, r)
		return
	}
	p.handleHTTP(w, r)
}

// handleHTTP proxies plain HTTP requests and inspects WeChat media URLs.
func (p *Proxy) handleHTTP(w http.ResponseWriter, r *http.Request) {
	if isWxChannelsMedia(r.URL) {
		p.captureMedia(r)
	}

	target := r.URL
	rp := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL = target
			req.Host = target.Host
			req.Header.Set("X-Forwarded-For", r.RemoteAddr)
		},
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		},
	}
	rp.ServeHTTP(w, r)
}

// handleTunnel establishes a TCP tunnel for HTTPS CONNECT requests.
func (p *Proxy) handleTunnel(w http.ResponseWriter, r *http.Request) {
	dest, err := net.Dial("tcp", r.Host)
	if err != nil {
		http.Error(w, fmt.Sprintf("cannot connect to %s: %v", r.Host, err), http.StatusBadGateway)
		return
	}
	defer dest.Close()

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking not supported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer clientConn.Close()

	_, _ = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	go func() { _, _ = io.Copy(dest, clientConn) }()
	_, _ = io.Copy(clientConn, dest)
}

// captureMedia extracts and stores media information from a WeChat Channels request.
func (p *Proxy) captureMedia(r *http.Request) {
	info := &MediaInfo{
		URL:     r.URL.String(),
		Headers: make(map[string]string),
	}
	for k, v := range r.Header {
		if len(v) > 0 {
			info.Headers[k] = v[0]
		}
	}
	q := r.URL.Query()
	info.FileKey = q.Get("filekey")
	info.DecryKey = q.Get("decrykey")

	p.mu.Lock()
	p.mediaMap[info.FileKey] = info
	p.mu.Unlock()

	log.Printf("[proxy] captured media: filekey=%s", info.FileKey)
	if p.onMedia != nil {
		p.onMedia(info)
	}
}

// isWxChannelsMedia reports whether the URL belongs to a WeChat Channels media endpoint.
func isWxChannelsMedia(u *url.URL) bool {
	host := u.Hostname()
	path := u.Path
	return (strings.Contains(host, "finder.video.qq.com") ||
		strings.Contains(host, "channels.weixin.qq.com")) &&
		(strings.Contains(path, "/finder/") || strings.Contains(path, "/download/"))
}
