// server.go 是路由 daemon 的服务面（specs/001 contracts/router-http.md §1）：
//
//   - 仅绑定 loopback；
//   - /healthz：GET 返回 {status,version}（无鉴权，无敏感信息）；
//     POST + 内部 stop token = 优雅退出的侧信道（CLI `router stop` 用）；
//   - 其余全部路径：要求 Bearer 伪 token（= 终端身份 tid），查路由表命中后
//     交给转发层（forward），未知/缺失 → 401 + 人类可读指引。
package router

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/cc-select/cc-select/internal/routes"
)

// stopHeader 是停止侧信道的内部头：POST /healthz 携带正确 token 即优雅退出。
// token 随每次 daemon 启动随机生成并写入状态文件（仅本用户可读）。
const stopHeader = "X-CC-Select-Stop"

// Server 是路由 daemon。
type Server struct {
	addr      string // 期望监听地址（":0" = 随机端口）
	version   string // healthz 回显 / Ensure 版本比对用
	forward   http.Handler
	stopToken string
	httpSrv   *http.Server
}

// NewServer 创建 daemon。forward 是已鉴权请求的转发管线（forward.go 组装）。
func NewServer(addr, version string, forward http.Handler) *Server {
	return &Server{addr: addr, version: version, forward: forward}
}

// Listen 绑定 loopback 端口并写状态文件（addr/pid/startedAt/version/stopToken）。
// 实际监听地址以返回的 listener 为准（":0" 时由内核分配）。
func (s *Server) Listen() (net.Listener, error) {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return nil, fmt.Errorf("router: 监听 %s 失败: %w", s.addr, err)
	}
	tok := make([]byte, 16)
	if _, err := rand.Read(tok); err != nil {
		ln.Close()
		return nil, fmt.Errorf("router: 生成 stop token 失败: %w", err)
	}
	s.stopToken = hex.EncodeToString(tok)
	s.httpSrv = &http.Server{Handler: s.handler()}
	if err := SaveState(&State{
		Addr:      ln.Addr().String(),
		PID:       os.Getpid(),
		StartedAt: time.Now().UTC(),
		Version:   s.version,
		StopToken: s.stopToken,
	}); err != nil {
		ln.Close()
		return nil, err
	}
	return ln, nil
}

// Serve 在 listener 上服务，直到停止侧信道触发（返回 nil）或出错。
func (s *Server) Serve(ln net.Listener) error {
	if err := s.httpSrv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Close 立即关闭（测试清理/异常路径用）。
func (s *Server) Close() error {
	if s.httpSrv == nil {
		return nil
	}
	return s.httpSrv.Close()
}

// handler 组装路由：/healthz 直通，其余经伪 token 鉴权后进转发层。
func (s *Server) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.Handle("/", s.auth(s.forward))
	return mux
}

// handleHealthz 处理 GET（探活）与 POST+stopHeader（优雅退出侧信道）。
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		if r.Header.Get(stopHeader) == "" || r.Header.Get(stopHeader) != s.stopToken {
			http.Error(w, "invalid stop token", http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"stopping"}`))
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = s.httpSrv.Shutdown(ctx)
		}()
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(HealthInfo{Status: "ok", Version: s.version})
}

// auth 校验 Bearer 伪 token（= tid）并在上下文挂载命中的路由条目。
// 未知/缺失 tid → 401，body 给出人类可读指引（研究 L1）。
func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tid := bearerToken(r)
		if tid == "" || !routes.ValidateTID(tid) {
			unauthorized(w)
			return
		}
		entry, ok := routes.Get(tid)
		if !ok {
			unauthorized(w)
			return
		}
		next.ServeHTTP(w, r.WithContext(withEntry(r.Context(), entry)))
	})
}

// unauthorized 输出 401 + 指引（裸启 claude / 未 ccs use 的场景）。
func unauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"type":    "cc_select_unauthorized",
			"message": "unknown terminal token; run `ccs use <provider>` (proxy mode) in your shell before launching claude; a bare `claude` start also requires clearing legacy ANTHROPIC_* env in ~/.claude/settings.json",
		},
	})
}

// bearerToken 从 Authorization 头提取 Bearer token。
func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	auth := r.Header.Get("Authorization")
	if len(auth) > len(prefix) && auth[:len(prefix)] == prefix {
		return auth[len(prefix):]
	}
	return ""
}
