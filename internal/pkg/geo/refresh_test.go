package geo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestEnsureLeavesAFreshDatabaseAloneWithoutAskingTheServer(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Write(testDatabase(t))
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "GeoLite2-City.mmdb")
	if err := os.WriteFile(path, testDatabase(t), 0o644); err != nil {
		t.Fatal(err)
	}

	if fetched, err := Ensure(context.Background(), path, srv.URL); err != nil || fetched {
		t.Fatalf("Ensure = (%v, %v), want (false, nil)", fetched, err)
	}
	if hits.Load() != 0 {
		t.Fatalf("server was asked %d times about a database written seconds ago", hits.Load())
	}
}

func TestAClientAnswersBeforeItsDatabaseArrivesAndAfterItLands(t *testing.T) {
	path := filepath.Join(t.TempDir(), "GeoLite2-City.mmdb")
	if err := os.WriteFile(path, testDatabase(t), 0o644); err != nil {
		t.Fatal(err)
	}

	// This is the boot shape: a client with no database at all.
	c, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	info, err := c.Lookup(netip.MustParseAddr("8.8.8.8"))
	if err != nil {
		t.Fatalf("a client with no database refused a lookup: %v", err)
	}
	if info.Country != "Unknown" {
		t.Fatalf("country = %q before the database landed, want Unknown", info.Country)
	}

	if err := c.Use(path); err != nil {
		t.Fatalf("Use: %v", err)
	}
	if _, err := c.Lookup(netip.MustParseAddr("8.8.8.8")); err != nil {
		t.Fatalf("lookup after the database landed: %v", err)
	}
	// Swapping again must not leave the previous reader open or panic a
	// concurrent caller.
	if err := c.Use(path); err != nil {
		t.Fatalf("second Use: %v", err)
	}
	c.Close()
}
