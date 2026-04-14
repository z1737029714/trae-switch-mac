package proxy

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"trae-switch/internal/config"
)

const (
	DefaultListenAddr = "127.0.0.1"
	DefaultListenPort = 443
)

type ProxyServer struct {
	listenAddr string
	listenPort int
	server     *http.Server
	certPEM    []byte
	keyPEM     []byte
	running    bool
	mu         sync.RWMutex
}

type ProxyStatus struct {
	Running       bool   `json:"running"`
	Address       string `json:"address"`
	Port          int    `json:"port"`
	TargetURL     string `json:"targetUrl"`
	ProviderReady bool   `json:"providerReady"`
	ProviderError string `json:"providerError"`
}

func NewProxyServer(listenAddr string, listenPort int) *ProxyServer {
	if listenAddr == "" {
		listenAddr = DefaultListenAddr
	}
	if listenPort == 0 {
		listenPort = DefaultListenPort
	}
	return &ProxyServer{
		listenAddr: listenAddr,
		listenPort: listenPort,
	}
}

func (p *ProxyServer) SetCertificate(certPEM, keyPEM []byte) {
	p.certPEM = certPEM
	p.keyPEM = keyPEM
}

func (p *ProxyServer) getProviderStatus() (string, bool, string) {
	targetURL, err := config.ValidateActiveProviderTarget()
	if err != nil {
		provider := config.GetActiveProvider()
		if provider != nil {
			return provider.OpenAIBase, false, err.Error()
		}
		return "", false, err.Error()
	}

	return targetURL.String(), true, ""
}

func buildProxyDirector(targetURL *url.URL) func(*http.Request) {
	return func(req *http.Request) {
		req.URL.Scheme = targetURL.Scheme
		req.URL.Host = targetURL.Host
		req.Host = targetURL.Host
		req.URL.Path = rewriteTargetPath(targetURL.Path, req.URL.Path)
		req.URL.RawPath = rewriteTargetPath(targetURL.EscapedPath(), req.URL.EscapedPath())
		req.URL.RawQuery = joinQuery(targetURL.RawQuery, req.URL.RawQuery)
		log.Printf("[Proxy] %s %s -> %s%s", req.Method, req.URL.Path, targetURL.Host, req.URL.Path)
	}
}

func rewriteTargetPath(basePath, requestPath string) string {
	if basePath == "" || basePath == "/" {
		if requestPath == "" {
			return "/"
		}
		return requestPath
	}

	trimmedBase := strings.TrimRight(basePath, "/")
	trimmedRequest := requestPath
	if strings.HasSuffix(trimmedBase, "/v1") && strings.HasPrefix(trimmedRequest, "/v1") {
		trimmedRequest = strings.TrimPrefix(trimmedRequest, "/v1")
	}
	trimmedRequest = strings.TrimLeft(trimmedRequest, "/")
	if trimmedRequest == "" {
		return trimmedBase
	}

	return trimmedBase + "/" + trimmedRequest
}

func joinQuery(targetQuery, requestQuery string) string {
	switch {
	case targetQuery == "":
		return requestQuery
	case requestQuery == "":
		return targetQuery
	default:
		return targetQuery + "&" + requestQuery
	}
}

func (p *ProxyServer) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return fmt.Errorf("proxy already running")
	}

	if len(p.certPEM) == 0 || len(p.keyPEM) == 0 {
		p.mu.Unlock()
		return fmt.Errorf("certificate not set")
	}

	cert, err := tls.X509KeyPair(p.certPEM, p.keyPEM)
	if err != nil {
		p.mu.Unlock()
		return fmt.Errorf("failed to load certificate: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	targetURL, err := config.ValidateActiveProviderTarget()
	if err != nil {
		p.mu.Unlock()
		return err
	}

	reverseProxy := httputil.NewSingleHostReverseProxy(targetURL)
	reverseProxy.Director = buildProxyDirector(targetURL)
	reverseProxy.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
		},
	}

	reverseProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("[Proxy Error] %s %s: %v", r.Method, r.URL.Path, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(fmt.Sprintf(`{"error": {"message": "Proxy error: %v", "type": "proxy_error"}}`, err)))
	}

	reverseProxy.ModifyResponse = func(resp *http.Response) error {
		resp.Header.Set("Access-Control-Allow-Origin", "*")
		resp.Header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		resp.Header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		return nil
	}

	models := config.GetModels()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.URL.Path == "/v1/models" && r.Method == "GET" {
			modelList := make([]map[string]interface{}, 0)
			for _, model := range models {
				modelList = append(modelList, map[string]interface{}{
					"id":       model,
					"object":   "model",
					"owned_by": "custom",
				})
			}

			response := map[string]interface{}{
				"object": "list",
				"data":   modelList,
			}
			responseBytes, _ := json.Marshal(response)

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.WriteHeader(http.StatusOK)
			w.Write(responseBytes)
			return
		}

		reverseProxy.ServeHTTP(w, r)
	})

	p.running = true
	p.mu.Unlock()

	addr := fmt.Sprintf("%s:%d", p.listenAddr, p.listenPort)
	listener, err := tls.Listen("tcp", addr, tlsConfig)
	if err != nil {
		p.mu.Lock()
		p.running = false
		p.mu.Unlock()
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	p.server = &http.Server{
		Handler: handler,
	}

	go func() {
		log.Printf("[Proxy] Server started on %s", addr)
		if err := p.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("[Proxy] Server error: %v", err)
		}
		p.mu.Lock()
		p.running = false
		p.mu.Unlock()
	}()

	return nil
}

func (p *ProxyServer) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return nil
	}

	if p.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := p.server.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown server: %w", err)
		}
	}

	p.running = false
	log.Println("[Proxy] Server stopped")
	return nil
}

func (p *ProxyServer) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

func (p *ProxyServer) GetStatus() ProxyStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	targetURL, providerReady, providerError := p.getProviderStatus()
	return ProxyStatus{
		Running:       p.running,
		Address:       p.listenAddr,
		Port:          p.listenPort,
		TargetURL:     targetURL,
		ProviderReady: providerReady,
		ProviderError: providerError,
	}
}

func CheckPortStatus(port int) (available bool, processInfo string) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	listener, err := net.Listen("tcp", addr)
	if err == nil {
		listener.Close()
		return true, ""
	}
	return false, "端口被占用"
}
