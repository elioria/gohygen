# Vendored test fixtures

The template corpora under this directory are vendored from upstream projects
for compatibility testing. They are used unmodified to verify that gohygen
renders templates authored for the original Node.js `hygen` tool.

- `metaverse/` — from [jondot/hygen](https://github.com/jondot/hygen)
  (`src/__tests__/metaverse/hygen-templates/_templates`). MIT License,
  © Dotan Nahum.
- `cra/` — from [jondot/hygen-CRA](https://github.com/jondot/hygen-CRA)
  (`_templates`). MIT License, © Dotan Nahum.

Both upstream projects are MIT-licensed. These fixtures are included solely to
exercise gohygen against real-world templates; gohygen itself is also MIT.
