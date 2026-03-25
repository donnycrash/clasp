package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewSupabaseProvider_LoadsExistingCreds(t *testing.T) {
	dir := t.TempDir()
	creds := supabaseCredentials{
		AccessToken:  "test-access",
		RefreshToken: "test-refresh",
		ExpiresAt:    time.Now().Add(time.Hour),
		Email:        "user@example.com",
		UserID:       "uid-123",
		FullName:     "Test User",
	}
	data, _ := json.Marshal(creds)
	SaveToken(filepath.Join(dir, supabaseCredFile), data)

	p := NewSupabaseProvider("https://test.supabase.co", "anon-key", dir)
	if !p.IsAuthenticated() {
		t.Fatal("expected authenticated after loading existing creds")
	}
}

func TestNewSupabaseProvider_NoCreds(t *testing.T) {
	dir := t.TempDir()
	p := NewSupabaseProvider("https://test.supabase.co", "anon-key", dir)
	if p.IsAuthenticated() {
		t.Fatal("expected not authenticated with no creds")
	}
}

func TestSupabaseProvider_Name(t *testing.T) {
	p := &SupabaseProvider{}
	if p.Name() != "supabase" {
		t.Errorf("Name() = %q, want %q", p.Name(), "supabase")
	}
}

func TestSupabaseProvider_Login_MissingURL(t *testing.T) {
	p := &SupabaseProvider{configDir: t.TempDir()}
	err := p.Login(context.Background())
	if err == nil {
		t.Fatal("expected error for missing project URL")
	}
	if got := err.Error(); got != "supabase: project_url is not configured (set auth.supabase.url in config)" {
		t.Errorf("unexpected error: %s", got)
	}
}

func TestSupabaseProvider_GetIdentity_NotAuthenticated(t *testing.T) {
	p := &SupabaseProvider{}
	_, err := p.GetIdentity()
	if err == nil {
		t.Fatal("expected error when not authenticated")
	}
}

func TestSupabaseProvider_GetIdentity_WithCreds(t *testing.T) {
	p := &SupabaseProvider{
		creds: &supabaseCredentials{
			FullName: "Jane Doe",
			Email:    "jane@example.com",
			UserID:   "uid-456",
		},
	}

	id, err := p.GetIdentity()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id.Username != "Jane Doe" {
		t.Errorf("Username = %q, want %q", id.Username, "Jane Doe")
	}
	if id.Email != "jane@example.com" {
		t.Errorf("Email = %q, want %q", id.Email, "jane@example.com")
	}
	if id.Provider != "supabase" {
		t.Errorf("Provider = %q, want %q", id.Provider, "supabase")
	}
}

func TestSupabaseProvider_GetIdentity_FallsBackToEmail(t *testing.T) {
	p := &SupabaseProvider{
		creds: &supabaseCredentials{
			Email:  "jane@example.com",
			UserID: "uid-456",
		},
	}

	id, err := p.GetIdentity()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id.Username != "jane@example.com" {
		t.Errorf("Username = %q, want %q (should fall back to email)", id.Username, "jane@example.com")
	}
}

func TestSupabaseProvider_GetAuthHeader_NotAuthenticated(t *testing.T) {
	p := &SupabaseProvider{}
	_, err := p.GetAuthHeader()
	if err == nil {
		t.Fatal("expected error when not authenticated")
	}
}

func TestSupabaseProvider_GetAuthHeader_ValidToken(t *testing.T) {
	p := &SupabaseProvider{
		creds: &supabaseCredentials{
			AccessToken:  "valid-token",
			RefreshToken: "refresh",
			ExpiresAt:    time.Now().Add(time.Hour),
		},
	}

	header, err := p.GetAuthHeader()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if header != "Bearer valid-token" {
		t.Errorf("header = %q, want %q", header, "Bearer valid-token")
	}
}

