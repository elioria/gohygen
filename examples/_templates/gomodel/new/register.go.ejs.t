---
to: models/registry.go
inject: true
after: // gohygen:models
skip_if: <%= h.changeCase.pascalCase(name) %>{}
eof_last: false
---
	&<%= h.changeCase.pascalCase(name) %>{},
