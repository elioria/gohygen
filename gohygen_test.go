package gohygen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scaffold writes a set of files (relative path -> content) under a fresh temp
// root and returns the root. Keys use forward slashes.
func scaffold(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func readFile(t *testing.T, dir, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

// newEngine builds an Engine whose templates live under root/_templates and
// whose output goes under a separate cwd.
func newEngine(t *testing.T, tmplRoot string) (*Engine, string) {
	t.Helper()
	cwd := t.TempDir()
	return NewEngine(Config{
		TemplatesDir: filepath.Join(tmplRoot, "_templates"),
		Cwd:          cwd,
	}), cwd
}

// --- add op + frontmatter EJS render + name derivation ---

func TestAddBasic(t *testing.T) {
	root := scaffold(t, map[string]string{
		"_templates/component/new/file.ejs.t": "---\nto: src/<%= name %>.js\n---\n" +
			"export class <%= Name %> {}\n",
	})
	e, cwd := newEngine(t, root)
	res, err := e.Run("component", "new", map[string]any{"name": "widget"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Actions) != 1 || res.Actions[0].Type != "add" {
		t.Fatalf("actions: %+v", res.Actions)
	}
	got := readFile(t, cwd, "src/widget.js")
	if got != "export class Widget {}\n" {
		t.Errorf("output: %q", got)
	}
}

func TestNameVariants(t *testing.T) {
	root := scaffold(t, map[string]string{
		"_templates/g/a/v.ejs.t": "---\nto: out.txt\n---\n<%= name %>|<%= Name %>|<%= names %>|<%= Names %>",
	})
	e, cwd := newEngine(t, root)
	if _, err := e.Run("g", "a", map[string]any{"name": "post"}); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, cwd, "out.txt")
	if got != "post|Post|posts|Posts" {
		t.Errorf("variants: %q", got)
	}
}

// --- helpers in the h namespace ---

func TestHelpers(t *testing.T) {
	root := scaffold(t, map[string]string{
		"_templates/g/a/v.ejs.t": "---\nto: out.txt\n---\n" +
			"<%= h.capitalize(name) %>|<%= h.changeCase.pascalCase(name) %>|" +
			"<%= h.changeCase.paramCase(name) %>|<%= h.changeCase.snakeCase(name) %>|" +
			"<%= h.inflection.pluralize(name) %>|<%= h.inflection.classify('user_accounts') %>",
	})
	e, cwd := newEngine(t, root)
	if _, err := e.Run("g", "a", map[string]any{"name": "userProfile"}); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, cwd, "out.txt")
	want := "UserProfile|UserProfile|user-profile|user_profile|userProfiles|UserAccount"
	if got != want {
		t.Errorf("helpers:\n got %q\nwant %q", got, want)
	}
}

// --- NESTED INCLUDES: the required feature. a includes b includes c includes d ---

func TestNestedIncludes(t *testing.T) {
	root := scaffold(t, map[string]string{
		"_templates/page/new/index.ejs.t": "---\nto: <%= name %>.html\n---\n" +
			"<%- include('partials/layout', { name: name, body: 'HELLO' }) %>",
		"_templates/partials/layout.ejs": "<html><%- include('partials/header', { name: name }) %>" +
			"<main><%= body %></main></html>",
		"_templates/partials/header.ejs": "<header><%- include('partials/brand', { name: name }) %></header>",
		"_templates/partials/brand.ejs":  "<brand><%= h.capitalize(name) %></brand>",
	})
	e, cwd := newEngine(t, root)
	if _, err := e.Run("page", "new", map[string]any{"name": "home"}); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, cwd, "home.html")
	want := "<html><header><brand>Home</brand></header><main>HELLO</main></html>"
	if got != want {
		t.Errorf("nested include:\n got %q\nwant %q", got, want)
	}
}

// --- inject op: after / before / skip_if idempotency / at_line / prepend / append ---

