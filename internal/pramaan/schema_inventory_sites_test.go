package pramaan

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// New schema construction outside the startup-checked registry must extend its
// feature inventory test. This catches otherwise invisible untested additions.
func TestSchemaInventoryCoversAllSourceSites(t *testing.T) {
	expected := map[string]int{
		"internal/app/integration/ai_actions.go":  5,
		"internal/tasks/switch_step.go":           4,
		"internal/app/research/service.go":        1,
		"internal/app/mcp/service.go":             2, // Discovery data and its validated assistant projection.
		"internal/app/aitools/registry.go":        2, // Startup-checked schemas bound to callers.
		"internal/pkg/generation/conversation.go": 1,
		"internal/pkg/generation/batch.go":        1,
		"internal/pkg/generation/tool_catalog.go": 1, // Local bounded action discovery.
	}
	seen := map[string]int{}
	root := filepath.Join("..", "..")
	e := filepath.WalkDir(filepath.Join(root, "internal"), func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			if d.Name() == "migrations" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "internal/app/aitools/tools") {
			return nil
		} // BuildRegistry + RegisterAdvisorTools inventory all registered definitions.
		file, e := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if e != nil {
			return e
		}
		ast.Inspect(file, func(n ast.Node) bool {
			kv, ok := n.(*ast.KeyValueExpr)
			if !ok {
				return true
			}
			key := ""
			switch k := kv.Key.(type) {
			case *ast.Ident:
				key = k.Name
			case *ast.BasicLit:
				key, _ = strconv.Unquote(k.Value)
			}
			if key == "InputSchema" || key == "jsonSchema" {
				seen[rel]++
			}
			return true
		})
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	for path, count := range seen {
		if expected[path] != count {
			t.Errorf("schema source %s has %d sites; inventory covers %d. Extend its inventory test.", path, count, expected[path])
		}
	}
	for path, count := range expected {
		if seen[path] != count {
			t.Errorf("update obsolete schema inventory for %s", path)
		}
	}
}
