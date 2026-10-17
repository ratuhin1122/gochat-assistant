// Package chat manages real-time WebSocket communication.
package chat

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/go-playground/validator/v10"
)

// IncomingMessage represents a JSON message received from the client.
type IncomingMessage struct {
	Type string `json:"type" validate:"required"`
	Text string `json:"text" validate:"required,max=500,nocontrol"`
}

// OutgoingMessage represents a JSON message sent to the client.
type OutgoingMessage struct {
	Type      string `json:"type"`
	Text      string `json:"text"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
}

// MessageValidator encapsulates validation rules for incoming chat messages.
type MessageValidator struct {
	validate *validator.Validate
}

// NewMessageValidator creates and configures a MessageValidator with custom rules.
func NewMessageValidator() *MessageValidator {
	v := validator.New()
	_ = v.RegisterValidation("nocontrol", validateNoControl)
	return &MessageValidator{validate: v}
}

// Validate trims text and validates fields according to rules.
// Rules: text required, trimmed, max 500 chars, no control characters.
func (mv *MessageValidator) Validate(msg *IncomingMessage) error {
	if msg == nil {
		return errors.New("message is nil")
	}

	msg.Text = strings.TrimSpace(msg.Text)

	if err := mv.validate.Struct(msg); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	return nil
}

// NewOutgoingMessage constructs an OutgoingMessage with generated ID and RFC3339 timestamp.
func NewOutgoingMessage(msgType, text string) OutgoingMessage {
	return OutgoingMessage{
		Type:      msgType,
		Text:      text,
		ID:        generateMessageID(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// NewErrorMessage constructs an error-type OutgoingMessage.
func NewErrorMessage(text string) OutgoingMessage {
	return NewOutgoingMessage("error", text)
}

// validateNoControl checks that the string contains no Unicode control characters.
func validateNoControl(fl validator.FieldLevel) bool {
	val := fl.Field().String()
	for _, r := range val {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// generateMessageID generates a random 16-byte hex identifier.
func generateMessageID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("msg_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
