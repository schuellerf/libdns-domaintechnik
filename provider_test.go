package domaintechnik

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/libdns/libdns"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Provider) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv, &Provider{
		APIToken:    "dt1_test_token",
		APIEndpoint: srv.URL,
	}
}

func TestAppendRecordsPresentsTXT(t *testing.T) {
	var gotName, gotValue, gotAuth, gotMethod, gotPath string
	srv, p := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		gotName = vals.Get("name")
		gotValue = vals.Get("value")
		w.WriteHeader(http.StatusOK)
	})
	_ = srv

	recs, err := p.AppendRecords(context.Background(), "schueller.pro.", []libdns.Record{
		libdns.TXT{Name: "_acme-challenge", Text: "THE_43_CHARACTER_VALUE"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q", gotMethod)
	}
	if gotPath != "/acme.present" {
		t.Errorf("path = %q", gotPath)
	}
	if gotAuth != "Bearer dt1_test_token" {
		t.Errorf("auth = %q", gotAuth)
	}
	if gotName != "_acme-challenge.schueller.pro" {
		t.Errorf("name = %q", gotName)
	}
	if gotValue != "THE_43_CHARACTER_VALUE" {
		t.Errorf("value = %q", gotValue)
	}
	if len(recs) != 1 {
		t.Fatalf("returned %d records", len(recs))
	}
}

func TestDeleteRecordsCleansUpTXT(t *testing.T) {
	var gotPath, gotName, gotValue string
	_, p := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		gotName = vals.Get("name")
		gotValue = vals.Get("value")
		w.WriteHeader(http.StatusOK)
	})

	_, err := p.DeleteRecords(context.Background(), "schueller.pro.", []libdns.Record{
		libdns.TXT{Name: "_acme-challenge", Text: "same-value"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/acme.cleanup" {
		t.Errorf("path = %q", gotPath)
	}
	if gotName != "_acme-challenge.schueller.pro" {
		t.Errorf("name = %q", gotName)
	}
	if gotValue != "same-value" {
		t.Errorf("value = %q", gotValue)
	}
}

func TestRecordNameAlreadyAbsolute(t *testing.T) {
	var gotName string
	_, p := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		gotName = vals.Get("name")
		w.WriteHeader(http.StatusOK)
	})

	_, err := p.AppendRecords(context.Background(), "schueller.pro.", []libdns.Record{
		libdns.TXT{Name: "_acme-challenge.schueller.pro.", Text: "v"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotName != "_acme-challenge.schueller.pro" {
		t.Errorf("name = %q", gotName)
	}
}

func TestRejectsNonTXT(t *testing.T) {
	called := false
	_, p := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	_, err := p.AppendRecords(context.Background(), "example.com.", []libdns.Record{
		libdns.Address{Name: "@", IP: netip.MustParseAddr("192.0.2.1")},
	})
	if err == nil {
		t.Fatal("expected error for non-TXT record")
	}
	if called {
		t.Fatal("API should not be called for non-TXT records")
	}
}

func TestHTTPErrorIncludesBody(t *testing.T) {
	_, p := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "token rejected", http.StatusUnauthorized)
	})

	err := p.Ping(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if want := "HTTP 401"; !strings.Contains(err.Error(), want) {
		t.Errorf("error %q missing %q", err, want)
	}
	if !strings.Contains(err.Error(), "token rejected") {
		t.Errorf("error %q missing body", err)
	}
}

func TestMissingToken(t *testing.T) {
	p := &Provider{APIEndpoint: "http://127.0.0.1:9"}
	err := p.Ping(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPing(t *testing.T) {
	var gotPath, gotMethod, gotCT string
	_, p := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	})
	if err := p.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost || gotPath != "/ping" {
		t.Errorf("request %s %s", gotMethod, gotPath)
	}
	if gotCT != "" {
		t.Errorf("unexpected content-type %q", gotCT)
	}
}

func TestDefaultEndpoint(t *testing.T) {
	p := &Provider{}
	if p.endpoint() != defaultAPIEndpoint {
		t.Errorf("endpoint = %q", p.endpoint())
	}
	p.APIEndpoint = defaultAPIEndpoint + "/"
	if p.endpoint() != defaultAPIEndpoint {
		t.Errorf("trimmed endpoint = %q", p.endpoint())
	}
}

func TestContextCanceled(t *testing.T) {
	_, p := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := p.Ping(ctx)
	if err == nil {
		t.Fatal("expected canceled context to fail")
	}
}

