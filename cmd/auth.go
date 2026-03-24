package cmd

import (
	"context"
	"fmt"

	"github.com/donnycrash/clasp/internal/config"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication",
	Long:  "Login, logout, and check authentication status.",
}

var authProviderFlag string

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with the configured provider",
	RunE:  runAuthLogin,
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current authentication status",
	RunE:  runAuthStatus,
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove stored credentials",
	RunE:  runAuthLogout,
}

func init() {
	authLoginCmd.Flags().StringVar(&authProviderFlag, "provider", "", "auth provider to use (github, apikey)")
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authLogoutCmd)
	rootCmd.AddCommand(authCmd)
}

func runAuthLogin(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(config.ConfigPath())
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	providerName := cfg.Auth.Provider
	if authProviderFlag != "" {
		providerName = authProviderFlag
	}

	// Temporarily override the provider in config so createProvider picks it up.
	cfg.Auth.Provider = providerName

	provider, err := createProvider(cfg)
	if err != nil {
		return fmt.Errorf("creating auth provider %q: %w", providerName, err)
	}

	ctx := context.Background()
	if err := provider.Login(ctx); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	return nil
}

func runAuthStatus(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(config.ConfigPath())
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	provider, err := createProvider(cfg)
	if err != nil {
		return fmt.Errorf("creating auth provider: %w", err)
	}

	fmt.Printf("Provider: %s\n", provider.Name())
	if !provider.IsAuthenticated() {
		fmt.Println("Status:   not authenticated")
		return nil
	}

	identity, err := provider.GetIdentity()
	if err != nil {
		return fmt.Errorf("getting identity: %w", err)
	}

	fmt.Println("Status:   authenticated")
	fmt.Printf("Username: %s\n", identity.Username)
	if identity.Email != "" {
		fmt.Printf("Email:    %s\n", identity.Email)
	}
	return nil
}

func runAuthLogout(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(config.ConfigPath())
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	provider, err := createProvider(cfg)
	if err != nil {
		return fmt.Errorf("creating auth provider: %w", err)
	}

	if err := provider.Logout(); err != nil {
		return fmt.Errorf("logout failed: %w", err)
	}

	fmt.Println("Logged out successfully.")
	return nil
}
