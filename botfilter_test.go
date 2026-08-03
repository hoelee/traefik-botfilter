package traefik_botfilter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequiredHeaderCreatesTemporaryBan(t *testing.T) {
	cfg := CreateConfig()
	cfg.RequireUserAgent = true
	cfg.RequireAccept = true
	cfg.RequireHost = true
	nextCalls := 0
	handler, err := New(context.Background(), http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		nextCalls++
		rw.WriteHeader(http.StatusOK)
	}), cfg, "test")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	first := httptest.NewRequest(http.MethodGet, "http://wiki.example/content/article", nil)
	first.RemoteAddr = "203.0.113.11:54321"
	first.Header.Set("Accept", "text/html")
	first.Header.Del("User-Agent")
	firstResponse := httptest.NewRecorder()
	handler.ServeHTTP(firstResponse, first)
	if firstResponse.Code != http.StatusForbidden {
		t.Fatalf("first response status = %d, want %d", firstResponse.Code, http.StatusForbidden)
	}
	if firstResponse.Header().Get("Retry-After") == "" {
		t.Fatal("first response did not include Retry-After")
	}

	second := httptest.NewRequest(http.MethodGet, "http://wiki.example/", nil)
	second.RemoteAddr = "203.0.113.11:54321"
	second.Header.Set("Accept", "text/html")
	second.Header.Set("User-Agent", "Mozilla/5.0 Chrome/120.0 AppleWebKit/537.36 Safari/537.36")
	secondResponse := httptest.NewRecorder()
	handler.ServeHTTP(secondResponse, second)
	if secondResponse.Code != http.StatusForbidden {
		t.Fatalf("second response status = %d, want cached ban", secondResponse.Code)
	}
	if nextCalls != 0 {
		t.Fatalf("next handler calls = %d, want 0", nextCalls)
	}
}

func TestWhitelistBypassesFilter(t *testing.T) {
	cfg := CreateConfig()
	cfg.RequireUserAgent = true
	cfg.WhitelistCIDRs = []string{"192.168.0.0/16"}
	handler, err := New(context.Background(), http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusNoContent)
	}), cfg, "test")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "http://wiki.example/", nil)
	request.RemoteAddr = "192.168.30.25:1234"
	request.Header.Del("User-Agent")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("response status = %d, want whitelist to reach next handler", response.Code)
	}
}

func TestEncodedScanPathBansBeforeUpstream(t *testing.T) {
	cfg := CreateConfig()
	cfg.RequireUserAgent = false
	cfg.RequireAccept = false
	nextCalls := 0
	handler, err := New(context.Background(), http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		nextCalls++
		rw.WriteHeader(http.StatusOK)
	}), cfg, "test")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "http://wiki.example/foo/..%2F.env", nil)
	request.RemoteAddr = "198.51.100.22:1234"
	request.Header.Set("User-Agent", "Mozilla/5.0 Chrome/120.0 AppleWebKit/537.36 Safari/537.36")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("response status = %d, want %d", response.Code, http.StatusForbidden)
	}
	if nextCalls != 0 {
		t.Fatalf("next handler calls = %d, want 0", nextCalls)
	}
}

func Test404ScoreBansOnSubsequentRequest(t *testing.T) {
	cfg := CreateConfig()
	cfg.RequireUserAgent = false
	cfg.RequireAccept = false
	cfg.RandomArticlePatterns = nil
	cfg.EmptyUserAgentScore = 0
	cfg.MissingAcceptScore = 0
	cfg.ScoreThreshold = 40
	nextCalls := 0
	handler, err := New(context.Background(), http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		nextCalls++
		rw.WriteHeader(http.StatusNotFound)
	}), cfg, "test")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	first := httptest.NewRequest(http.MethodGet, "http://wiki.example/not-found", nil)
	first.RemoteAddr = "198.51.100.23:1234"
	first.Header.Set("User-Agent", "Mozilla/5.0 Chrome/120.0 AppleWebKit/537.36 Safari/537.36")
	first.Header.Set("Accept", "text/html")
	firstResponse := httptest.NewRecorder()
	handler.ServeHTTP(firstResponse, first)
	if firstResponse.Code != http.StatusNotFound {
		t.Fatalf("first response status = %d, want 404", firstResponse.Code)
	}

	second := httptest.NewRequest(http.MethodGet, "http://wiki.example/another-miss", nil)
	second.RemoteAddr = "198.51.100.23:1234"
	second.Header.Set("User-Agent", "Mozilla/5.0 Chrome/120.0 AppleWebKit/537.36 Safari/537.36")
	second.Header.Set("Accept", "text/html")
	secondResponse := httptest.NewRecorder()
	handler.ServeHTTP(secondResponse, second)
	if secondResponse.Code != http.StatusForbidden {
		t.Fatalf("second response status = %d, want cached ban after two 404s", secondResponse.Code)
	}
	if nextCalls != 1 {
		t.Fatalf("next handler calls = %d, want 1", nextCalls)
	}
}

func TestTrustedProxyHeaderSeparatesClients(t *testing.T) {
	cfg := CreateConfig()
	cfg.RequireUserAgent = true
	cfg.ClientIPHeader = "X-Forwarded-For"
	cfg.TrustedProxyCIDRs = []string{"127.0.0.0/8"}
	nextCalls := 0
	handler, err := New(context.Background(), http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		nextCalls++
		rw.WriteHeader(http.StatusNoContent)
	}), cfg, "test")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	bad := httptest.NewRequest(http.MethodGet, "http://wiki.example/", nil)
	bad.RemoteAddr = "127.0.0.1:1234"
	bad.Header.Set("X-Forwarded-For", "198.51.100.61")
	bad.Header.Del("User-Agent")
	badResponse := httptest.NewRecorder()
	handler.ServeHTTP(badResponse, bad)
	if badResponse.Code != http.StatusForbidden {
		t.Fatalf("bad client status = %d, want 403", badResponse.Code)
	}

	good := httptest.NewRequest(http.MethodGet, "http://wiki.example/", nil)
	good.RemoteAddr = "127.0.0.1:1234"
	good.Header.Set("X-Forwarded-For", "198.51.100.62")
	good.Header.Set("User-Agent", "Mozilla/5.0 Chrome/120.0 AppleWebKit/537.36 Safari/537.36")
	goodResponse := httptest.NewRecorder()
	handler.ServeHTTP(goodResponse, good)
	if goodResponse.Code != http.StatusNoContent {
		t.Fatalf("good client status = %d, want 204", goodResponse.Code)
	}
	if nextCalls != 1 {
		t.Fatalf("next handler calls = %d, want 1", nextCalls)
	}
}

func TestCompileConfigRejectsInvalidCIDR(t *testing.T) {
	cfg := CreateConfig()
	cfg.WhitelistCIDRs = []string{"not-a-cidr"}
	if _, err := compileConfig(cfg); err == nil {
		t.Fatal("compileConfig() error = nil, want invalid CIDR error")
	}
}
