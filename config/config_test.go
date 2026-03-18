package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTelegramToken(t *testing.T) {
	t.Run("environment wins over dotenv", func(t *testing.T) {
		dir := t.TempDir()
		dotenvPath := filepath.Join(dir, ".env")
		t.Setenv("TELEGRAM_BOT_TOKEN", "env-token")

		token, err := LoadTelegramToken(dotenvPath)
		if err != nil {
			t.Fatalf("LoadTelegramToken returned error: %v", err)
		}

		if token != "env-token" {
			t.Fatalf("token = %q, want %q", token, "env-token")
		}
	})

	t.Run("dotenv is used as fallback", func(t *testing.T) {
		dir := t.TempDir()
		dotenvPath := filepath.Join(dir, ".env")

		t.Setenv("TELEGRAM_BOT_TOKEN", "")

		if err := os.WriteFile(dotenvPath, []byte("TELEGRAM_BOT_TOKEN=dotenv-token\n"), 0o600); err != nil {
			t.Fatalf("WriteFile returned error: %v", err)
		}

		token, err := LoadTelegramToken(dotenvPath)
		if err != nil {
			t.Fatalf("LoadTelegramToken returned error: %v", err)
		}

		if token != "dotenv-token" {
			t.Fatalf("token = %q, want %q", token, "dotenv-token")
		}
	})

	t.Run("missing token returns error", func(t *testing.T) {
		dir := t.TempDir()
		dotenvPath := filepath.Join(dir, ".env")

		t.Setenv("TELEGRAM_BOT_TOKEN", "")

		if _, err := LoadTelegramToken(dotenvPath); err == nil {
			t.Fatal("LoadTelegramToken returned nil error, want non-nil")
		}
	})
}
