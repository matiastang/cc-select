package presets

import (
	"encoding/json"
	"strings"
	"testing"
)

// 合法最小 bundle 的构造辅助。
func validBundle() map[string]any {
	return map[string]any{
		"schemaVersion": 1,
		"presets": []map[string]any{
			{
				"id":           "demo",
				"displayName":  "Demo",
				"category":     "cn_official",
				"apiFormat":    "anthropic",
				"authField":    "ANTHROPIC_AUTH_TOKEN",
				"envTemplate":  map[string]string{"ANTHROPIC_BASE_URL": "https://demo.example.com", "ANTHROPIC_AUTH_TOKEN": "${API_KEY}", "ANTHROPIC_MODEL": "demo-max"},
				"requiredVars": []string{"ANTHROPIC_AUTH_TOKEN"},
				"optionalVars": []string{"ANTHROPIC_MODEL"},
				"oauth":        false,
			},
		},
	}
}

func marshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestValidate_AcceptsValidBundle(t *testing.T) {
	if err := Validate(marshal(t, validBundle())); err != nil {
		t.Fatalf("valid bundle rejected: %v", err)
	}
}

func TestValidate_RejectsWrongSchemaVersion(t *testing.T) {
	b := validBundle()
	b["schemaVersion"] = 99
	if err := Validate(marshal(t, b)); err == nil || !strings.Contains(err.Error(), "schemaVersion") {
		t.Fatalf("want schemaVersion error, got %v", err)
	}
}

func TestValidate_RejectsDuplicateID(t *testing.T) {
	b := validBundle()
	dup := map[string]any{}
	for k, v := range b["presets"].([]map[string]any)[0] {
		dup[k] = v
	}
	b["presets"] = append(b["presets"].([]map[string]any), dup)
	if err := Validate(marshal(t, b)); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("want duplicate id error, got %v", err)
	}
}

func TestValidate_RejectsMissingRequiredFields(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(p map[string]any)
		want   string
	}{
		{"no id", func(p map[string]any) { p["id"] = "" }, "id"},
		{"no displayName", func(p map[string]any) { delete(p, "displayName") }, "displayName"},
		{"bad category", func(p map[string]any) { p["category"] = "nope" }, "category"},
		{"bad apiFormat", func(p map[string]any) { p["apiFormat"] = "nope" }, "apiFormat"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := validBundle()
			tc.mutate(b["presets"].([]map[string]any)[0])
			err := Validate(marshal(t, b))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q error, got %v", tc.want, err)
			}
		})
	}
}

func TestValidate_RejectsUndeclaredPlaceholder(t *testing.T) {
	b := validBundle()
	b["presets"].([]map[string]any)[0]["envTemplate"] = map[string]string{
		"ANTHROPIC_BASE_URL": "https://x.example.com/${REGION}",
	}
	err := Validate(marshal(t, b))
	if err == nil || !strings.Contains(err.Error(), "REGION") {
		t.Fatalf("want placeholder error, got %v", err)
	}
}

func TestValidate_RejectsHTTPURL(t *testing.T) {
	b := validBundle()
	b["presets"].([]map[string]any)[0]["apiKeyURL"] = "http://insecure.example.com"
	if err := Validate(marshal(t, b)); err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("want https error, got %v", err)
	}
}

func TestValidate_OAuthPresetNeedsNoAuthField(t *testing.T) {
	b := validBundle()
	p := b["presets"].([]map[string]any)[0]
	delete(p, "authField")
	delete(p, "envTemplate")
	p["oauth"] = true
	p["requiredVars"] = []string{}
	if err := Validate(marshal(t, b)); err != nil {
		t.Fatalf("oauth preset rejected: %v", err)
	}
}

func TestValidate_RejectsMissingAuthField(t *testing.T) {
	b := validBundle()
	delete(b["presets"].([]map[string]any)[0], "authField")
	if err := Validate(marshal(t, b)); err == nil || !strings.Contains(err.Error(), "authField") {
		t.Fatalf("want authField error, got %v", err)
	}
}

func TestEmbeddedBundlePassesValidation(t *testing.T) {
	if err := Validate(embeddedPresets); err != nil {
		t.Fatalf("embedded presets.json invalid: %v", err)
	}
}

func TestPresetCatalogContents(t *testing.T) {
	// v0.1.1 契约（contracts/presets-and-usage §1.3）：新增 MiniMax 与通义千问，
	// GLM 系默认模型保鲜至 glm-5.3。
	for _, id := range []string{"minimax", "qwen-coding", "zhipu-glm", "zhipu-glm-en", "custom"} {
		if _, ok := ByID(id); !ok {
			t.Errorf("preset %q missing", id)
		}
	}
	glm, _ := ByID("zhipu-glm")
	if glm.EnvTemplate["ANTHROPIC_MODEL"] != "glm-5.3" {
		t.Errorf("zhipu-glm default model = %q, want glm-5.3", glm.EnvTemplate["ANTHROPIC_MODEL"])
	}
	if glm.EnvTemplate["ANTHROPIC_DEFAULT_HAIKU_MODEL"] != "glm-5.3-flash" {
		t.Errorf("zhipu-glm haiku model = %q, want glm-5.3-flash", glm.EnvTemplate["ANTHROPIC_DEFAULT_HAIKU_MODEL"])
	}
	mm, _ := ByID("minimax")
	if mm.EnvTemplate["ANTHROPIC_BASE_URL"] != "https://api.minimax.io/anthropic" {
		t.Errorf("minimax base url = %q", mm.EnvTemplate["ANTHROPIC_BASE_URL"])
	}
	qw, _ := ByID("qwen-coding")
	if qw.EnvTemplate["ANTHROPIC_BASE_URL"] != "https://coding.dashscope.aliyuncs.com/apps/anthropic" {
		t.Errorf("qwen-coding base url = %q", qw.EnvTemplate["ANTHROPIC_BASE_URL"])
	}
}
