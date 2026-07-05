// Package i18n provides internationalization support for cc-select.
package i18n

import (
	"errors"
	"fmt"
	"sync"
)

// Locale is a supported language code.
type Locale string

const (
	// DefaultLocale is the fallback locale.
	DefaultLocale Locale = "en"
	// EN is English.
	EN Locale = "en"
	// ZH is Simplified Chinese.
	ZH Locale = "zh"
)

// SupportedLocales lists all supported locales.
var SupportedLocales = []Locale{EN, ZH}

// Bundle holds loaded translations and the active locale.
type Bundle struct {
	mu      sync.RWMutex
	locale  Locale
	catalog map[string]map[Locale]string // key -> locale -> message
}

// New creates a new Bundle from a loaded catalog.
func New(catalog map[string]map[Locale]string) *Bundle {
	return &Bundle{
		locale:  DefaultLocale,
		catalog: catalog,
	}
}

// SetLocale changes the active locale. Unsupported values fall back to DefaultLocale.
func (b *Bundle) SetLocale(l Locale) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if IsSupportedLocale(string(l)) {
		b.locale = l
	} else {
		b.locale = DefaultLocale
	}
}

// Locale returns the active locale.
func (b *Bundle) Locale() Locale {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.locale
}

// T returns the translation for key in the active locale, falling back to
// English and then to the raw key. It supports fmt.Sprintf-style formatting.
func (b *Bundle) T(key string, args ...any) string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	localeMap, ok := b.catalog[key]
	if !ok {
		return key
	}
	msg := localeMap[b.locale]
	if msg == "" {
		msg = localeMap[DefaultLocale]
	}
	if msg == "" {
		return key
	}
	if len(args) == 0 {
		return msg
	}
	return fmt.Sprintf(msg, args...)
}

var defaultBundle *Bundle

func init() {
	catalog, err := loadCatalog()
	if err != nil {
		catalog = make(map[string]map[Locale]string)
	}
	defaultBundle = New(catalog)
}

// E returns an error with the translated message for key.
// It is a convenience for errors.New(T(key, args...)).
func E(key string, args ...any) error {
	return errors.New(T(key, args...))
}

// Ew returns an error that wraps wrapErr, with a translated prefix built from key.
// The translation may contain fmt.Sprintf-style verbs for args, but must NOT
// contain %w (the wrapped error is appended after formatting).
func Ew(key string, wrapErr error, args ...any) error {
	return fmt.Errorf("%s: %w", T(key, args...), wrapErr)
}

// SetLocale sets the active locale for the default bundle.
func SetLocale(l Locale) {
	defaultBundle.SetLocale(l)
}

// T returns the translation from the default bundle.
func T(key string, args ...any) string {
	return defaultBundle.T(key, args...)
}

// CurrentLocale returns the active locale of the default bundle.
func CurrentLocale() Locale {
	return defaultBundle.Locale()
}
