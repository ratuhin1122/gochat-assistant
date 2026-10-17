package chat_test

import (
	"strings"
	"testing"

	"github.com/ratuhin1122/gochat-assistant/internal/chat"
)

func TestMessageValidator(t *testing.T) {
	v := chat.NewMessageValidator()

	tests := []struct {
		name       string
		msg        *chat.IncomingMessage
		wantErr    bool
		wantTrimmed string
	}{
		{
			name: "valid message",
			msg: &chat.IncomingMessage{
				Type: "text",
				Text: "Hello there!",
			},
			wantErr:    false,
			wantTrimmed: "Hello there!",
		},
		{
			name: "valid message with whitespace to trim",
			msg: &chat.IncomingMessage{
				Type: "text",
				Text: "   leading and trailing spaces   ",
			},
			wantErr:    false,
			wantTrimmed: "leading and trailing spaces",
		},
		{
			name: "empty text fails",
			msg: &chat.IncomingMessage{
				Type: "text",
				Text: "",
			},
			wantErr: true,
		},
		{
			name: "whitespace-only text trimmed to empty fails",
			msg: &chat.IncomingMessage{
				Type: "text",
				Text: "    \t   ",
			},
			wantErr: true,
		},
		{
			name: "missing type fails",
			msg: &chat.IncomingMessage{
				Type: "",
				Text: "valid text",
			},
			wantErr: true,
		},
		{
			name: "text exactly 500 chars passes",
			msg: &chat.IncomingMessage{
				Type: "text",
				Text: strings.Repeat("a", 500),
			},
			wantErr:    false,
			wantTrimmed: strings.Repeat("a", 500),
		},
		{
			name: "text over 500 chars fails",
			msg: &chat.IncomingMessage{
				Type: "text",
				Text: strings.Repeat("a", 501),
			},
			wantErr: true,
		},
		{
			name: "control character null byte fails",
			msg: &chat.IncomingMessage{
				Type: "text",
				Text: "hello\x00world",
			},
			wantErr: true,
		},
		{
			name: "control character bell fails",
			msg: &chat.IncomingMessage{
				Type: "text",
				Text: "bell\x07sound",
			},
			wantErr: true,
		},
		{
			name: "nil message fails",
			msg:  nil,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := v.Validate(tc.msg)
			if tc.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tc.wantErr && tc.msg != nil && tc.msg.Text != tc.wantTrimmed {
				t.Errorf("expected trimmed text %q, got %q", tc.wantTrimmed, tc.msg.Text)
			}
		})
	}
}

func TestNewOutgoingMessage(t *testing.T) {
	msg := chat.NewOutgoingMessage("text", "hello world")

	if msg.Type != "text" {
		t.Errorf("expected Type 'text', got %q", msg.Type)
	}
	if msg.Text != "hello world" {
		t.Errorf("expected Text 'hello world', got %q", msg.Text)
	}
	if msg.ID == "" {
		t.Errorf("expected non-empty ID")
	}
	if msg.Timestamp == "" {
		t.Errorf("expected non-empty Timestamp")
	}
}

func TestNewErrorMessage(t *testing.T) {
	msg := chat.NewErrorMessage("something went wrong")

	if msg.Type != "error" {
		t.Errorf("expected Type 'error', got %q", msg.Type)
	}
	if msg.Text != "something went wrong" {
		t.Errorf("expected Text 'something went wrong', got %q", msg.Text)
	}
	if msg.ID == "" {
		t.Errorf("expected non-empty ID")
	}
	if msg.Timestamp == "" {
		t.Errorf("expected non-empty Timestamp")
	}
}
