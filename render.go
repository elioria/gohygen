package gohygen

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/elioria/goejs"
	"gopkg.in/yaml.v3"
)

// RenderedAction is one template file after frontmatter + body have been
// rendered. attributes holds the parsed (and EJS-rendered) frontmatter; body is
// the rendered template body that an op will write or inject.
type RenderedAction struct {
	File       string
	Attributes Frontmatter
	Body       string
}

// Frontmatter is the parsed YAML header of a template, with the keys hygen
// recognizes. Unknown keys are kept in Extra. Regex-valued keys (before/after/
// skip_if expressed as !!js/regexp) are stored as compiled *Regexp via the
// rendered string; the ops compile them as needed.
type Frontmatter struct {
	To           string
	From         string
	Force        bool
	UnlessExists bool
	Inject       bool
	Before       string
	After        string
	Prepend      bool
	Append       bool
	AtLine       int
	hasAtLine    bool
	EOFLast      *bool
	SkipIf       string
	Sh           string
	ShIgnoreExit bool
	Echo         string
	Message      string
	Unless       string

	// regexp markers: true when the corresponding value came from a
	// !!js/regexp YAML tag (so the op treats it as a real regex, honoring flags).
	BeforeRe bool
	AfterRe  bool
	SkipIfRe bool

	Extra map[string]any
}

var fenceSplit = regexp.MustCompile(`(?m)^---\s*$`)

// splitFrontmatter separates the YAML header from the body. A file may start
// with the `---` fence directly or after leading whitespace. When no frontmatter
// is present, the whole text is treated as the body with empty attributes.
func splitFrontmatter(text string) (header string, body string) {
	trimmed := strings.TrimLeft(text, " \t\r\n")
	if !strings.HasPrefix(trimmed, "---") {
		return "", text
	}
	// Work from the position of the first fence.
	idx := strings.Index(text, "---")
	rest := text[idx+3:]
	// Find the closing fence on its own line.
	loc := fenceSplit.FindStringIndex(rest)
	if loc == nil {
		return "", text
	}
	header = rest[:loc[0]]
	body = rest[loc[1]:]
	// Drop a single leading newline of the body left by the fence line.
	body = strings.TrimPrefix(body, "\n")
	body = strings.TrimPrefix(body, "\r\n")
	return header, body
}

// slashRegex matches a slash-delimited regex literal: /pattern/flags
var slashRegex = regexp.MustCompile(`^/(.*)/([a-z]*)$`)

// parseHeader decodes the YAML frontmatter into a value map while recording
// which keys carried the !!js/regexp tag. yaml.v3 strips custom tags from the
// decoded value, so we walk the document node to recover them.
func parseHeader(header string) (map[string]any, map[string]bool, error) {
	out := map[string]any{}
	tagged := map[string]bool{}
	if strings.TrimSpace(header) == "" {
		return out, tagged, nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(header), &doc); err != nil {
		return nil, nil, err
	}
	if len(doc.Content) == 0 {
		return out, tagged, nil
	}
	mapping := doc.Content[0]
	if mapping.Kind != yaml.MappingNode {
		// fall back to a plain decode for non-mapping headers
		if err := yaml.Unmarshal([]byte(header), &out); err != nil {
			return nil, nil, err
		}
		return out, tagged, nil
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		keyNode := mapping.Content[i]
		valNode := mapping.Content[i+1]
		key := keyNode.Value
		if valNode.Tag == "!!js/regexp" || strings.Contains(valNode.Tag, "js/regexp") {
			tagged[key] = true
			out[key] = valNode.Value
			continue
		}
		var v any
		if err := valNode.Decode(&v); err != nil {
			out[key] = valNode.Value
			continue
		}
		out[key] = v
	}
	return out, tagged, nil
}

// regexValue determines whether a frontmatter string value should be treated as
// a regex. It is a regex when the YAML key carried the !!js/regexp tag, or when
// the value is written in inline /pattern/flags form. Returns the bare pattern,
// whether it is a regex, and the flag string.
func regexValue(val string, yamlTagged bool) (pattern string, isRegex bool, flags string) {
	if m := slashRegex.FindStringSubmatch(strings.TrimSpace(val)); m != nil {
		return m[1], true, m[2]
	}
	if yamlTagged {
		return val, true, ""
	}
	return val, false, ""
}

