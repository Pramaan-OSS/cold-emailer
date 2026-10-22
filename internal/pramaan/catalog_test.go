package pramaan

import (
	"context"
	"testing"
)

func TestCatalogueSchemaUsesExactSelectedKey(t *testing.T) {
	f := &responseCaller{response: map[string]any{"results": []map[string]any{{"action": "DIFFERENT_ACTION", "inputSchema": map[string]any{}}, {"action": "SELECTED_ACTION", "inputSchema": map[string]any{"type": "object"}}}}}
	row, e := (Apps{Client: f}).Inspect(context.Background(), "SELECTED_ACTION")
	if e != nil || row["action"] != "SELECTED_ACTION" || f.calls != 1 || f.input["query"] != "SELECTED_ACTION" || f.input["limit"] != 50 {
		t.Fatal(row, e, f.input)
	}
	f.response = map[string]any{"results": []map[string]any{{"action": "SELECTED_ACTION"}}}
	if _, e = (Apps{Client: f}).Inspect(context.Background(), "SELECTED_ACTION"); e == nil {
		t.Fatal("missing schema accepted")
	}
	f.response = map[string]any{"results": []map[string]any{{"action": "DIFFERENT_ACTION", "inputSchema": map[string]any{}}}}
	if _, e = (Apps{Client: f}).Inspect(context.Background(), "SELECTED_ACTION"); e == nil {
		t.Fatal("different action accepted")
	}
}
func TestCatalogueKeywordBoundAndConnectionRevocationScope(t *testing.T) {
	ctx := context.Background()
	f := &responseCaller{response: map[string]any{"results": []map[string]any{{"app": "google_search_console", "status": "active"}}}}
	g := Gateway{Client: f, Registry: memoryRegistry{}}
	if _, e := g.Call(ctx, "one", "apps.catalog.search", map[string]any{"query": " "}, "key"); e == nil || f.calls != 0 {
		t.Fatal("empty search reached transport")
	}
	if _, e := g.Call(ctx, "one", "apps.catalog.search", map[string]any{"query": "find records", "app": "google_search_console", "limit": 1000, "cursor": "next"}, "key"); e != nil {
		t.Fatal(e)
	}
	if f.input["app"] != "google_search_console" || f.input["limit"] != 50 || f.input["cursor"] != "next" {
		t.Fatal(f.input)
	}
	if _, e := g.Call(ctx, "one", "apps.connections.list", nil, "key"); e != nil {
		t.Fatal(e)
	}
	before := f.calls
	if _, e := g.Call(ctx, "two", "apps.connection.revoke", map[string]any{"app": "google_search_console"}, "key"); e == nil || f.calls != before {
		t.Fatal("cross-workspace revoke")
	}
	f.response = map[string]any{"revoked": 1}
	if _, e := g.Call(ctx, "one", "apps.connection.revoke", map[string]any{"app": "google_search_console", "endUserRef": "org_two"}, "key"); e != nil {
		t.Fatal(e)
	}
	if f.input["endUserRef"] != "org_one" {
		t.Fatal(f.input)
	}
}

func TestCatalogueExplicitCursorPageKeepsNextCursor(t *testing.T) {
	f := &responseCaller{response: map[string]any{"results": []map[string]any{{"action": "FIRST"}}, "nextCursor": "opaque-next"}}
	apps := Apps{Client: f}
	first, e := apps.Search(context.Background(), "Read file", "fixture_app")
	if e != nil || first["nextCursor"] != "opaque-next" || f.calls != 1 {
		t.Fatal(first, e)
	}
	f.response = map[string]any{"results": []map[string]any{{"action": "LAST"}}}
	last, e := apps.Search(context.Background(), "Read file", "fixture_app", "opaque-next")
	if e != nil || last["nextCursor"] != nil || f.calls != 2 || f.input["cursor"] != "opaque-next" || f.input["app"] != "fixture_app" || f.input["query"] != "Read file" {
		t.Fatal(last, e, f.input)
	}
}
func TestConnectedDownloadRegistersFileThenParsesWithoutUpload(t *testing.T) {
	ctx := context.Background()
	f := &responseCaller{response: map[string]any{"status": "succeeded", "output": map[string]any{"downloaded_file_content": map[string]any{"fileId": "file_synthetic", "url": "https://example.test/staging"}}}}
	g := Gateway{Client: f, Registry: memoryRegistry{"one": {"action:SELECTED_DOWNLOAD": true}}}
	result, e := g.Call(ctx, "one", "apps.action.execute", map[string]any{"action": "SELECTED_DOWNLOAD", "input": map[string]any{}}, "key")
	if e != nil || result.(map[string]any)["status"] != "succeeded" {
		t.Fatal(result, e)
	}
	f.response = map[string]any{"text": "Parsed synthetic text", "truncated": false}
	_, e = g.Call(ctx, "one", "document.parse", map[string]any{"fileId": "file_synthetic", "ocr": false, "url": "https://example.test/untrusted", "content": "bytes"}, "read")
	if e != nil || f.input["fileId"] != "file_synthetic" || f.input["url"] != nil || f.input["content"] != nil || f.calls != 2 {
		t.Fatal(e, f.input, f.calls)
	}
	before := f.calls
	if _, e = g.Call(ctx, "two", "document.parse", map[string]any{"fileId": "file_synthetic"}, "read"); e == nil || f.calls != before {
		t.Fatal("other workspace read allowed")
	}
}
func TestConnectedFileCopyFailureRetainsSucceededAction(t *testing.T) {
	f := &responseCaller{response: map[string]any{"status": "succeeded", "output": map[string]any{"downloaded_file_content": map[string]any{"stored": false, "note": "copy failed"}, "other": "retained"}}}
	g := Gateway{Client: f, Registry: memoryRegistry{"one": {"action:SELECTED_DOWNLOAD": true}}}
	result, e := g.Call(context.Background(), "one", "apps.action.execute", map[string]any{"action": "SELECTED_DOWNLOAD"}, "key")
	if e != nil || result.(map[string]any)["status"] != "succeeded" || len(ConnectedFileIDs(result)) != 0 || f.calls != 1 {
		t.Fatal(result, e)
	}
}
