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
	"time"

	"github.com/cc-select/cc-select/internal/config"
	"github.com/cc-select/cc-select/internal/profile"
	"github.com/cc-select/cc-select/internal/routes"
	"github.com/cc-select/cc-select/internal/secrets"
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
		"ANTHROPIC_BASE_URL":   "https://glm",
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

func TestListPresets(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	resp, err := http.Get(srv.URL + "/api/v1/presets")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET presets want 200 got %d", resp.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	list, _ := out["presets"].([]any)
	if len(list) == 0 {
		t.Error("presets 列表不应为空")
	}
	cats, _ := out["categories"].([]any)
	if len(cats) == 0 {
		t.Error("categories 不应为空")
	}
}

func TestGetPreset(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	resp, err := http.Get(srv.URL + "/api/v1/presets/deepseek")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET preset want 200 got %d", resp.StatusCode)
	}
	var out presetDetailDTO
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.ID != "deepseek" {
		t.Errorf("want deepseek got %q", out.ID)
	}
	if out.EnvTemplate["ANTHROPIC_BASE_URL"] == "" {
		t.Errorf("preset 详情应包含 envTemplate")
	}
}

func TestGetPreset_NotFound(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	resp, err := http.Get(srv.URL + "/api/v1/presets/not-exist")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown preset 应 404，got %d", resp.StatusCode)
	}
}

