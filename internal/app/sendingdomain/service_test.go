package sendingdomain

import (
	"context"
	"github.com/pramaan-oss/cold-email/internal/app/owndomain"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/pramaan-oss/cold-email/internal/errx"
	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/pkg/domainproof"
	"github.com/pramaan-oss/cold-email/internal/repository"
)

type fakeDNS struct {
	txt   map[string][]string
	cname map[string]string
	ips   map[string][]string
	fail  bool
}

func (f *fakeDNS) LookupTXT(_ context.Context, name string) ([]string, error) {
	if f.fail {
		return nil, &net.DNSError{Err: "timeout", Name: name, IsTimeout: true}
	}
	if v, ok := f.txt[name]; ok {
		return v, nil
	}
	return nil, &net.DNSError{Err: "no such host", Name: name, IsNotFound: true}
}

func (f *fakeDNS) LookupCNAME(_ context.Context, host string) (string, error) {
	if f.fail {
		return "", &net.DNSError{Err: "timeout", Name: host, IsTimeout: true}
	}
	if v, ok := f.cname[host]; ok {
		return v + ".", nil
	}
	if _, ok := f.ips[host]; ok {
		return host + ".", nil
	}
	return "", &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
}

func (f *fakeDNS) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	if f.fail {
		return nil, &net.DNSError{Err: "timeout", Name: host, IsTimeout: true}
	}
	if c, ok := f.cname[host]; ok {
		host = c
	}
	var out []net.IPAddr
	for _, ip := range f.ips[host] {
		out = append(out, net.IPAddr{IP: net.ParseIP(ip)})
	}
	if len(out) == 0 {
		return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
	}
	return out, nil
}

type memRedirects struct {
	rows map[uuid.UUID]*models.DomainRedirect
}

func (m *memRedirects) Upsert(_ context.Context, r *models.DomainRedirect, _ *uuid.UUID) error {
	if r.ServedBy == "" {
		r.ServedBy = models.RedirectServedByInstance
	}
	for _, e := range m.rows {
		if r.ServedBy == models.RedirectServedByCloud && e.ServedBy == models.RedirectServedByCloud && e.Domain == r.Domain && e.OrganizationID != r.OrganizationID {
			return repository.ErrRedirectTaken
		}
	}
	for _, e := range m.rows {
		if e.OrganizationID == r.OrganizationID && e.Domain == r.Domain {
			if (e.LinkedInstanceID == nil) != (r.LinkedInstanceID == nil) || (e.LinkedInstanceID != nil && *e.LinkedInstanceID != *r.LinkedInstanceID) {
				return repository.ErrRedirectOwned
			}
			if e.ServedBy != r.ServedBy {
				e.RemoteHost, e.RemoteRecords, e.Reach, e.Verified, e.VerifiedAt, e.LastError = "", nil, nil, false, nil, ""
			}
			e.TargetURL, e.IncludeWWW, e.ServedBy = r.TargetURL, r.IncludeWWW, r.ServedBy
			*r = *e
			return nil
		}
	}
	cp := *r
	cp.CreatedAt = time.Now()
	m.rows[r.ID] = &cp
	return nil
}
func (m *memRedirects) UpsertLinked(ctx context.Context, r *models.DomainRedirect, by *uuid.UUID, limit int) (bool, error) {
	if existing, _ := m.GetLinked(ctx, *r.LinkedInstanceID, r.Domain); existing == nil {
		if n, _ := m.CountLinked(ctx, *r.LinkedInstanceID); n >= limit {
			return false, nil
		}
	}
	return true, m.Upsert(ctx, r, by)
}
func (m *memRedirects) find(match func(*models.DomainRedirect) bool) *models.DomainRedirect {
	for _, e := range m.rows {
		if match(e) {
			cp := *e
			return &cp
		}
	}
	return nil
}
func (m *memRedirects) Get(_ context.Context, org uuid.UUID, domain string) (*models.DomainRedirect, error) {
	return m.find(func(e *models.DomainRedirect) bool { return e.OrganizationID == org && e.Domain == domain }), nil
}
func (m *memRedirects) List(_ context.Context, org uuid.UUID) ([]models.DomainRedirect, error) {
	var out []models.DomainRedirect
	for _, e := range m.rows {
		if e.OrganizationID == org && e.LinkedInstanceID == nil {
			out = append(out, *e)
		}
	}
	return out, nil
}
func (m *memRedirects) Delete(_ context.Context, org uuid.UUID, domain string) (bool, error) {
	for id, e := range m.rows {
		if e.OrganizationID == org && e.Domain == domain && e.LinkedInstanceID == nil {
			delete(m.rows, id)
			return true, nil
		}
	}
	return false, nil
}
func (m *memRedirects) SetCheck(_ context.Context, id uuid.UUID, verified bool, last string) error {
	for _, e := range m.rows {
		if e.ID != id && e.Domain == m.rows[id].Domain && e.Verified && verified {
			return repository.ErrRedirectTaken
		}
	}
	m.rows[id].Verified, m.rows[id].LastError = verified, last
	if verified && m.rows[id].VerifiedAt == nil {
		now := time.Now()
		m.rows[id].VerifiedAt = &now
	} else if !verified {
		m.rows[id].VerifiedAt = nil
	}
	return nil
}
func (m *memRedirects) SetReach(_ context.Context, id uuid.UUID, reach *models.RedirectReach) error {
	m.rows[id].Reach = reach
	return nil
}
func (m *memRedirects) SetRemote(_ context.Context, id uuid.UUID, host string, records []models.DNSRecord) error {
	m.rows[id].RemoteHost, m.rows[id].RemoteRecords = host, records
	return nil
}
func (m *memRedirects) UnverifyCloudServed(_ context.Context, _ uuid.UUID, last string) error {
	for _, e := range m.rows {
		if e.ServedBy == models.RedirectServedByCloud {
			e.Verified, e.LastError, e.Reach = false, last, nil
		}
	}
	return nil
}

