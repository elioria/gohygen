package gohygen

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// OpResult records the outcome of executing one rendered action.
type OpResult struct {
	Type    string // "add" | "inject" | "shell" | "echo" | "skip" | "message"
	Path    string // target file (add/inject) when applicable
	Message string // human-readable summary
	Skipped bool
	Output  string // shell output, if any
}

// execute dispatches a rendered action to the correct op based on which
// frontmatter keys are present, mirroring hygen's ops/index.ts routing.
func (e *Engine) execute(a *RenderedAction) (OpResult, error) {
	fm := a.Attributes

	// `unless` is a gohygen convenience guard: skip the whole action when the
	// rendered value is truthy/non-empty.
	if fm.Unless != "" && fm.Unless != "false" {
		return OpResult{Type: "skip", Skipped: true, Message: "unless: " + fm.Unless}, nil
	}

	switch {
	case fm.Echo != "":
		e.cfg.Logger(fm.Echo)
		return OpResult{Type: "echo", Message: fm.Echo}, nil
	case fm.Sh != "":
		return e.opShell(a)
	case fm.To != "" && fm.Inject:
		return e.opInject(a)
	case fm.To != "":
		return e.opAdd(a)
	default:
		return OpResult{Type: "skip", Skipped: true, Message: "no actionable frontmatter"}, nil
	}
}

// opAdd writes a new file (hygen ops/add.ts).
func (e *Engine) opAdd(a *RenderedAction) (OpResult, error) {
	fm := a.Attributes
	absTo := e.resolvePath(fm.To)

	exists := fileExists(absTo)

	if fm.UnlessExists && exists && !fm.Force {
		e.log("skipped", fm.To)
		return OpResult{Type: "add", Path: fm.To, Skipped: true, Message: "skipped (unless_exists)"}, nil
	}
	if exists && !fm.Force && !e.cfg.Overwrite {
		e.log("skipped (exists)", fm.To)
		return OpResult{Type: "add", Path: fm.To, Skipped: true, Message: "skipped (exists, not forced)"}, nil
	}
	if fm.SkipIf == "true" {
		e.log("skipped", fm.To)
		return OpResult{Type: "add", Path: fm.To, Skipped: true, Message: "skipped (skip_if)"}, nil
	}

	body := a.Body
	if fm.From != "" {
		// Load the body from a shared template under the templates root.
		shared := filepath.Join(e.cfg.TemplatesDir, filepath.FromSlash(fm.From))
		data, err := os.ReadFile(shared)
		if err != nil {
			return OpResult{}, fmt.Errorf("add: cannot read from %q: %w", fm.From, err)
		}
		body = string(data)
	}

	verb := "added"
	if fm.Force && exists {
		verb = "forced"
	}

	if !e.cfg.Dry {
		if err := os.MkdirAll(filepath.Dir(absTo), 0o755); err != nil {
			return OpResult{}, fmt.Errorf("add: mkdir: %w", err)
		}
		if err := os.WriteFile(absTo, []byte(body), 0o644); err != nil {
			return OpResult{}, fmt.Errorf("add: write: %w", err)
		}
	}
	e.log(verb, fm.To)
	return OpResult{Type: "add", Path: fm.To, Message: verb + ": " + fm.To}, nil
}

// opInject modifies an existing file (hygen ops/inject.ts + injector.ts).
func (e *Engine) opInject(a *RenderedAction) (OpResult, error) {
	fm := a.Attributes
	absTo := e.resolvePath(fm.To)

	data, err := os.ReadFile(absTo)
	if err != nil {
		return OpResult{}, fmt.Errorf("inject: target %q does not exist: %w", fm.To, err)
	}
	content := string(data)

	newContent, injected, err := inject(content, a.Body, fm)
	if err != nil {
		return OpResult{}, err
	}
	if !injected {
		e.log("skipped (inject)", fm.To)
		return OpResult{Type: "inject", Path: fm.To, Skipped: true, Message: "skipped (skip_if or no location)"}, nil
	}
	if !e.cfg.Dry {
		if err := os.WriteFile(absTo, []byte(newContent), 0o644); err != nil {
			return OpResult{}, fmt.Errorf("inject: write: %w", err)
		}
	}
	e.log("injected", fm.To)
	return OpResult{Type: "inject", Path: fm.To, Message: "inject: " + fm.To}, nil
}

