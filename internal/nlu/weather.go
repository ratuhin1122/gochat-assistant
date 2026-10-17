// Package nlu provides natural language understanding and response generation.
package nlu

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/ratuhin1122/gochat-assistant/internal/tools/weather"
)

var (
	// Patterns with a preposition (in, for, at, of) followed by a city name
	weatherInRegex     = regexp.MustCompile(`(?i)(?:weather|temperature|temp|forecast|conditions?|rain(?:ing)?|snow(?:ing)?)\s+(?:in|for|at|of)\s+([a-zA-Z\s.-]+)`)
	isItRainingInRegex = regexp.MustCompile(`(?i)is\s+it\s+(?:raining|snowing|sunny|cloudy|cold|hot|warm)\s+(?:in|at|for)\s+([a-zA-Z\s.-]+)`)
	cityWeatherRegex   = regexp.MustCompile(`(?i)^([a-zA-Z\s.-]+)\s+(?:weather|temperature|forecast)[\s?!.,]*$`)

	// Patterns where the city is missing
	missingCityRegex   = regexp.MustCompile(`(?i)^(?:what(?:'s|\s+is)\s+(?:the\s+)?)?(?:weather|temperature|temp|forecast)(?:\s+(?:like|today|now))?[\s?!.,]*$`)
	missingCityInRegex = regexp.MustCompile(`(?i)(?:weather|temperature|temp|forecast|is\s+it\s+(?:raining|snowing))\s+(?:in|for|at|of)[\s?!.,]*$`)
	isItRainingRegex   = regexp.MustCompile(`(?i)^is\s+it\s+(?:raining|snowing|sunny|cloudy|cold|hot)[\s?!.,]*$`)

	// Trailing time and polite words to clean from city names
	cityTrailingWords = regexp.MustCompile(`(?i)\s+(?:today|now|right\s+now|tomorrow|tonight|please)$`)
)

// WeatherResponder handles weather queries by extracting city names and calling the weather client.
type WeatherResponder struct {
	client weather.Service
}

// NewWeatherResponder creates a new WeatherResponder. If client is nil, it creates a default client.
func NewWeatherResponder(client weather.Service) *WeatherResponder {
	if client == nil {
		client = weather.NewClient()
	}
	return &WeatherResponder{
		client: client,
	}
}

// Respond implements the Responder interface for weather queries.
func (r *WeatherResponder) Respond(ctx context.Context, req Request) (Response, bool, error) {
	if err := ctx.Err(); err != nil {
		return Response{}, false, err
	}

	isWeather, city := detectWeatherIntent(req.Text)
	if !isWeather {
		return Response{}, false, nil
	}

	if city == "" {
		return Response{Text: "Which city?"}, true, nil
	}

	w, err := r.client.GetWeather(ctx, city)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return Response{}, false, err
		}
		if errors.Is(err, weather.ErrCityNotFound) {
			return Response{Text: fmt.Sprintf("Sorry, I couldn't find the weather for %q.", city)}, true, nil
		}
		return Response{Text: "Sorry, I'm having trouble getting the weather right now. Please try again later."}, true, nil
	}

	reply := fmt.Sprintf("The weather in %s is %s with a temperature of %.1f°C and wind speed of %.1f km/h.",
		w.City, w.Condition, w.Temperature, w.Wind)

	return Response{Text: reply}, true, nil
}

// detectWeatherIntent analyzes text to see if the user is asking about the weather and extracts the city name.
func detectWeatherIntent(text string) (bool, string) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false, ""
	}

	// 1. Missing city patterns
	if missingCityInRegex.MatchString(trimmed) || missingCityRegex.MatchString(trimmed) || isItRainingRegex.MatchString(trimmed) {
		return true, ""
	}

	// 2. Patterns with prepositions (e.g. "weather in Tokyo", "is it raining in Seattle")
	if m := weatherInRegex.FindStringSubmatch(trimmed); len(m) > 1 {
		city := cleanCity(m[1])
		return true, city
	}

	if m := isItRainingInRegex.FindStringSubmatch(trimmed); len(m) > 1 {
		city := cleanCity(m[1])
		return true, city
	}

	// 3. "<city> weather" pattern (e.g. "Tokyo weather")
	if m := cityWeatherRegex.FindStringSubmatch(trimmed); len(m) > 1 {
		city := cleanCity(m[1])
		lower := strings.ToLower(city)
		if lower != "what" && lower != "how" && lower != "the" && lower != "current" && lower != "todays" {
			return true, city
		}
		return true, ""
	}

	return false, ""
}

// cleanCity removes punctuation, extra whitespace, and trailing time keywords from extracted city names.
func cleanCity(raw string) string {
	raw = strings.Trim(raw, " \t\r\n?!.,;:'\"")
	raw = cityTrailingWords.ReplaceAllString(raw, "")
	return strings.Trim(raw, " \t\r\n?!.,;:'\"")
}
