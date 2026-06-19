package gohygen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These tests run REAL hygen templates — hygen's own metaverse corpus and the
// production jondot/hygen-CRA Create-React-App generators — through gohygen and
// assert the rendered output, proving compatibility with templates written for
// the original Node.js tool. The templates live under testdata/ (vendored from
// the upstream repositories).

// runGen renders+executes a generator against a temp cwd and returns it.
func runGen(t *testing.T, tmplDir, gen, action string, locals map[string]any) (*Engine, string, *Result) {
	t.Helper()
	cwd := t.TempDir()
	e := NewEngine(Config{TemplatesDir: tmplDir, Cwd: cwd})
	res, err := e.Run(gen, action, locals)
	if err != nil {
		t.Fatalf("%s/%s: %v", gen, action, err)
	}
	return e, cwd, res
}

func mustRead(t *testing.T, dir, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("expected file %s: %v", rel, err)
	}
	return string(data)
}

const metaverse = "testdata/metaverse"
const cra = "testdata/cra"

// --- hygen metaverse corpus ---

func TestMetaverseSample(t *testing.T) {
	_, cwd, _ := runGen(t, metaverse, "sample", "new", map[string]any{})
	got := mustRead(t, cwd, "given/my_app/template.md")
	if !strings.Contains(got, "hello metaverse!") {
		t.Errorf("sample: %q", got)
	}
}

func TestMetaverseWorker(t *testing.T) {
	_, cwd, _ := runGen(t, metaverse, "worker", "new", map[string]any{"name": "foo"})
	got := mustRead(t, cwd, "given/app/workers/foo.js")
	if !strings.Contains(got, "class Foo extends Worker") {
		t.Errorf("worker: %q", got)
	}
}

func TestMetaverseInflectionIrregularPlural(t *testing.T) {
	// "person" -> "people" (irregular). The to: uses h.inflection.pluralize.
	_, cwd, _ := runGen(t, metaverse, "inflection", "new", map[string]any{"name": "person"})
	if !fileExists(filepath.Join(cwd, "given/my_app/people.md")) {
		t.Error("expected given/my_app/people.md (irregular plural of person)")
	}
}

func TestMetaverseConditionalToNull(t *testing.T) {
	// The conditional generator has two files; one's `to:` evaluates to null and
	// must be skipped, the other always renders.
	_, cwd, res := runGen(t, metaverse, "conditional-rendering", "new", map[string]any{"notGiven": ""})
	if !fileExists(filepath.Join(cwd, "given/conditional/always.txt")) {
		t.Error("always.txt should be created")
	}
	if fileExists(filepath.Join(cwd, "given/conditional/shouldnt-be-here")) {
		t.Error("null-to file should NOT be created")
	}
	skipped := 0
	for _, a := range res.Actions {
		if a.Skipped {
			skipped++
		}
	}
	if skipped == 0 {
		t.Error("expected the null-to action to be skipped")
	}
}

func TestMetaversePositionalName(t *testing.T) {
	_, cwd, _ := runGen(t, metaverse, "positional-name", "new", map[string]any{"name": "acmecorp"})
	got := mustRead(t, cwd, "given/positional-name/acmecorp/always.txt")
	if !strings.Contains(got, "positional name") {
		t.Errorf("positional: %q", got)
	}
}

func TestMetaverseAttrsInBody(t *testing.T) {
	// Body is `<%= attributes.to %>` — proves the rendered frontmatter is exposed
	// to the body render.
	_, cwd, _ := runGen(t, metaverse, "attrs-in-body", "new", map[string]any{})
	got := strings.TrimSpace(mustRead(t, cwd, "given/attrs-in-body/hello.txt"))
	if got != "given/attrs-in-body/hello.txt" {
		t.Errorf("attrs-in-body: %q", got)
	}
}

// TestMetaverseMailer is hygen's flagship multi-file + inject example. The
// expected output is taken verbatim from hygen's own expected/ fixtures.
func TestMetaverseMailer(t *testing.T) {
	_, cwd, _ := runGen(t, metaverse, "mailer", "new", map[string]any{"name": "message", "message": "foo"})

	mailer := mustRead(t, cwd, "given/app/mailers/message.js")
	if !strings.Contains(mailer, "class Message extends Mailer") {
		t.Errorf("mailer.js: %q", mailer)
	}

	// the z_inject template injects "I was injected!!!" before /You owe/.
	html := mustRead(t, cwd, "given/app/mailers/foo/html.ejs")
	wantHTML := "This is the html email template.\n" +
		"Find me at <i>app/mailers/foo/html.ejs</i>\n\n" +
		"<br />\n<br />\n\n" +
		"I was injected!!!\nYou owe\n<%= bill %>"
	if strings.TrimRight(html, "\n") != strings.TrimRight(wantHTML, "\n") {
		t.Errorf("mailer html mismatch:\n got %q\nwant %q", html, wantHTML)
	}
}

