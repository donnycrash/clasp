package auth

import "context"

// Identity represents an authenticated user.
type Identity struct {
	Username   string `json:"username"`
	Email      string `json:"email,omitempty"`
	ProviderID string `json:"provider_id,omitempty"`
	Provider   string `json:"provider"`
}

// Provider is the interface that authentication backends must implement.
type Provider interface {
	// Name returns the provider's unique identifier (e.g. "github", "apikey").
	Name() string

	// Login performs the authentication flow, which may involve user interaction.
	Login(ctx context.Context) error

	// GetIdentity returns the identity of the currently authenticated user.
	GetIdentity() (Identity, error)

	// GetAuthHeader returns the value for the Authorization (or equivalent)
	// HTTP header used when uploading data.
	GetAuthHeader() (string, error)

	// Logout removes stored credentials and clears the authenticated state.
	Logout() error

	// IsAuthenticated reports whether the provider currently holds valid
	// credentials.
	IsAuthenticated() bool
}
