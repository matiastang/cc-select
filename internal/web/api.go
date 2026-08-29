package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cc-select/cc-select/internal/app"
	"github.com/cc-select/cc-select/internal/config"
	"github.com/cc-select/cc-select/internal/i18n"
	"github.com/cc-select/cc-select/internal/prefs"
	"github.com/cc-select/cc-select/internal/presets"
	"github.com/cc-select/cc-select/internal/profile"
	"github.com/cc-select/cc-select/internal/rcinteg"
	"github.com/cc-select/cc-select/internal/router"
	"github.com/cc-select/cc-select/internal/routes"
	"github.com/cc-select/cc-select/internal/secrets"
	"github.com/cc-select/cc-select/internal/updater"
	"github.com/cc-select/cc-select/internal/version"
)

// providerDTO 是列表视图（GET /providers）里单个 provider 的精简表示。
// 列表故意脱敏：API key 永远不返回明文，只返回 hasKey 布尔。见 docs/tech-stack.md §5 正确性要点。
// 完整配置（含明文，供编辑回填）走 GET /providers/{id} 的 providerDetailDTO。
type providerDTO struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Env           map[string]string `json:"env"`           // 仅非敏感值，用于列表摘要展示
	HasKey        bool              `json:"hasKey"`        // 是否配置了敏感变量（如 API key）
	VarKeys       []string          `json:"varKeys"`       // env 变量名列表（不含值，便于前端展示）
	IsolationMode string            `json:"isolationMode"` // per-provider 覆盖；空串 = 继承全局
}

// providerDetailDTO 是单个 provider 的完整表示（GET /providers/{id}）。
// Settings 是 profile settings.json 的磁盘原文——即便用户手改了文件也如实反映。
// 故意返回明文：编辑页需要展示真实配置（含 token），见用户确认的"明文显示"决策。
type providerDetailDTO struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Settings      json.RawMessage `json:"settings"`      // settings.json 磁盘原文；官方/缺失为 {}
	IsolationMode string          `json:"isolationMode"` // per-provider 覆盖；空串 = 继承全局
	Preset        string          `json:"preset"`        // 创建/编辑时选用的 preset id
	APIFormat     string          `json:"apiFormat"`     // 高级选项：API 格式
	AuthField     string          `json:"authField"`     // 高级选项：认证字段
}

// presetDTO 是 preset 列表/详情返回给前端的结构（不含完整 EnvTemplate，只给必要元信息）。
type presetDTO struct {
	ID           string   `json:"id"`
	DisplayName  string   `json:"displayName"`
	Category     string   `json:"category"`
	WebsiteURL   string   `json:"websiteURL,omitempty"`
	APIKeyURL    string   `json:"apiKeyURL,omitempty"`
	APIFormat    string   `json:"apiFormat"`
	AuthField    string   `json:"authField,omitempty"`
	RequiredVars []string `json:"requiredVars"`
	OptionalVars []string `json:"optionalVars"`
	OAuth        bool     `json:"oauth"`
}

// presetDetailDTO 包含 preset 的完整模板，供前端表单自动填充。
type presetDetailDTO struct {
	presetDTO
	EnvTemplate map[string]string `json:"envTemplate"`
}

// apiHandler 持有依赖，处理 /api/v1/* 路由。
type apiHandler struct{}

func newAPIHandler() *apiHandler { return &apiHandler{} }

// migrateSecretsFn 是存量批量迁移的注入点：生产用系统 keychain，测试换 FakeStore。
var migrateSecretsFn = func(cfg *config.Config) (int, []string) {
	return secrets.MigrateAll(secrets.New(), cfg)
}

// migrateSecretsEnvFn 是保存路径单 env 迁移的注入点（钥匙串开关开启时生效）。
var migrateSecretsEnvFn = func(providerID string, env map[string]string) (int, []string) {
	return secrets.MigrateEnv(secrets.New(), providerID, env)
}

