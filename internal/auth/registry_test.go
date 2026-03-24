package auth

import (
	"context"
	"testing"
)

// stubProvider is a minimal Provider implementation for registry tests.
type stubProvider struct {
	name string
}

func (s *stubProvider) Name() string                        { return s.name }
func (s *stubProvider) Login(_ context.Context) error       { return nil }
func (s *stubProvider) GetIdentity() (Identity, error)      { return Identity{Username: s.name}, nil }
func (s *stubProvider) GetAuthHeader() (string, error)      { return "Bearer " + s.name, nil }
func (s *stubProvider) Logout() error                       { return nil }
func (s *stubProvider) IsAuthenticated() bool               { return true }

func TestRegister_Get(t *testing.T) {
	// Clean up after this test so we don't pollute the global registry for
	// other tests.
	name := "test-provider-register-get"
	Register(name, func() Provider {
		return &stubProvider{name: name}
	})

	p, err := Get(name)
	if err != nil {
		t.Fatalf("Get(%q) returned error: %v", name, err)
	}
	if p.Name() != name {
		t.Errorf("provider Name() = %q, want %q", p.Name(), name)
	}
}

func TestGet_NotFound(t *testing.T) {
	_, err := Get("nonexistent-provider-xyz")
	if err == nil {
		t.Fatal("Get should return error for unknown provider")
	}
	if got := err.Error(); got == "" {
		t.Error("error message should not be empty")
	}
}

func TestList(t *testing.T) {
	// Register a known provider for this test.
	name := "test-provider-list"
	Register(name, func() Provider {
		return &stubProvider{name: name}
	})

	names := List()
	if len(names) == 0 {
		t.Fatal("List() returned empty slice")
	}

	found := false
	for _, n := range names {
		if n == name {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("List() = %v, expected to contain %q", names, name)
	}

	// Verify sorted order.
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("List() is not sorted: %v", names)
			break
		}
	}
}

func TestList_ContainsAPIKey(t *testing.T) {
	// The apikey provider registers itself via init(), so it should appear.
	names := List()
	found := false
	for _, n := range names {
		if n == "apikey" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("List() = %v, expected to contain 'apikey' (registered in init())", names)
	}
}
