package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

const (
	supabaseCredFile   = "supabase-credentials.json"
	callbackPort       = "18923"
	tokenRefreshBuffer = 60 * time.Second // refresh 60s before expiry
)

// supabaseCredentials is the on-disk format for stored Supabase tokens.
type supabaseCredentials struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	Email        string    `json:"email"`
	UserID       string    `json:"user_id"`
	FullName     string    `json:"full_name"`
}

// SupabaseProvider implements OAuth via Supabase (Google sign-in) with
// automatic token refresh using stored refresh tokens.
type SupabaseProvider struct {
	projectURL string // e.g. https://xxxx.supabase.co
	anonKey    string // Supabase anon/public key for API calls
	configDir  string
	creds      *supabaseCredentials
}

// NewSupabaseProvider creates a Supabase auth provider.
func NewSupabaseProvider(projectURL, anonKey, configDir string) *SupabaseProvider {
	projectURL = strings.TrimRight(projectURL, "/")
	p := &SupabaseProvider{
		projectURL: projectURL,
		anonKey:    anonKey,
		configDir:  configDir,
	}
	_ = p.loadCreds()
	return p
}

func init() {
	Register("supabase", func() Provider {
		return &SupabaseProvider{}
	})
}

func (p *SupabaseProvider) Name() string { return "supabase" }

// Login opens the browser to Supabase's Google OAuth page, captures the
// token via a localhost callback, and stores both access and refresh tokens.
func (p *SupabaseProvider) Login(ctx context.Context) error {
	if p.projectURL == "" {
		return fmt.Errorf("supabase: project_url is not configured (set auth.supabase.url in config)")
	}

	// Generate PKCE code verifier and challenge
	verifier, challenge, err := generatePKCE()
	if err != nil {
		return fmt.Errorf("supabase: generating PKCE: %w", err)
	}

	// Start local HTTP server to receive the callback
	listener, err := net.Listen("tcp", "127.0.0.1:"+callbackPort)
	if err != nil {
		return fmt.Errorf("supabase: starting callback server: %w", err)
	}
	defer listener.Close()

	redirectURI := fmt.Sprintf("http://127.0.0.1:%s/callback", callbackPort)

	// Build Supabase OAuth URL
	authURL := fmt.Sprintf("%s/auth/v1/authorize?provider=google&redirect_to=%s&code_challenge=%s&code_challenge_method=S256",
		p.projectURL,
		url.QueryEscape(redirectURI),
		url.QueryEscape(challenge),
	)

	fmt.Printf("\nOpen this URL in your browser to sign in:\n\n  %s\n\nWaiting for authentication...\n", authURL)

	// Wait for the callback with the auth code
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errMsg := r.URL.Query().Get("error_description")
			if errMsg == "" {
				errMsg = r.URL.Query().Get("error")
			}
			if errMsg == "" {
				errMsg = "no code in callback"
			}
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, "<html><body><h2>Authentication failed</h2><p>%s</p><p>You can close this tab.</p></body></html>", errMsg)
			errCh <- fmt.Errorf("supabase: %s", errMsg)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><body><h2>Authenticated!</h2><p>You can close this tab and return to the terminal.</p></body></html>")
		codeCh <- code
	})

	server := &http.Server{Handler: mux}
	go server.Serve(listener)
	defer server.Close()

	// Wait for code or timeout
	select {
	case code := <-codeCh:
		return p.exchangeCodeForTokens(ctx, code, verifier, redirectURI)
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Minute):
		return fmt.Errorf("supabase: authentication timed out (5 minutes)")
	}
}