func TestInjectAfter(t *testing.T) {
	cwd := t.TempDir()
	target := filepath.Join(cwd, "routes.js")
	os.WriteFile(target, []byte("const routes = [\n  // routes\n]\n"), 0o644)
	root := scaffold(t, map[string]string{
		"_templates/route/new/inject.ejs.t": "---\nto: routes.js\ninject: true\nafter: // routes\nskip_if: <%= name %>\n---\n" +
			"  '<%= name %>',",
	})
	e := NewEngine(Config{TemplatesDir: filepath.Join(root, "_templates"), Cwd: cwd})
	if _, err := e.Run("route", "new", map[string]any{"name": "users"}); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, cwd, "routes.js")
	want := "const routes = [\n  // routes\n  'users',\n]\n"
	if got != want {
		t.Errorf("inject after:\n got %q\nwant %q", got, want)
	}
	// idempotency: second run must not double-inject (skip_if matches 'users')
	if _, err := e.Run("route", "new", map[string]any{"name": "users"}); err != nil {
		t.Fatal(err)
	}
	if again := readFile(t, cwd, "routes.js"); again != want {
		t.Errorf("inject not idempotent:\n got %q", again)
	}
}

func TestInjectBeforeRegex(t *testing.T) {
	cwd := t.TempDir()
	os.WriteFile(filepath.Join(cwd, "bill.txt"), []byte("intro\nYou owe\n$5\n"), 0o644)
	root := scaffold(t, map[string]string{
		"_templates/g/a/z.ejs.t": "---\nto: bill.txt\ninject: true\nbefore: !!js/regexp /You owe/g\n---\n" +
			"INJECTED",
	})
	e := NewEngine(Config{TemplatesDir: filepath.Join(root, "_templates"), Cwd: cwd})
	if _, err := e.Run("g", "a", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, cwd, "bill.txt")
	if got != "intro\nINJECTED\nYou owe\n$5\n" {
		t.Errorf("inject before regex: %q", got)
	}
}

func TestInjectPrependAppendAtLine(t *testing.T) {
	cases := []struct {
		fmKey string
		want  string
	}{
		{"prepend: true", "X\nA\nB\nC\n"},
		{"append: true", "A\nB\nC\nX\n"},
		{"at_line: 1", "A\nX\nB\nC\n"},
	}
	for _, c := range cases {
		cwd := t.TempDir()
		os.WriteFile(filepath.Join(cwd, "f.txt"), []byte("A\nB\nC\n"), 0o644)
		root := scaffold(t, map[string]string{
			"_templates/g/a/i.ejs.t": "---\nto: f.txt\ninject: true\n" + c.fmKey + "\neof_last: false\n---\nX",
		})
		e := NewEngine(Config{TemplatesDir: filepath.Join(root, "_templates"), Cwd: cwd})
		if _, err := e.Run("g", "a", map[string]any{}); err != nil {
			t.Fatalf("%s: %v", c.fmKey, err)
		}
		got := readFile(t, cwd, "f.txt")
		if got != c.want {
			t.Errorf("%s: got %q want %q", c.fmKey, got, c.want)
		}
	}
}

// --- force / unless_exists ---