func (h *apiHandler) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/providers", h.handleProvidersCollection)
	mux.HandleFunc("/api/v1/providers/", h.handleProviderItem)
	mux.HandleFunc("/api/v1/presets", h.handlePresetsCollection)
	mux.HandleFunc("/api/v1/presets/", h.handlePresetItem)
	mux.HandleFunc("/api/v1/mode", h.handleMode)
	mux.HandleFunc("/api/v1/language", h.handleLanguage)
	mux.HandleFunc("/api/v1/shell-integration", h.handleShellIntegration)
	mux.HandleFunc("/api/v1/shell-integration/install", h.handleShellIntegrationInstall)
	mux.HandleFunc("/api/v1/update/check", h.handleUpdateCheck)
	mux.HandleFunc("/api/v1/update", h.handleUpdateRun)
	mux.HandleFunc("/api/v1/routes", h.handleRoutes)
	mux.HandleFunc("/api/v1/router/ensure", h.handleRouterEnsure)
	mux.HandleFunc("/api/v1/keychain", h.handleKeychain)
	return mux
}

// handleProvidersCollection 处理 GET（列）和 POST（建）。
func (h *apiHandler) handleProvidersCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listProviders(w, r)
	case http.MethodPost:
		h.createProvider(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleProviderItem 处理 GET/PUT/DELETE 单个 provider。
func (h *apiHandler) handleProviderItem(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/providers/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing provider id")
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.getProvider(w, r, id)
	case http.MethodPut:
		h.updateProvider(w, r, id)
	case http.MethodDelete:
		h.deleteProvider(w, r, id)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handlePresetItem 处理 GET /presets/<id>。
func (h *apiHandler) handlePresetItem(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/presets/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing preset id")
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	p, ok := presets.ByID(id)
	if !ok {
		writeError(w, http.StatusNotFound, "preset not found")
		return
	}
	writeJSON(w, http.StatusOK, toPresetDetailDTO(p))
}

// handlePresetsCollection 处理 GET /presets（列表）。
func (h *apiHandler) handlePresetsCollection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	all := presets.All()
	out := make([]presetDTO, len(all))
	for i, p := range all {
		out[i] = toPresetDTO(p)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"presets":    out,
		"categories": presets.Categories(),
	})
}

func toPresetDTO(p presets.Preset) presetDTO {
	return presetDTO{
		ID:           p.ID,
		DisplayName:  p.DisplayName,
		Category:     string(p.Category),
		WebsiteURL:   p.WebsiteURL,
		APIKeyURL:    p.APIKeyURL,
		APIFormat:    string(p.APIFormat),
		AuthField:    string(p.AuthField),
		RequiredVars: p.RequiredVars,
		OptionalVars: p.OptionalVars,
		OAuth:        p.OAuth,
	}
}

func toPresetDetailDTO(p presets.Preset) presetDetailDTO {
	return presetDetailDTO{
		presetDTO:   toPresetDTO(p),
		EnvTemplate: p.EnvTemplate,
	}
}