// exchangeCodeForTokens exchanges the auth code for access + refresh tokens.
func (p *SupabaseProvider) exchangeCodeForTokens(ctx context.Context, code, verifier, redirectURI string) error {
	tokenURL := fmt.Sprintf("%s/auth/v1/token?grant_type=pkce", p.projectURL)

	jsonBody, err := json.Marshal(map[string]string{
		"auth_code":     code,
		"code_verifier": verifier,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(string(jsonBody)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", p.extractAnonKey())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("exchanging code: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, string(respBody))
	}

	return p.parseAndStoreTokenResponse(respBody)
}

// GetIdentity returns the identity of the currently authenticated user.
func (p *SupabaseProvider) GetIdentity() (Identity, error) {
	if p.creds == nil {
		return Identity{}, fmt.Errorf("supabase: not authenticated")
	}
	name := p.creds.FullName
	if name == "" {
		name = p.creds.Email
	}
	return Identity{
		Username:   name,
		Email:      p.creds.Email,
		ProviderID: p.creds.UserID,
		Provider:   "supabase",
	}, nil
}

// GetAuthHeader returns the Bearer token, refreshing it first if expired.
func (p *SupabaseProvider) GetAuthHeader() (string, error) {
	if p.creds == nil {
		return "", fmt.Errorf("supabase: not authenticated")
	}

	// Auto-refresh if token is expired or about to expire
	if time.Now().Add(tokenRefreshBuffer).After(p.creds.ExpiresAt) {
		slog.Info("supabase: access token expired, refreshing...")
		if err := p.refreshAccessToken(); err != nil {
			return "", fmt.Errorf("supabase: token refresh failed: %w (run `clasp auth login --provider supabase` to re-authenticate)", err)
		}
		slog.Info("supabase: token refreshed successfully")
	}

	return "Bearer " + p.creds.AccessToken, nil
}

// Logout removes stored credentials.
func (p *SupabaseProvider) Logout() error {
	p.creds = nil
	return DeleteToken(p.credPath())
}

// IsAuthenticated reports whether valid credentials exist.
func (p *SupabaseProvider) IsAuthenticated() bool {
	if p.creds == nil || p.creds.RefreshToken == "" {
		return false
	}
	// Even if access token is expired, we're "authenticated" as long as
	// we have a refresh token — GetAuthHeader will refresh it.
	return true
}

// refreshAccessToken uses the stored refresh token to get a new access token.
func (p *SupabaseProvider) refreshAccessToken() error {
	tokenURL := fmt.Sprintf("%s/auth/v1/token?grant_type=refresh_token", p.projectURL)

	body := url.Values{
		"refresh_token": {p.creds.RefreshToken},
	}

	req, err := http.NewRequest(http.MethodPost, tokenURL, strings.NewReader(body.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("apikey", p.extractAnonKey())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("refresh request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		// Refresh token is invalid/revoked — user needs to re-auth
		return fmt.Errorf("refresh failed (%d): %s", resp.StatusCode, string(respBody))
	}

	return p.parseAndStoreTokenResponse(respBody)
}

// parseAndStoreTokenResponse parses Supabase's token response and stores creds.
func (p *SupabaseProvider) parseAndStoreTokenResponse(body []byte) error {
	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		User         struct {
			ID               string                 `json:"id"`
			Email            string                 `json:"email"`
			UserMetadata     map[string]interface{} `json:"user_metadata"`
			AppMetadata      map[string]interface{} `json:"app_metadata"`
		} `json:"user"`
	}

	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return fmt.Errorf("parsing token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return fmt.Errorf("empty access token in response")
	}

	fullName := ""
	if tokenResp.User.UserMetadata != nil {
		if name, ok := tokenResp.User.UserMetadata["full_name"].(string); ok {
			fullName = name
		} else if name, ok := tokenResp.User.UserMetadata["name"].(string); ok {
			fullName = name
		}
	}

	p.creds = &supabaseCredentials{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		Email:        tokenResp.User.Email,
		UserID:       tokenResp.User.ID,
		FullName:     fullName,
	}

	if err := p.saveCreds(); err != nil {
		return fmt.Errorf("saving credentials: %w", err)
	}

	fmt.Printf("Authenticated as %s (%s)\n", p.creds.FullName, p.creds.Email)
	return nil
}

// extractAnonKey returns the Supabase anon key from config or an empty string.
// The anon key is needed for Supabase REST API calls. If not configured,
// the calls may still work if the project doesn't require it.
func (p *SupabaseProvider) extractAnonKey() string {
	// This is set via config; loaded by the provider constructor.
	// For now we store it alongside the project URL.
	return p.anonKey
}

// credPath returns the full path to the Supabase credentials file.
func (p *SupabaseProvider) credPath() string {
	return filepath.Join(p.configDir, supabaseCredFile)
}

func (p *SupabaseProvider) saveCreds() error {
	data, err := json.Marshal(p.creds)
	if err != nil {
		return err
	}
	return SaveToken(p.credPath(), data)
}

func (p *SupabaseProvider) loadCreds() error {
	data, err := LoadToken(p.credPath())
	if err != nil {
		return err
	}
	var creds supabaseCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return err
	}
	if creds.RefreshToken != "" {
		p.creds = &creds
	}
	return nil
}

// PKCE helpers

func generatePKCE() (verifier, challenge string, err error) {
	// Generate 32 random bytes for the code verifier
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)

	// SHA-256 hash the verifier for the challenge
	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])

	return verifier, challenge, nil
}
