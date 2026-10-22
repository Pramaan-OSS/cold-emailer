package pramaan

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode/utf16"
)

// SchemaError identifies a local schema failure before any paid request is sent.
type SchemaError struct{ Name, Path, Reason string }

func (e *SchemaError) Error() string {
	return fmt.Sprintf("Schema %q at %s: %s", e.Name, e.Path, e.Reason)
}
func schemaError(name, path, reason string) error { return &SchemaError{name, path, reason} }
func pointer(s string) string                     { return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1") }

var schemaKeys = strings.Fields("type properties required additionalProperties items enum const anyOf $defs $ref title description default examples minimum maximum exclusiveMinimum exclusiveMaximum minLength maxLength minItems maxItems")

// NormalizeSchema removes the two unsupported annotations. Constraints are never
// dropped or approximated. Data inside enum, const, default and examples is untouched.
func NormalizeSchema(name string, value any) (map[string]any, error) {
	return checkSchema(name, value, true, false)
}

// ValidateSchema is strict, for fixtures and inventory tests of the wire schema.
func ValidateSchema(name string, value any) error {
	_, e := checkSchema(name, value, false, false)
	return e
}

// RemoteSchema also withholds connector-authored annotations from the assistant.
func RemoteSchema(name string, value any) (map[string]any, error) {
	return checkSchema(name, value, true, true)
}
func MustSchema(name string, value any) map[string]any {
	s, e := NormalizeSchema(name, value)
	if e != nil {
		panic(e)
	}
	return s
}

func checkSchema(name string, value any, normalize, remote bool) (map[string]any, error) {
	b, e := json.Marshal(value)
	if e != nil {
		return nil, schemaError(name, "#", "must contain finite JSON values without cycles")
	}
	var root map[string]any
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.UseNumber()
	if decoder.Decode(&root) != nil || root == nil {
		return nil, schemaError(name, "#", "root must be an object")
	}
	allowed := map[string]bool{}
	for _, k := range schemaKeys {
		allowed[k] = true
	}
	nodes := map[string]map[string]any{}
	var walk func(any, string, int) error
	walk = func(v any, path string, depth int) error {
		node, ok := v.(map[string]any)
		if !ok || node == nil {
			return schemaError(name, path, "must be a schema object")
		}
		if depth > 16 {
			return schemaError(name, path, "schema depth exceeds 16")
		}
		if len(nodes) >= 256 {
			return schemaError(name, path, "schema nodes exceed 256")
		}
		nodes[path] = node
		keys := make([]string, 0, len(node))
		for k := range node {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := node[k]
			p := path + "/" + pointer(k)
			if normalize && (k == "$schema" || k == "format") {
				delete(node, k)
				continue
			}
			if remote && (k == "title" || k == "description" || k == "default" || k == "examples") {
				delete(node, k)
				continue
			}
			if !allowed[k] {
				return schemaError(name, p, "unsupported keyword "+k)
			}
			switch k {
			case "type":
				ts, ok := v.([]any)
				if !ok {
					ts = []any{v}
				}
				seen := map[string]bool{}
				if len(ts) == 0 {
					return schemaError(name, p, "type must be nonempty")
				}
				for _, t := range ts {
					s, ok := t.(string)
					if !ok || !map[string]bool{"object": true, "array": true, "string": true, "number": true, "integer": true, "boolean": true, "null": true}[s] || seen[s] {
						return schemaError(name, p, "invalid or repeated type")
					}
					seen[s] = true
				}
			case "properties", "$defs":
				children, ok := v.(map[string]any)
				if !ok {
					return schemaError(name, p, "must be an object")
				}
				names := []string{}
				for n := range children {
					names = append(names, n)
				}
				sort.Strings(names)
				for _, n := range names {
					if e := walk(children[n], p+"/"+pointer(n), depth+1); e != nil {
						return e
					}
				}
			case "items":
				if e := walk(v, p, depth+1); e != nil {
					return e
				}
			case "additionalProperties":
				if _, ok := v.(bool); !ok {
					if e := walk(v, p, depth+1); e != nil {
						return e
					}
				}
			case "anyOf":
				xs, ok := v.([]any)
				if !ok || len(xs) == 0 || len(xs) > 16 {
					return schemaError(name, p, "requires 1 to 16 branches")
				}
				for i, x := range xs {
					if e := walk(x, fmt.Sprintf("%s/%d", p, i), depth+1); e != nil {
						return e
					}
				}
			case "enum":
				xs, ok := v.([]any)
				if !ok || len(xs) == 0 || len(xs) > 256 {
					return schemaError(name, p, "requires 1 to 256 values")
				}
			case "required":
				xs, ok := v.([]any)
				if !ok {
					return schemaError(name, p, "must be an array of unique strings")
				}
				seen := map[string]bool{}
				for _, x := range xs {
					s, ok := x.(string)
					if !ok || seen[s] {
						return schemaError(name, p, "must be an array of unique strings")
					}
					seen[s] = true
				}
			case "title", "description", "$ref":
				if _, ok := v.(string); !ok {
					return schemaError(name, p, "must be a string")
				}
			case "examples":
				if _, ok := v.([]any); !ok {
					return schemaError(name, p, "must be an array")
				}
			case "minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum", "minLength", "maxLength", "minItems", "maxItems":
				number, ok := v.(json.Number)
				n, numberErr := number.Float64()
				if !ok || numberErr != nil || math.IsInf(n, 0) || math.IsNaN(n) {
					return schemaError(name, p, "must be a finite number")
				}
				if (strings.HasSuffix(k, "Length") || strings.HasSuffix(k, "Items")) && (n < 0 || math.Trunc(n) != n) {
					return schemaError(name, p, "must be a nonnegative integer")
				}
			}
		}
		return nil
	}
	if e = walk(root, "#", 0); e != nil {
		return nil, e
	}
	expanded := 0
	var refs func(string, map[string]bool, int) error
	refs = func(path string, stack map[string]bool, depth int) error {
		expanded++
		if stack[path] || depth > 32 || expanded > 4096 {
			return schemaError(name, path, "recursive or excessive reference expansion")
		}
		stack[path] = true
		defer delete(stack, path)
		node := nodes[path]
		if ref, ok := node["$ref"].(string); ok {
			if !strings.HasPrefix(ref, "#/") {
				return schemaError(name, path+"/$ref", "only local #/... references are allowed")
			}
			for i := 0; i < len(ref); i++ {
				if ref[i] == '~' && (i+1 == len(ref) || (ref[i+1] != '0' && ref[i+1] != '1')) {
					return schemaError(name, path+"/$ref", "invalid reference escape")
				}
			}
			if nodes[ref] == nil {
				return schemaError(name, path+"/$ref", "reference must resolve to a schema node")
			}
			if e := refs(ref, stack, depth+1); e != nil {
				return e
			}
		}
		for p := range nodes {
			rest := strings.TrimPrefix(p, path+"/")
			if rest != p {
				// A child schema has one edge through properties/$defs/anyOf, or a direct edge.
				parts := strings.Split(rest, "/")
				if len(parts) == 1 || (len(parts) == 2 && (parts[0] == "properties" || parts[0] == "$defs" || parts[0] == "anyOf")) {
					if e := refs(p, stack, depth+1); e != nil {
						return e
					}
				}
			}
		}
		return nil
	}
	if e = refs("#", map[string]bool{}, 0); e != nil {
		return nil, e
	}
	b, _ = json.Marshal(root)
	if len(b) > 65536 {
		return nil, schemaError(name, "#", "schema exceeds 64 KiB")
	}
	var jsonValues func(any, string, int) error
	jsonValues = func(v any, path string, depth int) error {
		if depth > 32 {
			return schemaError(name, path, "JSON data depth exceeds 32")
		}
		switch x := v.(type) {
		case json.Number:
			n, e := x.Float64()
			if e != nil || math.IsInf(n, 0) || math.IsNaN(n) {
				return schemaError(name, path, "must be a finite JSON number")
			}
		case map[string]any:
			keys := []string{}
			for key := range x {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				if e := jsonValues(x[key], path+"/"+pointer(key), depth+1); e != nil {
					return e
				}
			}
		case []any:
			for i, value := range x {
				if e := jsonValues(value, fmt.Sprintf("%s/%d", path, i), depth+1); e != nil {
					return e
				}
			}
		}
		return nil
	}
	if e := jsonValues(root, "#", 0); e != nil {
		return nil, e
	}
	return root, nil
}

// PrepareInput covers all schema-bearing capability paths. Outcome inputs in
// this product carry only lead criteria, never schemas. Arbitrary action inputs
// are data and must not be walked as schemas.
func PrepareInput(id string, in any) (any, error) { return prepareInput(id, in, true) }
func ValidateInput(id string, in any) error       { _, e := prepareInput(id, in, false); return e }
func prepareInput(id string, in any, normalize bool) (any, error) {
	if err := validateMailInput(id, in); err != nil {
		return nil, err
	}
	if id != "llm.chat" && id != "llm.complete" && id != "llm.complete-bulk" && id != "document.extract" {
		return in, nil
	}
	b, e := json.Marshal(in)
	if e != nil {
		return nil, e
	}
	var obj map[string]any
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.UseNumber()
	if e = decoder.Decode(&obj); e != nil || obj == nil {
		return nil, schemaError(id, "#", "request must be an object")
	}
	check := func(name string, s any) (map[string]any, error) { return checkSchema(name, s, normalize, false) }
	if id == "llm.complete-bulk" {
		rows, ok := obj["rows"].([]any)
		if !ok {
			return nil, schemaError(id, "#/rows", "must be an array")
		}
		for i, row := range rows {
			v, e := prepareInput("llm.complete", row, normalize)
			if e != nil {
				return nil, fmt.Errorf("batch row %d: %w", i, e)
			}
			rows[i] = v
		}
		return obj, nil
	}
	key := "jsonSchema"
	if id == "document.extract" {
		key = "schema"
	}
	combined := []any{map[string]any{}}
	if s, ok := obj[key]; ok {
		s, e := check(id+" "+key, s)
		if e != nil {
			return nil, e
		}
		obj[key] = s
		combined[0] = s
		if id == "document.extract" {
			b, _ := json.Marshal(s)
			if len(utf16.Encode([]rune(string(b)))) > 8000 {
				return nil, schemaError(id, "#/schema", "extraction schema exceeds 8,000 characters")
			}
			if t, present := s["type"]; present && t != "object" {
				return nil, schemaError(id, "#/schema/type", "extraction requires an object schema")
			}
			if _, ok := s["properties"].(map[string]any); !ok {
				return nil, schemaError(id, "#/schema/properties", "extraction requires an object")
			}
		}
	}
	if id == "llm.chat" {
		if v, ok := obj["tools"]; ok {
			ts, ok := v.([]any)
			if !ok {
				return nil, schemaError(id, "#/tools", "must be an array")
			}
			if len(ts) > 64 {
				return nil, schemaError(id, "#/tools", "must contain at most 64 definitions")
			}
			if len(ts) == 0 {
				if normalize {
					delete(obj, "tools")
				} else {
					return nil, schemaError(id, "#/tools", "omit empty tools")
				}
			}
			for i, t := range ts {
				tool, ok := t.(map[string]any)
				if !ok {
					return nil, schemaError(id, fmt.Sprintf("#/tools/%d", i), "must be an object")
				}
				name, _ := tool["name"].(string)
				s, e := check("tool "+name, tool["parameters"])
				if e != nil {
					return nil, e
				}
				tool["parameters"] = s
				combined = append(combined, s)
			}
		}
		b, _ := json.Marshal(combined)
		if len(b) > 65536 {
			return nil, schemaError(id, "#/tools", "combined schemas exceed 64 KiB")
		}
	}
	if id == "llm.chat" || id == "llm.complete" {
		if _, present := obj["prompt"]; present {
			return nil, fmt.Errorf("%s requires messages; prompt is not a request field", id)
		}
		messages, ok := obj["messages"].([]any)
		if !ok || len(messages) == 0 {
			return nil, fmt.Errorf("%s requires a nonempty messages array", id)
		}
		if token, present := obj["maxTokens"]; present {
			number, ok := token.(json.Number)
			n, err := number.Int64()
			if !ok || err != nil || n < 1 || n > 8192 {
				return nil, fmt.Errorf("%s maxTokens must be an integer between 64 and 8192", id)
			}
			if n < 64 {
				if !normalize {
					return nil, fmt.Errorf("%s maxTokens must be at least 64", id)
				}
				obj["maxTokens"] = 64
			}
		}
	}
	return obj, nil
}

// CheckedCall applies the same boundary even when a test supplies a fake Caller.
func CheckedCall(ctx context.Context, c Caller, id string, in, out any, opts ...Option) error {
	v, e := PrepareInput(id, in)
	if e != nil {
		return e
	}
	if e := c.Call(ctx, id, v, out, opts...); e != nil {
		return e
	}
	b, _ := json.Marshal(out)
	return checkGenerationResult(id, v, b)
}