func TestMetaverseAddUnlessExists(t *testing.T) {
	// always.ejs.t writes unconditionally; y_overwrite has unless_exists.
	e, cwd, _ := runGen(t, metaverse, "add-unless-exists", "new", map[string]any{"message": "foo"})
	// re-run: unless_exists files must be skipped the second time.
	res2, err := e.Run("add-unless-exists", "new", map[string]any{"message": "foo"})
	if err != nil {
		t.Fatal(err)
	}
	anySkipped := false
	for _, a := range res2.Actions {
		if a.Skipped {
			anySkipped = true
		}
	}
	if !anySkipped {
		t.Error("expected a skip on re-run (exists / unless_exists)")
	}
	_ = cwd
}

// --- production jondot/hygen-CRA generators ---

func TestCRADefaultPureComponent(t *testing.T) {
	cwd := t.TempDir()
	os.MkdirAll(filepath.Join(cwd, "src/stories"), 0o755)
	os.WriteFile(filepath.Join(cwd, "src/stories/index.js"), []byte("// stories\n"), 0o644)
	e := NewEngine(Config{TemplatesDir: cra, Cwd: cwd})
	if _, err := e.Run("component", "new", map[string]any{"name": "user-profile"}); err != nil {
		t.Fatal(err)
	}
	idx := mustRead(t, cwd, "src/components/user-profile/index.js")
	// undasherize: user-profile -> UserProfile; default branch -> PureComponent.
	if !strings.Contains(idx, "class UserProfile extends PureComponent") {
		t.Errorf("CRA default:\n%s", idx)
	}
	story := mustRead(t, cwd, "src/components/user-profile/user-profile.story.js")
	if !strings.Contains(story, "storiesOf('UserProfile'") {
		t.Errorf("CRA story: %q", story)
	}
	// prepend inject into stories/index.js
	stories := mustRead(t, cwd, "src/stories/index.js")
	if !strings.HasPrefix(stories, "import '../components/user-profile/user-profile.story'") {
		t.Errorf("CRA inject prepend: %q", stories)
	}
}

func TestCRAFunctionalBranch(t *testing.T) {
	cwd := t.TempDir()
	os.MkdirAll(filepath.Join(cwd, "src/stories"), 0o755)
	os.WriteFile(filepath.Join(cwd, "src/stories/index.js"), []byte(""), 0o644)
	e := NewEngine(Config{TemplatesDir: cra, Cwd: cwd})
	if _, err := e.Run("component", "new", map[string]any{"name": "nav-bar", "functional": true}); err != nil {
		t.Fatal(err)
	}
	idx := mustRead(t, cwd, "src/components/nav-bar/index.js")
	if !strings.Contains(idx, "const NavBar = props =>") {
		t.Errorf("CRA functional:\n%s", idx)
	}
}

func TestCRAStatefulBranch(t *testing.T) {
	cwd := t.TempDir()
	os.MkdirAll(filepath.Join(cwd, "src/stories"), 0o755)
	os.WriteFile(filepath.Join(cwd, "src/stories/index.js"), []byte(""), 0o644)
	e := NewEngine(Config{TemplatesDir: cra, Cwd: cwd})
	if _, err := e.Run("component", "new", map[string]any{"name": "side-menu", "stateful": true}); err != nil {
		t.Fatal(err)
	}
	idx := mustRead(t, cwd, "src/components/side-menu/index.js")
	if !strings.Contains(idx, "class SideMenu extends Component") || strings.Contains(idx, "PureComponent") {
		t.Errorf("CRA stateful:\n%s", idx)
	}
}

func TestCRAInjectIdempotent(t *testing.T) {
	cwd := t.TempDir()
	os.MkdirAll(filepath.Join(cwd, "src/stories"), 0o755)
	os.WriteFile(filepath.Join(cwd, "src/stories/index.js"), []byte(""), 0o644)
	e := NewEngine(Config{TemplatesDir: cra, Cwd: cwd})
	for i := 0; i < 3; i++ {
		if _, err := e.Run("component", "new", map[string]any{"name": "card"}); err != nil {
			t.Fatal(err)
		}
	}
	stories := mustRead(t, cwd, "src/stories/index.js")
	if n := strings.Count(stories, "components/card/card.story"); n != 1 {
		t.Errorf("inject not idempotent: %d occurrences\n%s", n, stories)
	}
}
