package presets

import (
	"testing"
)

func TestAll_ReturnsCopy(t *testing.T) {
	got := All()
	if len(got) == 0 {
		t.Fatal("All() should return non-empty presets")
	}
	// Mutate returned slice and ensure builtin is not affected.
	got[0] = Preset{}
	again := All()
	if again[0].ID == "" {
		t.Error("All() returned a slice that aliases builtin")
	}
}

func TestByID(t *testing.T) {
	cases := []struct {
		id      string
		wantOK  bool
		wantName string
	}{
		{"deepseek", true, "DeepSeek"},
		{"custom", true, "自定义配置"},
		{"not-exist", false, ""},
	}
	for _, tc := range cases {
		p, ok := ByID(tc.id)
		if ok != tc.wantOK {
			t.Errorf("ByID(%q) ok=%v, want %v", tc.id, ok, tc.wantOK)
		}
		if ok && p.DisplayName != tc.wantName {
			t.Errorf("ByID(%q) name=%q, want %q", tc.id, p.DisplayName, tc.wantName)
		}
	}
}

func TestCategories(t *testing.T) {
	cats := Categories()
	if len(cats) == 0 {
		t.Fatal("Categories() should return non-empty categories")
	}
	seen := map[Category]int{}
	for _, c := range cats {
		seen[c]++
		if seen[c] > 1 {
			t.Errorf("category %q duplicated", c)
		}
	}
}

func TestApply_KeepsTemplateAndOverrides(t *testing.T) {
	p, ok := ByID("deepseek")
	if !ok {
		t.Fatal("deepseek preset not found")
	}
	overrides := map[string]string{
		"ANTHROPIC_MODEL": "custom-model",
	}
	env := Apply(p, overrides)
	if env["ANTHROPIC_BASE_URL"] != "https://api.deepseek.com/anthropic" {
		t.Errorf("base url not preserved, got %q", env["ANTHROPIC_BASE_URL"])
	}
	if env["ANTHROPIC_MODEL"] != "custom-model" {
		t.Errorf("model override not applied, got %q", env["ANTHROPIC_MODEL"])
	}
	if env["ANTHROPIC_AUTH_TOKEN"] != "${API_KEY}" {
		t.Errorf("auth token placeholder not preserved, got %q", env["ANTHROPIC_AUTH_TOKEN"])
	}
}

func TestApply_EmptyOverrideIgnored(t *testing.T) {
	p, ok := ByID("deepseek")
	if !ok {
		t.Fatal("deepseek preset not found")
	}
	env := Apply(p, map[string]string{"ANTHROPIC_MODEL": ""})
	if env["ANTHROPIC_MODEL"] != "deepseek-v4-pro" {
		t.Errorf("empty override should be ignored, got %q", env["ANTHROPIC_MODEL"])
	}
}

func TestExpand_ReplacesPlaceholders(t *testing.T) {
	env := map[string]string{
		"ANTHROPIC_BASE_URL":   "https://api.example.com/${REGION}/v1",
		"ANTHROPIC_AUTH_TOKEN": "${API_KEY}",
	}
	provided := map[string]string{
		"API_KEY": "sk-123",
		"REGION":  "us-east",
	}
	out, missing := Expand(env, provided)
	if len(missing) != 0 {
		t.Fatalf("unexpected missing placeholders: %v", missing)
	}
	if out["ANTHROPIC_AUTH_TOKEN"] != "sk-123" {
		t.Errorf("auth token=%q, want sk-123", out["ANTHROPIC_AUTH_TOKEN"])
	}
	if out["ANTHROPIC_BASE_URL"] != "https://api.example.com/us-east/v1" {
		t.Errorf("base url=%q", out["ANTHROPIC_BASE_URL"])
	}
}

func TestExpand_ReportsMissing(t *testing.T) {
	env := map[string]string{
		"ANTHROPIC_AUTH_TOKEN": "${API_KEY}",
		"AWS_REGION":           "${REGION}",
	}
	out, missing := Expand(env, nil)
	if len(missing) != 2 {
		t.Fatalf("want 2 missing, got %v", missing)
	}
	if missing[0] != "API_KEY" || missing[1] != "REGION" {
		t.Errorf("missing placeholders sorted incorrectly: %v", missing)
	}
	if out["ANTHROPIC_AUTH_TOKEN"] != "${API_KEY}" {
		t.Errorf("unresolved placeholder should be preserved, got %q", out["ANTHROPIC_AUTH_TOKEN"])
	}
}

func TestRequiredMissing(t *testing.T) {
	p, ok := ByID("deepseek")
	if !ok {
		t.Fatal("deepseek preset not found")
	}
	env := Apply(p, nil)
	missing := RequiredMissing(p, env)
	if len(missing) != 1 || missing[0] != "ANTHROPIC_AUTH_TOKEN" {
		t.Errorf("want [ANTHROPIC_AUTH_TOKEN], got %v", missing)
	}

	// Fill with actual key (still placeholder form).
	env["ANTHROPIC_AUTH_TOKEN"] = "sk-123"
	missing = RequiredMissing(p, env)
	if len(missing) != 0 {
		t.Errorf("want no missing, got %v", missing)
	}
}

