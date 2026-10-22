package pramaan

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type managedFake struct {
	ids      []string
	inputs   []any
	fail     error
	valid    bool
	existing bool
	keys     []string
}

func (f *managedFake) Call(_ context.Context, id string, in, out any, opts ...Option) error {
	f.ids = append(f.ids, id)
	f.inputs = append(f.inputs, in)
	options := callOptions{}
	for _, apply := range opts {
		apply(&options)
	}
	f.keys = append(f.keys, options.key)
	if id == "auth.backend.provision" || id == "auth.backend.domains.set" || id == "db.postgres.provision" {
		if options.key == "" {
			return ErrIdempotencyRequired
		}
	}
	if f.fail != nil {
		return f.fail
	}
	var value any
	switch id {
	case "db.postgres.provision":
		value = map[string]any{"status": "ready", "created": true, "connectionString": "postgres://member:private-value@pool.example.test/app", "directConnectionString": "postgres://member:private-value@direct.example.test/app"}
		if f.existing {
			value = map[string]any{"status": "ready", "created": false, "database": "app", "role": "member", "limits": map[string]any{}}
		}
	case "auth.backend.provision":
		value = map[string]any{"status": "ready", "authUrl": "https://signin.example.test/auth", "jwksUrl": "https://signin.example.test/auth/jwks"}
	case "auth.backend.domains.set":
		value = map[string]any{"allowedDomains": in.(map[string]any)["domains"]}
	case "auth.token.verify":
		value = map[string]any{"valid": f.valid}
		if f.valid {
			value.(map[string]any)["userId"] = "member-1"
		}
	case "db.postgres.get":
		value = map[string]any{"status": "ready"}
	default:
		return errors.New("unexpected capability")
	}
	b, _ := json.Marshal(value)
	return json.Unmarshal(b, out)
}
func TestManagedDatabaseProvisionReuseAndPermissions(t *testing.T) {
	dir := t.TempDir()
	f := &managedFake{}
	s := ManagedSetup{Directory: dir, Client: f}
	for range 2 {
		if e := s.Database(context.Background()); e != nil {
			t.Fatal(e)
		}
	}
	if strings.Join(f.ids, ",") != "db.postgres.provision" {
		t.Fatal(f.ids)
	}
	for _, name := range []string{"managed.json", "managed.env"} {
		info, e := os.Stat(filepath.Join(dir, name))
		if e != nil || info.Mode().Perm() != 0600 {
			t.Fatal(name, e)
		}
	}
	info, _ := os.Stat(dir)
	if info.Mode().Perm() != 0700 {
		t.Fatal("directory permissions")
	}
	state, e := ReadManagedState(dir)
	if e != nil || state.DatabaseURL == "" || state.DatabaseAttempt != "" {
		t.Fatal("credentials not persisted")
	}
	env, _ := os.ReadFile(filepath.Join(dir, "managed.env"))
	if !strings.Contains(string(env), "PRIMARY_DB_DIRECT='") {
		t.Fatal("migration address missing")
	}
	if !strings.Contains(string(env), "DATABASE_SSL='true'") {
		t.Fatal("managed database must use encrypted live-update connections")
	}
}
func TestManagedDatabaseUnavailableAndNoKey(t *testing.T) {
	for _, client := range []Caller{nil, &managedFake{fail: ErrUnavailable}} {
		var notices []string
		s := ManagedSetup{Client: client, Directory: t.TempDir(), Notice: func(v string) { notices = append(notices, v) }}
		if e := s.Database(context.Background()); e != nil {
			t.Fatal(e)
		}
		b, _ := os.ReadFile(filepath.Join(s.Directory, "managed.env"))
		if strings.Contains(string(b), "PRIMARY_DB=") {
			t.Fatal("must keep local database")
		}
		if client != nil && len(notices) != 1 {
			t.Fatal(notices)
		}
	}
}
func TestManagedDatabaseUnknownDoesNotProvisionAgain(t *testing.T) {
	f := &managedFake{fail: errors.New("lost response")}
	s := ManagedSetup{Client: f, Directory: t.TempDir()}
	if s.Database(context.Background()) == nil {
		t.Fatal("must refuse unknown")
	}
	f.fail = nil
	if s.Database(context.Background()) == nil {
		t.Fatal("must require recovery")
	}
	if strings.Join(f.ids, ",") != "db.postgres.provision,db.postgres.get" {
		t.Fatal(f.ids)
	}
}
func TestManagedSignInLocalPublicAndFallback(t *testing.T) {
	for _, app := range []string{"http://localhost:5173", "https://app.example.test"} {
		f := &managedFake{}
		s := ManagedSetup{Client: f, Directory: t.TempDir(), AppURL: app}
		for range 2 {
			if e := s.SignIn(context.Background()); e != nil {
				t.Fatal(e)
			}
		}
		state, e := ReadManagedState(s.Directory)
		if e != nil || !state.AuthReady {
			t.Fatal("not ready")
		}
		want := 2
		if strings.HasPrefix(app, "https:") {
			want = 4
		}
		if len(f.ids) != want {
			t.Fatal(f.ids)
		}
	}
	for _, client := range []Caller{nil, &managedFake{fail: ErrUnavailable}} {
		s := ManagedSetup{Client: client, Directory: t.TempDir(), AppURL: "http://localhost:5173"}
		if e := s.SignIn(context.Background()); e != nil {
			t.Fatal(e)
		}
		state, _ := ReadManagedState(s.Directory)
		if state.AuthReady {
			t.Fatal("fallback must not advertise managed sign-in")
		}
	}
}
func managedToken(t *testing.T, overrides map[string]any) string {
	t.Helper()
	v := map[string]any{"sub": "member-1", "iss": "https://signin.example.test", "aud": "https://signin.example.test", "email": "member@example.test", "name": "Test Member", "exp": time.Now().Add(time.Hour).Unix()}
	for k, x := range overrides {
		v[k] = x
	}
	b, _ := json.Marshal(v)
	return "header." + base64.RawURLEncoding.EncodeToString(b) + ".signature"
}
func TestManagedTokenVerifiedOrRefused(t *testing.T) {
	state := ManagedState{AuthReady: true, AuthURL: "https://signin.example.test/auth", AuthOrigin: "http://localhost:5173"}
	for _, valid := range []bool{true, false} {
		f := &managedFake{valid: valid}
		identity, e := VerifyManagedToken(context.Background(), f, state, managedToken(t, nil))
		if valid && (e != nil || identity.Email != "member@example.test" || identity.TokenHash == "") {
			t.Fatal("verified token refused", e)
		}
		if !valid && e == nil {
			t.Fatal("invalid accepted")
		}
	}
	for _, claims := range []map[string]any{{"iss": "https://other.example.test"}, {"sub": "other"}, {"exp": 0}, {"aud": "other"}, {"email": "invalid"}} {
		if _, e := VerifyManagedToken(context.Background(), &managedFake{valid: true}, state, managedToken(t, claims)); e == nil {
			t.Fatal("bad claims accepted", claims)
		}
	}
}

