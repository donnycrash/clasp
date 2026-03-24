package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/donnycrash/clasp/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View and modify CLASP configuration",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print the current configuration as YAML",
	RunE:  runConfigShow,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE:  runConfigSet,
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	rootCmd.AddCommand(configCmd)
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfgPath := config.ConfigPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	out, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}

	fmt.Print(string(out))
	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	cfgPath := config.ConfigPath()

	// Read the raw YAML as a generic map so we preserve existing structure.
	raw := make(map[string]interface{})
	data, err := os.ReadFile(cfgPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading config file: %w", err)
	}
	if len(data) > 0 {
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("parsing config file: %w", err)
		}
	}

	// Support dotted keys like "auth.provider" or "upload.batch_size".
	parts := strings.Split(key, ".")
	setNestedValue(raw, parts, value)

	out, err := yaml.Marshal(raw)
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}

	// Ensure the config directory exists.
	dir := config.ConfigDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	if err := os.WriteFile(cfgPath, out, 0o644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	fmt.Printf("Set %s = %s\n", key, value)
	return nil
}

// setNestedValue walks into nested maps following the key parts and sets the
// leaf value. Intermediate maps are created as needed.
func setNestedValue(m map[string]interface{}, parts []string, value string) {
	for i, part := range parts {
		if i == len(parts)-1 {
			m[part] = value
			return
		}
		child, ok := m[part]
		if !ok {
			child = make(map[string]interface{})
			m[part] = child
		}
		childMap, ok := child.(map[string]interface{})
		if !ok {
			childMap = make(map[string]interface{})
			m[part] = childMap
		}
		m = childMap
	}
}
