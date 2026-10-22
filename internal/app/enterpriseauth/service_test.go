package enterpriseauth

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
	"github.com/pramaan-oss/cold-email/internal/repository"
)

type fakeMembers struct {
	org, user uuid.UUID
	role      string
	present   bool
}

func (m *fakeMembers) GetMember(_ context.Context, org, user uuid.UUID) (*models.OrganizationMember, error) {
	if !m.present || org != m.org || user != m.user {
		return nil, nil
	}
	now := time.Now()
	return &models.OrganizationMember{OrganizationID: org, UserID: user, Role: m.role, AcceptedAt: &now}, nil
}

type fakeConnections struct {
	rows []repository.EnterpriseConnection
}

func (r *fakeConnections) Save(_ context.Context, org uuid.UUID, ref, name, created string) (repository.EnterpriseConnection, error) {
	date, _ := time.Parse(time.RFC3339, created)
	for _, row := range r.rows {
		if row.OrganizationID == org && row.Name == name && row.CreatedAt.Equal(date) {
			return row, nil
		}
	}
	row := repository.EnterpriseConnection{ID: uuid.New(), OrganizationID: org, Ref: ref, Name: name, CreatedAt: date}
	r.rows = append(r.rows, row)
	return row, nil
}
func (r *fakeConnections) Get(_ context.Context, org, id uuid.UUID) (repository.EnterpriseConnection, error) {
	for _, row := range r.rows {
		if row.OrganizationID == org && row.ID == id {
			return row, nil
		}
	}
	return repository.EnterpriseConnection{}, ErrRefused
}
func (r *fakeConnections) List(_ context.Context, org uuid.UUID) ([]repository.EnterpriseConnection, error) {
	out := []repository.EnterpriseConnection{}
	for _, row := range r.rows {
		if row.OrganizationID == org {
			out = append(out, row)
		}
	}
	return out, nil
}
func (r *fakeConnections) Delete(_ context.Context, org, id uuid.UUID) error {
	for i, row := range r.rows {
		if row.OrganizationID == org && row.ID == id {
			r.rows = append(r.rows[:i], r.rows[i+1:]...)
			return nil
		}
	}
	return ErrRefused
}

type fakeStates struct{ data map[string][]byte }

func (s *fakeStates) Put(_ context.Context, k string, b []byte, _ time.Duration) error {
	s.data[k] = b
	return nil
}
func (s *fakeStates) Take(_ context.Context, k string) ([]byte, error) {
	b := s.data[k]
	delete(s.data, k)
	return b, nil
}

type fakeIdentities struct {
	repository.IdentityRepository
	bindings map[string]uuid.UUID
	linked   int
}

func (i *fakeIdentities) FindUserByIdentity(_ context.Context, issuer, subject string) (uuid.UUID, error) {
	return i.bindings[issuer+"|"+subject], nil
}
func (i *fakeIdentities) Link(_ context.Context, user uuid.UUID, identity models.UserIdentity) error {
	k := identity.Issuer + "|" + identity.Subject
	if old := i.bindings[k]; old != uuid.Nil && old != user {
		return repository.ErrIdentityTaken
	}
	i.bindings[k] = user
	i.linked++
	if identity.Email != "" {
		return errors.New("email must not be a linking key")
	}
	return nil
}
func (i *fakeIdentities) TouchLogin(context.Context, string, string) error { return nil }

type fakePlatform struct {
	org     uuid.UUID
	calls   []string
	starts  map[string]map[string]any
	fail    bool
	subject string
}

