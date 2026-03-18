package config

import (
	"bufio"
	"errors"
	"os"
	"strings"
)

const tokenKey = "TELEGRAM_BOT_TOKEN"

// LoadTelegramToken resolves the bot token from the environment first, then from a .env file.
func LoadTelegramToken(dotenvPath string) (string, error) {
	if token := strings.TrimSpace(os.Getenv(tokenKey)); token != "" {
		return token, nil
	}

	file, err := os.Open(dotenvPath)
	if err != nil {
		return "", errors.New("TELEGRAM_BOT_TOKEN is required")
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		if strings.TrimSpace(key) != tokenKey {
			continue
		}

		token := strings.Trim(strings.TrimSpace(value), `"'`)
		if token != "" {
			return token, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return "", errors.New("TELEGRAM_BOT_TOKEN is required")
}
