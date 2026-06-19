package gohygen

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ignoredFiles are the action-folder entries that are never treated as
// templates (hygen's hook files plus OS junk).
var ignoredFiles = map[string]bool{
	"prompt.js": true, "prompt.cjs": true, "prompt.ts": true,
	"index.js": true, "index.cjs": true, "index.ts": true,
	".hygenignore": true, ".DS_Store": true, "Thumbs.db": true,
}

// Result is the aggregate outcome of running a generator/action.
type Result struct {
	Actions  []OpResult
	Messages []string
}

// Run executes one generator/action: discover the template files under
// <templates>/<generator>/<action>, render each, and execute its op. locals are
// the template variables (e.g. {"name": "user"}); CLI flags map naturally onto
// this map. Returns the per-file results and any collected `message:` lines.
func (e *Engine) Run(generator, action string, locals map[string]any) (*Result, error) {
	// Support hygen's action:subaction filter.
	mainAction, subaction := action, ""
	if i := strings.Index(action, ":"); i >= 0 {
		mainAction, subaction = action[:i], action[i+1:]
	}

	actionDir := filepath.Join(e.cfg.TemplatesDir, generator, mainAction)
	if fi, err := os.Stat(actionDir); err != nil || !fi.IsDir() {
		// _default fallback: hygen lets `gen name` map to the _default action.
		alt := filepath.Join(e.cfg.TemplatesDir, generator, "_default")
		if fi, err := os.Stat(alt); err == nil && fi.IsDir() {
			actionDir = alt
		} else {
			return nil, fmt.Errorf("action folder not found: %s/%s under %s", generator, mainAction, e.cfg.TemplatesDir)
		}
	}

	files, err := collectTemplates(actionDir)
	if err != nil {
		return nil, err
	}

	res := &Result{}
	for _, file := range files {
		if subaction != "" {
			rel, _ := filepath.Rel(actionDir, file)
			if !strings.Contains(rel, subaction) {
				continue
			}
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", file, err)
		}
		rendered, err := renderFile(file, string(data), locals, e)
		if err != nil {
			return nil, err
		}
		if rendered.Attributes.Message != "" {
			res.Messages = append(res.Messages, rendered.Attributes.Message)
		}
		out, err := e.execute(rendered)
		if err != nil {
			return nil, err
		}
		res.Actions = append(res.Actions, out)
	}

	for _, m := range res.Messages {
		e.cfg.Logger(m)
	}
	return res, nil
}

// RenderOnly renders every template in an action without executing the ops.
// Useful for previewing output (and exercised heavily by the test suite).
func (e *Engine) RenderOnly(generator, action string, locals map[string]any) ([]*RenderedAction, error) {
	actionDir := filepath.Join(e.cfg.TemplatesDir, generator, action)
	files, err := collectTemplates(actionDir)
	if err != nil {
		return nil, err
	}
	var out []*RenderedAction
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		r, err := renderFile(file, string(data), locals, e)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// collectTemplates walks an action folder and returns the template files in
// deterministic (localeCompare-like) order, skipping hook/junk files and any
// paths matched by a .hygenignore in the folder.
func collectTemplates(actionDir string) ([]string, error) {
	ignore := loadHygenIgnore(actionDir)
	var files []string
	err := filepath.Walk(actionDir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		base := filepath.Base(p)
		if ignoredFiles[base] {
			return nil
		}
		rel, _ := filepath.Rel(actionDir, p)
		if ignore != nil && ignore(filepath.ToSlash(rel)) {
			return nil
		}
		files = append(files, p)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i] < files[j] })
	return files, nil
}

// loadHygenIgnore reads a .hygenignore in the action folder and returns a
// matcher over slash-relative paths (simple substring/glob-free prefix match,
// sufficient for the common cases).
func loadHygenIgnore(dir string) func(string) bool {
	data, err := os.ReadFile(filepath.Join(dir, ".hygenignore"))
	if err != nil {
		return nil
	}
	var patterns []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	if len(patterns) == 0 {
		return nil
	}
	return func(rel string) bool {
		for _, p := range patterns {
			if strings.Contains(rel, p) {
				return true
			}
		}
		return false
	}
}
