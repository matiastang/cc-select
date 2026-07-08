package web

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cc-select/cc-select/internal/profile"
)

// newTestServer 用临时配置建一个 API-only 测试服务，预置一个 glm provider（含明文 token 的 profile）。
func newTestServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	dir := t.TempDir()
	cfg := filepath.Join(dir, "providers.json")
	os.Setenv("CC_SELECT_CONFIG", cfg)
	// providers.json 只存元信息；env 真值在 profile settings.json。
	_ = os.WriteFile(cfg, []byte(`{"providers":{"glm":{"id":"glm","name":"GLM"}}}`), 0o600)
	// 建一个含明文 token 的 profile（验证 GET 不泄露）。
	profile.Ensure("glm", map[string]string{
		"ANTHROPIC_BASE_URL":  "https://glm",
		"ANTHROPIC_AUTH_TOKEN": "tok-secret-123",
	})

	h := newAPIHandler()
	srv := httptest.NewServer(h.routes())
	return srv, cfg
}

func TestListProviders_HidesPlaintextKey(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	resp, err := http.Get(srv.URL + "/api/v1/providers")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)

	body := mustJSON(t, out)
	// 敏感 token 不应回传前端（toDTO 脱敏）。
	if strings.Contains(body, "tok-secret-123") {
		t.Errorf("GET 不应返回明文 token，body: %s", body)
	}
	got, _ := out["providers"].(map[string]any)
	glm, _ := got["glm"].(map[string]any)
	if glm["hasKey"] != true {
		t.Errorf("hasKey 应为 true（配了 token），got %v", glm["hasKey"])
	}
	// 非敏感 env 应明文返回。
	if glmEnv, _ := glm["env"].(map[string]any); glmEnv["ANTHROPIC_BASE_URL"] != "https://glm" {
		t.Errorf("非敏感 env 应明文返回，got %v", glm["env"])
	}
}

