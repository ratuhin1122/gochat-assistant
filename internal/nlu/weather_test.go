package nlu_test

import (
	"context"
	"strings"
	"testing"

	"github.com/ratuhin1122/gochat-assistant/internal/nlu"
	"github.com/ratuhin1122/gochat-assistant/internal/tools/weather"
)

type fakeWeatherClient struct {
	weather    *weather.Weather
	err        error
	calledCity string
	called     bool
}

func (f *fakeWeatherClient) GetWeather(ctx context.Context, city string) (*weather.Weather, error) {
	f.called = true
	f.calledCity = city
	return f.weather, f.err
}

func TestWeatherResponder_Success(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantCity   string
		fakeData   *weather.Weather
		wantSubstr []string
	}{
		{
			name:     "weather in city",
			input:    "weather in Tokyo",
			wantCity: "Tokyo",
			fakeData: &weather.Weather{
				City:        "Tokyo",
				Temperature: 22.5,
				Condition:   "Clear sky",
				Wind:        11.2,
			},
			wantSubstr: []string{"Tokyo", "Clear sky", "22.5°C", "11.2 km/h"},
		},
		{
			name:     "temperature in city with question mark",
			input:    "temperature in Paris?",
			wantCity: "Paris",
			fakeData: &weather.Weather{
				City:        "Paris",
				Temperature: 15.0,
				Condition:   "Partly cloudy",
				Wind:        8.5,
			},
			wantSubstr: []string{"Paris", "Partly cloudy", "15.0°C"},
		},
		{
			name:     "is it raining in city",
			input:    "is it raining in Seattle",
			wantCity: "Seattle",
			fakeData: &weather.Weather{
				City:        "Seattle",
				Temperature: 11.0,
				Condition:   "Slight rain",
				Wind:        14.0,
			},
			wantSubstr: []string{"Seattle", "Slight rain", "11.0°C"},
		},
		{
			name:     "weather in city with trailing today keyword",
			input:    "what is the weather in London today?",
			wantCity: "London",
			fakeData: &weather.Weather{
				City:        "London",
				Temperature: 13.2,
				Condition:   "Overcast",
				Wind:        16.1,
			},
			wantSubstr: []string{"London", "Overcast"},
		},
		{
			name:     "city weather pattern",
			input:    "Berlin weather",
			wantCity: "Berlin",
			fakeData: &weather.Weather{
				City:        "Berlin",
				Temperature: 18.0,
				Condition:   "Mainly clear",
				Wind:        9.0,
			},
			wantSubstr: []string{"Berlin", "Mainly clear"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeWeatherClient{weather: tc.fakeData}
			responder := nlu.NewWeatherResponder(fake)

			resp, handled, err := responder.Respond(context.Background(), nlu.Request{Text: tc.input})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !handled {
				t.Fatalf("expected handled = true for %q, got false", tc.input)
			}
			if !fake.called {
				t.Fatalf("expected fake client to be called, but was not")
			}
			if fake.calledCity != tc.wantCity {
				t.Errorf("called city = %q, want %q", fake.calledCity, tc.wantCity)
			}

			for _, sub := range tc.wantSubstr {
				if !strings.Contains(resp.Text, sub) {
					t.Errorf("response %q does not contain expected substring %q", resp.Text, sub)
				}
			}
		})
	}
}

func TestWeatherResponder_MissingCity(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "weather in without city", input: "weather in"},
		{name: "just weather", input: "weather"},
		{name: "what is the weather", input: "what is the weather?"},
		{name: "is it raining without city", input: "is it raining?"},
		{name: "temperature in without city", input: "temperature in"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeWeatherClient{}
			responder := nlu.NewWeatherResponder(fake)

			resp, handled, err := responder.Respond(context.Background(), nlu.Request{Text: tc.input})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !handled {
				t.Fatalf("expected handled = true for missing city prompt, got false")
			}
			if fake.called {
				t.Errorf("fake client should not be called when city is missing")
			}
			if resp.Text != "Which city?" {
				t.Errorf("response = %q, want 'Which city?'", resp.Text)
			}
		})
	}
}

func TestWeatherResponder_Unrelated(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "greeting", input: "hello"},
		{name: "joke", input: "tell me a joke"},
		{name: "who built you", input: "who built you"},
		{name: "random gibberish", input: "qwertyuiop asdfghjkl"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeWeatherClient{}
			responder := nlu.NewWeatherResponder(fake)

			_, handled, err := responder.Respond(context.Background(), nlu.Request{Text: tc.input})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if handled {
				t.Errorf("expected handled = false for %q, got true", tc.input)
			}
			if fake.called {
				t.Errorf("fake client should not be called for unrelated text")
			}
		})
	}
}

func TestWeatherResponder_CityNotFound(t *testing.T) {
	fake := &fakeWeatherClient{err: weather.ErrCityNotFound}
	responder := nlu.NewWeatherResponder(fake)

	resp, handled, err := responder.Respond(context.Background(), nlu.Request{Text: "weather in Atlantis"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatalf("expected handled = true for city not found error")
	}
	if !strings.Contains(resp.Text, "couldn't find") {
		t.Errorf("expected friendly error message, got %q", resp.Text)
	}
}

func TestWeatherResponder_ContextCanceled(t *testing.T) {
	fake := &fakeWeatherClient{}
	responder := nlu.NewWeatherResponder(fake)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := responder.Respond(ctx, nlu.Request{Text: "weather in Tokyo"})
	if err == nil {
		t.Fatal("expected error on canceled context, got nil")
	}
}

func TestDefaultChain_FAQThenWeather(t *testing.T) {
	chain, err := nlu.NewDefaultChain()
	if err != nil {
		t.Fatalf("failed to create default chain: %v", err)
	}

	// 1. FAQ handled first: "hello" matches greeting
	resp, err := chain.Respond(context.Background(), nlu.Request{Text: "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(resp.Text, "Hello!") {
		t.Errorf("expected FAQ greeting, got %q", resp.Text)
	}

	// 2. Weather handled: missing city asks "Which city?"
	resp, err = chain.Respond(context.Background(), nlu.Request{Text: "is it raining?"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "Which city?" {
		t.Errorf("expected 'Which city?', got %q", resp.Text)
	}

	// 3. Unrelated query falls back to default message
	resp, err = chain.Respond(context.Background(), nlu.Request{Text: "completely random query 98765"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != nlu.DefaultFallback {
		t.Errorf("expected fallback %q, got %q", nlu.DefaultFallback, resp.Text)
	}
}