func TestManagedDatabaseUnavailableUnknownDoesNotFallback(t *testing.T) {
	f := &managedFake{fail: &HTTPError{Status: 502, Code: "upstream_unavailable", Effect: "unknown"}}
	var notices []string
	s := ManagedSetup{Client: f, Directory: t.TempDir(), Notice: func(v string) { notices = append(notices, v) }}
	if s.Database(context.Background()) == nil || len(notices) != 0 {
		t.Fatal("unknown provisioning must not fall back or retry")
	}
	state, _ := ReadManagedState(s.Directory)
	if state.DatabaseAttempt == "" {
		t.Fatal("pending attempt was lost")
	}
}

func TestManagedSetupHTTPUnavailableUsesLocalFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(502)
		_, _ = w.Write([]byte(`{"error":{"code":"upstream_unavailable","effect":"none"}}`))
	}))
	defer server.Close()
	c, e := New("synthetic-key", server.URL, nil)
	if e != nil {
		t.Fatal(e)
	}
	notices := 0
	s := ManagedSetup{Client: c, Directory: t.TempDir(), AppURL: "http://localhost:5173", Notice: func(string) { notices++ }}
	if e = s.Database(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e = s.SignIn(context.Background()); e != nil {
		t.Fatal(e)
	}
	if notices != 2 {
		t.Fatal("fallback notices missing")
	}
}

