package model

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// The HTTP layer never serializes these rows: every response has its own wire
// type. A json tag here shows up in the OpenAPI schema and invites the next
// handler to embed the model again, which is how credentials and storage keys
// leak into responses and into the shared cache.
func TestModelsHaveNoJSONTags(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			fields, ok := n.(*ast.StructType)
			if !ok {
				return true
			}
			for _, field := range fields.Fields.List {
				if field.Tag != nil && strings.Contains(field.Tag.Value, `json:`) {
					t.Errorf("%s: model field carries a json tag: %s",
						fset.Position(field.Pos()), field.Tag.Value)
				}
			}
			return true
		})
	}
}
