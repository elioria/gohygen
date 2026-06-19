package gohygen

import (
	"github.com/elioria/goejs"
)

// Engine renders hygen-style templates with goejs. It owns the configuration
// and a goejs Loader rooted at the templates directory, which is what makes
// EJS include() — and therefore arbitrarily deep nested template composition —
// work: a template may include() a partial, which may include() another, and so
// on, all resolved relative to the templates root.
type Engine struct {
	cfg    Config
	loader goejs.Loader
}

// NewEngine builds an Engine for the given configuration.
func NewEngine(cfg Config) *Engine {
	cfg.applyDefaults()
	return &Engine{
		cfg:    cfg,
		loader: goejs.NewFileLoader(cfg.TemplatesDir),
	}
}

// renderString renders an EJS source string with the engine's full context
// (the supplied locals plus the derived name variants and helper namespace).
// The loader is attached so include() resolves nested partials.
func (e *Engine) renderString(src string, locals map[string]any) (string, error) {
	if src == "" {
		return "", nil
	}
	ctx := e.context(locals)
	return goejs.Render(src, ctx,
		goejs.WithLoader(e.loader),
		goejs.WithFilename(e.cfg.TemplatesDir+"/."),
	)
}

// context assembles the EJS locals exactly as hygen's context.ts does:
// defaults, then the caller's locals, then the derived cased/pluralized name
// variants, then the `h` helper namespace.
func (e *Engine) context(locals map[string]any) map[string]any {
	ctx := map[string]any{
		"name": "unnamed",
	}
	for k, v := range e.cfg.LocalsDefaults {
		ctx[k] = v
	}
	for k, v := range locals {
		ctx[k] = v
	}
	deriveNameVariants(ctx)
	ctx["h"] = buildHelpers(e.cfg.Helpers)
	return ctx
}

// deriveNameVariants adds Name, names, and Names alongside name, matching
// hygen's processLocals (only the `name` local is special-cased).
func deriveNameVariants(ctx map[string]any) {
	nv, ok := ctx["name"]
	if !ok {
		return
	}
	name := asString(nv)
	ctx["Name"] = capitalize(name)
	plural := Pluralize(name)
	ctx["names"] = plural
	ctx["Names"] = capitalize(plural)
}
