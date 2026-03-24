package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/donnycrash/clasp/internal/auth"
	"github.com/donnycrash/clasp/internal/collector"
	"github.com/donnycrash/clasp/internal/config"
	"github.com/donnycrash/clasp/internal/redactor"
	"github.com/donnycrash/clasp/internal/uploader"
	"github.com/donnycrash/clasp/internal/watermark"
	"github.com/spf13/cobra"
)

var uploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Collect, redact, and upload Claude usage data",
	Long:  "Runs the full pipeline: collect sessions, apply redaction rules, and upload to the analytics endpoint.",
	RunE:  runUpload,
}

func init() {
	rootCmd.AddCommand(uploadCmd)
}

func runUpload(cmd *cobra.Command, args []string) error {
	// 1. Load config.
	cfg, err := config.Load(config.ConfigPath())
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	slog.Info("config loaded", "path", config.ConfigPath())

	return doUpload(cfg)
}

// doUpload performs the upload pipeline and is shared by both the upload and run commands.
func doUpload(cfg *config.Config) error {
	// 2. Check auth — get identity and auth header.
	provider, err := createProvider(cfg)
	if err != nil {
		return fmt.Errorf("creating auth provider: %w", err)
	}

	if !provider.IsAuthenticated() {
		return fmt.Errorf("not authenticated; run 'clasp auth login' first")
	}

	identity, err := provider.GetIdentity()
	if err != nil {
		return fmt.Errorf("getting identity: %w", err)
	}
	slog.Info("authenticated", "provider", identity.Provider, "username", identity.Username)

	authHeader, err := provider.GetAuthHeader()
	if err != nil {
		return fmt.Errorf("getting auth header: %w", err)
	}

	// 3. Load watermark.
	wmPath := config.WatermarkPath()
	wm, err := watermark.Load(wmPath)
	if err != nil {
		return fmt.Errorf("loading watermark: %w", err)
	}

	// 4. Collect data.
	claudeDir := config.ClaudeDataDir(cfg)
	data, err := collector.Collect(claudeDir, wm)
	if err != nil {
		return fmt.Errorf("collecting data: %w", err)
	}

	if len(data.Sessions) == 0 && (data.Stats == nil || len(data.Stats.DailyActivity) == 0) {
		fmt.Println("Nothing new to upload.")
		return nil
	}

	// 5. Apply redaction.
	rules := redactor.RulesFromConfig(redactor.RedactionConfig{
		ProjectPath:    string(cfg.Redaction.ProjectPath),
		FirstPrompt:    string(cfg.Redaction.FirstPrompt),
		BriefSummary:   string(cfg.Redaction.BriefSummary),
		UnderlyingGoal: string(cfg.Redaction.UnderlyingGoal),
		FrictionDetail: string(cfg.Redaction.FrictionDetail),
	})
	rules.Apply(data)
	slog.Info("redaction applied")

	// 6. Build payload.
	payload := uploader.BuildPayload(data, uploader.Identity{
		Username: identity.Username,
	}, Version)

	// 7. Batch sessions.
	batchSize := cfg.Upload.BatchSize
	if batchSize <= 0 {
		batchSize = 50
	}

	batches := batchPayloads(payload, batchSize)
	slog.Info("batched sessions", "total_sessions", len(payload.Sessions), "batches", len(batches))

	// 8. Upload each batch.
	timeout, err := time.ParseDuration(cfg.Upload.Timeout)
	if err != nil {
		timeout = 30 * time.Second
	}
	backoff, err := time.ParseDuration(cfg.Upload.RetryBackoff)
	if err != nil {
		backoff = 30 * time.Second
	}

	client := uploader.NewClient(cfg.Endpoint, timeout, backoff, cfg.Upload.RetryMax)
	ctx := context.Background()

	var uploadedSessionIDs []string
	for i, batch := range batches {
		slog.Info("uploading batch", "batch", i+1, "of", len(batches), "sessions", len(batch.Sessions))
		if err := client.Upload(ctx, batch, authHeader); err != nil {
			return fmt.Errorf("uploading batch %d/%d: %w", i+1, len(batches), err)
		}
		for _, s := range batch.Sessions {
			uploadedSessionIDs = append(uploadedSessionIDs, s.SessionID)
		}
		slog.Info("batch uploaded successfully", "batch", i+1)
	}

	// 9. Update watermark on success.
	wm.MarkSessionsUploaded(uploadedSessionIDs)
	if data.Stats != nil && data.Stats.PeriodEnd != "" {
		wm.UpdateStatsDate(data.Stats.PeriodEnd)
	}

	// 10. Save watermark.
	if err := wm.Save(wmPath); err != nil {
		return fmt.Errorf("saving watermark: %w", err)
	}

	fmt.Printf("Upload complete: %d session(s) in %d batch(es).\n", len(uploadedSessionIDs), len(batches))
	return nil
}

// createProvider builds the appropriate auth provider from the config.
func createProvider(cfg *config.Config) (auth.Provider, error) {
	configDir := config.ConfigDir()
	switch cfg.Auth.Provider {
	case "github":
		return auth.NewGitHubProvider(cfg.Auth.GitHub.ClientID, configDir), nil
	case "apikey":
		return auth.NewAPIKeyProvider(configDir), nil
	default:
		return nil, fmt.Errorf("unknown auth provider: %s", cfg.Auth.Provider)
	}
}

// batchPayloads splits a single payload into multiple payloads, each containing
// at most batchSize sessions. The first batch includes the stats summary; the
// rest do not.
func batchPayloads(p *uploader.Payload, batchSize int) []*uploader.Payload {
	if len(p.Sessions) == 0 {
		// Still send one payload with stats only.
		return []*uploader.Payload{p}
	}

	var batches []*uploader.Payload
	for start := 0; start < len(p.Sessions); start += batchSize {
		end := start + batchSize
		if end > len(p.Sessions) {
			end = len(p.Sessions)
		}

		batch := &uploader.Payload{
			Metadata: p.Metadata,
			Sessions: p.Sessions[start:end],
		}
		// Only the first batch carries the stats summary.
		if start == 0 {
			batch.StatsSummary = p.StatsSummary
		}
		batches = append(batches, batch)
	}
	return batches
}