//   - GET  → {"isolationMode": "settings-only" | "full"}（未设置时返回默认）
//   - PUT  → 同结构，设置全局模式。
//
// per-provider 覆盖（providers.json 的 Provider.IsolationMode）暂不在 Web 暴露，
// 由 CLI（cc-select edit <id> --mode ...）管理；此处只管全局默认。
func (h *apiHandler) handleMode(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		pr, err := prefs.Load()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		mode := pr.IsolationMode
		if mode == "" {
			mode = prefs.DefaultMode
		}
		writeJSON(w, http.StatusOK, map[string]any{"isolationMode": string(mode)})
	case http.MethodPut:
		var in struct {
			IsolationMode prefs.Mode `json:"isolationMode"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if !in.IsolationMode.Valid() || in.IsolationMode == "" {
			writeError(w, http.StatusBadRequest, "isolationMode must be settings-only, full or proxy")
			return
		}
		pr, err := prefs.Load()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		pr.IsolationMode = in.IsolationMode
		if err := prefs.Save(pr); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		// 产品决策（2026-08-29）：keychain 迁移默认关闭，不再随 proxy 启用自动触发；
		// 显式开关见 /api/v1/keychain（GUI「保存到钥匙串」设置项）。
		writeJSON(w, http.StatusOK, map[string]any{"isolationMode": string(in.IsolationMode)})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleLanguage 处理 GET/PUT 显示语言偏好（~/.cc-select/prefs.json）。
//   - GET  → {"language": "en" | "zh"}（未设置时返回空串）
//   - PUT  → 同结构，设置语言。
func (h *apiHandler) handleLanguage(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		pr, err := prefs.Load()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"language": pr.NormalizeLanguage()})
	case http.MethodPut:
		var in struct {
			Language string `json:"language"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		l := i18n.NormalizeLocale(in.Language)
		if !i18n.IsSupportedLocale(string(l)) {
			writeError(w, http.StatusBadRequest, i18n.T("errors.web.invalidLanguage"))
			return
		}
		pr, err := prefs.Load()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		pr.Language = string(l)
		if err := prefs.Save(pr); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		i18n.SetLocale(l)
		writeJSON(w, http.StatusOK, map[string]any{"language": string(l)})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
func (h *apiHandler) handleShellIntegration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	st := rcinteg.DetectStatus()
	writeJSON(w, http.StatusOK, map[string]any{
		"supported":      st.Supported,
		"shell":          st.Shell,
		"installed":      st.Installed,
		"legacy":         st.Legacy,
		"rcPath":         st.RCPath,
		"canAutoInstall": st.CanAutoInstall,
		"targets":        st.Targets,
	})
}

// handleShellIntegrationInstall 处理 POST：一键安装 shell 集成。
// 无法自动写（PowerShell 未装等）时降级为 manual，返回 snippet 给前端展示。
func (h *apiHandler) handleShellIntegrationInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var in struct {
		Shell  string `json:"shell"`
		Target string `json:"target"` // PowerShell 变体：powershell7 | powershell5；空=PS7 优先
	}
	_ = json.NewDecoder(r.Body).Decode(&in) // shell/target 可选，空=自动检测
	res, err := rcinteg.Install(in.Shell, in.Target)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"action":  res.Action,
		"shell":   res.Shell,
		"rcPath":  res.RCPath,
		"snippet": res.Snippet,
		"message": res.Message,
	})
}

// handleUpdateCheck 处理 GET：查询是否有新版本（只读，不拒绝 brew/scoop——
// 只读查询对所有安装方式都有意义；拒绝是 POST 的职责）。
func (h *apiHandler) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	res, err := updater.Check(r.Context(), updater.Options{GitHubToken: updater.GitHubTokenFromEnv()})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"currentVersion": res.Current,
		"latestVersion":  res.Latest,
		"hasUpdate":      res.HasUpdate,
		"devBuild":       res.DevBuild,
		"assetName":      res.AssetName,
		"releaseNotes":   res.ReleaseNotes,
		"htmlUrl":        res.HTMLURL,
	})
}

// updateMu 串行化安装点击：两个并发 Replace 会竞争同一路径
// （Windows .old 删除 / Unix rename 目标）。第二个调用方得 409。
var updateMu sync.Mutex

// handleUpdateRun 处理 POST：跑完整更新流水线。
// 关键正确性：替换后本 server 进程继续服务（Unix 持有旧 inode；Windows 走
// renamed-.old），响应正常返回 restartRequired=true，新版本在下次启动生效。
func (h *apiHandler) handleUpdateRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !updateMu.TryLock() {
		writeError(w, http.StatusConflict, i18n.T("errors.update.locked"))
		return
	}
	defer updateMu.Unlock()

	out, err := updater.Run(r.Context(), updater.Options{GitHubToken: updater.GitHubTokenFromEnv()})
	if err != nil {
		var refused *updater.RefusedError
		if errors.As(err, &refused) {
			// 409 + kind：前端据此渲染精确的升级指引（brew/scoop/dev/不可写）。
			writeJSON(w, http.StatusConflict, map[string]any{
				"error":   err.Error(),
				"refused": true,
				"kind":    refused.Kind,
			})
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "installed",
		"fromVersion":     out.FromVersion,
		"toVersion":       out.ToVersion,
		"message":         out.Message,
		"restartRequired": out.Installed,
	})
}

