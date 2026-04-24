package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type WeatherTool struct {
	Tool
}

type WeatherInput struct {
	City string `json:"city" jsonschema:"the city to get weather for"`
}

type geocodingResponse struct {
	Results []struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"results"`
}

type weatherResponse struct {
	CurrentWeather struct {
		Temperature float64 `json:"temperature"`
		Windspeed   float64 `json:"windspeed"`
		WeatherCode int     `json:"weathercode"`
	} `json:"current_weather"`
}

func (t *WeatherTool) ToolInput() any {
	return WeatherInput{}
}

func (t *WeatherTool) ToolHandlerFn() func(ctx context.Context, req *mcp.CallToolRequest, in any) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, in any) (*mcp.CallToolResult, any, error) {
		input, ok := in.(WeatherInput)
		if !ok {
			return nil, nil, fmt.Errorf("invalid input type for get_weather")
		}
		result, err := getWeather(ctx, input.City)
		if err != nil {
			return nil, nil, err
		}
		return textResult(result), nil, nil
	}
}

func (t *WeatherTool) GetTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "get_weather",
		Description: "Get the current weather for a city.",
	}
}

func getWeather(ctx context.Context, city string) (string, error) {
	if city == "" {
		return "", fmt.Errorf("city argument required")
	}

	client := &http.Client{Timeout: 15 * time.Second}
	geoURL := fmt.Sprintf(
		"https://geocoding-api.open-meteo.com/v1/search?name=%s&count=1",
		url.QueryEscape(city),
	)
	var geoData geocodingResponse
	if err := getJSON(ctx, client, geoURL, &geoData); err != nil {
		return "", err
	}
	if len(geoData.Results) == 0 {
		return "", fmt.Errorf("city not found: %s", city)
	}

	lat := geoData.Results[0].Latitude
	lon := geoData.Results[0].Longitude
	weatherURL := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%f&longitude=%f&current_weather=true",
		lat,
		lon,
	)
	var weatherData weatherResponse
	if err := getJSON(ctx, client, weatherURL, &weatherData); err != nil {
		return "", err
	}

	out, err := json.Marshal(map[string]any{
		"city":          city,
		"temperature_c": weatherData.CurrentWeather.Temperature,
		"windspeed_kmh": weatherData.CurrentWeather.Windspeed,
		"weather_code":  weatherData.CurrentWeather.WeatherCode,
		"latitude":      lat,
		"longitude":     lon,
		"source":        "open-meteo",
	})
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func getJSON(ctx context.Context, client *http.Client, endpoint string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("request failed status=%d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
