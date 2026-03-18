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

If you want the `schedule` command to work, also export:

```bash
export YCLIENTS_BEARER_TOKEN='your-token'
```

## Deploy on a VPS

Run the process under `systemd` so it starts on boot and restarts if it exits unexpectedly.

## Configuration precedence

- `TELEGRAM_BOT_TOKEN` from the shell environment wins.
- `.env` is used only as a local fallback.
- Do not commit real bot tokens into source files.

## Yclients client package

The project also contains a focused `yclients` package for the booking timeslots endpoint:

```go
client := yclients.Client{
    BaseURL: "https://platform.yclients.com",
    Token:   os.Getenv("YCLIENTS_BEARER_TOKEN"),
}

slots, err := client.SearchAvailableTimeSlots(ctx, yclients.SearchTimeSlotsParams{
    LocationID: 1296020,
    Date:       "2026-03-18",
    ServiceID:  19432008, // optional
})
```

It returns parsed `[]time.Time` values for entries where the API marks `is_bookable` as `true`.

You can also fetch projected available services in Yclients API order:

```go
services, err := client.AvailableServices(ctx, 1296020)
```

Each item contains only:
- `ID`
- `Title`
- `PriceMin`

## Bot commands

- `hello` replies with `Hello!`
- `schedule` sends `Выберите дату` with inline buttons for the next 7 days in `Europe/Moscow`
- selecting a date triggers a filtered Yclients request for that day only and returns a message like `📅 20.03.2026` and `🕒 08:00, 09:00, ...`

## Run the real Yclients integration test

If `YCLIENTS_BEARER_TOKEN` is set, the integration test runs automatically:

```bash
export YCLIENTS_BEARER_TOKEN='your-token'
env GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod-cache go test ./yclients -run Integration -v
```
