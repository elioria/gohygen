# gohygen

**A native Go port of the [hygen](https://github.com/jondot/hygen) code generator, built on [goejs](../goejs).**

hygen is a fast, scalable scaffolding tool: you write generators as folders of
EJS templates with YAML frontmatter, and `hygen <generator> <action>` renders
them into your project — creating files, injecting into existing ones, running
shell commands. `gohygen` reimplements that workflow in pure Go. Because it
renders through `goejs` (a native EJS engine, no JavaScript runtime), the **same
`_templates` work without Node.js**.

It is also a real-world test of goejs — in particular of **nested template
includes**: one template can `include()` a partial, which includes another, to
any depth, with locals and helpers flowing down the chain.

```bash
gohygen component new --name userCard
#        added: src/components/UserCard/UserCard.tsx
#     injected: src/components/index.ts
```

---

## Install

```bash
go get github.com/elioria/gohygen
go build -o gohygen ./cmd/gohygen   # the CLI
```

---

## Template anatomy

A template is an EJS file with a `---`-delimited YAML header:

```ejs
---
to: src/components/<%= Name %>/<%= Name %>.tsx
---
import React from 'react';

export const <%= Name %> = () => <div><%= Name %></div>;
```

The header is **rendered as EJS first** (so `to:` can use locals), then parsed
as YAML. The body is then rendered with the locals plus the resolved header
under `attributes`. Generators live at `_templates/<generator>/<action>/`.

---

## Frontmatter reference

| Key | Meaning |
|-----|---------|
| `to` | Destination path (relative to cwd). Triggers **add**. |
| `inject: true` | With `to`, triggers **inject** into an existing file. |
| `from` | Load the body from a shared template under the templates root. |
| `force` | Overwrite an existing file without prompting. |
| `unless_exists` | Skip the add if the target already exists. |
| `before` / `after` | Inject location: a line matching the pattern. |
| `prepend` / `append` | Inject at the top / bottom of the file. |
| `at_line` | Inject at an exact line index. |
| `skip_if` | Skip the inject if the pattern is already present (idempotency). |
| `eof_last` | Force/strip a trailing newline on the injected body. |
| `sh` | Run a shell command with the rendered body on stdin. |
| `sh_ignore_exit` | Don't fail the run on a nonzero shell exit. |
| `echo` | Print a message. |
| `message` | Collect a message to print at the end of the run. |

`before`, `after`, and `skip_if` accept either an inline `/pattern/flags`
literal or the `!!js/regexp /pattern/flags` YAML form. A plain string is treated
as a regex source, exactly as in hygen.

---

## The template context

Inside every template these are available (matching hygen):

- The **locals** you pass (each CLI flag by name).
- Derived **name variants**: `Name` (capitalized), `names` (pluralized),
  `Names` (capitalized plural).
- The **`h` helper namespace**:
  - `h.capitalize(s)`
  - `h.inflection.*` — `pluralize`, `singularize`, `camelize`, `underscore`,
    `humanize`, `dasherize`, `titleize`, `classify`, `tableize`, `demodulize`,
    `undasherize`, `ordinalize`.
  - `h.changeCase.*` — `camelCase`, `pascalCase`, `snakeCase`, `paramCase`
    (kebab), `constantCase`, `dotCase`, `pathCase`, `sentenceCase`, `titleCase`,
    `headerCase`, `noCase`.
  - `h.path.*` — `relative`, `join`, `basename`, `dirname`, `extname`.
- In the body only: `attributes` (the rendered frontmatter).

Add your own helpers via `Config.Helpers`.

---

## Nested templates (the headline feature)

Templates compose through EJS `include()` to any depth. Each partial inherits
the including template's locals — so the `h` helpers and the derived name
variants are available in every partial without re-passing them — and the
include data argument extends them:

```ejs
<%# _templates/component/new/component.ejs.t %>
---
to: src/components/<%= Name %>/<%= Name %>.tsx
---
<%- include('partials/banner', { name: name }) %>
export const <%= Name %> = () => null;
```
```ejs
<%# _templates/partials/banner.ejs %>
// <%- include('partials/stamp', { name: name }) %> — generated
```
```ejs
<%# _templates/partials/stamp.ejs %>
<%= h.changeCase.constantCase(name) %>
```

`component → banner → stamp` resolves in one render, helpers intact. hygen's
`from:` shared-template mechanism is also supported.

---

## Library API

```go
eng := gohygen.NewEngine(gohygen.Config{
    TemplatesDir: "_templates",
    Cwd:          ".",
})

res, err := eng.Run("component", "new", map[string]any{"name": "userCard"})
// res.Actions[i].Type ∈ {"add","inject","shell","echo","skip"}

// Preview without writing:
rendered, _ := eng.RenderOnly("component", "new", map[string]any{"name": "x"})
```

`Config` options: `TemplatesDir`, `Cwd`, `Helpers`, `LocalsDefaults`, `Dry`,
`Overwrite`, `Logger`, `ShellRunner`.

---

## CLI

```bash
gohygen <generator> <action> [--name NAME] [--key value ...] [--dry]
```

Templates are read from `$HYGEN_TMPLS` or `./_templates`. Every `--flag`
becomes a template local; `--dry` previews without writing.

---

## Relationship to the family

- **[ejs4go](../ejs4go)** — EJS for Go via the goja JS engine.
- **[goejs](../goejs)** — EJS for Go via a native Go interpreter (no JS runtime).
- **gohygen** — hygen's generator workflow, on top of goejs.

---

## License

MIT.
