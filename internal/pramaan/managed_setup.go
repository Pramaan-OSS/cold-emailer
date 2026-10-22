package pramaan

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ManagedState is private startup state. Never log or return the database addresses.
type ManagedState struct {
	DatabaseURL       string            `json:"databaseUrl,omitempty"`
	DirectDatabaseURL string            `json:"directDatabaseUrl,omitempty"`
	DatabaseAttempt   string            `json:"databaseAttempt,omitempty"`
	ActionKeys        map[string]string `json:"actionKeys,omitempty"`
	AuthURL           string            `json:"authUrl,omitempty"`
	JWKSURL           string            `json:"jwksUrl,omitempty"`
	AuthOrigin        string            `json:"appOrigin,omitempty"`
	AuthReady         bool              `json:"authReady"`
}

type ManagedSetup struct {
	Client            Caller
	Directory, AppURL string
	Notice            func(string)
}

func (s ManagedSetup) notice(message string) {
	if s.Notice != nil {
		s.Notice(message)
	}
}
func privateFile(path string, data []byte) error {
	f, e := os.CreateTemp(filepath.Dir(path), ".managed-*")
	if e != nil {
		return errors.New("Private startup state could not be saved.")
	}
	name := f.Name()
	defer os.Remove(name)
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(data)
	}
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	if e == nil {
		e = os.Rename(name, path)
	}
	if e != nil {
		return errors.New("Private startup state could not be saved.")
	}
	return nil
}
func (s ManagedSetup) save(state ManagedState) error {
	b, e := json.Marshal(state)
	if e != nil {
		return errors.New("Private startup state could not be prepared.")
	}
	return privateFile(filepath.Join(s.Directory, "managed.json"), b)
}
func ReadManagedState(directory string) (ManagedState, error) {
	var state ManagedState
	b, e := os.ReadFile(filepath.Join(directory, "managed.json"))
	if os.IsNotExist(e) {
		return state, nil
	}
	if e != nil || json.Unmarshal(b, &state) != nil {
		return state, errors.New("Private startup state could not be read.")
	}
	return state, nil
}
func (s ManagedSetup) load() (ManagedState, error) {
	if e := os.MkdirAll(s.Directory, 0700); e != nil {
		return ManagedState{}, errors.New("Private startup directory could not be created.")
	}
	if e := os.Chmod(s.Directory, 0700); e != nil {
		return ManagedState{}, errors.New("Private startup directory could not be protected.")
	}
	return ReadManagedState(s.Directory)
}
func databaseAddress(raw string) bool {
	u, e := url.Parse(raw)
	return e == nil && (u.Scheme == "postgresql" || u.Scheme == "postgres") && u.Hostname() != "" && u.User != nil && u.Path != "" && !strings.ContainsAny(raw, "\r\n'\" ")
}
func (s ManagedSetup) Database(ctx context.Context) error {
	state, e := s.load()
	if e != nil {
		return e
	}
	if state.DatabaseURL != "" {
		if !databaseAddress(state.DatabaseURL) || !databaseAddress(state.DirectDatabaseURL) {
			return errors.New("The saved managed database settings need review.")
		}
		return s.environment(state)
	}
	if s.Client == nil {
		return s.environment(state)
	}
	if state.DatabaseAttempt != "" {
		// The one-time credential response may have been lost. Do not provision or rotate again.
		var status map[string]any
		_ = s.Client.Call(ctx, "db.postgres.get", map[string]any{}, &status)
		return errors.New("Managed database setup is unconfirmed. Check its status and recover credentials before retrying.")
	}
	key, e := s.actionKey(&state, "db.postgres.provision")
	if e != nil {
		return e
	}
	state.DatabaseAttempt = key
	if e = s.save(state); e != nil {
		return e
	}
	var out struct {
		Status  string `json:"status"`
		Created bool   `json:"created"`
		URL     string `json:"connectionString"`
		Direct  string `json:"directConnectionString"`
	}
	e = s.Client.Call(ctx, "db.postgres.provision", map[string]any{}, &out, Idempotency(key))
	if errors.Is(e, ErrUnavailable) {
		state.DatabaseAttempt = ""
		if e = s.save(state); e != nil {
			return e
		}
		s.notice("Managed database is not available yet; using the bundled local database.")
		return s.environment(state)
	}
	if e != nil {
		return managedSetupError("Managed database setup", e)
	}
	if !out.Created || (out.Status != "ready" && out.Status != "paused") || !databaseAddress(out.URL) || !databaseAddress(out.Direct) {
		return errors.New("Managed database credentials were not returned. Recover credentials before trying again.")
	}
	state.DatabaseURL, state.DirectDatabaseURL = out.URL, out.Direct
	state.DatabaseAttempt = ""
	if e = s.save(state); e != nil {
		return e
	}
	return s.environment(state)
}
func (s ManagedSetup) environment(state ManagedState) error {
	// Single quotes disable Compose interpolation; validated addresses cannot escape them.
	text := ""
	if state.AuthReady {
		text += "PRAMAAN_MANAGED_AUTH_URL='" + state.AuthURL + "'\nPRAMAAN_MANAGED_JWKS_URL='" + state.JWKSURL + "'\nPRAMAAN_MANAGED_APP_ORIGIN='" + state.AuthOrigin + "'\n"
	}
	if state.DatabaseURL != "" {
		text += "PRIMARY_DB='" + state.DatabaseURL + "'\nPRIMARY_DB_DIRECT='" + state.DirectDatabaseURL + "'\nDATABASE_SSL='true'\n"
	}
	return privateFile(filepath.Join(s.Directory, "managed.env"), []byte(text))
}
func publicAuthURL(raw string) bool {
	u, e := url.Parse(raw)
	return e == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && net.ParseIP(u.Hostname()) == nil && strings.Contains(u.Hostname(), ".") && !strings.ContainsAny(raw, "\r\n'\" ")
}
func appOrigin(raw string) (string, bool, error) {
	u, e := url.Parse(raw)
	if e != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" && u.Path != "/" {
		return "", false, errors.New("APP_URL must be the app's own address without a path.")
	}
	local := u.Hostname() == "localhost" || net.ParseIP(u.Hostname()) != nil && net.ParseIP(u.Hostname()).IsLoopback()
	if local && (u.Scheme == "http" || u.Scheme == "https") {
		return u.Scheme + "://" + u.Host, true, nil
	}
	if !publicAuthURL(raw) {
		return "", false, errors.New("APP_URL must use HTTPS for a public app.")
	}
	return u.Scheme + "://" + u.Host, false, nil
}
func (s ManagedSetup) SignIn(ctx context.Context) error {
	state, e := s.load()
	if e != nil {
		return e
	}
	state.AuthReady = false
	if s.Client == nil {
		return s.finishSignIn(state)
	}
	origin, local, e := appOrigin(s.AppURL)
	if e != nil {
		return e
	}
	var out struct {
		Status  string `json:"status"`
		AuthURL string `json:"authUrl"`
		JWKSURL string `json:"jwksUrl"`
	}
	key, e := s.actionKey(&state, "auth.backend.provision")
	if e != nil {
		return e
	}
	// Provision is free and idempotent, including recovery of a lost answer.
	e = s.Client.Call(ctx, "auth.backend.provision", map[string]any{}, &out, Idempotency(key))
	if e != nil {
		if !errors.Is(e, ErrUnavailable) {
			return managedSetupError("Managed sign-in setup", e)
		}
		if err := s.finishSignIn(state); err != nil {
			return err
		}
		s.notice("Managed sign-in is not available yet; use local sign-in.")
		return nil
	}
	if out.Status != "ready" || !publicAuthURL(out.AuthURL) || !publicAuthURL(out.JWKSURL) {
		return errors.New("Managed sign-in returned incomplete settings.")
	}
	auth, _ := url.Parse(out.AuthURL)
	jwks, _ := url.Parse(out.JWKSURL)
	if auth.Host != jwks.Host {
		return errors.New("Managed sign-in returned inconsistent settings.")
	}
	if local {
		s.notice("Managed sign-in allows localhost for development; no public address registration is needed.")
	} else {
		var allowed map[string]any
		key, e = s.actionKey(&state, "auth.backend.domains.set:"+origin)
		if e != nil {
			return e
		}
		if e = s.Client.Call(ctx, "auth.backend.domains.set", map[string]any{"domains": []string{origin}}, &allowed, Idempotency(key)); e != nil {
			if !errors.Is(e, ErrUnavailable) {
				return managedSetupError("Managed sign-in address setup", e)
			}
			if err := s.finishSignIn(state); err != nil {
				return err
			}
			s.notice("Managed sign-in address setup is not available yet; use local sign-in.")
			return nil
		}
	}
	state.AuthURL, state.JWKSURL, state.AuthOrigin, state.AuthReady = out.AuthURL, out.JWKSURL, origin, true
	return s.finishSignIn(state)
}

