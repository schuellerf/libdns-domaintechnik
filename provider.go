// Package domaintechnik implements libdns interfaces for Domaintechnik's
// ACME DNS-01 API (acme.present / acme.cleanup).
//
// This provider can only create and delete ACME challenge TXT records. It
// cannot list or manage arbitrary DNS records.
package domaintechnik

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/libdns/libdns"
)

const defaultAPIEndpoint = "https://manage.domaintechnik.at/dns-api/v1"

const (
	actionPresent = "acme.present"
	actionCleanup = "acme.cleanup"
	actionPing    = "ping"
)

// Provider facilitates ACME DNS-01 challenge records with Domaintechnik.
type Provider struct {
	// APIToken is the Domaintechnik DNS API bearer token (dt1_…).
	APIToken string `json:"api_token,omitempty"`

	// APIEndpoint is the API base URL without a trailing slash.
	// Defaults to https://manage.domaintechnik.at/dns-api/v1.
	APIEndpoint string `json:"api_endpoint,omitempty"`

	httpClient *http.Client
}

func (p *Provider) endpoint() string {
	ep := strings.TrimRight(p.APIEndpoint, "/")
	if ep == "" {
		return defaultAPIEndpoint
	}
	return ep
}

func (p *Provider) client() *http.Client {
	if p.httpClient != nil {
		return p.httpClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func recordName(zone string, rec libdns.Record) string {
	return strings.TrimSuffix(libdns.AbsoluteName(rec.RR().Name, zone), ".")
}

func txtValue(rec libdns.Record) (string, error) {
	rr := rec.RR()
	if rr.Type != "" && !strings.EqualFold(rr.Type, "TXT") {
		return "", fmt.Errorf("domaintechnik provider only supports TXT records, got %s", rr.Type)
	}
	return rr.Data, nil
}

func (p *Provider) postForm(ctx context.Context, action string, form url.Values) error {
	if p.APIToken == "" {
		return fmt.Errorf("domaintechnik API token is not set")
	}

	endpoint := p.endpoint() + "/" + action
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.APIToken)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := p.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(respBody))
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("domaintechnik %s: HTTP %d: %s", action, resp.StatusCode, msg)
	}
	return nil
}

func (p *Provider) mutate(ctx context.Context, action string, zone string, records []libdns.Record) ([]libdns.Record, error) {
	var done []libdns.Record
	for _, rec := range records {
		value, err := txtValue(rec)
		if err != nil {
			return done, err
		}
		name := recordName(zone, rec)
		form := url.Values{}
		form.Set("name", name)
		form.Set("value", value)
		if err := p.postForm(ctx, action, form); err != nil {
			return done, err
		}
		rr := rec.RR()
		done = append(done, libdns.TXT{
			Name: rr.Name,
			TTL:  rr.TTL,
			Text: value,
		})
	}
	return done, nil
}

// Ping checks that the API token is accepted without changing DNS records.
func (p *Provider) Ping(ctx context.Context) error {
	return p.postForm(ctx, actionPing, nil)
}

// AppendRecords publishes ACME challenge TXT records via acme.present.
func (p *Provider) AppendRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	return p.mutate(ctx, actionPresent, zone, records)
}

// DeleteRecords removes ACME challenge TXT records via acme.cleanup.
func (p *Provider) DeleteRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	return p.mutate(ctx, actionCleanup, zone, records)
}

// Interface guards
var (
	_ libdns.RecordAppender = (*Provider)(nil)
	_ libdns.RecordDeleter  = (*Provider)(nil)
)
