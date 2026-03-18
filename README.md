# Telegram Hello Bot

This project is a small Go Telegram bot that uses long polling and replies `Hello!` when it receives `hello`.

## Requirements

- Go 1.26.0
- A Telegram bot token in `TELEGRAM_BOT_TOKEN` or a local `.env` file

## Run tests

```bash
env GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod-cache go test ./...
```

## Run the bot

```bash
export TELEGRAM_BOT_TOKEN=your-token
env GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod-cache go run .
```

Or place the token in a local `.env` file:

```bash
echo 'TELEGRAM_BOT_TOKEN=your-token' > .env
env GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod-cache go run .
```

## Deploy on a VPS

Run the process under `systemd` so it starts on boot and restarts if it exits unexpectedly.

## Configuration precedence

- `TELEGRAM_BOT_TOKEN` from the shell environment wins.
- `.env` is used only as a local fallback.
- Do not commit real bot tokens into source files.