func (m *memRedirects) CloudServedElsewhere(_ context.Context, org uuid.UUID, domain string) (bool, error) {
	return m.find(func(e *models.DomainRedirect) bool {
		return e.OrganizationID != org && e.Domain == domain && e.ServedBy == models.RedirectServedByCloud
	}) != nil, nil
}
func (m *memRedirects) Due(context.Context, int) ([]models.DomainRedirect, error) { return nil, nil }
func (m *memRedirects) Lookup(ctx context.Context, host string) (string, bool, error) {
	got, ok, err := m.LookupServed(ctx, host)
	if !ok {
		return "", ok, err
	}
	return got.TargetURL, true, nil
}
func (m *memRedirects) LookupServed(_ context.Context, host string) (*models.DomainRedirectLookup, bool, error) {
	e := m.find(func(e *models.DomainRedirect) bool {
		return e.Verified && e.ServedBy == models.RedirectServedByInstance && (e.Domain == host || (e.IncludeWWW && "www."+e.Domain == host))
	})
	if e == nil {
		return nil, false, nil
	}
	return &models.DomainRedirectLookup{TargetURL: e.TargetURL, OrganizationID: e.OrganizationID, LinkedInstanceID: e.LinkedInstanceID}, true, nil
}
func (m *memRedirects) ListLinked(_ context.Context, inst uuid.UUID) ([]models.DomainRedirect, error) {
	var out []models.DomainRedirect
	for _, e := range m.rows {
		if e.LinkedInstanceID != nil && *e.LinkedInstanceID == inst {
			out = append(out, *e)
		}
	}
	return out, nil
}
func (m *memRedirects) GetLinked(_ context.Context, inst uuid.UUID, domain string) (*models.DomainRedirect, error) {
	return m.find(func(e *models.DomainRedirect) bool {
		return e.LinkedInstanceID != nil && *e.LinkedInstanceID == inst && e.Domain == domain
	}), nil
}
func (m *memRedirects) DeleteLinked(_ context.Context, inst uuid.UUID, domain string) (bool, error) {
	for id, e := range m.rows {
		if e.LinkedInstanceID != nil && *e.LinkedInstanceID == inst && e.Domain == domain {
			delete(m.rows, id)
			return true, nil
		}
	}
	return false, nil
}
func (m *memRedirects) CountLinked(ctx context.Context, inst uuid.UUID) (int, error) {
	l, _ := m.ListLinked(ctx, inst)
	return len(l), nil
}

// fakeReach answers every probe with one verdict and counts them.
type fakeReach struct {
	verdict models.RedirectReach
	probes  int
}

func (f *fakeReach) Probe(context.Context, string, string) models.RedirectReach {
	f.probes++
	return f.verdict
}

// fakeMailboxes holds mailboxes on the counted domains; owned adds the workspace's other own domains.
type fakeMailboxes struct {
	counts map[string]int
	owned  []string
}

func (f *fakeMailboxes) OwnedDomains(context.Context, uuid.UUID) ([]string, *errx.Error) {
	out := append([]string{}, f.owned...)
	for d := range f.counts {
		out = append(out, d)
	}
	return out, nil
}

// testOwned are the websites the test workspace redirects to.
var testOwned = []string{"acme.com", "acme.co.uk", "evil.example", "x.com"}

