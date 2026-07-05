package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed locales/*.json
var localeFS embed.FS

// loadCatalog reads all embedded locale files and flattens nested JSON
// objects into dot-separated keys.
func loadCatalog() (map[string]map[Locale]string, error) {
	entries, err := localeFS.ReadDir("locales")
	if err != nil {
		return nil, err
	}
	catalog := make(map[string]map[Locale]string)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		lang := strings.TrimSuffix(e.Name(), ".json")
		if !IsSupportedLocale(lang) {
			continue
		}
		data, err := localeFS.ReadFile("locales/" + e.Name())
		if err != nil {
			return nil, err
		}
		var root map[string]any
		if err := json.Unmarshal(data, &root); err != nil {
			return nil, fmt.Errorf("parse %s: %w", e.Name(), err)
		}
		flatten(root, "", Locale(lang), catalog)
	}
	return catalog, nil
}

func flatten(node map[string]any, prefix string, locale Locale, catalog map[string]map[Locale]string) {
	for k, v := range node {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch val := v.(type) {
		case string:
			if catalog[key] == nil {
				catalog[key] = make(map[Locale]string)
			}
			catalog[key][locale] = val
		case map[string]any:
			flatten(val, key, locale, catalog)
		default:
			// Ignore non-string/non-map values.
		}
	}
}