// ManagedIdentity only comes from a platform-verified token. Browser profile fields are never trusted.
type ManagedIdentity struct {
	Subject, Issuer, Email, Name string
	EmailVerified                bool
	TokenHash                    string
}

func VerifyManagedToken(ctx context.Context, client Caller, state ManagedState, raw string) (ManagedIdentity, error) {
	refused := errors.New("Managed sign-in was refused. Sign in again.")
	if client == nil || !state.AuthReady || len(raw) > 8192 || len(raw) == 0 {
		return ManagedIdentity{}, refused
	}
	auth, e := url.Parse(state.AuthURL)
	if e != nil {
		return ManagedIdentity{}, refused
	}
	issuer := auth.Scheme + "://" + auth.Host
	var out struct {
		Valid     bool   `json:"valid"`
		UserID    string `json:"userId"`
		ExpiresAt string `json:"expiresAt"`
	}
	if e = client.Call(ctx, "auth.token.verify", map[string]any{"token": raw, "audience": issuer, "authorizedParties": []string{state.AuthOrigin}}, &out); e != nil || !out.Valid || out.UserID == "" {
		return ManagedIdentity{}, refused
	}
	identity, e := verifiedManagedClaims(raw, issuer, out.UserID)
	if e != nil {
		return ManagedIdentity{}, refused
	}
	if out.ExpiresAt != "" {
		expires, e := time.Parse(time.RFC3339, out.ExpiresAt)
		if e != nil || !expires.After(time.Now()) {
			return ManagedIdentity{}, refused
		}
	}
	return identity, nil
}

