// Command examples runs the bundled gohygen example generators against a fresh
// output directory and prints what each produced. It is a self-contained demo:
//
//	go run ./examples
//
// It exercises three real generators living under examples/_templates:
//
//   - component  — the production jondot/hygen-CRA React component generator
//     (undasherize naming, stateful/functional/pure conditional branches, and a
//     prepend-inject into a stories index).
//   - gomodel    — a Go model generator showcasing DEEP NESTED INCLUDES
//     (model -> partials/header -> partials/license) plus an idempotent inject
//     into a model registry.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/elioria/gohygen"
)

func main() {
	out, err := os.MkdirTemp("", "gohygen-examples-*")
	if err != nil {
		panic(err)
	}
	fmt.Println("output directory:", out)

	// Seed files the inject ops target.
	must(os.MkdirAll(filepath.Join(out, "src/stories"), 0o755))
	must(os.WriteFile(filepath.Join(out, "src/stories/index.js"), []byte("// stories registry\n"), 0o644))
	must(os.MkdirAll(filepath.Join(out, "models"), 0o755))
	must(os.WriteFile(filepath.Join(out, "models/registry.go"), []byte(
		"package models\n\n// AllModels lists every registered model.\nvar AllModels = []any{\n\t// gohygen:models\n}\n"), 0o644))

	tmpl := templatesDir()
	eng := gohygen.NewEngine(gohygen.Config{
		TemplatesDir: tmpl,
		Cwd:          out,
		Logger:       func(s string) { fmt.Println("  " + s) },
	})

	runs := []struct {
		gen, action string
		locals      map[string]any
		note        string
	}{
		{"component", "new", map[string]any{"name": "user-card"}, "CRA component (default PureComponent)"},
		{"component", "new", map[string]any{"name": "nav-bar", "functional": true}, "CRA component (--functional)"},
		{"gomodel", "new", map[string]any{"name": "blogPost"}, "Go model with nested includes"},
		{"gomodel", "new", map[string]any{"name": "comment"}, "second model (registry inject grows)"},
	}

	for _, r := range runs {
		fmt.Printf("\n$ gohygen %s %s  (%s)\n", r.gen, r.action, r.note)
		if _, err := eng.Run(r.gen, r.action, r.locals); err != nil {
			fmt.Println("  ERROR:", err)
		}
	}

	fmt.Println("\n--- models/blog_post.go (note the nested-include header) ---")
	fmt.Println(read(filepath.Join(out, "models/blog_post.go")))
	fmt.Println("--- models/registry.go (two idempotent injects) ---")
	fmt.Println(read(filepath.Join(out, "models/registry.go")))
	fmt.Println("--- src/components/user-card/index.js ---")
	fmt.Println(read(filepath.Join(out, "src/components/user-card/index.js")))
}

func templatesDir() string {
	// Resolve examples/_templates relative to this source file's directory so
	// `go run ./examples` works from the repo root.
	if wd, err := os.Getwd(); err == nil {
		c := filepath.Join(wd, "examples", "_templates")
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			return c
		}
		c = filepath.Join(wd, "_templates")
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			return c
		}
	}
	return "examples/_templates"
}

func read(p string) string {
	data, err := os.ReadFile(p)
	if err != nil {
		return "(missing: " + err.Error() + ")"
	}
	return string(data)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
