package auth

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

const apikeyCredFile = "apikey-credentials.json"

// apiKeyCredentials is the on-disk format for stored API keys.
type apiKeyCredentials struct {
	APIKey string `json:"api_key"`
}

// APIKeyProvider implements authentication via a user-supplied API key.
type APIKeyProvider struct {
	configDir string
	creds     *apiKeyCredentials
}

// NewAPIKeyProvider creates an API key auth provider.
//
//   - configDir: directory where credential files are stored.
func NewAPIKeyProvider(configDir string) *APIKeyProvider {
	p := &APIKeyProvider{configDir: configDir}
	_ = p.loadCreds()
	return p
}

func init() {
	Register("apikey", func() Provider {
		return &APIKeyProvider{}
	})
}

func (p *APIKeyProvider) Name() string { return "apikey" }

// Login prompts the user to enter an API key via stdin, then stores it.
func (p *APIKeyProvider) Login(_ context.Context) error {
	fmt.Print("Enter your API key: ")
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("read api key: %w", err)
	}

	key := strings.TrimSpace(line)
	if key == "" {
		return fmt.Errorf("api key cannot be empty")
	}

	p.creds = &apiKeyCredentials{APIKey: key}
	if err := p.saveCreds(); err != nil {
		return fmt.Errorf("save api key: %w", err)
	}

	slog.Info("api key saved successfully")
	fmt.Println("API key stored.")
	return nil
}

func (p *APIKeyProvider) GetIdentity() (Identity, error) {
	if p.creds == nil {
		return Identity{}, fmt.Errorf("apikey: not authenticated")
	}
	return Identity{
		Username: "apikey-user",
		Provider: "apikey",
	}, nil
}

func (p *APIKeyProvider) GetAuthHeader() (string, error) {
	if p.creds == nil {
		return "", fmt.Errorf("apikey: not authenticated")
	}
	return "X-API-Key " + p.creds.APIKey, nil
}

func (p *APIKeyProvider) Logout() error {
	p.creds = nil
	return DeleteToken(p.credPath())
}

func (p *APIKeyProvider) IsAuthenticated() bool {
	return p.creds != nil && p.creds.APIKey != ""
}

func (p *APIKeyProvider) credPath() string {
	return filepath.Join(p.configDir, apikeyCredFile)
}

func (p *APIKeyProvider) saveCreds() error {
	data, err := json.Marshal(p.creds)
	if err != nil {
		return err
	}
	return SaveToken(p.credPath(), data)
}

func (p *APIKeyProvider) loadCreds() error {
	data, err := LoadToken(p.credPath())
	if err != nil {
		return err
	}
	var creds apiKeyCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return err
	}
	if creds.APIKey != "" {
		p.creds = &creds
	}
	return nil
}