func (p *fakePlatform) Call(_ context.Context, id string, in, out any, _ ...pramaan.Option) error {
	p.calls = append(p.calls, id)
	if p.fail {
		return errors.New("synthetic failure")
	}
	b, _ := json.Marshal(in)
	var body map[string]any
	json.Unmarshal(b, &body)
	var result any
	switch id {
	case "auth.enterprise-connection.create":
		if body["organizationRef"] != pramaan.EnterpriseOrganization(p.org) || body["issuer"] == nil || body["redirectUris"] == nil {
			return errors.New("bad setup")
		}
		result = pramaan.EnterpriseConnection{Ref: "created-ref", OrganizationRef: pramaan.EnterpriseOrganization(p.org), Name: "Organisation sign-in", Status: "ready", CreatedAt: "2026-10-13T00:00:00Z"}
	case "auth.enterprise-connections.list":
		if body["organizationRef"] != pramaan.EnterpriseOrganization(p.org) {
			return errors.New("wrong organisation")
		}
		result = pramaan.EnterprisePage{Results: []pramaan.EnterpriseConnection{{Ref: "list-ref", OrganizationRef: pramaan.EnterpriseOrganization(p.org), Name: "Organisation sign-in", Status: "ready", CreatedAt: "2026-10-13T00:00:00Z"}}}
	case "auth.enterprise-connection.delete":
		if body["ref"] != "created-ref" || body["confirm"] != true {
			return errors.New("wrong connection")
		}
		result = map[string]any{"deleted": true}
	case "auth.enterprise.start":
		p.starts[body["state"].(string)] = body
		result = pramaan.EnterpriseLink{URL: "https://api.pramaan.dev/connect/link/synthetic", ExpiresAt: time.Now().Add(10 * time.Minute).Format(time.RFC3339)}
	case "auth.social.exchange":
		state, _ := body["state"].(string)
		started := p.starts[state]
		if started == nil || started["connectionRef"] != body["connectionRef"] || started["nonce"] != body["expectedNonce"] || body["providerKey"] != "enterprise" {
			return errors.New("not started here")
		}
		delete(p.starts, state)
		result = pramaan.EnterpriseIdentity{ProviderKey: "enterprise", Subject: p.subject, Email: "someone-else@example.test", EmailVerified: false}
	default:
		return errors.New("unexpected capability")
	}
	b, _ = json.Marshal(result)
	return json.Unmarshal(b, out)
}
func fixture(t *testing.T) (*Service, *fakePlatform, *fakeMembers, *fakeIdentities, repository.EnterpriseConnection) {
	t.Helper()
	org, user := uuid.New(), uuid.New()
	p := &fakePlatform{org: org, starts: map[string]map[string]any{}, subject: "subject-1"}
	m := &fakeMembers{org: org, user: user, role: "owner", present: true}
	ids := &fakeIdentities{bindings: map[string]uuid.UUID{}}
	svc := &Service{Platform: pramaan.Enterprise{Client: p, RedirectURI: "https://app.example.test/v1/auth/enterprise/callback"}, Members: m, Connections: &fakeConnections{}, Identities: ids, States: &fakeStates{data: map[string][]byte{}}}
	row, e := svc.Create(context.Background(), org, user, uuid.NewString(), pramaan.EnterpriseCreate{Name: "Organisation sign-in", Issuer: "https://issuer.example.test", ClientID: "public-client", TokenAuthMethod: "none", Approved: true})
	if e != nil {
		t.Fatal(e)
	}
	return svc, p, m, ids, row
}
func TestEnterpriseAdminConnectionLifecycleAndStableNamespace(t *testing.T) {
	s, p, m, _, row := fixture(t)
	ctx := context.Background()
	rows, _, e := s.List(ctx, m.org, m.user, "")
	if e != nil || len(rows) != 1 || rows[0].ID != row.ID || rows[0].Ref != "created-ref" {
		t.Fatal(rows, e)
	}
	if e = s.Delete(ctx, m.org, m.user, row.ID, uuid.NewString(), true); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Connections.Get(ctx, m.org, row.ID); e == nil {
		t.Fatal("deleted connection retained")
	}
	if len(p.calls) != 3 {
		t.Fatal(p.calls)
	}
}
func TestEnterpriseAdminRoleAndApprovalRefusedBeforeCalls(t *testing.T) {
	s, p, m, _, row := fixture(t)
	p.calls = nil
	m.role = "member"
	ctx := context.Background()
	if _, e := s.Create(ctx, m.org, m.user, uuid.NewString(), pramaan.EnterpriseCreate{Approved: true}); e == nil {
		t.Fatal("member create")
	}
	if _, _, e := s.List(ctx, m.org, m.user, ""); e == nil {
		t.Fatal("member list")
	}
	if e := s.Delete(ctx, m.org, m.user, row.ID, uuid.NewString(), true); e == nil {
		t.Fatal("member delete")
	}
	if len(p.calls) != 0 {
		t.Fatal(p.calls)
	}
	m.role = "admin"
	if e := s.Delete(ctx, m.org, m.user, row.ID, uuid.NewString(), false); e == nil {
		t.Fatal("no approval")
	}
	if len(p.calls) != 0 {
		t.Fatal(p.calls)
	}
}
func TestEnterpriseLinkThenLocalSignInNeverUsesEmail(t *testing.T) {
	s, p, m, ids, row := fixture(t)
	ctx := context.Background()
	start, e := s.Begin(ctx, m.org, row.ID, m.user, true)
	if e != nil {
		t.Fatal(e)
	}
	res, e := s.Redeem(ctx, "one-code", start.State, start.State)
	if e != nil || res.UserID != m.user || res.Binding != start.Binding || ids.linked != 1 {
		t.Fatal(res, e)
	}
	if ids.bindings["enterprise:"+row.ID.String()+"|subject-1"] != m.user {
		t.Fatal(ids.bindings)
	}
	if _, e = s.Redeem(ctx, "one-code", start.State, start.State); e == nil {
		t.Fatal("replay")
	}
	begin, e := s.Begin(ctx, m.org, row.ID, uuid.Nil, false)
	if e != nil {
		t.Fatal(e)
	}
	res, e = s.Redeem(ctx, "next-code", begin.State, begin.State)
	if e != nil || res.UserID != m.user || ids.linked != 1 {
		t.Fatal(res, e)
	}
	if len(p.calls) != 5 {
		t.Fatal(p.calls)
	}
}
func TestEnterpriseMissingWrongOrUnstartedStateCannotRedeem(t *testing.T) {
	s, p, m, _, row := fixture(t)
	ctx := context.Background()
	start, e := s.Begin(ctx, m.org, row.ID, m.user, true)
	if e != nil {
		t.Fatal(e)
	}
	before := len(p.calls)
	for _, states := range [][2]string{{"", start.State}, {start.State, ""}, {"wrong", start.State}, {start.State, "wrong"}, {"unstarted-state-123456789", "unstarted-state-123456789"}} {
		if _, e = s.Redeem(ctx, "code", states[0], states[1]); e == nil {
			t.Fatal(states)
		}
	}
	if len(p.calls) != before {
		t.Fatal("bad states sent to exchange", p.calls)
	}
	if _, e = s.Redeem(ctx, "code", start.State, start.State); e != nil {
		t.Fatal("wrong state consumed right flow", e)
	}
}
func TestEnterpriseUnknownIdentityCannotClaimVerifiedEmailOrMembership(t *testing.T) {
	s, p, m, ids, row := fixture(t)
	ctx := context.Background()
	start, e := s.Begin(ctx, m.org, row.ID, uuid.Nil, false)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Redeem(ctx, "code", start.State, start.State); e == nil {
		t.Fatal("unknown identity linked")
	}
	if ids.linked != 0 {
		t.Fatal("implicit linking")
	}
	p.subject = "subject-2"
	ids.bindings["enterprise:"+row.ID.String()+"|subject-2"] = uuid.New()
	start, _ = s.Begin(ctx, m.org, row.ID, uuid.Nil, false)
	if _, e = s.Redeem(ctx, "other-code", start.State, start.State); e == nil {
		t.Fatal("nonmember login")
	}
}
func TestEnterpriseCrossOrganisationAndRemovedMembershipRefused(t *testing.T) {
	s, p, m, _, row := fixture(t)
	ctx := context.Background()
	calls := len(p.calls)
	if _, e := s.Begin(ctx, uuid.New(), row.ID, m.user, true); e == nil {
		t.Fatal("cross-org link")
	}
	if _, e := s.Begin(ctx, uuid.New(), row.ID, uuid.Nil, false); e == nil {
		t.Fatal("cross-org begin")
	}
	if len(p.calls) != calls {
		t.Fatal(p.calls)
	}
	start, _ := s.Begin(ctx, m.org, row.ID, m.user, true)
	m.present = false
	if _, e := s.Redeem(ctx, "code", start.State, start.State); e == nil {
		t.Fatal("removed member")
	}
}
func TestEnterpriseFailedExchangeIsNotRetried(t *testing.T) {
	s, p, m, _, row := fixture(t)
	ctx := context.Background()
	start, _ := s.Begin(ctx, m.org, row.ID, m.user, true)
	p.fail = true
	before := len(p.calls)
	if _, e := s.Redeem(ctx, "code", start.State, start.State); e == nil {
		t.Fatal("failed exchange")
	}
	if _, e := s.Redeem(ctx, "code", start.State, start.State); e == nil {
		t.Fatal("replayed failure")
	}
	if len(p.calls) != before+1 {
		t.Fatal(p.calls)
	}
}
