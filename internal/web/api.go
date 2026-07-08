package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/cc-select/cc-select/internal/app"
	"github.com/cc-select/cc-select/internal/config"
	"github.com/cc-select/cc-select/internal/i18n"
	"github.com/cc-select/cc-select/internal/prefs"
	"github.com/cc-select/cc-select/internal/profile"
	"github.com/cc-select/cc-select/internal/rcinteg"
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
}

// apiHandler 持有依赖，处理 /api/v1/* 路由。
type apiHandler struct{}

func newAPIHandler() *apiHandler { return &apiHandler{} }

func (h *apiHandler) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/providers", h.handleProvidersCollection)
	mux.HandleFunc("/api/v1/providers/", h.handleProviderItem)
	mux.HandleFunc("/api/v1/mode", h.handleMode)
	mux.HandleFunc("/api/v1/language", h.handleLanguage)
	mux.HandleFunc("/api/v1/shell-integration", h.handleShellIntegration)
	mux.HandleFunc("/api/v1/shell-integration/install", h.handleShellIntegrationInstall)
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

// handleMode 处理 GET/PUT 全局隔离模式（~/.cc-select/prefs.json）。
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
		if in.IsolationMode != prefs.ModeSettingsOnly && in.IsolationMode != prefs.ModeFull {
			writeError(w, http.StatusBadRequest, "isolationMode must be settings-only or full")
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
		Shell string `json:"shell"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in) // shell 可选，空=自动检测
	res, err := rcinteg.Install(in.Shell)
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
		ID            string          `json:"id"`
		Name          string          `json:"name"`
		Settings      json.RawMessage `json:"settings"`
		IsolationMode prefs.Mode      `json:"isolationMode"`
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
		writeError(w, http.StatusBadRequest, "isolationMode must be empty, settings-only or full")
		return
	}
	data, err := normalizeSettings(in.Settings)
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
	if err := applySettings(in.ID, data, mode); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.Config.Providers[in.ID] = config.Provider{
		ID:            in.ID,
		Name:          in.Name,
		IsolationMode: in.IsolationMode,
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
		Name          string          `json:"name"`
		Settings      json.RawMessage `json:"settings"`
		IsolationMode prefs.Mode      `json:"isolationMode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if !in.IsolationMode.Valid() {
		writeError(w, http.StatusBadRequest, "isolationMode must be empty, settings-only or full")
		return
	}
	if in.Name == "" {
		in.Name = id
	}
	data, err := normalizeSettings(in.Settings)
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
	// 整体覆盖：按 mode 写 profile。
	if err := applySettings(id, data, mode); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.Config.Providers[id] = config.Provider{
		ID:            id,
		Name:          in.Name,
		IsolationMode: in.IsolationMode,
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

// applySettings 按 mode 把 settings 写入 profile。
//   - Mode A（full）：原文写入 data，保留 env 之外字段（permissions、model 等）。
//   - Mode B（settings-only）：只取 env 做整体替换，非 env 字段来自全局 ~/.claude/settings.json。
// data 应是已校验/规范化的 JSON 对象字节。官方 provider 无 profile（no-op）。
func applySettings(id string, data []byte, mode prefs.Mode) error {
	switch mode {
	case prefs.ModeFull:
		if _, err := profile.EnsureRaw(id, data); err != nil {
			return err
		}
	default:
		var settings map[string]any
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf(i18n.T("profile.parseGlobalSettings"), err)
		}
		envAny, _ := settings["env"].(map[string]any)
		env := map[string]string{}
		for k, v := range envAny {
			if s, ok := v.(string); ok {
				env[k] = s
			}
		}
		if _, _, err := profile.Sync(id, env, mode); err != nil {
			return err
		}
	}
	return nil
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
	env, _ := profile.ReadEnv(p.ID)
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
	return providerDetailDTO{
		ID:            id,
		Name:          p.DisplayName(),
		Settings:      json.RawMessage(raw),
		IsolationMode: string(p.IsolationMode),
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
