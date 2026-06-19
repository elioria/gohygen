package gohygen

import (
	"os"
	"path/filepath"
)

// Config controls template discovery and rendering.
type Config struct {
	// TemplatesDir is the root holding _templates-style generators. When empty,
	// it is resolved from HYGEN_TMPLS, then ./_templates, then "_templates".
	TemplatesDir string

	// Cwd is the base directory output paths are resolved against (default: the
	// process working directory).
	Cwd string

	// Helpers are extra helper functions/values merged into the `h` namespace,
	// overriding built-ins by key. Functions should use the func(...any) any
	// signature (goejs converts them automatically).
	Helpers map[string]any

	// LocalsDefaults seed every render before the caller's locals are applied.
	LocalsDefaults map[string]any

	// Dry, when true, performs no filesystem writes; ops report what they would
	// do via the returned results.
	Dry bool

	// Overwrite forces add to overwrite existing files without prompting
	// (equivalent to hygen's HYGEN_OVERWRITE).
	Overwrite bool

	// Logger receives human-readable progress lines. When nil, output is
	// discarded (results are still returned).
	Logger func(string)

	// ShellRunner executes `sh:` frontmatter commands. When nil, a default
	// runner using /bin/sh is used. Receives the command and the rendered body
	// (piped to stdin); returns combined output.
	ShellRunner func(cmd, stdin string) (string, error)
}

func (c *Config) applyDefaults() {
	if c.Cwd == "" {
		if wd, err := os.Getwd(); err == nil {
			c.Cwd = wd
		} else {
			c.Cwd = "."
		}
	}
	if c.TemplatesDir == "" {
		c.TemplatesDir = resolveTemplatesDir(c.Cwd)
	}
	if c.Logger == nil {
		c.Logger = func(string) {}
	}
}

// resolveTemplatesDir mirrors hygen's resolution order: HYGEN_TMPLS, then
// <cwd>/_templates, then the literal "_templates".
func resolveTemplatesDir(cwd string) string {
	if env := os.Getenv("HYGEN_TMPLS"); env != "" {
		return env
	}
	candidate := filepath.Join(cwd, "_templates")
	if fi, err := os.Stat(candidate); err == nil && fi.IsDir() {
		return candidate
	}
	return "_templates"
}
