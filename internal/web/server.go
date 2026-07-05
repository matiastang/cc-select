package web

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cc-select/cc-select/internal/i18n"
)

// Server 是 cc-select 的本地 Web 配置服务（仅 127.0.0.1）。
type Server struct {
	port    int
	handler http.Handler
}

// NewServer 创建服务配置。port=0 表示 Start 时由系统分配空闲端口。
func NewServer(port int) *Server {
	mux := http.NewServeMux()
	mux.Handle("/api/v1/", newAPIHandler().routes())
	mux.Handle("/", http.FileServer(http.FS(assetsFS())))
	// hostGuard 包一层，拒绝非回环 Host（防 DNS 重绑定）。
	return &Server{port: port, handler: hostGuard(mux)}
}

// Start 阻塞地服务 HTTP 请求，直到 ctx 取消或出错。port=0 时用系统分配的空闲端口。
// 返回的 actualPort 是实际监听端口（启动后通过 Port 回调或日志获取见 gui.go）。
func (s *Server) Start(ctx context.Context, onReady func(actualPort int)) error {
	addr := fmt.Sprintf("127.0.0.1:%d", s.port)
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           s.handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// 用 net.Listen 提前拿到实际端口（port=0 场景），再在 goroutine 里 Serve。
	ln, err := listen(addr)
	if err != nil {
		return fmt.Errorf(i18n.T("errors.web.listen"), addr, err)
	}
	if onReady != nil {
		onReady(actualPort(ln))
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- httpSrv.Serve(ln) }()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return httpSrv.Shutdown(shutCtx)
	case err := <-serveErr:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	}
}
