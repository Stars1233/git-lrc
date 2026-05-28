package setup

import (
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestBuildSigninURL_ValidLocalhostCallback(t *testing.T) {
	signinURL, err := BuildSigninURL("http://127.0.0.1:8080/callback")
	if err != nil {
		t.Fatalf("BuildSigninURL returned error: %v", err)
	}

	parsed, err := url.Parse(signinURL)
	if err != nil {
		t.Fatalf("failed to parse signin URL: %v", err)
	}

	if parsed.Scheme != "https" {
		t.Fatalf("expected https scheme, got %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		t.Fatalf("expected non-empty host")
	}

	q := parsed.Query()
	if got := q.Get("app"); got != "livereview" {
		t.Fatalf("expected app=livereview, got %q", got)
	}
	if got := q.Get("appRedirectURI"); got != "http://127.0.0.1:8080/callback" {
		t.Fatalf("unexpected appRedirectURI value %q", got)
	}
}

func TestBuildSigninURL_RejectsNonLocalCallback(t *testing.T) {
	_, err := BuildSigninURL("https://evil.example.com/callback")
	if err == nil {
		t.Fatal("expected error for non-local callback URL")
	}
}

func TestBuildSigninURL_AllowsCodespacesForwardedCallback(t *testing.T) {
	t.Setenv("CODESPACE_NAME", "lively-space-train")
	t.Setenv("GITHUB_CODESPACES_PORT_FORWARDING_DOMAIN", "app.github.dev")

	callbackURL := "https://lively-space-train-4321.app.github.dev/callback"
	signinURL, err := BuildSigninURL(callbackURL)
	if err != nil {
		t.Fatalf("BuildSigninURL returned error: %v", err)
	}

	parsed, err := url.Parse(signinURL)
	if err != nil {
		t.Fatalf("failed to parse signin URL: %v", err)
	}
	if got := parsed.Query().Get("appRedirectURI"); got != callbackURL {
		t.Fatalf("unexpected appRedirectURI value %q", got)
	}
}

func TestNewSetupHTTPClient_BlocksCrossHostRedirect(t *testing.T) {
	client := newSetupHTTPClient(5 * time.Second)
	if client.CheckRedirect == nil {
		t.Fatal("expected CheckRedirect to be configured")
	}

	req := &http.Request{URL: &url.URL{Scheme: "https", Host: "evil.example.com"}}
	via := []*http.Request{{URL: &url.URL{Scheme: "https", Host: "livereview.hexmos.com"}}}

	err := client.CheckRedirect(req, via)
	if err != http.ErrUseLastResponse {
		t.Fatalf("expected http.ErrUseLastResponse, got %v", err)
	}
}
