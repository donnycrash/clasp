package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

const (
	githubDeviceCodeURL = "https://github.com/login/device/code"
	githubTokenURL      = "https://github.com/login/oauth/access_token"
	githubUserURL       = "https://api.github.com/user"
	githubCredFile      = "github-credentials.json"
)

// githubCredentials is the on-disk format for stored GitHub tokens.
type githubCredentials struct {
	AccessToken string `json:"access_token"`
	Username    string `json:"username"`
}

// GitHubProvider implements the OAuth Device Flow for GitHub authentication.
type GitHubProvider struct {
	clientID  string
	configDir string
	creds     *githubCredentials
}

// NewGitHubProvider creates a GitHub auth provider.
//
//   - clientID: the OAuth App's client ID registered with GitHub.
//   - configDir: directory where credential files are stored.
func NewGitHubProvider(clientID, configDir string) *GitHubProvider {
	p := &GitHubProvider{
		clientID:  clientID,
		configDir: configDir,
	}
	// Attempt to load existing credentials.
	_ = p.loadCreds()
	return p
}

func init() {
	Register("github", func() Provider {
		return &GitHubProvider{}
	})
}

func (p *GitHubProvider) Name() string { return "github" }

// Login performs the GitHub OAuth Device Flow, displaying a user code for the
// user to enter at the verification URL, then polls for the access token.
func (p *GitHubProvider) Login(ctx context.Context) error {
	if p.clientID == "" {
		return fmt.Errorf("github: client_id is not configured")
	}

	// Step 1: Request device and user codes.
	dc, err := p.requestDeviceCode(ctx)
	if err != nil {
		return fmt.Errorf("github: request device code: %w", err)
	}

	fmt.Printf("\nTo authenticate, open the following URL in your browser:\n\n")
	fmt.Printf("  %s\n\n", dc.VerificationURI)
	fmt.Printf("And enter code: %s\n\n", dc.UserCode)

	// Step 2: Poll for access token.
	token, err := p.pollForToken(ctx, dc)
	if err != nil {
		return fmt.Errorf("github: poll for token: %w", err)
	}

	// Step 3: Fetch user info.
	username, err := p.fetchUsername(ctx, token)
	if err != nil {
		return fmt.Errorf("github: fetch user info: %w", err)
	}

	p.creds = &githubCredentials{
		AccessToken: token,
		Username:    username,
	}

	if err := p.saveCreds(); err != nil {
		return fmt.Errorf("github: save credentials: %w", err)
	}

	slog.Info("github login successful", "username", username)
	fmt.Printf("Authenticated as %s\n", username)
	return nil
}

func (p *GitHubProvider) GetIdentity() (Identity, error) {
	if p.creds == nil {
		return Identity{}, fmt.Errorf("github: not authenticated")
	}
	return Identity{
		Username:   p.creds.Username,
		ProviderID: p.creds.Username,
		Provider:   "github",
	}, nil
}

func (p *GitHubProvider) GetAuthHeader() (string, error) {
	if p.creds == nil {
		return "", fmt.Errorf("github: not authenticated")
	}
	return "Bearer " + p.creds.AccessToken, nil
}

func (p *GitHubProvider) Logout() error {
	p.creds = nil
	return DeleteToken(p.credPath())
}

func (p *GitHubProvider) IsAuthenticated() bool {
	return p.creds != nil && p.creds.AccessToken != ""
}

// credPath returns the full path to the GitHub credentials file.
func (p *GitHubProvider) credPath() string {
	return filepath.Join(p.configDir, githubCredFile)
}

func (p *GitHubProvider) saveCreds() error {
	data, err := json.Marshal(p.creds)
	if err != nil {
		return err
	}
	return SaveToken(p.credPath(), data)
}

func (p *GitHubProvider) loadCreds() error {
	data, err := LoadToken(p.credPath())
	if err != nil {
		return err
	}
	var creds githubCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return err
	}
	if creds.AccessToken != "" {
		p.creds = &creds
	}
	return nil
}

// deviceCodeResponse mirrors GitHub's device code endpoint response.
type deviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

func (p *GitHubProvider) requestDeviceCode(ctx context.Context) (*deviceCodeResponse, error) {
	form := url.Values{
		"client_id": {p.clientID},
		"scope":     {"read:user"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, githubDeviceCodeURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var dc deviceCodeResponse
	if err := json.Unmarshal(body, &dc); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &dc, nil
}

func (p *GitHubProvider) pollForToken(ctx context.Context, dc *deviceCodeResponse) (string, error) {
	interval := time.Duration(dc.Interval) * time.Second
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}
	deadline := time.Now().Add(time.Duration(dc.ExpiresIn) * time.Second)

	for {
		if time.Now().After(deadline) {
			return "", fmt.Errorf("device code expired")
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(interval):
		}

		token, err := p.exchangeDeviceCode(ctx, dc.DeviceCode)
		if err != nil {
			// If it's a "slow_down" or "authorization_pending" we keep polling.
			if isPollContinue(err) {
				slog.Debug("github: still waiting for user authorization")
				continue
			}
			return "", err
		}
		return token, nil
	}
}

// tokenResponse mirrors GitHub's token endpoint response.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

func (p *GitHubProvider) exchangeDeviceCode(ctx context.Context, deviceCode string) (string, error) {
	form := url.Values{
		"client_id":   {p.clientID},
		"device_code": {deviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, githubTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}

	if tr.Error != "" {
		return "", &oauthError{Code: tr.Error, Description: tr.ErrorDesc}
	}

	if tr.AccessToken == "" {
		return "", fmt.Errorf("empty access token in response")
	}

	return tr.AccessToken, nil
}

func (p *GitHubProvider) fetchUsername(ctx context.Context, token string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubUserURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github API error %d: %s", resp.StatusCode, string(body))
	}

	var user struct {
		Login string `json:"login"`
	}
	if err := json.Unmarshal(body, &user); err != nil {
		return "", fmt.Errorf("decode user response: %w", err)
	}

	if user.Login == "" {
		return "", fmt.Errorf("empty login in github user response")
	}

	return user.Login, nil
}

// oauthError represents an OAuth error response from GitHub.
type oauthError struct {
	Code        string
	Description string
}

func (e *oauthError) Error() string {
	if e.Description != "" {
		return fmt.Sprintf("oauth: %s: %s", e.Code, e.Description)
	}
	return fmt.Sprintf("oauth: %s", e.Code)
}

// isPollContinue returns true if the error indicates we should keep polling.
func isPollContinue(err error) bool {
	if oe, ok := err.(*oauthError); ok {
		return oe.Code == "authorization_pending" || oe.Code == "slow_down"
	}
	return false
}
