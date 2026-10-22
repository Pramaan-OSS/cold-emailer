package pramaan

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func object() map[string]any { return map[string]any{"type": "object", "properties": map[string]any{}} }
func TestSchemaNormalizationPreservesConstraintsAndData(t *testing.T) {
	original := map[string]any{"$schema": "draft", "type": "object", "properties": map[string]any{"email": map[string]any{"type": "string", "format": "email", "maxLength": 100}, "format": map[string]any{"const": map[string]any{"format": "kept", "pattern": "data"}}}, "additionalProperties": false}
	s, e := NormalizeSchema("contact", original)
	if e != nil {
		t.Fatal(e)
	}
	if e = ValidateSchema("contact", s); e != nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal(s)
	if strings.Contains(string(b), `"format":"email"`) || strings.Contains(string(b), "$schema") || !strings.Contains(string(b), `"pattern":"data"`) || !strings.Contains(string(b), `"maxLength":100`) {
		t.Fatal(string(b))
	}
	if original["$schema"] == nil {
		t.Fatal("mutated caller schema")
	}
	remote, e := RemoteSchema("connected", map[string]any{"type": "object", "description": "untrusted instructions", "const": map[string]any{"description": "literal data"}})
	if e != nil || remote["description"] != nil || remote["const"].(map[string]any)["description"] != "literal data" {
		t.Fatalf("%v %v", remote, e)
	}
}
func TestSchemaRefusalNamesToolAndPath(t *testing.T) {
	for _, key := range strings.Fields("pattern oneOf allOf not if then else patternProperties uniqueItems minProperties maxProperties definitions dependentRequired") {
		t.Run(key, func(t *testing.T) {
			s := object()
			s["properties"] = map[string]any{"email": map[string]any{key: true}}
			_, e := NormalizeSchema("tool update_contact", s)
			if e == nil || !strings.Contains(e.Error(), "update_contact") || !strings.Contains(e.Error(), "#/properties/email/"+key) {
				t.Fatal(e)
			}
		})
	}
	for _, key := range []string{"$schema", "format"} {
		s := object()
		s[key] = "annotation"
		if ValidateSchema("wire", s) == nil {
			t.Fatal("fixture accepted", key)
		}
	}
}
func TestSchemaLimitsAndReferences(t *testing.T) {
	tests := map[string]any{"root": true, "external ref": map[string]any{"$ref": "https://example.test/schema"}, "missing ref": map[string]any{"$ref": "#/$defs/missing"}, "recursive ref": map[string]any{"$defs": map[string]any{"loop": map[string]any{"$ref": "#/$defs/loop"}}, "$ref": "#/$defs/loop"}, "non-schema ref": map[string]any{"const": map[string]any{"type": "string"}, "$ref": "#/const"}, "bad escape": map[string]any{"$ref": "#/$defs/a~2b"}, "bytes": map[string]any{"description": strings.Repeat("x", 65536)}, "bad required": map[string]any{"required": []any{"x", "x"}}, "negative bound": map[string]any{"minItems": -1}, "tuple": map[string]any{"items": []any{object()}}}
	for _, size := range []int{16, 17} {
		branches := []any{}
		for i := 0; i < size; i++ {
			branches = append(branches, map[string]any{"const": i})
		}
		s := map[string]any{"anyOf": branches}
		e := ValidateSchema("branches", s)
		if (e == nil) != (size == 16) {
			t.Fatal(size, e)
		}
	}
	for _, size := range []int{256, 257} {
		values := []any{}
		for i := 0; i < size; i++ {
			values = append(values, i)
		}
		e := ValidateSchema("enum", map[string]any{"enum": values})
		if (e == nil) != (size == 256) {
			t.Fatal(size, e)
		}
	}
	for _, size := range []int{255, 256} {
		props := map[string]any{}
		for i := 0; i < size; i++ {
			props[fmt.Sprint(i)] = map[string]any{"type": "string"}
		}
		e := ValidateSchema("nodes", map[string]any{"properties": props})
		if (e == nil) != (size == 255) {
			t.Fatal(size, e)
		}
	}
	for _, depth := range []int{16, 17} {
		var s any = map[string]any{"type": "string"}
		for i := 0; i < depth; i++ {
			s = map[string]any{"items": s}
		}
		e := ValidateSchema("depth", s)
		if (e == nil) != (depth == 16) {
			t.Fatal(depth, e)
		}
	}
	for name, s := range tests {
		t.Run(name, func(t *testing.T) {
			if ValidateSchema(name, s) == nil {
				t.Fatal("accepted invalid schema")
			}
		})
	}
	valid := map[string]any{"$defs": map[string]any{"a/b~c": map[string]any{"type": "string"}}, "properties": map[string]any{"value": map[string]any{"$ref": "#/$defs/a~1b~0c"}}}
	if e := ValidateSchema("local", valid); e != nil {
		t.Fatal(e)
	}
}
func TestSchemaBoundaryForEverySchemaBearingCapability(t *testing.T) {
	bad := map[string]any{"type": "object", "oneOf": []any{object()}}
	for _, id := range []string{"llm.chat", "llm.complete", "llm.complete-bulk", "document.extract"} {
		t.Run(id, func(t *testing.T) {
			in := map[string]any{"jsonSchema": bad}
			if id == "document.extract" {
				in = map[string]any{"schema": bad}
			}
			if id == "llm.complete-bulk" {
				in = map[string]any{"rows": []any{in}}
			}
			if _, e := PrepareInput(id, in); e == nil {
				t.Fatal("accepted unsupported response schema")
			}
		})
	}
	input := map[string]any{"tools": []any{map[string]any{"name": "download_file", "parameters": bad}}}
	if _, e := PrepareInput("llm.chat", input); e == nil || !strings.Contains(e.Error(), "download_file") {
		t.Fatal(e)
	}
	tools := []any{}
	for i := 0; i < 2; i++ {
		s := object()
		s["description"] = strings.Repeat("x", 33000)
		tools = append(tools, map[string]any{"name": fmt.Sprint(i), "parameters": s})
	}
	if _, e := PrepareInput("llm.chat", map[string]any{"tools": tools}); e == nil {
		t.Fatal("accepted combined schemas over 64 KiB")
	}
	// Ordinary action/outcome criteria are data, even when a field is named schema.
	if _, e := PrepareInput("apps.action.execute", map[string]any{"inputs": map[string]any{"schema": bad}}); e != nil {
		t.Fatal(e)
	}
}
func TestTransportNormalizesAndStrictFakeRefuses(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var v any
		_ = json.NewDecoder(r.Body).Decode(&v)
		if e := ValidateInput("llm.chat", v); e != nil {
			http.Error(w, e.Error(), 400)
			return
		}
		_, _ = w.Write([]byte(`{"text":"ready"}`))
	}))
	defer server.Close()
	c, _ := New("fixture", server.URL, nil)
	in := map[string]any{"tools": []any{map[string]any{"name": "lookup", "parameters": map[string]any{"type": "object", "$schema": "draft", "properties": map[string]any{"email": map[string]any{"type": "string", "format": "email"}}}}}}
	in["messages"] = []any{map[string]any{"role": "user", "content": "Synthetic request"}}
	if e := c.Call(context.Background(), "llm.chat", in, nil); e != nil {
		t.Fatal(e)
	}
	in["tools"].([]any)[0].(map[string]any)["parameters"].(map[string]any)["pattern"] = "constraint"
	if e := c.Call(context.Background(), "llm.chat", in, nil); e == nil || calls != 1 {
		t.Fatal("invalid schema reached transport", calls, e)
	}
	if _, e := c.Start(context.Background(), "llm.complete-bulk", map[string]any{"rows": []any{map[string]any{"jsonSchema": map[string]any{"oneOf": []any{object()}}}}}); e == nil || calls != 1 {
		t.Fatal("invalid batch reached transport", calls, e)
	}
}

