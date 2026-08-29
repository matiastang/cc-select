package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cc-select/cc-select/internal/config"
	"github.com/cc-select/cc-select/internal/profile"
	"github.com/cc-select/cc-select/internal/routes"
	"github.com/cc-select/cc-select/internal/secrets"
)

// writeProviders 写一份含 glm/minimax 的临时 providers.json。
func writeProviders(t *testing.T) {
	t.Helper()
	data := `{"providers":{"glm":{"id":"glm","name":"GLM"},"minimax":{"id":"minimax","name":"MiniMax"},"claude-official":{"id":"claude-official"}}}`
	p := os.Getenv("CC_SELECT_CONFIG")
	if p == "" {
		t.Fatal("先 setTempCfg")
	}
	if err := os.WriteFile(p, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestRouteSwitch_SwitchesTerminalRoute(t *testing.T) {
	setTempCfg(t)
	writeProviders(t)
	tid, _ := routes.NewTID()
	if err := routes.Set(tid, "glm"); err != nil {
		t.Fatal(err)
	}
	t.Setenv(config.TerminalIDVar, tid)

	out, _, err := execRoot(t, "", "route", "switch", "minimax")
	if err != nil {
		t.Fatalf("switch: %v（out=%s）", err, out)
	}
	if !strings.Contains(out, "glm") || !strings.Contains(out, "minimax") {
		t.Errorf("输出应含 <old> → <new>: %q", out)
	}
	e, ok := routes.Get(tid)
	if !ok || e.Provider != "minimax" {
		t.Errorf("路由表应更新为 minimax: %+v ok=%v", e, ok)
	}
}

func TestRouteSwitch_TIDFlagOverridesEnv(t *testing.T) {
	setTempCfg(t)
	writeProviders(t)
	tidFlag, _ := routes.NewTID()
	t.Setenv(config.TerminalIDVar, "ccs-11111111111111111111111111111111") // env 指向另一终端

	if _, _, err := execRoot(t, "", "route", "switch", "--tid", tidFlag, "glm"); err != nil {
		t.Fatalf("switch --tid: %v", err)
	}
	if e, _ := routes.Get(tidFlag); e.Provider != "glm" {
		t.Errorf("--tid 应优先生效: %+v", e)
	}
	if e, _ := routes.Get("ccs-11111111111111111111111111111111"); e.Provider == "glm" {
		t.Error("env tid 不应被改动")
	}
}

func TestRouteSwitch_Rejections(t *testing.T) {
	setTempCfg(t)
	writeProviders(t)
	tid, _ := routes.NewTID()
	t.Setenv(config.TerminalIDVar, tid)

	// 官方 provider 不参与 Mode P（v1，研究 D2）。
	if _, _, err := execRoot(t, "", "route", "switch", config.OfficialProviderID); err == nil {
		t.Error("官方 provider 应被拒绝")
	}
	// 未知 provider。
	if _, _, err := execRoot(t, "", "route", "switch", "nosuch"); err == nil {
		t.Error("未知 provider 应报错")
	}
	// 失败不落盘：表里不应有任何条目。
	if entries, _ := routes.List(); len(entries) != 0 {
		t.Errorf("失败不得落盘: %+v", entries)
	}
	// 无 tid（env 与 --tid 均缺）。
	t.Setenv(config.TerminalIDVar, "")
	if _, _, err := execRoot(t, "", "route", "switch", "glm"); err == nil {
		t.Error("缺 tid 应报错")
	}
	// 非法 tid 格式。
	if _, _, err := execRoot(t, "", "route", "switch", "--tid", "garbage", "glm"); err == nil {
		t.Error("非法 tid 应报错")
	}
}

func TestRouteList_ShowsShortTID(t *testing.T) {
	setTempCfg(t)
	a, _ := routes.NewTID()
	b, _ := routes.NewTID()
	_ = routes.Set(a, "glm")
	_ = routes.Set(b, "minimax")

	out, _, err := execRoot(t, "", "route", "list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, tid := range []string{a, b} {
		if !strings.Contains(out, tid[:12]) {
			t.Errorf("list 应显示 tid 短码 %s: %q", tid[:12], out)
		}
		if strings.Contains(out, tid) {
			t.Errorf("list 不应显示完整 tid: %q", out)
		}
	}
}

func TestRouteStatus_IncludesRouteAndRouter(t *testing.T) {
	setTempCfg(t)
	tid, _ := routes.NewTID()
	_ = routes.Set(tid, "glm")
	t.Setenv(config.TerminalIDVar, tid)

	out, _, err := execRoot(t, "", "route", "status")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(out, tid[:12]) || !strings.Contains(out, "glm") {
		t.Errorf("status 应含 tid 短码与 provider: %q", out)
	}
	if !strings.Contains(out, "router=") {
		t.Errorf("status 应含 router 存活状态: %q", out)
	}
}

func TestRoutePrune_RemovesStale(t *testing.T) {
	setTempCfg(t)
	stale, fresh, _ := twoTIDs(t)
	_ = routes.Set(stale, "glm")
	_ = routes.Set(fresh, "minimax")
	// 把 stale 拨回 8 天前。
	tbl, _ := routes.Load()
	for i := range tbl.Routes {
		if tbl.Routes[i].TID == stale {
			tbl.Routes[i].UpdatedAt = time.Now().UTC().Add(-8 * 24 * time.Hour)
		}
	}
	_ = routes.Save(tbl)

	out, _, err := execRoot(t, "", "route", "prune")
	if err != nil {
		t.Fatalf("prune: %v（out=%s）", err, out)
	}
	if _, ok := routes.Get(stale); ok {
		t.Error("过期条目应被清理")
	}
	if _, ok := routes.Get(fresh); !ok {
		t.Error("新条目应保留")
	}
}

func twoTIDs(t *testing.T) (a, b string, err error) {
	t.Helper()
	a, err = routes.NewTID()
	if err != nil {
		return
	}
	b, err = routes.NewTID()
	return
}

var _ = filepath.Join // 保留 import 以防后续用

func TestCurrent_RouteTableWinsOverActive(t *testing.T) {
	setTempCfg(t)
	writeProviders(t)
	tid, _ := routes.NewTID()
	_ = routes.Set(tid, "minimax")
	t.Setenv(config.TerminalIDVar, tid)
	t.Setenv(config.ActiveVar, "glm") // 会话内切换后 shell 侧滞后（FR-008 场景）

	out, _, err := execRoot(t, "", "current")
	if err != nil {
		t.Fatalf("current: %v", err)
	}
	if !strings.Contains(out, "minimax") || strings.Contains(out, "glm") {
		t.Errorf("Mode P 下应以路由表为真值: %q", out)
	}
}

func TestCurrent_NoRouteFallsBackToActive(t *testing.T) {
	setTempCfg(t)
	writeProviders(t)
	tid, _ := routes.NewTID() // 有 TID 但路由表无条目
	t.Setenv(config.TerminalIDVar, tid)
	t.Setenv(config.ActiveVar, "glm")

	out, _, err := execRoot(t, "", "current")
	if err != nil {
		t.Fatalf("current: %v", err)
	}
	if !strings.Contains(out, "glm") {
		t.Errorf("无路由条目应回退读 CC_SELECT_ACTIVE: %q", out)
	}
}

func TestMode_SetProxyDoesNotAutoMigrate(t *testing.T) {
	// 产品决策（2026-08-29）：keychain 迁移默认关闭——启用 proxy 模式不得
	// 擅自把用户设置的明文 key 改成占位符。GUI 显式开启钥匙串开关后才迁移。
	setTempCfg(t)
	os.WriteFile(os.Getenv("CC_SELECT_CONFIG"),
		[]byte(`{"providers":{"glm":{"id":"glm","env":{"ANTHROPIC_AUTH_TOKEN":"sk-plain","ANTHROPIC_MODEL":"glm-5.1"}}}}`), 0o600)
	var stubCalls int
	orig := migrateSecretsFn
	migrateSecretsFn = func(cfg *config.Config) (int, []string) {
		stubCalls++
		return secrets.MigrateAll(secrets.NewFake(), cfg)
	}
	t.Cleanup(func() { migrateSecretsFn = orig })

	if _, _, err := execRoot(t, "", "mode", "proxy"); err != nil {
		t.Fatalf("mode proxy: %v", err)
	}
	if stubCalls != 0 {
		t.Fatalf("不应自动迁移（默认关闭），got %d 次", stubCalls)
	}
	data, _ := os.ReadFile(os.Getenv("CC_SELECT_CONFIG"))
	if !strings.Contains(string(data), "sk-plain") {
		t.Errorf("用户设置的明文必须保持原样: %s", data)
	}
}

func TestUse_ProxyModeDoesNotAutoMigrate(t *testing.T) {
	setTempCfg(t)
	os.WriteFile(os.Getenv("CC_SELECT_CONFIG"),
		[]byte(`{"providers":{"glm":{"id":"glm","env":{"ANTHROPIC_AUTH_TOKEN":"sk-plain"}}}}`), 0o600)
	stubEnsure(t, "127.0.0.1:48270", nil)

	if _, _, err := execRoot(t, "", "use", "glm", "--mode", "proxy", "--shell", "zsh"); err != nil {
		t.Fatalf("use --mode proxy: %v", err)
	}
	data, _ := os.ReadFile(os.Getenv("CC_SELECT_CONFIG"))
	if !strings.Contains(string(data), "sk-plain") {
		t.Errorf("use（proxy）不得擅自改写用户明文（keychain 默认关闭）: %s", data)
	}
}

func TestRoutePrune_RejectsNonPositiveDuration(t *testing.T) {
	setTempCfg(t)
	if _, _, err := execRoot(t, "", "route", "prune", "--older-than", "-1h"); err == nil {
		t.Error("负时长应被拒绝（会把 cutoff 推到未来、清掉活跃路由）")
	}
}

func TestRouteList_ToleratesShortCorruptTID(t *testing.T) {
	setTempCfg(t)
	// 直写一条畸形 tid（Save 不校验，模拟手改/损坏）。
	tbl := &routes.Table{Routes: []routes.Entry{{TID: "short", Provider: "glm"}}}
	_ = routes.Save(tbl)
	if _, _, err := execRoot(t, "", "route", "list"); err != nil {
		t.Errorf("畸形 tid 不应 panic: %v", err)
	}
}

func TestEdit_GlobalProxyModeSucceeds(t *testing.T) {
	setTempCfg(t)
	// 完整 env（避免 edit 走交互式 key 提示）。
	os.WriteFile(os.Getenv("CC_SELECT_CONFIG"),
		[]byte(`{"providers":{"glm":{"id":"glm","name":"GLM","env":{"ANTHROPIC_BASE_URL":"https://open.bigmodel.cn/api/anthropic","ANTHROPIC_AUTH_TOKEN":"sk-real"}}}}`), 0o600)
	os.WriteFile(filepath.Join(filepath.Dir(os.Getenv("CC_SELECT_CONFIG")), "prefs.json"),
		[]byte(`{"isolationMode":"proxy"}`), 0o600)

	if _, _, err := execRoot(t, "", "edit", "glm", "--model", "glm-5.3"); err != nil {
		t.Fatalf("全局 proxy 下 edit 不应被守卫拦截: %v", err)
	}
	data, _ := os.ReadFile(os.Getenv("CC_SELECT_CONFIG"))
	if !strings.Contains(string(data), "glm-5.3") {
		t.Errorf("edit 应写入 providers.json 真值: %s", data)
	}
	env, _ := profile.ReadEnv("glm")
	if env["ANTHROPIC_BASE_URL"] != "http://127.0.0.1:48270" {
		t.Errorf("profile 应为代理派生产物: %+v", env)
	}
}

// T013：热切刷新选择器（specs/002 US3 / research D3）——route switch 成功后经继承的
// $CLAUDE_CONFIG_DIR 定位「发射 profile」的 settings.json 并刷新为新 provider 清单。

func writeProvidersWithModels(t *testing.T) {
	t.Helper()
	data := `{"providers":{
		"glm":{"id":"glm","env":{"ANTHROPIC_MODEL":"glm-5.3[1m]","ANTHROPIC_DEFAULT_HAIKU_MODEL":"glm-5.3-Flash"}},
		"minimax":{"id":"minimax","env":{"ANTHROPIC_MODEL":"mm-m2.7"}},
		"bare":{"id":"bare"}}}`
	if err := os.WriteFile(os.Getenv("CC_SELECT_CONFIG"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestRouteSwitch_RefreshesLaunchProfileSettings(t *testing.T) {
	setTempCfg(t)
	writeProvidersWithModels(t)
	tid, _ := routes.NewTID()
	if err := routes.Set(tid, "glm"); err != nil {
		t.Fatal(err)
	}
	t.Setenv(config.TerminalIDVar, tid)

	// 发射 profile（模拟热切前 glm 的注入态）。
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
	settings := filepath.Join(dir, "settings.json")
	os.WriteFile(settings, []byte(`{"env":{"ANTHROPIC_BASE_URL":"http://127.0.0.1:48270"},"model":"glm-5.3[1m]",
		"permissions":{"allow":["foo"]},"modelPicker":{"replaceBuiltInOptions":true,"options":[{"model":"glm-5.3[1m]"}]}}`), 0o600)

	if _, _, err := execRoot(t, "", "route", "switch", "minimax"); err != nil {
		t.Fatalf("switch: %v", err)
	}

	m := map[string]any{}
	b, _ := os.ReadFile(settings)
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("刷新后应为合法 JSON: %v", err)
	}
	// modelPicker/model 变为新 provider；env/permissions 保留。
	mp, _ := m["modelPicker"].(map[string]any)
	opts, _ := mp["options"].([]any)
	if len(opts) != 1 || opts[0].(map[string]any)["model"] != "mm-m2.7" {
		t.Fatalf("modelPicker 应为 minimax 清单: %v", mp)
	}
	if m["model"] != "mm-m2.7" {
		t.Fatalf("model 应为 mm-m2.7: %v", m["model"])
	}
	if _, ok := m["permissions"].(map[string]any); !ok {
		t.Fatalf("permissions 应保留: %v", m)
	}
	env, _ := m["env"].(map[string]any)
	if env["ANTHROPIC_BASE_URL"] != "http://127.0.0.1:48270" {
		t.Fatalf("env 不得被触碰: %v", env)
	}
}

func TestRouteSwitch_NoConfigDirSkipsRefresh(t *testing.T) {
	setTempCfg(t)
	writeProvidersWithModels(t)
	tid, _ := routes.NewTID()
	t.Setenv(config.TerminalIDVar, tid)
	t.Setenv("CLAUDE_CONFIG_DIR", "") // 显式清空

	if _, _, err := execRoot(t, "", "route", "switch", "glm"); err != nil {
		t.Fatalf("无 CLAUDE_CONFIG_DIR 时切换仍应成功: %v", err)
	}
}

func TestRouteSwitch_CorruptSettingsSwitchSucceedsWithWarning(t *testing.T) {
	setTempCfg(t)
	writeProvidersWithModels(t)
	tid, _ := routes.NewTID()
	t.Setenv(config.TerminalIDVar, tid)

	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
	os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{broken`), 0o600)

	_, stderr, err := execRoot(t, "", "route", "switch", "minimax")
	if err != nil {
		t.Fatalf("settings 损坏时路由切换仍应成功: %v", err)
	}
	if !strings.Contains(stderr, "modelPicker") && !strings.Contains(stderr, "settings") {
		t.Errorf("应输出刷新失败告警: %q", stderr)
	}
	e, _ := routes.Get(tid)
	if e.Provider != "minimax" {
		t.Fatalf("路由表真值不受刷新失败影响: %+v", e)
	}
}
