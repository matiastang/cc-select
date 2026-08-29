// Package routes 存取 Mode P 的终端路由表（tid → provider）。
//
// 路由表是「会话内热切」的真值来源（specs/001 data-model §2）：
// cc-select use / route switch 写入，路由 daemon 每请求重读。
// 存储：~/.cc-select/routes.json（原子写 0600，与 providers.json 同目录）。
//
// 本包是纯存储层，不校验 provider 是否存在于 providers.json——该校验属
// CLI/Web 调用方职责（分层：routes 不依赖 config，避免环）。
package routes

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// tableVersion 是路由表 schema 版本；结构变更时递增。
const tableVersion = 1

// validTID 限定终端身份格式：ccs- + 32 位小写 hex（128bit 随机）。
var validTID = regexp.MustCompile(`^ccs-[0-9a-f]{32}$`)

// Entry 是单个终端的路由条目。
type Entry struct {
	TID       string    `json:"tid"`
	Provider  string    `json:"provider"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Table 是 routes.json 的完整内容。
type Table struct {
	Version int     `json:"version"`
	Routes  []Entry `json:"routes"`
}

// Path 返回 routes.json 绝对路径：与 providers.json（CC_SELECT_CONFIG）同目录，
// 默认 ~/.cc-select/routes.json。同 prefs.path() 约定，便于测试隔离。
func Path() (string, error) {
	if p := os.Getenv("CC_SELECT_CONFIG"); p != "" {
		return filepath.Join(filepath.Dir(p), "routes.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cc-select", "routes.json"), nil
}

// Load 读取路由表。文件不存在返回空表（Version=当前 schema），不报错。
func Load() (*Table, error) {
	p, err := Path()
	if err != nil {
		return nil, fmt.Errorf("routes: 定位路由表失败: %w", err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &Table{Version: tableVersion}, nil
		}
		return nil, fmt.Errorf("routes: 读取路由表失败: %w", err)
	}
	var tbl Table
	if err := json.Unmarshal(data, &tbl); err != nil {
		return nil, fmt.Errorf("routes: 解析路由表失败（%s）: %w", p, err)
	}
	return &tbl, nil
}

// Save 以原子方式写入路由表：临时文件（0600）+ rename，同 prefs.Save 模式。
func Save(tbl *Table) error {
	p, err := Path()
	if err != nil {
		return fmt.Errorf("routes: 定位路由表失败: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return fmt.Errorf("routes: 创建目录失败: %w", err)
	}
	tbl.Version = tableVersion
	data, err := json.MarshalIndent(tbl, "", "  ")
	if err != nil {
		return fmt.Errorf("routes: 序列化失败: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(p), ".routes-*.json.tmp")
	if err != nil {
		return fmt.Errorf("routes: 创建临时文件失败: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("routes: 写入临时文件失败: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("routes: 设置权限失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("routes: 关闭临时文件失败: %w", err)
	}
	if err := os.Rename(tmpName, p); err != nil {
		cleanup()
		return fmt.Errorf("routes: 替换路由表失败: %w", err)
	}
	return nil
}

// Get 查询 tid 的当前路由条目；无条目返回 ok=false。
func Get(tid string) (Entry, bool) {
	tbl, err := Load()
	if err != nil {
		return Entry{}, false
	}
	for _, e := range tbl.Routes {
		if e.TID == tid {
			return e, true
		}
	}
	return Entry{}, false
}

// withLock 串行化跨进程的读-改-写：对 routes.json 同目录的 .routes.lock
// 加独占文件锁后执行 fn。两个终端并发 `ccs use` / route switch / web prune
// 不再丢失条目（last-writer-wins 竞态的修复）。
func withLock(fn func() error) error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return fmt.Errorf("routes: 创建目录失败: %w", err)
	}
	f, err := os.OpenFile(filepath.Join(filepath.Dir(p), ".routes.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("routes: 打开锁文件失败: %w", err)
	}
	defer f.Close()
	if err := lockFile(f); err != nil {
		return fmt.Errorf("routes: 加锁失败: %w", err)
	}
	defer func() { _ = unlockFile(f) }()
	return fn()
}

// Set 幂等地设置 tid 的路由（存在则覆盖 provider 并刷新 updatedAt）。
// tid 格式非法即拒绝（防脏数据进表）。文件锁保护读-改-写全程。
func Set(tid, provider string) error {
	if !ValidateTID(tid) {
		return fmt.Errorf("routes: 非法终端身份 %q", tid)
	}
	return withLock(func() error {
		tbl, err := Load()
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		for i, e := range tbl.Routes {
			if e.TID == tid {
				tbl.Routes[i].Provider = provider
				tbl.Routes[i].UpdatedAt = now
				return Save(tbl)
			}
		}
		tbl.Routes = append(tbl.Routes, Entry{TID: tid, Provider: provider, UpdatedAt: now})
		return Save(tbl)
	})
}

// List 返回全部路由条目（无序保证）。
func List() ([]Entry, error) {
	tbl, err := Load()
	if err != nil {
		return nil, err
	}
	return tbl.Routes, nil
}

// Prune 删除 updatedAt 早于阈值的条目，返回删除数量。幂等。文件锁保护读-改-写。
func Prune(olderThan time.Duration) (int, error) {
	var pruned int
	err := withLock(func() error {
		tbl, err := Load()
		if err != nil {
			return err
		}
		cutoff := time.Now().UTC().Add(-olderThan)
		kept := tbl.Routes[:0]
		for _, e := range tbl.Routes {
			if e.UpdatedAt.Before(cutoff) {
				pruned++
				continue
			}
			kept = append(kept, e)
		}
		if pruned == 0 {
			return nil
		}
		tbl.Routes = kept
		return Save(tbl)
	})
	return pruned, err
}

// ValidateTID 判断 tid 是否为合法终端身份格式。
func ValidateTID(tid string) bool {
	return validTID.MatchString(tid)
}

// NewTID 生成一个新的终端身份：ccs- + 128bit 随机 hex。
// 随机量使伪 token 不可猜测（伪 token 即 tid 本身，见 specs/001 contracts/cli.md）。
func NewTID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("routes: 生成终端身份失败: %w", err)
	}
	return "ccs-" + hex.EncodeToString(b[:]), nil
}
