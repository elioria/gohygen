package gohygen

import (
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/elioria/goejs"
)

// Helpers is the `h` namespace injected into every template's locals. It mirrors
// hygen's helpers.ts: a capitalize function plus the inflection, changeCase, and
// path sub-namespaces. The values are goejs objects/functions so templates can
// call e.g. h.inflection.pluralize(name) or h.changeCase.paramCase(name).
//
// Custom helpers supplied through Config.Helpers are merged on top, overriding
// the built-ins by key.
func buildHelpers(custom map[string]any) map[string]any {
	h := map[string]any{
		"capitalize":   goFn1(capitalize),
		"inflection":   inflectionNS(),
		"changeCase":   changeCaseNS(),
		"path":         pathNS(),
	}
	for k, v := range custom {
		h[k] = v
	}
	return h
}

// goFn1 adapts a Go func(string) string into a value callable from templates.
func goFn1(fn func(string) string) func(args ...any) any {
	return func(args ...any) any {
		s := ""
		if len(args) > 0 {
			s = toStr(args[0])
		}
		return fn(s)
	}
}

func toStr(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return goejs.ToString(v)
}

// --- capitalize ---

// capitalize uppercases only the first character, matching hygen's helper.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = []rune(strings.ToUpper(string(r[0])))[0]
	return string(r)
}

// --- inflection namespace ---

func inflectionNS() map[string]any {
	return map[string]any{
		"pluralize":   goFn1(Pluralize),
		"singularize": goFn1(Singularize),
		"camelize":    goFnVar(camelize),
		"underscore":  goFn1(Underscore),
		"humanize":    goFn1(humanize),
		"capitalize":  goFn1(capitalize),
		"dasherize":   goFn1(dasherize),
		"titleize":    goFn1(titleize),
		"demodulize":  goFn1(demodulize),
		"tableize":    goFn1(tableize),
		"classify":    goFn1(classify),
		"undasherize": goFn1(undasherize),
		"ordinalize":  goFn1(ordinalize),
	}
}

// goFnVar adapts a func taking a string and an optional bool flag.
func goFnVar(fn func(string, bool) string) func(args ...any) any {
	return func(args ...any) any {
		s := ""
		if len(args) > 0 {
			s = toStr(args[0])
		}
		flag := false
		if len(args) > 1 {
			flag = goejs.ToBoolean(args[1])
		}
		return fn(s, flag)
	}
}

// --- changeCase namespace ---

func changeCaseNS() map[string]any {
	return map[string]any{
		"camelCase":    goFn1(camelCase),
		"pascalCase":   goFn1(pascalCase),
		"snakeCase":    goFn1(snakeCase),
		"paramCase":    goFn1(paramCase),
		"kebabCase":    goFn1(paramCase),
		"constantCase": goFn1(constantCase),
		"dotCase":      goFn1(dotCase),
		"pathCase":     goFn1(pathCase),
		"sentenceCase": goFn1(sentenceCase),
		"titleCase":    goFn1(titleCaseWords),
		"noCase":       goFn1(noCase),
		"headerCase":   goFn1(headerCase),
		"lowerCase":    goFn1(strings.ToLower),
		"upperCase":    goFn1(strings.ToUpper),
	}
}

// --- path namespace ---

func pathNS() map[string]any {
	return map[string]any{
		"relative": func(args ...any) any {
			if len(args) < 2 {
				return ""
			}
			r, err := filepath.Rel(toStr(args[0]), toStr(args[1]))
			if err != nil {
				return ""
			}
			return filepath.ToSlash(r)
		},
		"join": func(args ...any) any {
			parts := make([]string, len(args))
			for i, a := range args {
				parts[i] = toStr(a)
			}
			return path.Join(parts...)
		},
		"basename": func(args ...any) any {
			if len(args) == 0 {
				return ""
			}
			b := path.Base(toStr(args[0]))
			if len(args) > 1 {
				b = strings.TrimSuffix(b, toStr(args[1]))
			}
			return b
		},
		"dirname": func(args ...any) any {
			if len(args) == 0 {
				return ""
			}
			return path.Dir(toStr(args[0]))
		},
		"extname": func(args ...any) any {
			if len(args) == 0 {
				return ""
			}
			return path.Ext(toStr(args[0]))
		},
	}
}

// --- word-splitting core shared by the case helpers ---

var (
	reCamelBoundary = regexp.MustCompile(`([a-z0-9])([A-Z])`)
	reSplitNonAlnum = regexp.MustCompile(`[^a-zA-Z0-9]+`)
)

// words breaks an identifier of any casing (camelCase, snake_case, kebab-case,
// "space separated") into its lowercase component words.
func words(s string) []string {
	s = reCamelBoundary.ReplaceAllString(s, "$1 $2")
	s = reSplitNonAlnum.ReplaceAllString(s, " ")
	fields := strings.Fields(s)
	for i, w := range fields {
		fields[i] = strings.ToLower(w)
	}
	return fields
}

func camelCase(s string) string {
	w := words(s)
	if len(w) == 0 {
		return ""
	}
	out := w[0]
	for _, p := range w[1:] {
		out += capitalize(p)
	}
	return out
}

func pascalCase(s string) string {
	var b strings.Builder
	for _, p := range words(s) {
		b.WriteString(capitalize(p))
	}
	return b.String()
}

func snakeCase(s string) string    { return strings.Join(words(s), "_") }
func paramCase(s string) string    { return strings.Join(words(s), "-") }
func dotCase(s string) string      { return strings.Join(words(s), ".") }
func pathCase(s string) string     { return strings.Join(words(s), "/") }
func noCase(s string) string       { return strings.Join(words(s), " ") }
func constantCase(s string) string { return strings.ToUpper(strings.Join(words(s), "_")) }

func sentenceCase(s string) string {
	w := words(s)
	if len(w) == 0 {
		return ""
	}
	return capitalize(strings.Join(w, " "))
}

func titleCaseWords(s string) string {
	w := words(s)
	for i := range w {
		w[i] = capitalize(w[i])
	}
	return strings.Join(w, " ")
}

func headerCase(s string) string {
	w := words(s)
	for i := range w {
		w[i] = capitalize(w[i])
	}
	return strings.Join(w, "-")
}
