// Package weather provides an Open-Meteo API client for fetching weather forecasts.
package weather

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultTimeout        = 5 * time.Second
	defaultGeoBaseURL     = "https://geocoding-api.open-meteo.com/v1/search"
	defaultWeatherBaseURL = "https://api.open-meteo.com/v1/forecast"
)

var (
	// ErrCityNotFound is returned when the geocoding service cannot find the requested city.
	ErrCityNotFound = errors.New("city not found")
	// ErrEmptyCity is returned when an empty city string is provided.
	ErrEmptyCity = errors.New("city name cannot be empty")
)

// Weather contains the resolved weather data for a city.
type Weather struct {
	City        string  `json:"city"`
	Temperature float64 `json:"temperature"`
	Condition   string  `json:"condition"`
	Wind        float64 `json:"wind"`
}

// Service defines a small interface for weather lookups.
type Service interface {
	GetWeather(ctx context.Context, city string) (*Weather, error)
}

// Provider is an alias for Service.
type Provider = Service

// Client fetches weather data using Open-Meteo geocoding and forecast APIs.
type Client struct {
	httpClient     *http.Client
	geoBaseURL     string
	weatherBaseURL string
	timeout        time.Duration
}

// Option allows customizing Client settings.
type Option func(*Client)

// WithHTTPClient sets a custom http.Client.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

// WithBaseURLs sets custom base URLs for geocoding and forecast (useful for testing).
func WithBaseURLs(geoURL, weatherURL string) Option {
	return func(c *Client) {
		if geoURL != "" {
			c.geoBaseURL = geoURL
		}
		if weatherURL != "" {
			c.weatherBaseURL = weatherURL
		}
	}
}

// WithTimeout sets a custom request timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		if d > 0 {
			c.timeout = d
		}
	}
}

// NewClient constructs a new Open-Meteo weather client using HTTPS and a 5-second timeout.
func NewClient(opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		geoBaseURL:     defaultGeoBaseURL,
		weatherBaseURL: defaultWeatherBaseURL,
		timeout:        defaultTimeout,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// geocodingResponse models the Open-Meteo geocoding search response.
type geocodingResponse struct {
	Results []struct {
		Name      string  `json:"name"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Country   string  `json:"country"`
	} `json:"results"`
}

// forecastResponse models the Open-Meteo forecast API response.
type forecastResponse struct {
	Current struct {
		Temperature float64 `json:"temperature_2m"`
		WeatherCode int     `json:"weather_code"`
		WindSpeed   float64 `json:"wind_speed_10m"`
	} `json:"current"`
	CurrentWeather struct {
		Temperature float64 `json:"temperature"`
		WeatherCode int     `json:"weathercode"`
		WindSpeed   float64 `json:"windspeed"`
	} `json:"current_weather"`
}

// GetWeather queries geocoding for latitude/longitude, then fetches current weather.
func (c *Client) GetWeather(ctx context.Context, city string) (*Weather, error) {
	city = strings.TrimSpace(city)
	if city == "" {
		return nil, ErrEmptyCity
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// 1. Geocode city name to lat/lon
	geoURL := fmt.Sprintf("%s?name=%s&count=1&language=en&format=json",
		c.geoBaseURL, url.QueryEscape(city))

	geoReq, err := http.NewRequestWithContext(ctx, http.MethodGet, geoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create geocoding request: %w", err)
	}

	geoResp, err := c.httpClient.Do(geoReq)
	if err != nil {
		return nil, fmt.Errorf("geocoding request failed: %w", err)
	}
	defer geoResp.Body.Close()

	if geoResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("geocoding returned status %d", geoResp.StatusCode)
	}

	var geoData geocodingResponse
	if err := json.NewDecoder(geoResp.Body).Decode(&geoData); err != nil {
		return nil, fmt.Errorf("decode geocoding response: %w", err)
	}

	if len(geoData.Results) == 0 {
		return nil, fmt.Errorf("%w: %q", ErrCityNotFound, city)
	}

	location := geoData.Results[0]

	// 2. Fetch forecast data for coordinates
	forecastURL := fmt.Sprintf("%s?latitude=%f&longitude=%f&current=temperature_2m,weather_code,wind_speed_10m&current_weather=true",
		c.weatherBaseURL, location.Latitude, location.Longitude)

	forecastReq, err := http.NewRequestWithContext(ctx, http.MethodGet, forecastURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create forecast request: %w", err)
	}

	forecastResp, err := c.httpClient.Do(forecastReq)
	if err != nil {
		return nil, fmt.Errorf("forecast request failed: %w", err)
	}
	defer forecastResp.Body.Close()

	if forecastResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("forecast returned status %d", forecastResp.StatusCode)
	}

	var fcData forecastResponse
	if err := json.NewDecoder(forecastResp.Body).Decode(&fcData); err != nil {
		return nil, fmt.Errorf("decode forecast response: %w", err)
	}

	// Extract values from current or fallback to current_weather
	temp := fcData.Current.Temperature
	code := fcData.Current.WeatherCode
	wind := fcData.Current.WindSpeed

	if temp == 0 && wind == 0 && fcData.CurrentWeather.Temperature != 0 {
		temp = fcData.CurrentWeather.Temperature
		code = fcData.CurrentWeather.WeatherCode
		wind = fcData.CurrentWeather.WindSpeed
	}

	return &Weather{
		City:        location.Name,
		Temperature: temp,
		Condition:   WeatherCodeToText(code),
		Wind:        wind,
	}, nil
}

// Get is an alias for GetWeather.
func (c *Client) Get(ctx context.Context, city string) (*Weather, error) {
	return c.GetWeather(ctx, city)
}

// WeatherCodeToText translates WMO weather interpretation codes into descriptive text.
func WeatherCodeToText(code int) string {
	switch code {
	case 0:
		return "Clear sky"
	case 1:
		return "Mainly clear"
	case 2:
		return "Partly cloudy"
	case 3:
		return "Overcast"
	case 45:
		return "Fog"
	case 48:
		return "Depositing rime fog"
	case 51:
		return "Light drizzle"
	case 53:
		return "Moderate drizzle"
	case 55:
		return "Dense drizzle"
	case 56:
		return "Light freezing drizzle"
	case 57:
		return "Dense freezing drizzle"
	case 61:
		return "Slight rain"
	case 63:
		return "Moderate rain"
	case 65:
		return "Heavy rain"
	case 66:
		return "Light freezing rain"
	case 67:
		return "Heavy freezing rain"
	case 71:
		return "Slight snow fall"
	case 73:
		return "Moderate snow fall"
	case 75:
		return "Heavy snow fall"
	case 77:
		return "Snow grains"
	case 80:
		return "Slight rain showers"
	case 81:
		return "Moderate rain showers"
	case 82:
		return "Violent rain showers"
	case 85:
		return "Slight snow showers"
	case 86:
		return "Heavy snow showers"
	case 95:
		return "Thunderstorm"
	case 96:
		return "Thunderstorm with slight hail"
	case 99:
		return "Thunderstorm with heavy hail"
	default:
		return "Unknown"
	}
}