func TestLiteralNumbersAreFiniteAndNotRounded(t *testing.T) {
	if e := ValidateSchema("numeric literal", map[string]any{"const": json.Number("1e999")}); e == nil || !strings.Contains(e.Error(), "#/const") {
		t.Fatal(e)
	}
	s, e := NormalizeSchema("numeric literal", map[string]any{"const": json.Number("9007199254740993")})
	if e != nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal(s)
	if !strings.Contains(string(b), "9007199254740993") {
		t.Fatal(string(b))
	}
}

func TestExtractionUsesItsSmallerContractLimit(t *testing.T) {
	for _, s := range []map[string]any{{"type": "array", "properties": map[string]any{}}, {"type": "object", "properties": map[string]any{}, "description": strings.Repeat("🙂", 4000)}} {
		if _, e := PrepareInput("document.extract", map[string]any{"schema": s}); e == nil {
			t.Fatal("accepted incompatible extraction schema")
		}
	}
}

func TestChatRejectsTooManyToolsBeforeTransport(t *testing.T) {
	tools := make([]any, 65)
	for i := range tools {
		tools[i] = map[string]any{"name": fmt.Sprintf("action_%d", i), "parameters": map[string]any{"type": "object", "properties": map[string]any{}}}
	}
	_, err := PrepareInput("llm.chat", map[string]any{"messages": []any{map[string]any{"role": "user", "content": "Synthetic"}}, "tools": tools})
	if err == nil || !strings.Contains(err.Error(), "#/tools") || !strings.Contains(err.Error(), "64") {
		t.Fatalf("oversized declarations must fail locally: %v", err)
	}
}