func TestUnlessExists(t *testing.T) {
	root := scaffold(t, map[string]string{
		"_templates/g/a/f.ejs.t": "---\nto: x.txt\nunless_exists: true\n---\nNEW",
	})
	e, cwd := newEngine(t, root)
	os.WriteFile(filepath.Join(cwd, "x.txt"), []byte("ORIGINAL"), 0o644)
	res, err := e.Run("g", "a", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Actions[0].Skipped {
		t.Error("expected skip when file exists with unless_exists")
	}
	if got := readFile(t, cwd, "x.txt"); got != "ORIGINAL" {
		t.Errorf("file overwritten: %q", got)
	}
}

func TestForceOverwrite(t *testing.T) {
	root := scaffold(t, map[string]string{
		"_templates/g/a/f.ejs.t": "---\nto: x.txt\nforce: true\n---\nNEW",
	})
	e, cwd := newEngine(t, root)
	os.WriteFile(filepath.Join(cwd, "x.txt"), []byte("OLD"), 0o644)
	if _, err := e.Run("g", "a", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, cwd, "x.txt"); got != "NEW" {
		t.Errorf("force overwrite: %q", got)
	}
}

// --- from: shared template body ---

func TestFromSharedBody(t *testing.T) {
	root := scaffold(t, map[string]string{
		"_templates/g/a/use.ejs.t": "---\nto: out.txt\nfrom: shared/snippet.txt\n---\nIGNORED BODY",
		"_templates/shared/snippet.txt": "SHARED CONTENT",
	})
	e, cwd := newEngine(t, root)
	if _, err := e.Run("g", "a", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, cwd, "out.txt"); got != "SHARED CONTENT" {
		t.Errorf("from: %q", got)
	}
}

// --- shell op ---

func TestShellOp(t *testing.T) {
	root := scaffold(t, map[string]string{
		"_templates/g/a/s.ejs.t": "---\nsh: cat\n---\nfrom-stdin-<%= name %>",
	})
	e, _ := newEngine(t, root)
	res, err := e.Run("g", "a", map[string]any{"name": "z"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Actions[0].Output, "from-stdin-z") {
		t.Errorf("shell output: %q", res.Actions[0].Output)
	}
}

// --- echo + message ---

func TestEchoAndMessage(t *testing.T) {
	var logged []string
	root := scaffold(t, map[string]string{
		"_templates/g/a/e.ejs.t": "---\necho: hello <%= name %>\n---\n",
		"_templates/g/a/m.ejs.t": "---\nto: out.txt\nmessage: run npm install\n---\nbody",
	})
	cwd := t.TempDir()
	e := NewEngine(Config{
		TemplatesDir: filepath.Join(root, "_templates"),
		Cwd:          cwd,
		Logger:       func(s string) { logged = append(logged, s) },
	})
	res, err := e.Run("g", "a", map[string]any{"name": "world"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Messages) != 1 || res.Messages[0] != "run npm install" {
		t.Errorf("messages: %+v", res.Messages)
	}
	joined := strings.Join(logged, "\n")
	if !strings.Contains(joined, "hello world") {
		t.Errorf("echo not logged: %v", logged)
	}
}

// --- literal EJS delimiters in a template-that-generates-templates ---

func TestLiteralDelimiters(t *testing.T) {
	root := scaffold(t, map[string]string{
		"_templates/gen/new/t.ejs.t": "---\nto: out.ejs.t\n---\n" +
			"<%%= name %%> stays literal, <%= name %> resolves",
	})
	e, cwd := newEngine(t, root)
	if _, err := e.Run("gen", "new", map[string]any{"name": "X"}); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, cwd, "out.ejs.t")
	if got != "<%= name %> stays literal, X resolves" {
		t.Errorf("literal delims: %q", got)
	}
}

// --- subaction filter ---

func TestSubactionFilter(t *testing.T) {
	root := scaffold(t, map[string]string{
		"_templates/g/new/model.ejs.t":      "---\nto: model.txt\n---\nM",
		"_templates/g/new/controller.ejs.t": "---\nto: controller.txt\n---\nC",
	})
	e, cwd := newEngine(t, root)
	// action "new:model" should only run the model template.
	if _, err := e.Run("g", "new:model", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if !fileExists(filepath.Join(cwd, "model.txt")) {
		t.Error("model.txt should exist")
	}
	if fileExists(filepath.Join(cwd, "controller.txt")) {
		t.Error("controller.txt should NOT exist (filtered out)")
	}
}

// --- dry run writes nothing ---

func TestDryRun(t *testing.T) {
	root := scaffold(t, map[string]string{
		"_templates/g/a/f.ejs.t": "---\nto: x.txt\n---\nbody",
	})
	cwd := t.TempDir()
	e := NewEngine(Config{TemplatesDir: filepath.Join(root, "_templates"), Cwd: cwd, Dry: true})
	if _, err := e.Run("g", "a", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if fileExists(filepath.Join(cwd, "x.txt")) {
		t.Error("dry run should not write")
	}
}
