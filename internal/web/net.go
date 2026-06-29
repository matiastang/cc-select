package web

import (
	"net"
	"net/http"
)

// listen 在给定地址（127.0.0.1:port）监听 TCP。
func listen(addr string) (net.Listener, error) {
	return net.Listen("tcp", addr)
}

// actualPort 从 listener 提取实际端口（port=0 时由系统分配）。
func actualPort(ln net.Listener) int {
	if tcp, ok := ln.Addr().(*net.TCPAddr); ok {
		return tcp.Port
	}
	return 0
}

// hostGuard 拒绝 Host 头不是本机回环的请求，阻断 DNS 重绑定攻击：
// 服务虽只监听 127.0.0.1，但浏览器可被诱导用恶意域名（重绑定到 127.0.0.1）访问，
// 借同源策略读取 GET /providers/{id} 返回的明文 token。校验 Host 即可堵住该向量。
func hostGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h // 去掉端口
		}
		switch host {
		case "127.0.0.1", "localhost", "::1", "[::1]":
			next.ServeHTTP(w, r)
		default:
			http.Error(w, "forbidden: invalid Host header", http.StatusForbidden)
		}
	})
}
