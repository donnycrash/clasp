package auth

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveToken_LoadToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token.json")
	data := []byte(`{"access_token":"abc123","refresh_token":"xyz789"}`)

	if err := SaveToken(path, data); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}

	loaded, err := LoadToken(path)
	if err != nil {
		t.Fatalf("LoadToken: %v", err)
	}

	if !bytes.Equal(data, loaded) {
		t.Errorf("loaded data = %q, want %q", string(loaded), string(data))
	}
}

func TestSaveToken_Permissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret-token")
	data := []byte("supersecret")

	if err := SaveToken(path, data); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}
}

func TestSaveToken_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	// Nested path with directories that don't exist yet.
	path := filepath.Join(dir, "a", "b", "c", "token.json")
	data := []byte("token-data")

	if err := SaveToken(path, data); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}

	// Verify the file was created.
	loaded, err := LoadToken(path)
	if err != nil {
		t.Fatalf("LoadToken: %v", err)
	}
	if !bytes.Equal(data, loaded) {
		t.Errorf("loaded data = %q, want %q", string(loaded), string(data))
	}

	// Verify the parent directory exists.
	parentDir := filepath.Dir(path)
	info, err := os.Stat(parentDir)
	if err != nil {
		t.Fatalf("Stat parent dir: %v", err)
	}
	if !info.IsDir() {
		t.Error("parent path is not a directory")
	}
}

func TestLoadToken_NotExist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent-token.json")

	_, err := LoadToken(path)
	if err == nil {
		t.Fatal("LoadToken should return error for non-existent file")
	}
}

func TestDeleteToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token-to-delete.json")
	data := []byte("delete-me")

	if err := SaveToken(path, data); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}

	// Verify it exists.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file should exist before delete: %v", err)
	}

	if err := DeleteToken(path); err != nil {
		t.Fatalf("DeleteToken: %v", err)
	}

	// Verify it's gone.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file should not exist after delete, Stat error = %v", err)
	}
}

func TestDeleteToken_NotExist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "already-gone.json")

	// Should not return error for a non-existent file.
	if err := DeleteToken(path); err != nil {
		t.Fatalf("DeleteToken on non-existent file should return nil, got: %v", err)
	}
}