func TestManagedStartupKeysPersistAndSeparateActions(t *testing.T) {
	dir := t.TempDir()
	f := &managedFake{fail: ErrUnavailable}
	s := ManagedSetup{Client: f, Directory: dir, AppURL: "https://app.example.test"}
	for range 2 {
		if e := s.Database(context.Background()); e != nil {
			t.Fatal(e)
		}
	}
	if f.keys[0] == "" || f.keys[0] != f.keys[1] {
		t.Fatal("database reference changed")
	}
	f.fail = nil
	for range 2 {
		if e := s.SignIn(context.Background()); e != nil {
			t.Fatal(e)
		}
	}
	if f.keys[2] != f.keys[4] || f.keys[3] != f.keys[5] || f.keys[2] == f.keys[3] {
		t.Fatal("sign-in references must persist and be distinct")
	}
	state, e := ReadManagedState(dir)
	if e != nil || len(state.ActionKeys) != 3 {
		t.Fatal("references not saved", e)
	}
	other := ManagedSetup{Client: &managedFake{}, Directory: t.TempDir(), AppURL: s.AppURL}
	if e = other.SignIn(context.Background()); e != nil {
		t.Fatal(e)
	}
	next, _ := ReadManagedState(other.Directory)
	if state.ActionKeys["auth.backend.provision"] == next.ActionKeys["auth.backend.provision"] {
		t.Fatal("installations share an action key")
	}
}
func TestManagedProvisionTransportHeadersAndRefusalReason(t *testing.T) {
	for _, reject := range []bool{false, true} {
		t.Run(map[bool]string{false: "headers", true: "refusal"}[reject], func(t *testing.T) {
			keys := map[string]string{}
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				key := r.Header.Get("Idempotency-Key")
				if key == "" {
					t.Error("missing header")
				}
				if previous := keys[r.URL.Path]; previous != "" && previous != key {
					t.Error("changed header")
				}
				keys[r.URL.Path] = key
				w.Header().Set("Content-Type", "application/json")
				if reject {
					w.WriteHeader(400)
					_, _ = w.Write([]byte(`{"error":{"code":"invalid_request","message":"Idempotency-Key header is required for this capability. private-remote-detail"}}`))
					return
				}
				if strings.HasSuffix(r.URL.Path, "auth.backend.provision") {
					_, _ = w.Write([]byte(`{"status":"ready","authUrl":"https://signin.example.test/auth","jwksUrl":"https://signin.example.test/jwks"}`))
				} else {
					_, _ = w.Write([]byte(`{"allowedDomains":["https://app.example.test"]}`))
				}
			}))
			defer server.Close()
			client, _ := New("synthetic-key", server.URL, nil)
			s := ManagedSetup{Client: client, Directory: t.TempDir(), AppURL: "https://app.example.test"}
			e := s.SignIn(context.Background())
			if reject {
				if e == nil || !strings.Contains(e.Error(), "Idempotency-Key header is required") || strings.Contains(e.Error(), "private-remote-detail") {
					t.Fatal("refusal hidden or unsafe", e)
				}
				if calls != 1 {
					t.Fatal("refusal retried")
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			if e = s.SignIn(context.Background()); e != nil {
				t.Fatal(e)
			}
			if len(keys) != 2 || calls != 4 {
				t.Fatal(keys, calls)
			}
		})
	}
}

func TestOtherCapabilityActionsAlwaysSendAttemptHeader(t *testing.T) {
	for _, id := range []string{"auth.user.delete", "email.message.send", "apps.action.execute"} {
		t.Run(id, func(t *testing.T) {
			count := 0
			var first string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				count++
				value := r.Header.Get("Idempotency-Key")
				if value == "" {
					t.Error("action header absent")
				}
				if first != "" && first != value {
					t.Error("explicit attempt key changed")
				}
				first = value
				_, _ = w.Write([]byte(`{}`))
			}))
			defer server.Close()
			client, _ := New("synthetic-key", server.URL, nil)
			for range 2 {
				if e := client.Call(context.Background(), id, map[string]any{"userId": "synthetic-member", "confirm": true}, nil, Idempotency("synthetic-single-attempt")); e != nil {
					t.Fatal(e)
				}
			}
			if count != 2 {
				t.Fatal(count)
			}
		})
	}
}

func TestManagedDatabaseExistingWithoutCredentialsRequiresRecovery(t *testing.T) {
	f := &managedFake{existing: true}
	dir := t.TempDir()
	var notices []string
	setup := ManagedSetup{Client: f, Directory: dir, Notice: func(v string) { notices = append(notices, v) }}
	if e := setup.Database(context.Background()); e == nil || !strings.Contains(e.Error(), "Recover credentials") {
		t.Fatalf("existing database must require recovery: %v", e)
	}
	state, e := ReadManagedState(dir)
	if e != nil || state.DatabaseAttempt == "" || state.DatabaseURL != "" || state.DirectDatabaseURL != "" {
		t.Fatal("must preserve the unresolved attempt without fabricating credentials")
	}
	if e = setup.Database(context.Background()); e == nil {
		t.Fatal("must not provision again or silently fall back")
	}
	if strings.Join(f.ids, ",") != "db.postgres.provision,db.postgres.get" || len(notices) != 0 {
		t.Fatalf("unexpected calls/fallback: %v / %v", f.ids, notices)
	}
}
