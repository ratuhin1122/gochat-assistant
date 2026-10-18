// Package nlu provides natural language understanding and response generation.
package nlu

import (
	"context"
	"fmt"

	"github.com/ratuhin1122/gochat-assistant/internal/observability"
)

// DefaultFallback is the default message returned when no responder handles the request.
const DefaultFallback = "Sorry, I didn't understand that."

// Request represents an incoming user request to be processed by a Responder.
type Request struct {
	Text string
}

// Response represents a generated reply from a Responder or Chain.
type Response struct {
	Text string
}

// Responder evaluates a Request and optionally provides a Response.
// The returned bool indicates whether the Responder handled the request.
type Responder interface {
	Respond(ctx context.Context, req Request) (Response, bool, error)
}

// Chain manages an ordered list of Responders and evaluates them sequentially.
// If none of the responders handle the request, it returns a default fallback response.
type Chain struct {
	responders []Responder
	fallback   string
}

// NewChain constructs a Chain with the given responders and the default fallback message.
func NewChain(responders ...Responder) *Chain {
	return &Chain{
		responders: responders,
		fallback:   DefaultFallback,
	}
}

// NewDefaultChain constructs the default Chain with FAQResponder registered first,
// WeatherResponder registered after FAQ, and any additional responders (such as LLMResponder) appended.
func NewDefaultChain(additional ...Responder) (*Chain, error) {
	faq, err := NewFAQResponder()
	if err != nil {
		return nil, fmt.Errorf("create default faq responder: %w", err)
	}
	weatherResp := NewWeatherResponder(nil)
	responders := make([]Responder, 0, 2+len(additional))
	responders = append(responders, faq, weatherResp)
	responders = append(responders, additional...)
	return NewChain(responders...), nil
}

// NewChainWithFallback constructs a Chain with a custom fallback message.
func NewChainWithFallback(fallback string, responders ...Responder) *Chain {
	if fallback == "" {
		fallback = DefaultFallback
	}
	return &Chain{
		responders: responders,
		fallback:   fallback,
	}
}

// Respond executes each responder in order. The first responder that returns handled=true
// determines the response. If an error is returned by a responder, execution stops and
// the error is returned. If no responder handles the request, the fallback response is returned.
func (c *Chain) Respond(ctx context.Context, req Request) (Response, error) {
	if c == nil {
		return Response{Text: DefaultFallback}, nil
	}

	if err := ctx.Err(); err != nil {
		return Response{}, err
	}

	for i, r := range c.responders {
		if err := ctx.Err(); err != nil {
			return Response{}, err
		}

		resp, handled, err := r.Respond(ctx, req)
		if err != nil {
			return Response{}, fmt.Errorf("responder at index %d failed: %w", i, err)
		}
		if handled {
			observability.RecordMessageByResponder(responderName(r))
			return resp, nil
		}
	}

	observability.RecordMessageByResponder("default")
	return Response{Text: c.fallback}, nil
}

// responderName extracts a friendly name for a Responder.
func responderName(r Responder) string {
	if n, ok := r.(interface{ Name() string }); ok {
		return n.Name()
	}
	switch r.(type) {
	case *FAQResponder:
		return "faq"
	case *WeatherResponder:
		return "weather"
	case *LLMResponder:
		return "llm"
	default:
		return "unknown"
	}
}