func (h *apiHandler) listProviders(w http.ResponseWriter, _ *http.Request) {
	a, err := app.New()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := map[string]providerDTO{}
	for id, p := range a.Config.Providers {
		out[id] = toDTO(p)
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": out})
}

func (h *apiHandler) createProvider(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID            string            `json:"id"`
		Name          string            `json:"name"`
		Preset        string            `json:"preset,omitempty"`
		APIKey        string            `json:"apiKey,omitempty"`
		Overrides     map[string]string `json:"overrides,omitempty"`
		APIFormat     string            `json:"apiFormat,omitempty"`
		AuthField     string            `json:"authField,omitempty"`
		Settings      json.RawMessage   `json:"settings,omitempty"`
		IsolationMode prefs.Mode        `json:"isolationMode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if in.ID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	// 校验 id 合法（防路径穿越）——id 来自请求体，会拼进 profile 目录路径。
	if err := config.ValidateID(in.ID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.Name == "" {
		in.Name = in.ID
	}
	if !in.IsolationMode.Valid() {
		writeError(w, http.StatusBadRequest, "isolationMode must be empty, settings-only, full or proxy")
		return
	}

	data, err := resolveProviderSettings(in.Preset, in.APIKey, in.Overrides, in.APIFormat, in.AuthField, in.Settings)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	a, err := app.New()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, exists := a.Config.Providers[in.ID]; exists {
		writeError(w, http.StatusConflict, "provider already exists")
		return
	}
	mode := prefs.ResolveMode("", in.IsolationMode, a.Prefs.IsolationMode)
	envSaved, aerr := applySettings(in.ID, data, mode)
	if aerr != nil {
		writeError(w, http.StatusInternalServerError, aerr.Error())
		return
	}
	a.Config.Providers[in.ID] = config.Provider{
		ID:            in.ID,
		Name:          in.Name,
		Env:           envSaved,
		IsolationMode: in.IsolationMode,
		PresetID:      in.Preset,
		APIFormat:     in.APIFormat,
		AuthField:     in.AuthField,
	}
	if err := config.Save(a.Config); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toDetailDTO(a.Config.Providers[in.ID], in.ID))
}

func (h *apiHandler) getProvider(w http.ResponseWriter, _ *http.Request, id string) {
	a, err := app.New()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	p, ok := a.Config.Providers[id]
	if !ok {
		writeError(w, http.StatusNotFound, "provider not found")
		return
	}
	writeJSON(w, http.StatusOK, toDetailDTO(p, id))
}

func (h *apiHandler) updateProvider(w http.ResponseWriter, r *http.Request, id string) {
	// 官方 provider 使用系统默认配置、不建 profile，写 settings 会被 EnsureRaw 静默丢弃。
	// 故在 API 层明确拒绝，避免"看似保存成功、实则丢失"的误导（前端也禁用了编辑入口）。
	if id == config.OfficialProviderID {
		writeError(w, http.StatusBadRequest, i18n.T("errors.web.officialSettings"))
		return
	}
	var in struct {
		Name          string            `json:"name"`
		Preset        string            `json:"preset,omitempty"`
		APIKey        string            `json:"apiKey,omitempty"`
		Overrides     map[string]string `json:"overrides,omitempty"`
		APIFormat     string            `json:"apiFormat,omitempty"`
		AuthField     string            `json:"authField,omitempty"`
		Settings      json.RawMessage   `json:"settings,omitempty"`
		IsolationMode prefs.Mode        `json:"isolationMode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if !in.IsolationMode.Valid() {
		writeError(w, http.StatusBadRequest, "isolationMode must be empty, settings-only, full or proxy")
		return
	}
	if in.Name == "" {
		in.Name = id
	}

	data, err := resolveProviderSettings(in.Preset, in.APIKey, in.Overrides, in.APIFormat, in.AuthField, in.Settings)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	a, err := app.New()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, ok := a.Config.Providers[id]; !ok {
		writeError(w, http.StatusNotFound, "provider not found")
		return
	}
	mode := prefs.ResolveMode("", in.IsolationMode, a.Prefs.IsolationMode)
	// 整体覆盖：按 mode 写 profile（env 真值随 applySettings 落盘 providers.json）。
	envSaved, aerr := applySettings(id, data, mode)
	if aerr != nil {
		writeError(w, http.StatusInternalServerError, aerr.Error())
		return
	}
	a.Config.Providers[id] = config.Provider{
		ID:            id,
		Name:          in.Name,
		Env:           envSaved, // 写回快照，防下方 config.Save 旧值覆盖刚落盘的 env
		IsolationMode: in.IsolationMode,
		PresetID:      in.Preset,
		APIFormat:     in.APIFormat,
		AuthField:     in.AuthField,
	}
	if err := config.Save(a.Config); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toDetailDTO(a.Config.Providers[id], id))
}

