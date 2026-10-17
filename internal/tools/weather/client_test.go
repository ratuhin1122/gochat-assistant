package weather_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ratuhin1122/gochat-assistant/internal/tools/weather"
)

func TestClient_Interface(t *testing.T) {
	var _ weather.Service = (*weather.Client)(nil)
	var _ weather.Provider = (*weather.Client)(nil)
}

func TestClient_GetWeather_Success(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/geo", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		if name != "Tokyo" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		resp := map[string]any{
			"results": []map[string]any{
				{
					"name":      "Tokyo",
					"latitude":  35.6895,
					"longitude": 139.6917,
					"country":   "Japan",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/forecast", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"current": map[string]any{
				"temperature_2m": 21.5,
				"weather_code":   0,
				"wind_speed_10m": 10.3,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := weather.NewClient(
		weather.WithHTTPClient(srv.Client()),
		weather.WithBaseURLs(srv.URL+"/geo", srv.URL+"/forecast"),
	)

	w, err := client.GetWeather(context.Background(), "Tokyo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if w.City != "Tokyo" {
		t.Errorf("expected city Tokyo, got %q", w.City)
	}
	if w.Temperature != 21.5 {
		t.Errorf("expected temperature 21.5, got %f", w.Temperature)
	}
	if w.Condition != "Clear sky" {
		t.Errorf("expected condition 'Clear sky', got %q", w.Condition)
	}
	if w.Wind != 10.3 {
		t.Errorf("expected wind 10.3, got %f", w.Wind)
	}

	// Also verify Get alias method
	wAlias, err := client.Get(context.Background(), "Tokyo")
	if err != nil || wAlias.City != "Tokyo" {
		t.Errorf("Get alias failed: %v", err)
	}
}

func TestClient_GetWeather_WeatherCodes(t *testing.T) {
	tests := []struct {
		code      int
		wantCond  string
	}{
		{code: 0, wantCond: "Clear sky"},
		{code: 1, wantCond: "Mainly clear"},
		{code: 2, wantCond: "Partly cloudy"},
		{code: 3, wantCond: "Overcast"},
		{code: 45, wantCond: "Fog"},
		{code: 51, wantCond: "Light drizzle"},
		{code: 61, wantCond: "Slight rain"},
		{code: 65, wantCond: "Heavy rain"},
		{code: 71, wantCond: "Slight snow fall"},
		{code: 80, wantCond: "Slight rain showers"},
		{code: 95, wantCond: "Thunderstorm"},
		{code: 999, wantCond: "Unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.wantCond, func(t *testing.T) {
			cond := weather.WeatherCodeToText(tc.code)
			if cond != tc.wantCond {
				t.Errorf("code %d: got %q, want %q", tc.code, cond, tc.wantCond)
			}
		})
	}
}

func TestClient_GetWeather_CityNotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/geo", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{"results": []any{}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := weather.NewClient(
		weather.WithHTTPClient(srv.Client()),
		weather.WithBaseURLs(srv.URL+"/geo", srv.URL+"/forecast"),
	)

	_, err := client.GetWeather(context.Background(), "NoSuchCityExists123")
	if err == nil {
		t.Fatalf("expected error for nonexistent city, got nil")
	}
	if !errors.Is(err, weather.ErrCityNotFound) {
		t.Errorf("expected ErrCityNotFound, got %v", err)
	}
}

func TestClient_GetWeather_EmptyCity(t *testing.T) {
	client := weather.NewClient()

	_, err := client.GetWeather(context.Background(), "   ")
	if !errors.Is(err, weather.ErrEmptyCity) {
		t.Errorf("expected ErrEmptyCity, got %v", err)
	}
}

func TestClient_GetWeather_Errors(t *testing.T) {
	tests := []struct {
		name       string
		geoHandler http.HandlerFunc
		fcHandler  http.HandlerFunc
	}{
		{
			name: "geocoding 500 error",
			geoHandler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "server error", http.StatusInternalServerError)
			},
			fcHandler: func(w http.ResponseWriter, r *http.Request) {},
		},
		{
			name: "geocoding malformed JSON",
			geoHandler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("invalid json"))
			},
			fcHandler: func(w http.ResponseWriter, r *http.Request) {},
		},
		{
			name: "forecast 500 error",
			geoHandler: func(w http.ResponseWriter, r *http.Request) {
				resp := map[string]any{
					"results": []map[string]any{
						{"name": "London", "latitude": 51.5, "longitude": -0.1},
					},
				}
				_ = json.NewEncoder(w).Encode(resp)
			},
			fcHandler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "forecast failure", http.StatusInternalServerError)
			},
		},
		{
			name: "forecast malformed JSON",
			geoHandler: func(w http.ResponseWriter, r *http.Request) {
				resp := map[string]any{
					"results": []map[string]any{
						{"name": "London", "latitude": 51.5, "longitude": -0.1},
					},
				}
				_ = json.NewEncoder(w).Encode(resp)
			},
			fcHandler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("invalid json"))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/geo", tc.geoHandler)
			mux.HandleFunc("/forecast", tc.fcHandler)

			srv := httptest.NewServer(mux)
			defer srv.Close()

			client := weather.NewClient(
				weather.WithHTTPClient(srv.Client()),
				weather.WithBaseURLs(srv.URL+"/geo", srv.URL+"/forecast"),
			)

			_, err := client.GetWeather(context.Background(), "London")
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}

func TestClient_GetWeather_ContextCanceled(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/geo", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := weather.NewClient(
		weather.WithHTTPClient(srv.Client()),
		weather.WithBaseURLs(srv.URL+"/geo", srv.URL+"/forecast"),
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // canceled immediately

	_, err := client.GetWeather(ctx, "Paris")
	if err == nil {
		t.Fatal("expected error on canceled context, got nil")
	}
}