func TestSupabaseProvider_GetAuthHeader_AutoRefreshesExpiredToken(t *testing.T) {
	// Set up a mock Supabase server that returns a new token on refresh
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/v1/token" && r.URL.Query().Get("grant_type") == "refresh_token" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token":  "refreshed-token",
				"refresh_token": "new-refresh",
				"expires_in":    3600,
				"user": map[string]interface{}{
					"id":            "uid-123",
					"email":         "user@example.com",
					"user_metadata": map[string]interface{}{"full_name": "Test User"},
				},
			})
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	dir := t.TempDir()
	p := &SupabaseProvider{
		projectURL: server.URL,
		anonKey:    "test-anon-key",
		configDir:  dir,
		creds: &supabaseCredentials{
			AccessToken:  "expired-token",
			RefreshToken: "old-refresh",
			ExpiresAt:    time.Now().Add(-time.Hour), // already expired
			Email:        "user@example.com",
			UserID:       "uid-123",
		},
	}

	header, err := p.GetAuthHeader()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if header != "Bearer refreshed-token" {
		t.Errorf("header = %q, want %q", header, "Bearer refreshed-token")
	}
	if p.creds.RefreshToken != "new-refresh" {
		t.Errorf("refresh token not updated: got %q", p.creds.RefreshToken)
	}
}

func TestSupabaseProvider_GetAuthHeader_RefreshFailsGracefully(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		fmt.Fprint(w, `{"error":"invalid_grant"}`)
	}))
	defer server.Close()

	p := &SupabaseProvider{
		projectURL: server.URL,
		anonKey:    "test-anon-key",
		configDir:  t.TempDir(),
		creds: &supabaseCredentials{
			AccessToken:  "expired",
			RefreshToken: "revoked-refresh",
			ExpiresAt:    time.Now().Add(-time.Hour),
		},
	}

	_, err := p.GetAuthHeader()
	if err == nil {
		t.Fatal("expected error when refresh fails")
	}
}

func TestSupabaseProvider_IsAuthenticated_WithRefreshToken(t *testing.T) {
	p := &SupabaseProvider{
		creds: &supabaseCredentials{
			AccessToken:  "expired",
			RefreshToken: "valid-refresh",
			ExpiresAt:    time.Now().Add(-time.Hour), // expired but has refresh
		},
	}
	if !p.IsAuthenticated() {
		t.Fatal("should be authenticated as long as refresh token exists")
	}
}

func TestSupabaseProvider_IsAuthenticated_NoRefreshToken(t *testing.T) {
	p := &SupabaseProvider{
		creds: &supabaseCredentials{
			AccessToken: "something",
		},
	}
	if p.IsAuthenticated() {
		t.Fatal("should not be authenticated without refresh token")
	}
}

func TestSupabaseProvider_Logout(t *testing.T) {
	dir := t.TempDir()
	credPath := filepath.Join(dir, supabaseCredFile)
	SaveToken(credPath, []byte(`{"access_token":"x","refresh_token":"y"}`))

	p := &SupabaseProvider{
		configDir: dir,
		creds: &supabaseCredentials{
			AccessToken:  "x",
			RefreshToken: "y",
		},
	}

	if err := p.Logout(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.creds != nil {
		t.Error("creds should be nil after logout")
	}
	if _, err := os.Stat(credPath); !os.IsNotExist(err) {
		t.Error("credentials file should be deleted after logout")
	}
}

func TestSupabaseProvider_SaveLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	p := &SupabaseProvider{
		configDir: dir,
		creds: &supabaseCredentials{
			AccessToken:  "at-123",
			RefreshToken: "rt-456",
			ExpiresAt:    time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
			Email:        "test@example.com",
			UserID:       "uid-789",
			FullName:     "Round Trip",
		},
	}

	if err := p.saveCreds(); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	p2 := &SupabaseProvider{configDir: dir}
	if err := p2.loadCreds(); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if p2.creds.AccessToken != "at-123" {
		t.Errorf("AccessToken = %q, want %q", p2.creds.AccessToken, "at-123")
	}
	if p2.creds.RefreshToken != "rt-456" {
		t.Errorf("RefreshToken = %q, want %q", p2.creds.RefreshToken, "rt-456")
	}
	if p2.creds.Email != "test@example.com" {
		t.Errorf("Email = %q, want %q", p2.creds.Email, "test@example.com")
	}
	if p2.creds.FullName != "Round Trip" {
		t.Errorf("FullName = %q, want %q", p2.creds.FullName, "Round Trip")
	}
}

