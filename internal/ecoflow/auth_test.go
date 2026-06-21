package ecoflow

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/xkhronoz/ecoflow-ble-nutd/internal/config"
)

type countingRoundTripper struct {
	calls atomic.Int32
}

func (c *countingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	c.calls.Add(1)
	return jsonHTTPResponse(http.StatusTeapot, `{}`), nil
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestLoginClientResolveUserIDUsesExplicitUserID(t *testing.T) {
	t.Parallel()

	rt := &countingRoundTripper{}
	client := &LoginClient{
		HTTPClient: &http.Client{Transport: rt},
	}
	got, err := client.ResolveUserID(context.Background(), config.ProviderAuthConfig{
		UserID: "user-123",
	})
	if err != nil {
		t.Fatalf("ResolveUserID: %v", err)
	}
	if got != "user-123" {
		t.Fatalf("user_id = %q", got)
	}
	if rt.calls.Load() != 0 {
		t.Fatalf("unexpected HTTP calls: %d", rt.calls.Load())
	}
}

func TestLoginClientResolveUserIDViaCloud(t *testing.T) {
	t.Parallel()

	client := &LoginClient{
		HTTPClient: &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return jsonHTTPResponse(http.StatusOK, `{"code":"0","message":"OK","data":{"user":{"userId":"cloud-user"}}}`), nil
		})},
		BaseURLByRegion: map[string]string{"api": "https://api.ecoflow.test"},
	}
	got, err := client.ResolveUserID(context.Background(), config.ProviderAuthConfig{
		Email:    "user@example.com",
		Password: "secret",
		Region:   "auto",
	})
	if err != nil {
		t.Fatalf("ResolveUserID: %v", err)
	}
	if got != "cloud-user" {
		t.Fatalf("user_id = %q", got)
	}
}

func TestLoginClientResolveUserIDFailureStopsStartup(t *testing.T) {
	t.Parallel()

	client := &LoginClient{
		HTTPClient: &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return jsonHTTPResponse(http.StatusOK, `{"code":"1001","message":"bad credentials"}`), nil
		})},
		BaseURLByRegion: map[string]string{"api": "https://api.ecoflow.test"},
	}
	if _, err := client.ResolveUserID(context.Background(), config.ProviderAuthConfig{
		Email:    "user@example.com",
		Password: "bad",
		Region:   "api",
	}); err == nil || !strings.Contains(err.Error(), "bad credentials") {
		t.Fatalf("expected clear login error, got %v", err)
	}
}

func jsonHTTPResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}
