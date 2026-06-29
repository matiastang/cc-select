package config

import "testing"

func TestProvider_LookupAndMissing(t *testing.T) {
	c := &Config{Providers: map[string]Provider{
		"glm": {ID: "glm", Name: "智谱 GLM"},
	}}

	got, err := c.Provider("glm")
	if err != nil {
		t.Fatalf("已存在的 provider 不应报错: %v", err)
	}
	if got.ID != "glm" || got.Name != "智谱 GLM" {
		t.Errorf("Provider 返回错误内容: %+v", got)
	}

	if _, err := c.Provider("nope"); err == nil {
		t.Error("不存在的 provider 应返回错误")
	}
}

func TestValidateID(t *testing.T) {
	valid := []string{"glm", "deepseek", "claude-official", "my_provider", "v1.2", "A-Z_0.9"}
	for _, id := range valid {
		if err := ValidateID(id); err != nil {
			t.Errorf("%q 应合法，got %v", id, err)
		}
	}

	invalid := []string{
		"",                  // 空
		".",                 // 当前目录
		"..",                // 上级目录
		"../evil",           // 路径穿越
		"../../tmp/x",       // 路径穿越
		"a/b",               // 含分隔符
		`a\b`,               // Windows 分隔符
		"with space",        // 空白
		"semi;colon",        // 特殊字符
		"name$VAR",          // shell 元字符
	}
	for _, id := range invalid {
		if err := ValidateID(id); err == nil {
			t.Errorf("%q 应被拒绝", id)
		}
	}
}