func TestRequiredMissing_PlaceholderStillMissing(t *testing.T) {
	p, ok := ByID("deepseek")
	if !ok {
		t.Fatal("deepseek preset not found")
	}
	env := Apply(p, nil)
	env["ANTHROPIC_AUTH_TOKEN"] = "${API_KEY}"
	missing := RequiredMissing(p, env)
	if len(missing) != 1 || missing[0] != "ANTHROPIC_AUTH_TOKEN" {
		t.Errorf("placeholder should count as missing, got %v", missing)
	}
}

func TestPlaceholdersIn(t *testing.T) {
	env := map[string]string{
		"A": "${X}",
		"B": "${Y}${Z}",
		"C": "no-placeholder",
	}
	got := PlaceholdersIn(env)
	want := []string{"X", "Y", "Z"}
	if len(got) != len(want) {
		t.Fatalf("want %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("index %d: want %q got %q", i, want[i], got[i])
		}
	}
}

func TestBuildEnv_DeepSeek(t *testing.T) {
	env, missing, err := BuildEnv("deepseek", "sk-123", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Fatalf("unexpected missing: %v", missing)
	}
	if env["ANTHROPIC_BASE_URL"] != "https://api.deepseek.com/anthropic" {
		t.Errorf("base url=%q", env["ANTHROPIC_BASE_URL"])
	}
	if env["ANTHROPIC_AUTH_TOKEN"] != "sk-123" {
		t.Errorf("auth token=%q", env["ANTHROPIC_AUTH_TOKEN"])
	}
	if env["ANTHROPIC_MODEL"] != "deepseek-v4-pro" {
		t.Errorf("model=%q", env["ANTHROPIC_MODEL"])
	}
}

func TestBuildEnv_MissingKey(t *testing.T) {
	_, missing, err := BuildEnv("deepseek", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 1 || missing[0] != "ANTHROPIC_AUTH_TOKEN" {
		t.Errorf("want missing API key, got %v", missing)
	}
}

func TestBuildEnv_UnknownPreset(t *testing.T) {
	_, _, err := BuildEnv("unknown", "", nil)
	if err == nil {
		t.Error("expected error for unknown preset")
	}
}

func TestBuildEnv_AWSBedrockAKSK(t *testing.T) {
	env, missing, err := BuildEnv("aws-bedrock-aksk", "bedrock-key", map[string]string{
		"AWS_ACCESS_KEY_ID":     "AK",
		"AWS_SECRET_ACCESS_KEY": "SK",
		"AWS_REGION":            "us-west-2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Fatalf("unexpected missing: %v", missing)
	}
	if env["ANTHROPIC_BASE_URL"] != "https://bedrock-runtime.us-west-2.amazonaws.com" {
		t.Errorf("base url=%q", env["ANTHROPIC_BASE_URL"])
	}
	if env["ANTHROPIC_AUTH_TOKEN"] != "bedrock-key" {
		t.Errorf("auth token=%q", env["ANTHROPIC_AUTH_TOKEN"])
	}
	if env["AWS_ACCESS_KEY_ID"] != "AK" {
		t.Errorf("aws access key=%q", env["AWS_ACCESS_KEY_ID"])
	}
}

func TestBuildEnv_OAuthNoKeyNeeded(t *testing.T) {
	env, missing, err := BuildEnv("github-copilot", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Fatalf("oauth preset should not require key, got %v", missing)
	}
	if env["ANTHROPIC_BASE_URL"] != "https://api.githubcopilot.com" {
		t.Errorf("base url=%q", env["ANTHROPIC_BASE_URL"])
	}
}

func TestAllBuiltin_HaveRequiredMetadata(t *testing.T) {
	for _, p := range builtin {
		if p.ID == "" {
			t.Errorf("preset has empty id: %+v", p)
		}
		if p.DisplayName == "" {
			t.Errorf("preset %q has empty display name", p.ID)
		}
		if p.Category == "" {
			t.Errorf("preset %q has empty category", p.ID)
		}
		if !p.OAuth {
			if p.AuthField == "" {
				t.Errorf("preset %q non-oauth but no auth field", p.ID)
			}
			found := false
			for k := range p.EnvTemplate {
				if k == string(p.AuthField) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("preset %q auth field %q not in env template", p.ID, p.AuthField)
			}
		}
	}
}

func TestAllBuiltin_UniqueIDs(t *testing.T) {
	seen := map[string]struct{}{}
	for _, p := range builtin {
		if _, ok := seen[p.ID]; ok {
			t.Errorf("duplicate preset id %q", p.ID)
		}
		seen[p.ID] = struct{}{}
	}
}