func (h *apiHandler) deleteProvider(w http.ResponseWriter, _ *http.Request, id string) {
	if id == config.OfficialProviderID {
		writeError(w, http.StatusBadRequest, "cannot delete built-in provider")
		return
	}
	a, err := app.New()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, ok := a.Config.Providers[id]; !ok {
		writeError(w, http.StatusNotFound, "provider not found")
		return
	}
	_ = profile.Remove(id) // 删 profile 目录（含 settings.json）
	delete(a.Config.Providers, id)
	if err := config.Save(a.Config); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// resolveProviderSettings 根据 preset 或自定义 settings 生成待写入 profile 的 JSON 数据。
// 若 preset 非空，则忽略 settings；否则使用 settings（自定义模式）。
func resolveProviderSettings(presetID, apiKey string, overrides map[string]string, apiFormat, authField string, settings json.RawMessage) ([]byte, error) {
	if presetID != "" {
		if _, ok := presets.ByID(presetID); !ok {
			return nil, fmt.Errorf("unknown preset %q", presetID)
		}
		// API 格式与认证字段作为覆盖传入（高级选项）。
		if apiFormat != "" {
			overrides["_api_format"] = apiFormat
		}
		if authField != "" {
			overrides["_auth_field"] = authField
		}
		env, missing, err := presets.BuildEnv(presetID, apiKey, overrides)
		if err != nil {
			return nil, err
		}
		if len(missing) > 0 {
			return nil, fmt.Errorf("missing required fields: %s", presets.FormatMissing(missing))
		}
		settingsObj := map[string]any{"env": env}
		return json.MarshalIndent(settingsObj, "", "  ")
	}
	if len(settings) == 0 {
		return nil, fmt.Errorf("settings is required when preset is omitted")
	}
	return normalizeSettings(settings)
}

// normalizeSettings 校验并规范化用户提交的 settings：必须是非空 JSON 对象，
// 返回缩进美化后的字节（写入 settings.json）。支持 env 之外的任意字段。
func normalizeSettings(raw json.RawMessage) ([]byte, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("settings is required")
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf(i18n.T("errors.web.invalidSettingsJSON"), err)
	}
	if _, ok := v.(map[string]any); !ok {
		return nil, i18n.E("errors.web.settingsMustBeObject")
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf(i18n.T("errors.web.serializeSettings"), err)
	}
	return out, nil
}

// applySettings 按 mode 把 settings 写入 profile，并把 env 真值持久化到 providers.json。
//   - Mode A（full）：原文写入 data，保留 env 之外字段（permissions、model 等）。
//   - Mode B（settings-only）/ Mode P：只取 env 做整体替换；Mode P 下 profile 是
//     派生产物（仅代理 BASE_URL），env 的持久真值必须落 providers.json。
//
// data 应是已校验/规范化的 JSON 对象字节。官方 provider 无 profile（no-op）。
func applySettings(id string, data []byte, mode prefs.Mode) (map[string]string, error) {
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf(i18n.T("profile.parseGlobalSettings"), err)
	}
	envAny, _ := settings["env"].(map[string]any)
	env := map[string]string{}
	for k, v := range envAny {
		if s, ok := v.(string); ok {
			env[k] = s
		}
	}
	// 钥匙串开关显式开启（默认关）时，保存路径把敏感值占位化（产品决策 2026-08-29）。
	// 偏好读取失败必须 fail-closed（评审 P1）：开着钥匙串的用户不得因 prefs.json
	// 损坏被静默降级为明文保存——宁可让本次保存失败并报错。
	pr, perr := prefs.Load()
	if perr != nil {
		return nil, perr
	}
	if pr.KeychainEnabled {
		_, _ = migrateSecretsEnvFn(id, env) // 单条迁移失败保持明文继续（不阻断保存）
	}
	// env 真值 → providers.json（与 CLI add/edit 的 writeProvider 行为对齐）。
	if err := saveProviderEnv(id, env); err != nil {
		return nil, err
	}
	if mode == prefs.ModeFull {
		if _, err := profile.EnsureRaw(id, data); err != nil {
			return nil, err
		}
		return env, nil
	}
	if _, _, err := router.SyncProfile(id, env, mode); err != nil {
		return nil, err
	}
	return env, nil
}