func (f *fakeMailboxes) DomainsOverview(context.Context, uuid.UUID) ([]models.SendingDomain, *errx.Error) {
	return nil, nil
}
func (f *fakeMailboxes) CountDomainMailboxes(_ context.Context, _ uuid.UUID, domain string) (int, *errx.Error) {
	return f.counts[domain], nil
}
func (f *fakeMailboxes) SetDomainTracking(context.Context, uuid.UUID, string, string, bool, *time.Time) (int, *errx.Error) {
	return 3, nil
}

func newTest(dns *fakeDNS) (*Service, *memRedirects) {
	repo := &memRedirects{rows: map[uuid.UUID]*models.DomainRedirect{}}
	s := NewService(repo, &fakeMailboxes{counts: map[string]int{"acme.io": 2}, owned: testOwned}, dns, domainproof.New("test-secret"))
	s.target = func() string { return "track.coldemail.test" }
	s.reach = &fakeReach{verdict: models.RedirectReach{Status: models.RedirectReachOK}}
	return s, repo
}

func baseDNS() *fakeDNS {
	return &fakeDNS{txt: map[string][]string{}, cname: map[string]string{}, ips: map[string][]string{"track.coldemail.test": {"203.0.113.10"}}}
}

func TestCleanTarget(t *testing.T) {
	if got, xerr := cleanTarget("acme.com", "acme.io"); xerr != nil || got != "https://acme.com" {
		t.Fatalf("got %q, %v", got, xerr)
	}
	for _, bad := range []string{"javascript:alert(1)", "https://acme.io/x", "https://www.acme.io", "https://user@evil.com", ""} {
		if _, xerr := cleanTarget(bad, "acme.io"); xerr == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
}

func TestRedirectVerifiesOnlyWithOwnershipAndAddress(t *testing.T) {
	dns := baseDNS()
	s, repo := newTest(dns)
	org := uuid.New()
	r, xerr := s.SetRedirect(context.Background(), org, uuid.New(), "ACME.io", RedirectInput{TargetURL: "https://acme.com"})
	if xerr != nil {
		t.Fatal(xerr)
	}
	if r.Verified {
		t.Fatal("verified with no DNS at all")
	}
	var txt, root models.DNSRecord
	for _, rec := range r.Records {
		switch rec.Purpose {
		case "ownership":
			txt = rec
		case "root":
			root = rec
		}
	}
	if txt.Name != "_coldemail.acme.io" || !strings.HasPrefix(txt.Value, "coldemail-verify=") || root.Type != "A" || root.Value != "203.0.113.10" {
		t.Fatalf("records = %+v", r.Records)
	}

	// Pointing the root here without the TXT record proves nothing about who owns it.
	dns.ips["acme.io"] = []string{"203.0.113.10"}
	if r, _ = s.VerifyRedirect(context.Background(), org, "acme.io"); r.Verified {
		t.Fatal("verified without the ownership record")
	}
	dns.txt["_coldemail.acme.io"] = []string{txt.Value}
	if r, _ = s.VerifyRedirect(context.Background(), org, "acme.io"); !r.Verified {
		t.Fatalf("not verified with both records: %s", r.LastError)
	}

	// A resolver outage does not take a verified redirect down; a removed record does.
	dns.fail = true
	if r, _ = s.VerifyRedirect(context.Background(), org, "acme.io"); !r.Verified {
		t.Fatal("a lookup failure unverified the redirect")
	}
	dns.fail = false
	delete(dns.txt, "_coldemail.acme.io")
	if r, _ = s.VerifyRedirect(context.Background(), org, "acme.io"); r.Verified {
		t.Fatal("still verified after the ownership record was removed")
	}
	_ = repo
}

func TestRedirectRefusesForeignAndSharedDomains(t *testing.T) {
	s, _ := newTest(baseDNS())
	org := uuid.New()
	if _, xerr := s.SetRedirect(context.Background(), org, uuid.New(), "other.io", RedirectInput{TargetURL: "https://x.com"}); xerr == nil || xerr.Identifier != ErrIDNotYours {
		t.Fatalf("a domain with no mailbox = %v", xerr)
	}
	if _, xerr := s.SetRedirect(context.Background(), org, uuid.New(), "gmail.com", RedirectInput{TargetURL: "https://x.com"}); xerr == nil || xerr.Identifier != ErrIDConsumerDomain {
		t.Fatalf("a shared provider domain = %v", xerr)
	}
}

func TestTrackingSuggestionFindsAnExistingRecord(t *testing.T) {
	dns := baseDNS()
	s, _ := newTest(dns)
	if sug := s.TrackingSuggestion(context.Background(), "acme.io", nil); sug.Status != "suggested" || sug.Host != "link.acme.io" {
		t.Fatalf("suggestion = %+v", sug)
	}
	dns.cname["track.acme.io"] = "track.coldemail.test"
	if sug := s.TrackingSuggestion(context.Background(), "acme.io", nil); sug.Status != "found" || sug.Host != "track.acme.io" {
		t.Fatalf("suggestion = %+v", sug)
	}
	inUse := []models.TrackingDomainUse{{Host: "t.acme.io", Verified: true, Mailboxes: 2}}
	if sug := s.TrackingSuggestion(context.Background(), "acme.io", inUse); sug.Status != "active" || sug.Host != "t.acme.io" {
		t.Fatalf("suggestion = %+v", sug)
	}
}

func TestRedirectProofIsNotTheOneAnotherWorkspacePublished(t *testing.T) {
	dns := baseDNS()
	s, _ := newTest(dns)
	victim, attacker := uuid.New(), uuid.New()
	dns.ips["acme.io"] = []string{"203.0.113.10"}
	// The victim's record is public; publishing it proves nothing for another workspace.
	dns.txt["_coldemail.acme.io"] = []string{s.proof.Value(victim, "acme.io")}
	r, xerr := s.SetRedirect(context.Background(), attacker, uuid.New(), "acme.io", RedirectInput{TargetURL: "https://evil.example"})
	if xerr != nil {
		t.Fatal(xerr)
	}
	if r.Verified {
		t.Fatal("another workspace's proof verified the redirect")
	}
}

type fakeVendors struct {
	links    map[string]models.VendorDomainLink
	forwards map[string]string
	records  []string
}

func (f *fakeVendors) DomainLinks(context.Context, uuid.UUID) (map[string]models.VendorDomainLink, *errx.Error) {
	return f.links, nil
}
func (f *fakeVendors) SetForwarding(_ context.Context, _ uuid.UUID, domain, target string) (*models.VendorDomainLink, *errx.Error) {
	f.forwards[domain] = target
	l := f.links[domain]
	l.Forwarding = target
	return &l, nil
}
func (f *fakeVendors) UpsertDNS(_ context.Context, _ uuid.UUID, domain, typ, name, value string) *errx.Error {
	f.records = append(f.records, domain+" "+typ+" "+name+" "+value)
	return nil
}

func TestRedirectGoesOnlyToTheWorkspacesOwnDomains(t *testing.T) {
	dns := baseDNS()
	s, _ := newTest(dns)
	mb := &fakeMailboxes{counts: map[string]int{"acme.io": 2}, owned: []string{"acme.com"}}
	s.mailboxes = mb
	ctx := context.Background()
	org := uuid.New()

	for _, bad := range []string{"https://evil-acme.com", "https://acme.com.evil.io/", "https://gmail.com"} {
		if _, xerr := s.SetRedirect(ctx, org, uuid.New(), "acme.io", RedirectInput{TargetURL: bad}); xerr == nil || xerr.Identifier != owndomain.ErrIDNotOwn {
			t.Errorf("%q = %v", bad, xerr)
		}
	}
	r, xerr := s.SetRedirect(ctx, org, uuid.New(), "acme.io", RedirectInput{TargetURL: "https://shop.acme.com/start"})
	if xerr != nil {
		t.Fatal(xerr)
	}
	if r.TargetActive == nil || !*r.TargetActive {
		t.Fatalf("an own destination is not active: %+v", r.TargetActive)
	}
	dns.ips["acme.io"] = []string{"203.0.113.10"}
	dns.txt["_coldemail.acme.io"] = []string{s.proof.Value(org, "acme.io")}
	if r, _ = s.VerifyRedirect(ctx, org, "acme.io"); !r.Verified {
		t.Fatalf("not verified: %s", r.LastError)
	}
	if target, ok, err := s.Lookup(ctx, "acme.io"); err != nil || !ok || target != "https://shop.acme.com/start" {
		t.Fatalf("Lookup = %q, %v, %v", target, ok, err)
	}

	// The workspace stops sending from acme.com: the saved redirect is no longer served or shown as in use.
	mb.owned = nil
	if _, ok, err := s.Lookup(ctx, "acme.io"); err != nil || ok {
		t.Fatalf("a destination off the workspace's domains is still served: %v, %v", ok, err)
	}
	if r, _ = s.VerifyRedirect(ctx, org, "acme.io"); r.TargetActive == nil || *r.TargetActive || r.TargetInactiveReason == "" {
		t.Fatalf("an inactive destination is not flagged: %+v", r)
	}
}
