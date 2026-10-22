package pramaan

import "strings"

// ConnectedFileIDs recognizes platform handles, never staging addresses.
// Traverse the result because one action can return several downloaded objects.
func ConnectedFileIDs(value any) []string {
	refs := []string{}
	seen := map[string]bool{}
	var walk func(any, int)
	walk = func(v any, depth int) {
		if depth > 12 {
			return
		}
		switch row := v.(type) {
		case map[string]any:
			if row["stored"] == false {
				return
			}
			if ref := stringField(row, "fileId"); strings.HasPrefix(ref, "file_") && !seen[ref] {
				refs = append(refs, ref)
				seen[ref] = true
			}
			for _, child := range row {
				walk(child, depth+1)
			}
		case []any:
			for _, child := range row {
				walk(child, depth+1)
			}
		}
	}
	walk(value, 0)
	return refs
}
