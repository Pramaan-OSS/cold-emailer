package sendingdomain

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pramaan-oss/cold-email/internal/models"
)

const reachTarget = "https://www.acme.com"

// closedAddr is a local port nothing listens on, so dialing it is refused.
func closedAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

// reachFixture serves the domain's http and https sides from local servers; nil means nothing listens.
func reachFixture(t *testing.T, plain, secure http.HandlerFunc, trusted bool) *httpReach {
	t.Helper()
	plainAddr, secureAddr := closedAddr(t), closedAddr(t)
	tr := &http.Transport{TLSClientConfig: &tls.Config{}}
	if plain != nil {
		srv := httptest.NewServer(plain)
		t.Cleanup(srv.Close)
		plainAddr = srv.Listener.Addr().String()
	}
	if secure != nil {
		srv := httptest.NewTLSServer(secure)
		t.Cleanup(srv.Close)
		secureAddr = srv.Listener.Addr().String()
		if trusted {
			pool := x509.NewCertPool()
			pool.AddCert(srv.Certificate())
			tr.TLSClientConfig.RootCAs = pool
		}
	}
	tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		_, port, _ := net.SplitHostPort(addr)
		to := plainAddr
		if port == "443" {
			to = secureAddr
		}
		return (&net.Dialer{}).DialContext(ctx, network, to)
	}
	return &httpReach{transport: tr}
}

func redirectTo(loc string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, loc, http.StatusFound) }
}

func TestReachDoesNotBlameARedirectItCouldNotOpen(t *testing.T) {
	// A server that cannot open its own public address times out rather than being refused.
	p := &httpReach{transport: &http.Transport{DialContext: func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("i/o timeout")
	}}}
	got := p.Probe(context.Background(), "example.com", reachTarget)
	if got.Status != models.RedirectReachUnreachable || got.Detail == "" {
		t.Fatalf("an unanswered check = %+v", got)
	}
}

func TestSameTargetIgnoresOnlyWhatAVisitorWouldNot(t *testing.T) {
	for _, pair := range [][2]string{{"https://acme.com", "https://ACME.com/"}, {"https://acme.com/a/", "https://acme.com/a"}} {
		if !sameTarget(pair[0], pair[1]) {
			t.Errorf("%q and %q differ", pair[0], pair[1])
		}
	}
	for _, pair := range [][2]string{{"https://acme.com", "http://acme.com"}, {"https://acme.com/a", "https://acme.com/b"}, {"https://acme.com/?x=1", "https://acme.com/"}} {
		if sameTarget(pair[0], pair[1]) {
			t.Errorf("%q and %q match", pair[0], pair[1])
		}
	}
}

func selfSigned(t *testing.T, cn string) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: cn},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}