// renderFile renders one template file's frontmatter and body against locals.
// The frontmatter is parsed to discover which scalar values to EJS-render, each
// string value is rendered, the header is re-parsed into a typed Frontmatter,
// and finally the body is rendered with locals plus an `attributes` object.
func renderFile(file, text string, locals map[string]any, eng *Engine) (*RenderedAction, error) {
	header, body := splitFrontmatter(text)

	rawAttrs, regexTagged, err := parseHeader(header)
	if err != nil {
		return nil, fmt.Errorf("%s: invalid frontmatter YAML: %w", file, err)
	}

	// Render each string value through EJS; non-strings pass through. Track
	// which keys were !!js/regexp (either via the YAML tag, captured during
	// parsing, or the inline literal form) so the ops treat them as regexes.
	rendered := map[string]any{}
	reKeys := map[string]bool{}
	for k, v := range rawAttrs {
		switch val := v.(type) {
		case string:
			pat, isRe, flags := regexValue(val, regexTagged[k])
			if isRe {
				rp, err := eng.renderString(pat, locals)
				if err != nil {
					return nil, fmt.Errorf("%s: frontmatter %q: %w", file, k, err)
				}
				if f := goFlags(flags); f != "" {
					rp = "(?" + f + ")" + rp
				}
				rendered[k] = rp
				reKeys[k] = true
				continue
			}
			out, err := eng.renderString(val, locals)
			if err != nil {
				return nil, fmt.Errorf("%s: frontmatter %q: %w", file, k, err)
			}
			rendered[k] = out
		default:
			rendered[k] = v
		}
	}

	fm := toFrontmatter(rendered, reKeys)

	// Render the body with locals + attributes (the rendered frontmatter).
	bodyLocals := cloneLocals(locals)
	bodyLocals["attributes"] = rendered
	renderedBody, err := eng.renderString(body, bodyLocals)
	if err != nil {
		return nil, fmt.Errorf("%s: body: %w", file, err)
	}

	return &RenderedAction{File: file, Attributes: fm, Body: renderedBody}, nil
}

// goFlags converts JS regex flags to Go (?...) inline flags (i and m are the
// ones that translate; g/global is a match-all concern handled by the op).
func goFlags(flags string) string {
	var b strings.Builder
	if strings.Contains(flags, "i") {
		b.WriteString("i")
	}
	if strings.Contains(flags, "m") {
		b.WriteString("m")
	}
	if strings.Contains(flags, "s") {
		b.WriteString("s")
	}
	if b.Len() == 0 {
		return ""
	}
	return b.String()
}

// toFrontmatter maps a rendered attribute map into the typed Frontmatter.
func toFrontmatter(m map[string]any, reKeys map[string]bool) Frontmatter {
	fm := Frontmatter{Extra: map[string]any{}}
	known := map[string]bool{
		"to": true, "from": true, "force": true, "unless_exists": true,
		"inject": true, "before": true, "after": true, "prepend": true,
		"append": true, "at_line": true, "eof_last": true, "skip_if": true,
		"sh": true, "sh_ignore_exit": true, "echo": true, "message": true,
		"unless": true,
	}
	for k, v := range m {
		switch k {
		case "to":
			fm.To = asString(v)
		case "from":
			fm.From = asString(v)
		case "force":
			fm.Force = asBool(v)
		case "unless_exists":
			fm.UnlessExists = asBool(v)
		case "inject":
			fm.Inject = asBool(v)
		case "before":
			fm.Before = asString(v)
			fm.BeforeRe = reKeys[k]
		case "after":
			fm.After = asString(v)
			fm.AfterRe = reKeys[k]
		case "prepend":
			fm.Prepend = asBool(v)
		case "append":
			fm.Append = asBool(v)
		case "at_line":
			fm.AtLine = asInt(v)
			fm.hasAtLine = true
		case "eof_last":
			b := asBool(v)
			fm.EOFLast = &b
		case "skip_if":
			fm.SkipIf = asString(v)
			fm.SkipIfRe = reKeys[k]
		case "sh":
			fm.Sh = asString(v)
		case "sh_ignore_exit":
			fm.ShIgnoreExit = asBool(v)
		case "echo":
			fm.Echo = asString(v)
		case "message":
			fm.Message = asString(v)
		case "unless":
			fm.Unless = asString(v)
		}
		if !known[k] {
			fm.Extra[k] = v
		}
	}
	return fm
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return goejs.ToString(v)
}

func asBool(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return x == "true"
	}
	return goejs.ToBoolean(v)
}

func asInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case float64:
		return int(x)
	}
	return int(goejs.ToNumber(v))
}

func cloneLocals(m map[string]any) map[string]any {
	out := make(map[string]any, len(m)+1)
	for k, v := range m {
		out[k] = v
	}
	return out
}
