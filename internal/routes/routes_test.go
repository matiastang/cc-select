package routes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// setTempRoutes 让 routes.json 落入临时目录（复用 CC_SELECT_CONFIG 约定：取其同级）。
func setTempRoutes(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("CC_SELECT_CONFIG", filepath.Join(dir, "providers.json"))
}

func TestLoad_MissingReturnsEmpty(t *testing.T) {
	setTempRoutes(t)
	tbl, err := Load()
	if err != nil {
		t.Fatalf("缺文件应返回空表 + nil: %v", err)
	}
	if len(tbl.Routes) != 0 {
		t.Errorf("缺文件应无路由条目，got %d", len(tbl.Routes))
	}
}

func TestSetGet_RoundTrip(t *testing.T) {
	setTempRoutes(t)
	tid := NewTIDOrFatal(t)
	if err := Set(tid, "minimax"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	e, ok := Get(tid)
	if !ok {
		t.Fatal("Set 后 Get 应命中")
	}
	if e.Provider != "minimax" {
		t.Errorf("provider want minimax got %q", e.Provider)
	}
	if e.TID != tid {
		t.Errorf("tid want %q got %q", tid, e.TID)
	}
	if e.UpdatedAt.IsZero() {
		t.Error("updatedAt 应非零")
	}
}

func TestSet_OverwriteSameTID(t *testing.T) {
	setTempRoutes(t)
	tid := NewTIDOrFatal(t)
	if err := Set(tid, "glm"); err != nil {
		t.Fatalf("Set glm: %v", err)
	}
	first, _ := Get(tid)
	time.Sleep(1100 * time.Millisecond) // 跨秒，确保 updatedAt 推进
	if err := Set(tid, "minimax"); err != nil {
		t.Fatalf("Set minimax: %v", err)
	}
	e, ok := Get(tid)
	if !ok || e.Provider != "minimax" {
		t.Fatalf("同 tid 二次 Set 应覆盖：got %+v ok=%v", e, ok)
	}
	if !e.UpdatedAt.After(first.UpdatedAt) {
		t.Errorf("updatedAt 应刷新：first=%v now=%v", first.UpdatedAt, e.UpdatedAt)
	}
	entries, _ := List()
	if len(entries) != 1 {
		t.Errorf("同 tid 覆盖不应新增条目，got %d", len(entries))
	}
}

func TestSet_InvalidTIDRejected(t *testing.T) {
	setTempRoutes(t)
	for _, bad := range []string{
		"", "minimax", "ccs-short", "ccs-" + strings.Repeat("g", 32),
		"CCS-" + strings.Repeat("a", 32), "ccs-" + strings.Repeat("a", 31),
	} {
		if err := Set(bad, "glm"); err == nil {
			t.Errorf("非法 tid %q 应被拒绝", bad)
		}
	}
}

func TestSave_FilePerms(t *testing.T) {
	setTempRoutes(t)
	tid := NewTIDOrFatal(t)
	if err := Set(tid, "glm"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	p, _ := Path()
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("routes.json 权限 want 0600 got %o", fi.Mode().Perm())
	}
}

func TestPrune_OlderThan(t *testing.T) {
	setTempRoutes(t)
	oldTID, newTID := NewTIDOrFatal(t), NewTIDOrFatal(t)
	if err := Set(oldTID, "glm"); err != nil {
		t.Fatalf("Set old: %v", err)
	}
	// 把 old 条目时间拨回 8 天前。
	tbl, _ := Load()
	for i := range tbl.Routes {
		if tbl.Routes[i].TID == oldTID {
			tbl.Routes[i].UpdatedAt = time.Now().UTC().Add(-8 * 24 * time.Hour)
		}
	}
	if err := Save(tbl); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := Set(newTID, "minimax"); err != nil {
		t.Fatalf("Set new: %v", err)
	}
	n, err := Prune(7 * 24 * time.Hour)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if n != 1 {
		t.Errorf("prune 数量 want 1 got %d", n)
	}
	if _, ok := Get(oldTID); ok {
		t.Error("过期条目应被删除")
	}
	if _, ok := Get(newTID); !ok {
		t.Error("新条目应保留")
	}
	// 幂等：再跑一次无过期条目。
	n, err = Prune(7 * 24 * time.Hour)
	if err != nil || n != 0 {
		t.Errorf("二次 prune 应为 0：got %d err=%v", n, err)
	}
}

func TestNewTID_FormatAndUnique(t *testing.T) {
	seen := map[string]bool{}
	for range 100 {
		tid, err := NewTID()
		if err != nil {
			t.Fatalf("NewTID: %v", err)
		}
		if !ValidateTID(tid) {
			t.Fatalf("NewTID 产物未过校验：%q", tid)
		}
		if seen[tid] {
			t.Fatalf("NewTID 重复：%q", tid)
		}
		seen[tid] = true
	}
}

func TestValidateTID(t *testing.T) {
	if !ValidateTID("ccs-" + strings.Repeat("ab", 16)) {
		t.Error("合法 tid 应通过")
	}
	for _, bad := range []string{"", "ccs-", "x-" + strings.Repeat("a", 32), "ccs-" + strings.Repeat("A", 32)} {
		if ValidateTID(bad) {
			t.Errorf("%q 应非法", bad)
		}
	}
}

// NewTIDOrFatal 是测试辅助：生成一个合法 tid，失败即终止。
func NewTIDOrFatal(t *testing.T) string {
	t.Helper()
	tid, err := NewTID()
	if err != nil {
		t.Fatalf("NewTID: %v", err)
	}
	return tid
}