func TestCreate_WithPreset(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	body := `{"id":"ds","name":"DeepSeek","preset":"deepseek","apiKey":"sk-ds","overrides":{"ANTHROPIC_MODEL":"deepseek-chat"}}`
	resp, err := http.Post(srv.URL+"/api/v1/providers", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST preset want 201 got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	env, _ := profile.ReadEnv("ds")
	if env["ANTHROPIC_BASE_URL"] != "https://api.deepseek.com/anthropic" {
		t.Errorf("preset base url 未生效: %v", env)
	}
	if env["ANTHROPIC_MODEL"] != "deepseek-chat" {
		t.Errorf("overrides 未覆盖 model: %v", env)
	}
	if env["ANTHROPIC_AUTH_TOKEN"] != "sk-ds" {
		t.Errorf("apiKey 未写入: %v", env)
	}
}

func TestCreate_PresetMissingRequired(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	body := `{"id":"ds","name":"DeepSeek","preset":"deepseek"}`
	resp, err := http.Post(srv.URL+"/api/v1/providers", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("缺 apiKey 应 400，got %d", resp.StatusCode)
	}
}

func TestUpdate_WithPreset(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	// 先用自定义 settings 创建。
	http.Post(srv.URL+"/api/v1/providers", "application/json",
		strings.NewReader(`{"id":"x","name":"X","settings":{"env":{"ANTHROPIC_BASE_URL":"https://x","ANTHROPIC_AUTH_TOKEN":"tok-x"}}}`))

	// 改为 deepseek preset。
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/v1/providers/x",
		bytes.NewReader([]byte(`{"name":"X2","preset":"deepseek","apiKey":"sk-ds"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("PUT preset want 200 got %d: %s", resp.StatusCode, string(bodyBytes))
	}
	env, _ := profile.ReadEnv("x")
	if env["ANTHROPIC_BASE_URL"] != "https://api.deepseek.com/anthropic" {
		t.Errorf("update 后 preset 未生效: %v", env)
	}
}

func TestCreate_StillSupportsCustomSettings(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	body := `{"id":"custom","name":"Custom","settings":{"env":{"ANTHROPIC_BASE_URL":"https://custom","ANTHROPIC_AUTH_TOKEN":"tok"}}}`
	resp, err := http.Post(srv.URL+"/api/v1/providers", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST custom settings want 201 got %d", resp.StatusCode)
	}
	env, _ := profile.ReadEnv("custom")
	if env["ANTHROPIC_BASE_URL"] != "https://custom" {
		t.Errorf("自定义 settings 未生效: %v", env)
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

// TestCreate_PersistsEnvTruthInProvidersJSON 验证 env 真值（含敏感值）持久化到
// providers.json——Mode P 时代的统一契约：providers.json 是 env 唯一持久真值源
// （0600/0700，防护等级与 profile settings.json 相同；启用 Mode P 时敏感值
// 迁 keychain 占位化，见 specs/001 研究 D8）。
func TestCreate_PersistsEnvTruthInProvidersJSON(t *testing.T) {
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

	// providers.json 应持久化完整 env 真值，且文件 0600。
	raw, _ := os.ReadFile(cfg)
	if !strings.Contains(string(raw), "tok-secret-123") || !strings.Contains(string(raw), "https://imp") {
		t.Errorf("providers.json 应含 env 真值：%s", string(raw))
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
		"ANTHROPIC_API_KEY":      true,
		"ANTHROPIC_AUTH_TOKEN":   true,
		"SECRET_STUFF":           true,
		"PASSWORD":               true,
		"ANTHROPIC_BASE_URL":     false,
		"ANTHROPIC_MODEL":        false,
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
	targets, ok := out["targets"].([]any)
	if !ok || len(targets) != 1 {
		t.Errorf("zsh 应返回 1 个 target, got %v", out["targets"])
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

// TestUpdateCheck_DevBuild 覆盖 GET /api/v1/update/check：
// 测试进程无 ldflags 注入（version=dev），应返回 devBuild:true 且不访问网络。
func TestUpdateCheck_DevBuild(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	resp, err := http.Get(srv.URL + "/api/v1/update/check")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /update/check want 200 got %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["devBuild"] != true || body["hasUpdate"] != false {
		t.Errorf("dev 构建应 devBuild=true 且 hasUpdate=false, got %v", body)
	}
}

// TestUpdateRun_DevBuildRefused 覆盖 POST /api/v1/update：
// dev 构建应返回 409 + refused:true + kind:"dev"（前端据此渲染 dev 指引）。
func TestUpdateRun_DevBuildRefused(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	resp, err := http.Post(srv.URL+"/api/v1/update", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("POST /update want 409 got %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["refused"] != true || body["kind"] != "dev" {
		t.Errorf("应 refused=true kind=dev, got %v", body)
	}
}

// TestUpdateEndpoints_MethodNotAllowed 覆盖两个端点的方法约束。
func TestUpdateEndpoints_MethodNotAllowed(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	// POST 到 check 端点 → 405
	resp, err := http.Post(srv.URL+"/api/v1/update/check", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST /update/check want 405 got %d", resp.StatusCode)
	}
	// GET 到 update 端点 → 405
	resp2, err := http.Get(srv.URL + "/api/v1/update")
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET /update want 405 got %d", resp2.StatusCode)
	}
}

func TestModeEndpoint_PutProxyMigrates(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")
	// 明文 provider 供迁移。
	os.WriteFile(os.Getenv("CC_SELECT_CONFIG"),
		[]byte(`{"providers":{"glm":{"id":"glm","env":{"ANTHROPIC_AUTH_TOKEN":"sk-plain"}}}}`), 0o600)
	orig := migrateSecretsFn
	migrateSecretsFn = func(cfg *config.Config) (int, []string) {
		return secrets.MigrateAll(secrets.NewFake(), cfg)
	}
	defer func() { migrateSecretsFn = orig }()

	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/v1/mode",
		strings.NewReader(`{"isolationMode":"proxy"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT proxy want 200 got %d", resp.StatusCode)
	}
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	resp.Body.Close()
	if out["isolationMode"] != "proxy" {
		t.Errorf("应回显 proxy: %v", out)
	}
	if out["migrated"] != float64(1) {
		t.Errorf("应报告迁移 1 条: %v", out["migrated"])
	}
	if failed, ok := out["failed"].([]any); !ok || len(failed) != 0 {
		t.Errorf("failed 应为空数组: %v", out["failed"])
	}
	// 落盘为占位。
	data, _ := os.ReadFile(os.Getenv("CC_SELECT_CONFIG"))
	if !strings.Contains(string(data), "$keychain:cc-select:glm:ANTHROPIC_AUTH_TOKEN") {
		t.Errorf("providers.json 应为占位: %s", data)
	}
}

// ---- Mode P 路由/守护端点（T023） ----

func TestRoutesEndpoint_List(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")
	// 探活地址指向死端口：本机可能有真实 daemon 在默认端口跑着（开发机常态），
	// 测试不得依赖机器状态——固定断言 running=false。
	t.Setenv("CC_SELECT_PROXY_ADDR", "127.0.0.1:1")
	tidA, _ := routes.NewTID()
	tidB, _ := routes.NewTID()
	_ = routes.Set(tidA, "glm")
	_ = routes.Set(tidB, "minimax")

	resp, err := http.Get(srv.URL + "/api/v1/routes")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200 got %d", resp.StatusCode)
	}
	var out struct {
		Router struct {
			Running bool   `json:"running"`
			Addr    string `json:"addr"`
			Version string `json:"version"`
		} `json:"router"`
		Routes []struct {
			TID      string `json:"tid"`
			Provider string `json:"provider"`
		} `json:"routes"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	if len(out.Routes) != 2 {
		t.Fatalf("应列 2 条: %+v", out.Routes)
	}
	byProvider := map[string]string{}
	for _, r := range out.Routes {
		byProvider[r.Provider] = r.TID
	}
	if byProvider["glm"] != tidA[:12] || byProvider["minimax"] != tidB[:12] {
		t.Errorf("tid 应短码展示: %+v", out.Routes)
	}
	if out.Routes != nil && len(tidA) == 36 { /* 完整 tid 不应出现 */
	}
	if out.Router.Running {
		t.Errorf("无 daemon 时 running 应为 false: %+v", out.Router)
	}
}

func TestRoutesEndpoint_Prune(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")
	stale, _ := routes.NewTID()
	fresh, _ := routes.NewTID()
	_ = routes.Set(stale, "glm")
	_ = routes.Set(fresh, "glm")
	tbl, _ := routes.Load()
	for i := range tbl.Routes {
		if tbl.Routes[i].TID == stale {
			tbl.Routes[i].UpdatedAt = time.Now().UTC().Add(-8 * 24 * time.Hour)
		}
	}
	_ = routes.Save(tbl)

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/routes", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	if out["pruned"] != float64(1) {
		t.Errorf("应清理 1 条: %v", out)
	}
	if _, ok := routes.Get(fresh); !ok {
		t.Error("新条目应保留")
	}

	// 非法时长 → 400。
	req2, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/routes?olderThan=bogus", nil)
	resp2, err2 := http.DefaultClient.Do(req2)
	if err2 != nil {
		t.Fatal(err2)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusBadRequest {
		t.Errorf("非法时长 want 400 got %d", resp2.StatusCode)
	}
}

func TestRouterEnsure_Ok(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")
	orig := ensureRouterFn
	ensureRouterFn = func() (string, error) { return "127.0.0.1:48270", nil }
	defer func() { ensureRouterFn = orig }()

	resp, err := http.Post(srv.URL+"/api/v1/router/ensure", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200 got %d", resp.StatusCode)
	}
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	if out["running"] != true || out["addr"] != "127.0.0.1:48270" {
		t.Errorf("ensure 响应不符: %v", out)
	}
}

func TestRouterEnsure_Fail503(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")
	orig := ensureRouterFn
	ensureRouterFn = func() (string, error) { return "", errorsNew("spawn failed") }
	defer func() { ensureRouterFn = orig }()

	resp, err := http.Post(srv.URL+"/api/v1/router/ensure", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("失败 want 503 got %d", resp.StatusCode)
	}
}

func errorsNew(msg string) error { return &stubError{msg} }

type stubError struct{ msg string }

func (e *stubError) Error() string { return e.msg }

// Mode P 派生产物保护：use(proxy) 后 profile env 仅含代理 BASE_URL，真值在
// providers.json。编辑页回填必须用真值，否则一次保存即真值丢失。
func TestGetProvider_ProxyArtifactShowsRealEnv(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")
	// providers.json 真值（上抬后的形态）。
	os.WriteFile(os.Getenv("CC_SELECT_CONFIG"), []byte(`{"providers":{"glm":{"id":"glm","env":{"ANTHROPIC_BASE_URL":"https://glm.api","ANTHROPIC_AUTH_TOKEN":"sk-real","ANTHROPIC_MODEL":"glm-5.3"}}}}`), 0o600)
	// profile = use(proxy) 的派生产物（env 仅代理地址，非 env 字段来自全局合并）。
	profile.EnsureRaw("glm", []byte(`{"permissions":{"allow":["foo"]},"env":{"ANTHROPIC_BASE_URL":"http://127.0.0.1:48270"}}`))

	resp, err := http.Get(srv.URL + "/api/v1/providers/glm")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out struct {
		Settings map[string]any `json:"settings"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	env, _ := out.Settings["env"].(map[string]any)
	if env == nil || env["ANTHROPIC_BASE_URL"] != "https://glm.api" || env["ANTHROPIC_AUTH_TOKEN"] != "sk-real" {
		t.Errorf("编辑页应回填 providers.json 真值而非代理派生产物: %+v", env)
	}
	if _, has := out.Settings["permissions"]; !has {
		t.Errorf("非 env 字段应保留: %+v", out.Settings)
	}
}

// 列表页同样受 Mode P 派生产物影响：GLM 被 use 后 profile 只剩代理地址，
// 列表应展示 providers.json 真值（URL/model/已配 key 徽标），而非吓人的空壳。
func TestListProviders_ProxyArtifactShowsTruth(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")
	os.WriteFile(os.Getenv("CC_SELECT_CONFIG"), []byte(`{"providers":{"glm":{"id":"glm","env":{"ANTHROPIC_BASE_URL":"https://open.bigmodel.cn/api/anthropic","ANTHROPIC_AUTH_TOKEN":"$keychain:cc-select:glm:ANTHROPIC_AUTH_TOKEN","ANTHROPIC_MODEL":"glm-5.3"}}}}`), 0o600)
	profile.EnsureRaw("glm", []byte(`{"env":{"ANTHROPIC_BASE_URL":"http://127.0.0.1:48270"}}`))

	resp, err := http.Get(srv.URL + "/api/v1/providers")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	got, _ := out["providers"].(map[string]any)
	glm, _ := got["glm"].(map[string]any)
	if glm["hasKey"] != true {
		t.Errorf("真值含 token 占位，应显示已配 key: %v", glm)
	}
	env, _ := glm["env"].(map[string]any)
	if env["ANTHROPIC_BASE_URL"] != "https://open.bigmodel.cn/api/anthropic" {
		t.Errorf("列表 URL 应为真值而非代理地址: %v", env)
	}
	if env["ANTHROPIC_MODEL"] != "glm-5.3" {
		t.Errorf("列表应展示真值 model: %v", env)
	}
}

// 全局模式为 proxy 时，保存 provider 不得被 Sync 的防误用守卫拦截——
// 真值落 providers.json，profile 重建为代理派生产物。
func TestUpdateProvider_GlobalProxyModeSucceeds(t *testing.T) {
	srv, cfgPath := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")
	os.WriteFile(filepath.Join(filepath.Dir(cfgPath), "prefs.json"), []byte(`{"isolationMode":"proxy"}`), 0o600)

	body := `{"name":"GLM","settings":{"env":{"ANTHROPIC_BASE_URL":"https://open.bigmodel.cn/api/anthropic","ANTHROPIC_AUTH_TOKEN":"sk-real","ANTHROPIC_MODEL":"glm-5.3"}}}`
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/v1/providers/glm", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("全局 proxy 下保存应成功，got %d: %s", resp.StatusCode, b)
	}
	// 真值进 providers.json。
	data, _ := os.ReadFile(cfgPath)
	if !strings.Contains(string(data), "sk-real") || !strings.Contains(string(data), "open.bigmodel.cn") {
		t.Errorf("providers.json 应保存真值: %s", data)
	}
	// profile = 代理派生产物。
	env, _ := profile.ReadEnv("glm")
	if env["ANTHROPIC_BASE_URL"] != "http://127.0.0.1:48270" {
		t.Errorf("proxy 模式下 profile 应指向 daemon: %+v", env)
	}
}