func TestSupabaseProvider_ExchangeCodeForTokens(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/v1/token" {
			w.WriteHeader(404)
			return
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("apikey") != "test-anon" {
			t.Errorf("apikey = %q, want test-anon", r.Header.Get("apikey"))
		}

		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["auth_code"] != "test-code" {
			t.Errorf("auth_code = %q, want test-code", body["auth_code"])
		}
		if body["code_verifier"] != "test-verifier" {
			t.Errorf("code_verifier = %q, want test-verifier", body["code_verifier"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "new-access",
			"refresh_token": "new-refresh",
			"expires_in":    3600,
			"user": map[string]interface{}{
				"id":            "uid-abc",
				"email":         "dev@company.com",
				"user_metadata": map[string]interface{}{"full_name": "Dev User"},
			},
		})
	}))
	defer server.Close()

	dir := t.TempDir()
	p := &SupabaseProvider{
		projectURL: server.URL,
		anonKey:    "test-anon",
		configDir:  dir,
	}

	err := p.exchangeCodeForTokens(context.Background(), "test-code", "test-verifier", "http://localhost/callback")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.creds.AccessToken != "new-access" {
		t.Errorf("AccessToken = %q, want new-access", p.creds.AccessToken)
	}
	if p.creds.Email != "dev@company.com" {
		t.Errorf("Email = %q, want dev@company.com", p.creds.Email)
	}
	if p.creds.FullName != "Dev User" {
		t.Errorf("FullName = %q, want Dev User", p.creds.FullName)
	}
}

func TestSupabaseProvider_RefreshAccessToken(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.URL.Query().Get("grant_type") != "refresh_token" {
			t.Errorf("grant_type = %q, want refresh_token", r.URL.Query().Get("grant_type"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  fmt.Sprintf("refreshed-%d", callCount),
			"refresh_token": "rotated-refresh",
			"expires_in":    7200,
			"user": map[string]interface{}{
				"id":            "uid-123",
				"email":         "user@co.com",
				"user_metadata": map[string]interface{}{"name": "Refreshed User"},
			},
		})
	}))
	defer server.Close()

	dir := t.TempDir()
	p := &SupabaseProvider{
		projectURL: server.URL,
		anonKey:    "key",
		configDir:  dir,
		creds: &supabaseCredentials{
			AccessToken:  "old",
			RefreshToken: "old-refresh",
			ExpiresAt:    time.Now().Add(-time.Hour),
		},
	}

	if err := p.refreshAccessToken(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.creds.AccessToken != "refreshed-1" {
		t.Errorf("AccessToken = %q, want refreshed-1", p.creds.AccessToken)
	}
	if p.creds.RefreshToken != "rotated-refresh" {
		t.Errorf("RefreshToken = %q, want rotated-refresh", p.creds.RefreshToken)
	}
	if p.creds.FullName != "Refreshed User" {
		t.Errorf("FullName = %q, want Refreshed User", p.creds.FullName)
	}
}

func TestGeneratePKCE(t *testing.T) {
	v1, c1, err := generatePKCE()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(v1) == 0 {
		t.Fatal("verifier is empty")
	}
	if len(c1) == 0 {
		t.Fatal("challenge is empty")
	}

	// Should be different each time
	v2, c2, _ := generatePKCE()
	if v1 == v2 {
		t.Error("two verifiers should not be identical")
	}
	if c1 == c2 {
		t.Error("two challenges should not be identical")
	}
}

func TestSupabaseRegistered(t *testing.T) {
	names := List()
	found := false
	for _, n := range names {
		if n == "supabase" {
			found = true
			break
		}
	}
	if !found {
		t.Error("supabase provider not registered")
	}
}