// saveProviderEnv 把 env 写入 providers.json 对应 provider（存在则更新 Env 字段）。
func saveProviderEnv(id string, env map[string]string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	p, ok := cfg.Providers[id]
	if !ok {
		p = config.Provider{ID: id} // create 流程：applySettings 先于元信息落库
	}
	p.Env = env
	cfg.Providers[id] = p
	return config.Save(cfg)
}

// isSensitiveVar 判断变量名是否敏感（用于 toDTO 脱敏：值不回传前端）。
func isSensitiveVar(name string) bool {
	up := strings.ToUpper(name)
	for _, sub := range []string{"KEY", "TOKEN", "SECRET", "PASSWORD", "PASS"} {
		if strings.Contains(up, sub) {
			return true
		}
	}
	return false
}

// toDTO 把 provider 转为列表用的脱敏 DTO（不泄露敏感值）。env 从 profile settings.json 读真值。
func toDTO(p config.Provider) providerDTO {
	env := displayedEnv(p)
	dto := providerDTO{
		ID:            p.ID,
		Name:          p.DisplayName(),
		Env:           map[string]string{},
		VarKeys:       make([]string, 0, len(env)),
		IsolationMode: string(p.IsolationMode),
	}
	for k, v := range env {
		dto.VarKeys = append(dto.VarKeys, k)
		if isSensitiveVar(k) {
			dto.HasKey = true
			continue // 敏感值不回传前端
		}
		dto.Env[k] = v
	}
	return dto
}

// toDetailDTO 返回 provider 的完整明文 settings（编辑回填用）。
// 从磁盘现读 settings.json，保证反映真实内容（即便用户手改过文件）。
// 官方 provider 或文件缺失时 Settings 退化为空对象 {}。
func toDetailDTO(p config.Provider, id string) providerDetailDTO {
	raw, _ := profile.ReadRaw(id)
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	// Mode P 派生产物保护（specs/001 实测发现）：use(proxy) 会把 profile env 覆写为
	// 仅含代理 BASE_URL，真值在 providers.json（上抬后）。编辑页若以派生产物回填，
	// 用户一次保存就会把真值覆盖丢失——此处把 env 部分替换回真值（非 env 字段保留）。
	if len(p.Env) > 0 && isProxyArtifactEnv(profileEnvOnlyBaseURL(p)) {
		raw = restoreEnvFromTruth(raw, p.Env)
	}
	return providerDetailDTO{
		ID:            id,
		Name:          p.DisplayName(),
		Settings:      json.RawMessage(raw),
		IsolationMode: string(p.IsolationMode),
		Preset:        p.PresetID,
		APIFormat:     p.APIFormat,
		AuthField:     p.AuthField,
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// ---- Mode P 路由/守护端点（specs/001 contracts/web-api.md §2–4） ----

// routeEntryDTO 是路由表条目的 API 表示（tid 恒为短码，与 CLI 展示规则一致）。
type routeEntryDTO struct {
	TID       string `json:"tid"`
	Provider  string `json:"provider"`
	UpdatedAt string `json:"updatedAt"`
}

// ensureRouterFn 是 web 侧 ensure 注入点：生产走 router.EnsureDeps + SpawnDetached，
// 测试换桩（避免真拉进程）。
var ensureRouterFn = func() (string, error) {
	deps := router.EnsureDeps{Spawn: router.SpawnDetached, SelfVersion: version.Version}
	return deps.Ensure()
}

// handleRoutes 处理 GET（列路由 + daemon 状态）与 DELETE（prune）。
func (h *apiHandler) handleRoutes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		entries, err := routes.List()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		running, addr, ver := false, "", ""
		if a, aerr := router.ResolveAddr(); aerr == nil {
			addr = a
			if hi, perr := router.Probe(a); perr == nil {
				running, ver = true, hi.Version
			}
		}
		out := make([]routeEntryDTO, 0, len(entries))
		for _, e := range entries {
			tid := e.TID
			if len(tid) > 12 {
				tid = tid[:12]
			}
			out = append(out, routeEntryDTO{TID: tid, Provider: e.Provider, UpdatedAt: e.UpdatedAt.Format(time.RFC3339)})
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"router": map[string]any{"running": running, "addr": addr, "version": ver},
			"routes": out,
		})
	case http.MethodDelete:
		q := r.URL.Query().Get("olderThan")
		if q == "" {
			q = "168h"
		}
		d, err := time.ParseDuration(q)
		if err != nil || d <= 0 {
			writeError(w, http.StatusBadRequest, "invalid olderThan: "+err.Error())
			return
		}
		n, err := routes.Prune(d)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"pruned": n})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleRouterEnsure 处理 POST /router/ensure（GUI「启动/修复路由服务」）。
