// Package gohygen is a native Go port of the hygen (github.com/jondot/hygen)
// code generator, built on the goejs EJS engine. It runs hygen-style template
// generators — folders of EJS files with YAML frontmatter — entirely in Go,
// with no Node.js runtime.
//
// A generator lives at <templates>/<generator>/<action>/, and each file there
// is an EJS template with a `---`-delimited YAML header. The header is rendered
// as EJS first (so `to: src/<%= name %>.js` resolves locals), then parsed; the
// body is rendered with the locals plus the resolved frontmatter under
// `attributes`. The header decides which operation runs:
//
//	add     — `to:` with no `inject:`; writes a new file.
//	inject  — `to:` + `inject: true`; splices the body into an existing file at
//	          a location given by before/after/prepend/append/at_line, guarded by
//	          skip_if for idempotency.
//	shell   — `sh:`; runs a command with the rendered body on stdin.
//	echo    — `echo:`; prints a message.
//
// Frontmatter keys supported: to, from, force, unless_exists, inject, before,
// after, prepend, append, at_line, eof_last, skip_if, sh, sh_ignore_exit, echo,
// message, plus an `unless` guard. before/after/skip_if accept the
// `!!js/regexp /pattern/flags` form or an inline /pattern/flags literal.
//
// The injected EJS context mirrors hygen: the caller's locals, the derived
// name variants (Name, names, Names), and an `h` helper namespace
// (h.capitalize, h.inflection.*, h.changeCase.*, h.path.*). Custom helpers can
// be added through Config.Helpers.
//
// # Nested templates
//
// Because gohygen renders through goejs with a Loader rooted at the templates
// directory, templates compose via EJS include() to any depth: a template can
// include() a partial, which includes() another, and so on. Each partial
// inherits the including template's locals (the `h` helpers and derived name
// variants flow down automatically), and the include data argument extends
// them. hygen's own `from:` shared-template mechanism is also supported.
//
//	tmpl, _ := gohygen.NewEngine(gohygen.Config{TemplatesDir: "_templates"}), ...
//	res, _ := eng.Run("component", "new", map[string]any{"name": "widget"})
package gohygen