func TestCreateAndDeleteProvider(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	// POST 新建：完整 settings.json（含 env）。
	body := `{"id":"deepseek","name":"DS","settings":{"env":{"ANTHROPIC_BASE_URL":"https://ds","ANTHROPIC_MODEL":"deepseek"}}}`
	resp, err := http.Post(srv.URL+"/api/v1/providers", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST want 201 got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// DELETE。
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/providers/deepseek", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE want 204 got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// TestCreate_EmptyNameFallsBackToID 验证添加时展示名留空，应实际写入 ID。
func TestCreate_EmptyNameFallsBackToID(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	body := `{"id":"noname","name":"","settings":{"env":{"ANTHROPIC_BASE_URL":"https://no"}}}`
	resp, err := http.Post(srv.URL+"/api/v1/providers", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST want 201 got %d", resp.StatusCode)
	}
	var detail providerDetailDTO
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		t.Fatal(err)
	}
	if detail.Name != "noname" {
		t.Errorf("空 name 应回退到 ID，want noname got %q", detail.Name)
	}

	// 再次 GET 确认落盘后仍返回 ID。
	resp2, err := http.Get(srv.URL + "/api/v1/providers/noname")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	var detail2 providerDetailDTO
	if err := json.NewDecoder(resp2.Body).Decode(&detail2); err != nil {
		t.Fatal(err)
	}
	if detail2.Name != "noname" {
		t.Errorf("GET 空 name provider 应回退到 ID，want noname got %q", detail2.Name)
	}
}

// TestUpdate_EmptyNameFallsBackToID 验证编辑时展示名清空，应实际写入 ID。
func TestUpdate_EmptyNameFallsBackToID(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	// 先建一个带展示名的 provider。
	http.Post(srv.URL+"/api/v1/providers", "application/json",
		strings.NewReader(`{"id":"x","name":"X","settings":{"env":{"ANTHROPIC_BASE_URL":"https://x"}}}`))

	// PUT 把展示名清空。
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/v1/providers/x",
		bytes.NewReader([]byte(`{"name":"","settings":{"env":{"ANTHROPIC_BASE_URL":"https://x2"}}}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT want 200 got %d", resp.StatusCode)
	}
	var detail providerDetailDTO
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		t.Fatal(err)
	}
	if detail.Name != "x" {
		t.Errorf("PUT 空 name 应回退到 ID，want x got %q", detail.Name)
	}
}

// TestGetDetail_EmptyNameFallsBackToID 验证对已有的空 name provider（如历史数据/手改文件），
// GET /providers/{id} 也应返回 ID 作为展示名。
func TestGetDetail_EmptyNameFallsBackToID(t *testing.T) {
	srv, cfg := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	// 模拟一个 name 为空的遗留 provider。
	_ = os.WriteFile(cfg, []byte(`{"providers":{"legacy":{"id":"legacy","name":""}}}`), 0o600)

	resp, err := http.Get(srv.URL + "/api/v1/providers/legacy")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var detail providerDetailDTO
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		t.Fatal(err)
	}
	if detail.Name != "legacy" {
		t.Errorf("遗留空 name provider GET 应回退到 ID，want legacy got %q", detail.Name)
	}
}

func TestCannotDeleteOfficialProvider(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/providers/claude-official", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("删官方 provider 应 400，got %d", resp.StatusCode)
	}
}

func TestPutProvider(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	// 先建。
	http.Post(srv.URL+"/api/v1/providers", "application/json",
		strings.NewReader(`{"id":"x","name":"X","settings":{"env":{"ANTHROPIC_BASE_URL":"https://x"}}}`))
	// PUT 更新（整体覆盖 settings）。
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/v1/providers/x",
		bytes.NewReader([]byte(`{"name":"X2","settings":{"env":{"ANTHROPIC_BASE_URL":"https://x2"}}}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT want 200 got %d", resp.StatusCode)
	}
	// profile settings.json 应是新值。
	env, _ := profile.ReadEnv("x")
	if env["ANTHROPIC_BASE_URL"] != "https://x2" {
		t.Errorf("PUT 后应是新值，got %v", env)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, _ := json.Marshal(v)
	return string(b)
}

// TestCreate_NoPlaintextInProvidersJSON 验证敏感值只进 profile settings.json，
// 不落到全局共享的 providers.json（元信息只存 id/name）。
func TestCreate_NoPlaintextInProvidersJSON(t *testing.T) {
	srv, cfg := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	body := `{"id":"imp","name":"Imp","settings":{"env":{` +
		`"ANTHROPIC_AUTH_TOKEN":"tok-secret-123",` +
		`"ANTHROPIC_BASE_URL":"https://imp"}}}`
	resp, err := http.Post(srv.URL+"/api/v1/providers", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST want 201 got %d", resp.StatusCode)
	}

	// providers.json（全局元信息）不应含任何 env 值（含 token、base url）。
	raw, _ := os.ReadFile(cfg)
	if strings.Contains(string(raw), "tok-secret-123") || strings.Contains(string(raw), "https://imp") {
		t.Errorf("providers.json 不应含 env 值：%s", string(raw))
	}
	// profile settings.json 应含明文 env（含敏感 token）——claude 靠它工作。
	env, err := profile.ReadEnv("imp")
	if err != nil {
		t.Fatal(err)
	}
	if env["ANTHROPIC_AUTH_TOKEN"] != "tok-secret-123" {
		t.Errorf("profile 应含明文 token，got %v", env)
	}
	if env["ANTHROPIC_BASE_URL"] != "https://imp" {
		t.Errorf("profile 应含 base url，got %v", env)
	}
}

func TestIsSensitiveVar(t *testing.T) {
	cases := map[string]bool{
		"ANTHROPIC_API_KEY":     true,
		"ANTHROPIC_AUTH_TOKEN":  true,
		"SECRET_STUFF":          true,
		"PASSWORD":              true,
		"ANTHROPIC_BASE_URL":    false,
		"ANTHROPIC_MODEL":       false,
		"CLAUDE_CODE_ENTRYPOINT": false,
	}
	for name, want := range cases {
		if got := isSensitiveVar(name); got != want {
			t.Errorf("isSensitiveVar(%q) = %v, want %v", name, got, want)
		}
	}
}

// TestCreate_FullSettingsPersist 验证 settings.json 可携带 env 之外的任意字段
// （permissions、model 等），且原样写入磁盘。
func TestCreate_FullSettingsPersist(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	body := `{"id":"full","name":"Full","isolationMode":"full","settings":{` +
		`"env":{"ANTHROPIC_BASE_URL":"https://full"},` +
		`"model":"opusplan",` +
		`"permissions":{"allow":["Bash(ls:*)"]}}}`
	resp, err := http.Post(srv.URL+"/api/v1/providers", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST want 201 got %d", resp.StatusCode)
	}

	raw, err := profile.ReadRaw("full")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("settings.json 非法 JSON: %v", err)
	}
	if got["model"] != "opusplan" {
		t.Errorf("非 env 字段 model 应持久化，got %v", got["model"])
	}
	if _, ok := got["permissions"]; !ok {
		t.Errorf("非 env 字段 permissions 应持久化，got %v", got)
	}
}

// TestGet_ReflectsManualFileEdit 验证"手改 settings.json 后，GET 反映真实内容"。
// 这是本次改造的核心需求之一。
func TestGet_ReflectsManualFileEdit(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	// 直接（绕过 web）覆盖 glm 的 settings.json。
	path, err := profile.Path("glm")
	if err != nil {
		t.Fatal(err)
	}
	manual := `{"env":{"ANTHROPIC_BASE_URL":"https://manually-edited"},"model":"sonnet"}`
	if err := os.WriteFile(path, []byte(manual), 0o600); err != nil {
		t.Fatal(err)
	}

	resp, err := http.Get(srv.URL + "/api/v1/providers/glm")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out struct {
		Settings map[string]any `json:"settings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Settings["model"] != "sonnet" {
		t.Errorf("GET 应反映手改后的 model，got %v", out.Settings["model"])
	}
	env, _ := out.Settings["env"].(map[string]any)
	if env["ANTHROPIC_BASE_URL"] != "https://manually-edited" {
		t.Errorf("GET 应反映手改后的 base url，got %v", env)
	}
}

// TestCreate_RejectsNonObjectSettings 验证 settings 必须是 JSON 对象（非数组/标量）。
func TestCreate_RejectsNonObjectSettings(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	for _, body := range []string{
		`{"id":"bad1","settings":[1,2,3]}`,
		`{"id":"bad2","settings":"a string"}`,
		`{"id":"bad3"}`, // 缺 settings
	} {
		resp, err := http.Post(srv.URL+"/api/v1/providers", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("body %s 应 400，got %d", body, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

// TestGet_MissingSettingsFile 验证 settings.json 文件缺失时 GET 退化为 {}。
func TestGet_MissingSettingsFile(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	// 删掉 glm 的 settings.json，模拟文件缺失。
	path, _ := profile.Path("glm")
	os.Remove(path)

	resp, err := http.Get(srv.URL + "/api/v1/providers/glm")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"settings":{}`) {
		t.Errorf("文件缺失时 settings 应退化为 {}，got %s", body)
	}
}

// TestUpdate_RejectsOfficial 验证官方 provider 不可改 settings：PUT 应 400，
// 避免 EnsureRaw 静默丢弃用户输入造成"看似成功实则丢失"。
func TestUpdate_RejectsOfficial(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/v1/providers/claude-official",
		bytes.NewReader([]byte(`{"name":"X","settings":{"env":{"ANTHROPIC_BASE_URL":"https://x"}}}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("PUT 官方 provider 应 400，got %d", resp.StatusCode)
	}
}

// ---- 隔离模式端点 ----

func TestModeEndpoint_GetDefault(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	resp, err := http.Get(srv.URL + "/api/v1/mode")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	if out["isolationMode"] != "settings-only" {
		t.Errorf("默认应 settings-only, got %v", out["isolationMode"])
	}
}

func TestModeEndpoint_Put(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/v1/mode",
		strings.NewReader(`{"isolationMode":"full"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT want 200 got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 再 GET 应为 full（已落盘 prefs.json）。
	resp2, err := http.Get(srv.URL + "/api/v1/mode")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	var out map[string]any
	json.NewDecoder(resp2.Body).Decode(&out)
	if out["isolationMode"] != "full" {
		t.Errorf("PUT 后应 full, got %v", out["isolationMode"])
	}
}

func TestModeEndpoint_RejectsInvalid(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/v1/mode",
		strings.NewReader(`{"isolationMode":"bogus"}`))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("非法值应 400, got %d", resp.StatusCode)
	}
}

func TestLanguageEndpoint_GetDefault(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	resp, err := http.Get(srv.URL + "/api/v1/language")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	if out["language"] != "" {
		t.Errorf("默认应空 language, got %v", out["language"])
	}
}

func TestLanguageEndpoint_Put(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/v1/language",
		strings.NewReader(`{"language":"zh"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT want 200 got %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp2, err := http.Get(srv.URL + "/api/v1/language")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	var out map[string]any
	json.NewDecoder(resp2.Body).Decode(&out)
	if out["language"] != "zh" {
		t.Errorf("PUT 后应 zh, got %v", out["language"])
	}
}

func TestLanguageEndpoint_RejectsInvalid(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/v1/language",
		strings.NewReader(`{"language":"fr"}`))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("非法值应 400, got %d", resp.StatusCode)
	}
}

// ---- shell 集成端点 ----

// setTestShellEnv 让 DetectStatus/Install 在临时 home 上确定地工作（zsh）。
func setTestShellEnv(t *testing.T) string {
	t.Helper()
	t.Setenv("CC_SELECT_SHELL", "zsh")
	home := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	} else {
		t.Setenv("HOME", home)
	}
	return home
}

func TestShellIntegration_GetStatus(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")
	setTestShellEnv(t)

	resp, err := http.Get(srv.URL + "/api/v1/shell-integration")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	if out["supported"] != true {
		t.Errorf("zsh 应 supported, got %v", out["supported"])
	}
	if out["installed"] == true {
		t.Errorf("临时 home 应未安装, got installed=%v", out["installed"])
	}
	if out["canAutoInstall"] != true {
		t.Errorf("zsh 应可自动安装, got %v", out["canAutoInstall"])
	}
}

func TestShellIntegration_InstallAppended(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")
	home := setTestShellEnv(t)

	resp, err := http.Post(srv.URL+"/api/v1/shell-integration/install",
		"application/json", strings.NewReader(`{"shell":"zsh"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	if out["action"] != "appended" {
		t.Errorf("首次应 appended, got %v", out["action"])
	}
	data, err := os.ReadFile(filepath.Join(home, ".zshrc"))
	if err != nil {
		t.Fatalf("zshrc 应存在: %v", err)
	}
	if !strings.Contains(string(data), "cc-select shell integration") {
		t.Errorf("zshrc 应含 marker: %s", data)
	}
}

func TestShellIntegration_MethodNotAllowed(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/shell-integration", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("DELETE 应 405, got %d", resp.StatusCode)
	}
}