func (h *apiHandler) handleRouterEnsure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	addr, err := ensureRouterFn()
	if err != nil {
		// 503 + 可诊断错误与恢复指引（FR-011）。
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"running": true, "addr": addr})
}

// restoreEnvFromTruth 在 raw（profile settings.json 原文）是「仅含代理 BASE_URL 的
// 派生产物」时，把 env 键替换为 truth（providers.json 真值），其余字段原样保留。
// 非 派生产物 形态则原样返回（Mode A 原文编辑语义不受影响）。
// displayedEnv 返回展示用 env：profile 是 Mode P 派生产物（仅代理 BASE_URL）
// 且 providers.json 有真值时，用真值——列表徽标/URL/model 均反映真实配置。
func displayedEnv(p config.Provider) map[string]string {
	env, _ := profile.ReadEnv(p.ID)
	if len(p.Env) > 0 && isProxyArtifactEnv(env) {
		return p.Env
	}
	return env
}

// profileEnvOnlyBaseURL 读取 profile env（isProxyArtifactEnv 判定用）。
func profileEnvOnlyBaseURL(p config.Provider) map[string]string {
	env, _ := profile.ReadEnv(p.ID)
	return env
}

// isProxyArtifactEnv 判定 env 是否为 use(proxy) 的派生产物形态：
// 仅含一个 ANTHROPIC_BASE_URL 且其值恰为当前 daemon 地址。
func isProxyArtifactEnv(env map[string]string) bool {
	if len(env) != 1 {
		return false
	}
	addr, err := router.ResolveAddr()
	if err != nil {
		return false
	}
	return env["ANTHROPIC_BASE_URL"] == "http://"+addr
}

func restoreEnvFromTruth(raw []byte, truth map[string]string) []byte {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return raw
	}
	envAny, ok := m["env"].(map[string]any)
	if !ok || len(envAny) != 1 {
		return raw
	}
	v, isStr := envAny["ANTHROPIC_BASE_URL"].(string)
	if !isStr || !isProxyArtifactEnv(map[string]string{"ANTHROPIC_BASE_URL": v}) {
		return raw
	}
	envOut := make(map[string]any, len(truth))
	for k, v := range truth {
		envOut[k] = v
	}
	m["env"] = envOut
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return raw
	}
	return b
}

// handleKeychain 是钥匙串开关（产品决策 2026-08-29：默认关闭，显式开启才迁移）。
//   - GET → {"enabled": bool}
//   - PUT {"enabled": true} → 存偏好 + 存量明文一次性迁移，响应附 {migrated, failed}
//   - PUT {"enabled": false} → 仅存偏好（已占位化的条目不自动回迁，重新保存明文即恢复）
func (h *apiHandler) handleKeychain(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		pr, err := prefs.Load()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"enabled": pr.KeychainEnabled})
	case http.MethodPut:
		var in struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		pr, err := prefs.Load()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		pr.KeychainEnabled = in.Enabled
		if err := prefs.Save(pr); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !in.Enabled {
			writeJSON(w, http.StatusOK, map[string]any{"enabled": false})
			return
		}
		cfg, cerr := config.Load()
		if cerr != nil {
			writeError(w, http.StatusInternalServerError, cerr.Error())
			return
		}
		n, failed := migrateSecretsFn(cfg)
		if serr := config.Save(cfg); serr != nil {
			writeError(w, http.StatusInternalServerError, serr.Error())
			return
		}
		if failed == nil {
			failed = []string{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"enabled": true, "migrated": n, "failed": failed})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
