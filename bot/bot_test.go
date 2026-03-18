package bot

import "testing"

func TestReplyForText(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantReply string
		wantOK    bool
	}{
		{
			name:      "exact hello matches",
			input:     "hello",
			wantReply: "Hello!",
			wantOK:    true,
		},
		{
			name:      "mixed case hello matches",
			input:     "Hello",
			wantReply: "Hello!",
			wantOK:    true,
		},
		{
			name:      "other text is ignored",
			input:     "bye",
			wantReply: "",
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotReply, gotOK := ReplyForText(tt.input)

			if gotReply != tt.wantReply {
				t.Fatalf("reply = %q, want %q", gotReply, tt.wantReply)
			}

			if gotOK != tt.wantOK {
				t.Fatalf("ok = %v, want %v", gotOK, tt.wantOK)
			}
		})
	}
}
