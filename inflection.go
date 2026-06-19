package gohygen

import (
	"regexp"
	"strings"
)

// This file ports the slice of the `inflection` and `change-case` behavior that
// hygen templates actually use. It is a pragmatic English inflector — not a full
// linguistic engine — covering the regular plural/singular rules plus the common
// irregulars, which is what scaffolding identifiers need.

type ruleRe struct {
	re   *regexp.Regexp
	repl string
}

func mustRule(pattern, repl string) ruleRe {
	return ruleRe{re: regexp.MustCompile(pattern), repl: repl}
}

var pluralRules = []ruleRe{
	mustRule(`(?i)(quiz)$`, "${1}zes"),
	mustRule(`(?i)^(ox)$`, "${1}en"),
	mustRule(`(?i)([m|l])ouse$`, "${1}ice"),
	mustRule(`(?i)(matr|vert|ind)ix|ex$`, "${1}ices"),
	mustRule(`(?i)(x|ch|ss|sh)$`, "${1}es"),
	mustRule(`(?i)([^aeiouy]|qu)y$`, "${1}ies"),
	mustRule(`(?i)(hive)$`, "${1}s"),
	mustRule(`(?i)(?:([^f])fe|([lr])f)$`, "${1}${2}ves"),
	mustRule(`(?i)(shea|lea|loa|thie)f$`, "${1}ves"),
	mustRule(`(?i)sis$`, "ses"),
	mustRule(`(?i)([ti])um$`, "${1}a"),
	mustRule(`(?i)(tomat|potat|ech|her|vet)o$`, "${1}oes"),
	mustRule(`(?i)(bu)s$`, "${1}ses"),
	mustRule(`(?i)(alias|status)$`, "${1}es"),
	mustRule(`(?i)(octop|vir)us$`, "${1}i"),
	mustRule(`(?i)(ax|test)is$`, "${1}es"),
	mustRule(`(?i)s$`, "s"),
	mustRule(`(?i)$`, "s"),
}

var singularRules = []ruleRe{
	mustRule(`(?i)(quiz)zes$`, "${1}"),
	mustRule(`(?i)(matr)ices$`, "${1}ix"),
	mustRule(`(?i)(vert|ind)ices$`, "${1}ex"),
	mustRule(`(?i)^(ox)en$`, "${1}"),
	mustRule(`(?i)(alias|status)(es)?$`, "${1}"),
	mustRule(`(?i)(octop|vir)(us|i)$`, "${1}us"),
	mustRule(`(?i)([ftw]ax)es$`, "${1}"),
	mustRule(`(?i)(cris|test)(is|es)$`, "${1}is"),
	mustRule(`(?i)(shoe)s$`, "${1}"),
	mustRule(`(?i)(o)es$`, "${1}"),
	mustRule(`(?i)(bus)(es)?$`, "${1}"),
	mustRule(`(?i)([m|l])ice$`, "${1}ouse"),
	mustRule(`(?i)(x|ch|ss|sh)es$`, "${1}"),
	mustRule(`(?i)(m)ovies$`, "${1}ovie"),
	mustRule(`(?i)(s)eries$`, "${1}eries"),
	mustRule(`(?i)([^aeiouy]|qu)ies$`, "${1}y"),
	mustRule(`(?i)([lr])ves$`, "${1}f"),
	mustRule(`(?i)(tive)s$`, "${1}"),
	mustRule(`(?i)(hive)s$`, "${1}"),
	mustRule(`(?i)(li|wi|kni)ves$`, "${1}fe"),
	mustRule(`(?i)(shea|loa|lea|thie)ves$`, "${1}f"),
	mustRule(`(?i)(^analy)(sis|ses)$`, "${1}sis"),
	mustRule(`(?i)(analy|ba|diagno|parenthe|progno|synop|the)(sis|ses)$`, "${1}sis"),
	mustRule(`(?i)([ti])a$`, "${1}um"),
	mustRule(`(?i)(n)ews$`, "${1}ews"),
	mustRule(`(?i)s$`, ""),
}