// inject computes the new file content after applying an injection. It returns
// (content, injected, err) where injected is false when skip_if matched or no
// location key resolved.
func inject(content, body string, fm Frontmatter) (string, bool, error) {
	// skip_if guard: if the pattern is already present, do nothing (idempotency).
	if fm.SkipIf != "" {
		re, err := compileGuard(fm.SkipIf, fm.SkipIfRe)
		if err != nil {
			return "", false, fmt.Errorf("inject: bad skip_if: %w", err)
		}
		if re.MatchString(content) {
			return content, false, nil
		}
	}

	nl := detectNewline(content)
	lines := strings.Split(content, nl)

	idx, err := injectIndex(lines, content, nl, fm)
	if err != nil {
		return "", false, err
	}
	if idx < 0 {
		return content, false, nil
	}

	insert := body
	if fm.EOFLast != nil {
		if *fm.EOFLast {
			if !strings.HasSuffix(insert, "\n") {
				insert += "\n"
			}
		} else {
			insert = strings.TrimSuffix(insert, "\n")
		}
	}

	// splice insert at idx
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:idx]...)
	out = append(out, insert)
	out = append(out, lines[idx:]...)
	return strings.Join(out, nl), true, nil
}

// injectIndex resolves the insertion line index from the first present location
// key, following hygen's precedence: at_line, prepend, append, before, after.
func injectIndex(lines []string, content, nl string, fm Frontmatter) (int, error) {
	if fm.hasAtLine {
		return clamp(fm.AtLine, 0, len(lines)), nil
	}
	if fm.Prepend {
		return 0, nil
	}
	if fm.Append {
		if len(lines) == 0 {
			return 0, nil
		}
		return len(lines) - 1, nil
	}
	if fm.Before != "" {
		return pragmaticIndex(fm.Before, fm.BeforeRe, lines, content, nl, true)
	}
	if fm.After != "" {
		return pragmaticIndex(fm.After, fm.AfterRe, lines, content, nl, false)
	}
	return -1, nil
}

// pragmaticIndex finds the injection line for a before/after pattern. It first
// tries a per-line match; failing that, it falls back to a multiline match over
// the whole content and converts the offset to a line number — this is what
// lets `after: dependencies` hit the "dependencies": { block in a JSON file.
func pragmaticIndex(pattern string, isRegex bool, lines []string, content, nl string, before bool) (int, error) {
	re, err := compileGuard(pattern, isRegex)
	if err != nil {
		return -1, fmt.Errorf("inject: bad %s pattern: %w", locationName(before), err)
	}
	for i, line := range lines {
		if re.MatchString(line) {
			if before {
				return i, nil
			}
			return i + 1, nil
		}
	}
	// multiline fallback
	mre, err := regexp.Compile("(?m)" + re.String())
	if err != nil {
		return -1, nil
	}
	loc := mre.FindStringIndex(content)
	if loc == nil {
		return -1, nil
	}
	offset := loc[0]
	if !before {
		offset = loc[1]
	}
	return strings.Count(content[:offset], nl), nil
}

func locationName(before bool) string {
	if before {
		return "before"
	}
	return "after"
}

// compileGuard builds a regexp from a frontmatter pattern. When isRegex is true
// the value is already a Go-compatible pattern (it carried !!js/regexp); plain
// text is treated as a regex source the way hygen does (content.match(pattern)).
func compileGuard(pattern string, isRegex bool) (*regexp.Regexp, error) {
	return regexp.Compile(pattern)
}

// detectNewline picks the dominant newline style of content (\r\n vs \n).
func detectNewline(content string) string {
	crlf := strings.Count(content, "\r\n")
	lf := strings.Count(content, "\n") - crlf
	if crlf > lf {
		return "\r\n"
	}
	return "\n"
}

func (e *Engine) opShell(a *RenderedAction) (OpResult, error) {
	fm := a.Attributes
	if e.cfg.Dry {
		return OpResult{Type: "shell", Message: "sh (dry): " + fm.Sh}, nil
	}
	runner := e.cfg.ShellRunner
	if runner == nil {
		runner = defaultShellRunner
	}
	out, err := runner(fm.Sh, a.Body)
	if err != nil && !fm.ShIgnoreExit {
		return OpResult{}, fmt.Errorf("sh failed: %w\n%s", err, out)
	}
	e.log("sh", fm.Sh)
	return OpResult{Type: "shell", Message: "sh: " + fm.Sh, Output: out}, nil
}

func (e *Engine) resolvePath(to string) string {
	p := filepath.FromSlash(to)
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(e.cfg.Cwd, p)
}

func (e *Engine) log(verb, target string) {
	e.cfg.Logger(fmt.Sprintf("%12s: %s", verb, target))
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func clamp(n, lo, hi int) int {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}