func (s ManagedSetup) finishSignIn(state ManagedState) error {
	if e := s.save(state); e != nil {
		return e
	}
	return s.environment(state)
}

// ManagedSignInFromEnv reads public sign-in settings only; database secrets stay private.
func ManagedSignInFromEnv() ManagedState {
	auth, jwks, origin := os.Getenv("PRAMAAN_MANAGED_AUTH_URL"), os.Getenv("PRAMAAN_MANAGED_JWKS_URL"), os.Getenv("PRAMAAN_MANAGED_APP_ORIGIN")
	app, _, e := appOrigin(origin)
	if e != nil || !publicAuthURL(auth) || !publicAuthURL(jwks) {
		return ManagedState{}
	}
	a, _ := url.Parse(auth)
	j, _ := url.Parse(jwks)
	if a.Host != j.Host {
		return ManagedState{}
	}
	return ManagedState{AuthURL: auth, JWKSURL: jwks, AuthOrigin: app, AuthReady: true}
}

// Save the reference before submitting an action. A changed origin is a different action.
func (s ManagedSetup) actionKey(state *ManagedState, action string) (string, error) {
	if state.ActionKeys == nil {
		state.ActionKeys = map[string]string{}
	}
	if key := state.ActionKeys[action]; key != "" {
		return key, nil
	}
	key, e := requestKey("")
	if e != nil {
		return "", e
	}
	state.ActionKeys[action] = key
	if e = s.save(*state); e != nil {
		return "", e
	}
	return key, nil
}
func managedSetupError(feature string, cause error) error {
	if errors.Is(cause, ErrIdempotencyRequired) {
		return errors.New(feature + " was refused: an Idempotency-Key header is required. Check the saved startup request reference.")
	}
	var wire *HTTPError
	if errors.As(cause, &wire) {
		switch wire.Code {
		case "invalid_request":
			return errors.New(feature + " was refused: the request was not accepted. Check its input and idempotency reference before trying again.")
		case "unauthorized", "forbidden":
			return errors.New(feature + " was refused: authorization was not accepted. Check the platform settings.")
		case "conflict":
			return errors.New(feature + " was refused: the saved request conflicts with an earlier attempt. Check its status before trying again.")
		case "rate_limited":
			return errors.New(feature + " was refused: too many requests. Try again later.")
		}
	}
	return errors.New(feature + " is unconfirmed. Check its status before trying again.")
}