// irregulars maps singular->plural for words the regular rules get wrong.
var irregulars = map[string]string{
	"person": "people", "man": "men", "child": "children", "sex": "sexes",
	"move": "moves", "foot": "feet", "tooth": "teeth", "goose": "geese",
}

// uncountable words are returned unchanged by pluralize/singularize.
var uncountable = map[string]bool{
	"equipment": true, "information": true, "rice": true, "money": true,
	"species": true, "series": true, "fish": true, "sheep": true,
	"data": true, "deer": true, "news": true,
}

// Pluralize returns the plural form of an English word.
func Pluralize(word string) string {
	if word == "" {
		return word
	}
	lower := strings.ToLower(word)
	if uncountable[lower] {
		return word
	}
	if pl, ok := irregulars[lower]; ok {
		return preserveCase(word, pl)
	}
	for _, r := range pluralRules {
		if r.re.MatchString(word) {
			return r.re.ReplaceAllString(word, r.repl)
		}
	}
	return word
}

// Singularize returns the singular form of an English word.
func Singularize(word string) string {
	if word == "" {
		return word
	}
	lower := strings.ToLower(word)
	if uncountable[lower] {
		return word
	}
	for sing, pl := range irregulars {
		if lower == pl {
			return preserveCase(word, sing)
		}
	}
	for _, r := range singularRules {
		if r.re.MatchString(word) {
			return r.re.ReplaceAllString(word, r.repl)
		}
	}
	return word
}

// preserveCase applies the leading-capital of src onto dst.
func preserveCase(src, dst string) string {
	if src == "" || dst == "" {
		return dst
	}
	if strings.ToUpper(src[:1]) == src[:1] {
		return capitalize(dst)
	}
	return dst
}

// Underscore converts a CamelCased or dasherized word to snake_case (lower).
func Underscore(s string) string {
	return strings.Join(words(s), "_")
}

func dasherize(s string) string {
	return strings.ReplaceAll(s, "_", "-")
}

func humanize(s string) string {
	w := words(strings.TrimSuffix(s, "_id"))
	if len(w) == 0 {
		return ""
	}
	return capitalize(strings.Join(w, " "))
}

func titleize(s string) string {
	w := words(s)
	for i := range w {
		w[i] = capitalize(w[i])
	}
	return strings.Join(w, " ")
}

// classify singularizes and PascalCases (e.g. "user_accounts" -> "UserAccount").
func classify(s string) string {
	return pascalCase(Singularize(s))
}

// tableize pluralizes and snake_cases (e.g. "UserAccount" -> "user_accounts").
func tableize(s string) string {
	return Pluralize(Underscore(s))
}

// demodulize strips a namespace prefix ("Foo::Bar" or "foo.Bar" -> "Bar").
func demodulize(s string) string {
	if i := strings.LastIndex(s, "::"); i >= 0 {
		return s[i+2:]
	}
	if i := strings.LastIndex(s, "."); i >= 0 {
		return s[i+1:]
	}
	return s
}

// undasherize turns kebab/snake case into PascalCase (hygen's added helper).
func undasherize(s string) string {
	parts := regexp.MustCompile(`[-_]`).Split(s, -1)
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]) + strings.ToLower(p[1:]))
	}
	return b.String()
}

// camelize converts to CamelCase; when lowerFirst is true the first letter is
// lowercased (matching inflection.camelize(str, true)).
func camelize(s string, lowerFirst bool) string {
	out := pascalCase(s)
	if lowerFirst && out != "" {
		out = strings.ToLower(out[:1]) + out[1:]
	}
	return out
}

var reOrdinal = regexp.MustCompile(`\d+`)

// ordinalize appends English ordinal suffixes to the numbers in a string.
func ordinalize(s string) string {
	return reOrdinal.ReplaceAllStringFunc(s, func(num string) string {
		return num + ordinalSuffix(num)
	})
}

func ordinalSuffix(num string) string {
	n := 0
	for _, c := range num {
		n = n*10 + int(c-'0')
	}
	if n%100 >= 11 && n%100 <= 13 {
		return "th"
	}
	switch n % 10 {
	case 1:
		return "st"
	case 2:
		return "nd"
	case 3:
		return "rd"
	default:
		return "th"
	}
}
