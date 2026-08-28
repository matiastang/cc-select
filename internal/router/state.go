// router 包是 Mode P 的本地路由守护进程（specs/001 contracts/router-http.md）。
//
// state.go 负责 daemon 状态文件与自愈（研究 D7/D11）：
//   - ~/.cc-select/router.json 记录 {addr, pid, startedAt, version}；
//   - addr 一经固化必须沿用（claude 的 BASE_URL 已写进 profile，换端口 = 运行中会话全体失联）；
//   - Ensure：healthz 健康复用 / 死亡或版本不匹配则拉起并等待就绪。
package router

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// DefaultAddr 是 daemon 的默认监听地址（仅 loopback）。
const DefaultAddr = "127.0.0.1:48270"

// ProxyAddrEnv 允许用户覆盖监听地址（仅首次启动生效，之后以状态文件为准）。
const ProxyAddrEnv = "CC_SELECT_PROXY_ADDR"

// State 是 router.json 的内容。PID 仅诊断用——存活判定以 healthz 为准（PID 复用不可靠）。
// StopToken 是停止侧信道的内部 token（每次启动随机生成；见 server.go stopHeader）。
type State struct {
	Addr      string    `json:"addr"`
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"startedAt"`
	Version   string    `json:"version"`
	StopToken string    `json:"stopToken,omitempty"`
}

// StatePath 返回 router.json 绝对路径：与 providers.json（CC_SELECT_CONFIG）同目录，
// 默认 ~/.cc-select/router.json。同 prefs/routes 约定，便于测试隔离。
func StatePath() (string, error) {
	if p := os.Getenv("CC_SELECT_CONFIG"); p != "" {
		return filepath.Join(filepath.Dir(p), "router.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cc-select", "router.json"), nil
}

// LoadState 读取状态文件；不存在返回 (nil, nil)。
func LoadState() (*State, error) {
	p, err := StatePath()
	if err != nil {
		return nil, fmt.Errorf("router: 定位状态文件失败: %w", err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("router: 读取状态文件失败: %w", err)
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("router: 解析状态文件失败（%s）: %w", p, err)
	}
	return &st, nil
}

// SaveState 以原子方式写入状态文件（临时文件 0600 + rename）。
func SaveState(st *State) error {
	p, err := StatePath()
	if err != nil {
		return fmt.Errorf("router: 定位状态文件失败: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return fmt.Errorf("router: 创建目录失败: %w", err)
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("router: 序列化状态失败: %w", err)
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(p), ".router-*.json.tmp")
	if err != nil {
		return fmt.Errorf("router: 创建临时文件失败: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("router: 写入临时文件失败: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("router: 设置权限失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("router: 关闭临时文件失败: %w", err)
	}
	if err := os.Rename(tmpName, p); err != nil {
		cleanup()
		return fmt.Errorf("router: 替换状态文件失败: %w", err)
	}
	return nil
}

// ResolveAddr 解析 daemon 应使用/探活的地址。优先级（研究 D7/D11）：
// 状态文件 > CC_SELECT_PROXY_ADDR > DefaultAddr——已固化的 addr 不漂移。
func ResolveAddr() (string, error) {
	if st, err := LoadState(); err == nil && st != nil && st.Addr != "" {
		return st.Addr, nil
	}
	if env := os.Getenv(ProxyAddrEnv); env != "" {
		return env, nil
	}
	return DefaultAddr, nil
}

// HealthInfo 是 /healthz 的响应体。
type HealthInfo struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// probeTimeout 单次 healthz 探测超时（本地回环，应当极快）。
const probeTimeout = 500 * time.Millisecond

// Probe 探测 addr 上的 daemon 是否健康（status=ok）。
func Probe(addr string) (*HealthInfo, error) {
	client := &http.Client{Timeout: probeTimeout}
	resp, err := client.Get("http://" + addr + "/healthz")
	if err != nil {
		return nil, fmt.Errorf("router: healthz 探测失败（%s）: %w", addr, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("router: healthz 状态码 %d（%s）", resp.StatusCode, addr)
	}
	var h HealthInfo
	if err := json.NewDecoder(resp.Body).Decode(&h); err != nil {
		return nil, fmt.Errorf("router: healthz 响应不可解析（%s）: %w", addr, err)
	}
	if h.Status != "ok" {
		return nil, fmt.Errorf("router: healthz status=%q（%s）", h.Status, addr)
	}
	return &h, nil
}

// EnsureDeps 把 Ensure 的副作用注入化：生产 Spawn = detached 拉起 serve，
// 测试注入 httptest。SelfVersion 用于版本不匹配换新（升级后旧 daemon 重启）。
type EnsureDeps struct {
	Spawn       func(addr string) error
	SelfVersion string
	Wait        time.Duration // 拉起后等待 healthz 的预算；0 = 默认 2s
}

// healthy 判定 addr 是否健康且版本匹配（SelfVersion 为空则不校验版本）。
func (d EnsureDeps) healthy(addr string) bool {
	h, err := Probe(addr)
	if err != nil || h.Status != "ok" {
		return false
	}
	return d.SelfVersion == "" || h.Version == d.SelfVersion
}

// Ensure 确保 daemon 在位：健康则原样复用 addr；死亡/版本不匹配则 Spawn 拉起
// 并在预算内轮询 healthz。返回可用 addr（任何路径下都与 ResolveAddr 一致，不漂移）。
func (d EnsureDeps) Ensure() (string, error) {
	addr, err := ResolveAddr()
	if err != nil {
		return "", err
	}
	if d.healthy(addr) {
		return addr, nil
	}
	if d.Spawn == nil {
		return "", errors.New("router: daemon 未运行且未提供拉起方式")
	}
	if err := d.Spawn(addr); err != nil {
		return "", fmt.Errorf("router: 拉起 daemon 失败（%s）: %w", addr, err)
	}
	wait := d.Wait
	if wait <= 0 {
		wait = 2 * time.Second
	}
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		if d.healthy(addr) {
			return addr, nil
		}
	}
	return "", fmt.Errorf("router: daemon 拉起后 %.1fs 内未就绪（%s）；可尝试 cc-select router serve --foreground 排障", wait.Seconds(), addr)
}
