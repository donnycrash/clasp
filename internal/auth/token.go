package auth

import (
	"fmt"
	"os"
	"path/filepath"
)

// SaveToken writes credential data to path with restrictive permissions (0600).
// Parent directories are created as needed.
func SaveToken(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create token directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write token file: %w", err)
	}
	return nil
}

// LoadToken reads and returns the contents of the credential file at path.
func LoadToken(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read token file: %w", err)
	}
	return data, nil
}

// DeleteToken removes the credential file at path. It returns nil if the
// file does not exist.
func DeleteToken(path string) error {
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete token file: %w", err)
	}
	return nil
}
