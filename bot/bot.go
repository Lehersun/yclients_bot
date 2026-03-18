package bot

import "strings"

// ReplyForText decides whether an incoming text message should trigger a reply.
func ReplyForText(text string) (string, bool) {
	if strings.EqualFold(strings.TrimSpace(text), "hello") {
		return "Hello!", true
	}

	return "", false
}
