---
to: models/<%= h.changeCase.snakeCase(name) %>.go
---
<%- include('partials/header', { name: name }) -%>
package models

// <%= h.changeCase.pascalCase(name) %> is a generated model.
type <%= h.changeCase.pascalCase(name) %> struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// TableName returns the database table for <%= h.changeCase.pascalCase(name) %>.
func (<%= h.changeCase.pascalCase(name) %>) TableName() string {
	return "<%= h.inflection.pluralize(h.changeCase.snakeCase(name)) %>"
}
