package comm

import (
	"crypto/tls"
	"fmt"
	"github.com/astaxie/beego"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type ShortLink struct {
	Code      string
	TargetURL string
	Expiry    time.Time
}

type TempProxy struct {
	ListenAddr   string
	ActualAddr   string
	server       *http.Server
	listener     net.Listener
	running      bool
	shortLinks   map[string]*ShortLink
	shortLinksMu sync.RWMutex
	allowedHosts map[string]bool
	once         sync.Once
}

var (
	globalProxy *TempProxy
	once        sync.Once
	initMu      sync.Mutex
)

// 初始化代理（只生效一次）
func InitProxy(port string, hosts ...string) *TempProxy {
	initMu.Lock()
	defer initMu.Unlock()

	if globalProxy != nil {
		log.Printf("⚠️ 已存在，跳过重复初始化")
		return globalProxy
	}

	httpaddr := "0.0.0.0:"

	proxy := &TempProxy{
		ListenAddr:   httpaddr + port,
		running:      false,
		shortLinks:   make(map[string]*ShortLink),
		allowedHosts: make(map[string]bool),
	}

	for _, host := range hosts {
		proxy.allowedHosts[host] = true
	}

	globalProxy = proxy
	return proxy
}

// 获取全局实例
func GetProxy() *TempProxy {
	once.Do(func() {
		if globalProxy == nil {
			log.Printf("⚠️ 警告：未初始化，使用默认配置")
			InitProxy("8888")
		}
	})
	return globalProxy
}

func (p *TempProxy) findFreePort(startPort int) (string, error) {
	for port := startPort; port <= startPort+50; port++ {
		addr := fmt.Sprintf("0.0.0.0:%d", port)
		listener, err := net.Listen("tcp", addr)
		if err == nil {
			listener.Close()
			return addr, nil
		}
	}
	return "", fmt.Errorf("端口范围 %d-%d 无空闲", startPort, startPort+50)
}

func (p *TempProxy) Start() error {
	var err error
	p.once.Do(func() {
		if p.running {
			return
		}

		_, portStr, _ := net.SplitHostPort(p.ListenAddr)
		startPort := 8888
		fmt.Sscanf(portStr, "%d", &startPort)

		listener, e := net.Listen("tcp", p.ListenAddr)
		if e != nil {
			freeAddr, e := p.findFreePort(startPort)
			if e != nil {
				err = e
				return
			}
			listener, e = net.Listen("tcp", freeAddr)
			if e != nil {
				err = e
				return
			}
			p.ActualAddr = listener.Addr().String()
			log.Printf("⚠️  %s 被占用 → 切换到: %s", p.ListenAddr, p.ActualAddr)
		} else {
			p.ActualAddr = listener.Addr().String()
			log.Printf("✅ 验证地址启动: http://%s , ip地址在app.conf中配置httpaddr即可~", p.ActualAddr)
		}

		p.listener = listener
		p.server = &http.Server{Addr: p.ActualAddr}
		p.server.Handler = p.createMux()
		p.running = true

		go func() {
			if e := p.server.Serve(p.listener); e != nil && e != http.ErrServerClosed {
				log.Printf("❌ 服务错误: %v", e)
			}
		}()
	})
	return err
}

func (p *TempProxy) Shutdown() {
	p.running = false
	if p.server != nil {
		p.server.Close()
	}
	log.Printf("🛑 已关闭")
}

func (p *TempProxy) createMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", p.proxyHandler)
	return mux
}

func (p *TempProxy) proxyHandler(w http.ResponseWriter, r *http.Request) {
	if !p.running {
		http.Error(w, "服务关闭", http.StatusServiceUnavailable)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/s/") {
		p.handleShortLink(w, r)
		return
	}
	http.Error(w, "使用: 生成短链接访问", http.StatusOK)
}

func (p *TempProxy) handleShortLink(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimPrefix(r.URL.Path, "/s/")
	if len(code) != 6 {
		http.Error(w, "无效", http.StatusBadRequest)
		return
	}

	p.shortLinksMu.RLock()
	link, exists := p.shortLinks[code]
	var expired bool
	if exists && time.Now().After(link.Expiry) {
		expired = true
		exists = false
	}
	p.shortLinksMu.RUnlock()

	if !exists {
		http.Error(w, "不存在或过期", http.StatusGone)
		return
	}
	if expired {
		p.shortLinksMu.Lock()
		delete(p.shortLinks, code)
		p.shortLinksMu.Unlock()
		http.Error(w, "已过期", http.StatusGone)
		return
	}

	target, err := url.Parse(link.TargetURL)
	if err != nil {
		http.Error(w, "目标错误", http.StatusInternalServerError)
		return
	}

	p.reverseProxy(w, r, target)
}

func (p *TempProxy) reverseProxy(w http.ResponseWriter, r *http.Request, target *url.URL) {
	// 1. 创建代理请求
	proxyReq := new(http.Request)
	*proxyReq = *r
	proxyReq.URL = target
	proxyReq.Host = target.Host
	proxyReq.RequestURI = ""

	// 2. 透传所有 Header
	proxyReq.Header = make(http.Header)
	for k, vv := range r.Header {
		for _, v := range vv {
			proxyReq.Header.Add(k, v)
		}
	}

	// 3. 确保 User-Agent、Referer 不为空（防被屏蔽）
	if proxyReq.Header.Get("User-Agent") == "" {
		proxyReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	}
	if proxyReq.Header.Get("Accept") == "" {
		proxyReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	}

	// 4. 自定义 Transport（完整 TLS 支持）
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			ServerName:         target.Hostname(),
			InsecureSkipVerify: false, // 生产环境建议 false
		},
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 20 * time.Second,
		DisableKeepAlives:     false,
	}

	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// 允许最多 10 次重定向
			if len(via) > 10 {
				return fmt.Errorf("过多重定向")
			}
			// ✅ 关键：重定向时也要走代理
			req.Host = req.URL.Host
			return nil
		},
		Timeout: 30 * time.Second,
	}

	// 5. 发起请求
	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, "请求失败: "+err.Error(), http.StatusBadGateway)
		log.Printf("❌ 代理失败: %v", err)
		return
	}
	defer resp.Body.Close()

	// 6. 复制所有响应头（包括 Set-Cookie）
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}

	// 7. 写入状态码和 body
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("❌ 响应写入失败: %v", err)
	}
}

func (p *TempProxy) randShortCode() string {
	const chars = "abcdef0123456789"
	b := make([]byte, 6)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

// ✅ 全局函数：生成短链接
func GenerateShortURL(target string) (string, error) {
	proxy := GetProxy()

	u, err := url.Parse(target)
	if err != nil {
		return "", fmt.Errorf("解析失败")
	}

	if len(proxy.allowedHosts) > 0 {
		if !proxy.allowedHosts[u.Host] {
			return "", fmt.Errorf("禁止域名: %s", u.Host)
		}
	}

	var code string
	for i := 0; i < 10; i++ {
		code = proxy.randShortCode()
		proxy.shortLinksMu.RLock()
		_, exists := proxy.shortLinks[code]
		proxy.shortLinksMu.RUnlock()
		if !exists {
			break
		}
	}
	if code == "" {
		return "", fmt.Errorf("生成失败")
	}

	proxy.shortLinksMu.Lock()
	proxy.shortLinks[code] = &ShortLink{
		Code:      code,
		TargetURL: target,
		Expiry:    time.Now().Add(3 * time.Minute),
	}
	proxy.shortLinksMu.Unlock()

	return fmt.Sprintf("http://%s:%d/s/%s", beego.AppConfig.String("codeaddr"), 8888, code), nil
}
