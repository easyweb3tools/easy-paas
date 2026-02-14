package signal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"polymarket/internal/config"
	"polymarket/internal/models"
)

// WeatherAPICollector polls weather APIs (OpenWeather compatible) and emits "weather_deviation" signals.
// MVP: signals are city-scoped (no market_id); the WeatherStrategy maps city -> labeled markets.
type WeatherAPICollector struct {
	HTTP   *http.Client
	Logger *zap.Logger

	Cities  []string
	Sources []config.WeatherAPISource

	mu        sync.Mutex
	lastPoll  *time.Time
	lastError *string
	status    string
}

func (c *WeatherAPICollector) Name() string { return "weather_api" }

func (c *WeatherAPICollector) SourceInfo() SourceInfo {
	interval := c.pollInterval()
	endpoint := ""
	if len(c.Sources) > 0 {
		endpoint = c.Sources[0].Endpoint
	}
	return SourceInfo{
		SourceType:   "api_poll",
		Endpoint:     endpoint,
		PollInterval: interval,
	}
}

func (c *WeatherAPICollector) Start(ctx context.Context, out chan<- models.Signal) error {
	if c == nil {
		return nil
	}
	if c.HTTP == nil {
		c.HTTP = &http.Client{Timeout: 15 * time.Second}
	}
	interval := c.pollInterval()
	if interval <= 0 {
		interval = 5 * time.Minute
	}

	// Run immediately once.
	c.pollOnce(ctx, out)

	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			c.pollOnce(ctx, out)
		}
	}
}

func (c *WeatherAPICollector) Stop() error { return nil }

func (c *WeatherAPICollector) Health() HealthStatus {
	if c == nil {
		return HealthStatus{Status: "unknown"}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	status := c.status
	if strings.TrimSpace(status) == "" {
		status = "unknown"
	}
	return HealthStatus{
		Status:     status,
		LastPollAt: c.lastPoll,
		LastError:  c.lastError,
	}
}

func (c *WeatherAPICollector) pollInterval() time.Duration {
	interval := time.Duration(0)
	for _, s := range c.Sources {
		if s.PollInterval <= 0 {
			continue
		}
		if interval <= 0 || s.PollInterval < interval {
			interval = s.PollInterval
		}
	}
	return interval
}

func (c *WeatherAPICollector) pollOnce(ctx context.Context, out chan<- models.Signal) {
	now := time.Now().UTC()
	cities := cleanCityList(c.Cities)
	if len(cities) == 0 || len(c.Sources) == 0 {
		c.setHealth(now, "degraded", stringPtr("no cities or sources configured"))
		return
	}

	okCount := 0
	var lastErr error

	for _, city := range cities {
		temp, details, err := c.fetchWeightedForecastTempF(ctx, city)
		if err != nil {
			lastErr = err
			continue
		}
		okCount++
		payload := map[string]any{
			"city":            city,
			"forecast_temp_f": temp,
			"details":         details,
		}
		raw, _ := json.Marshal(payload)
		expires := now.Add(2 * c.pollInterval())
		sig := models.Signal{
			SignalType: "weather_deviation",
			Source:     "weather_api",
			Strength:   0.7,
			Direction:  "NEUTRAL",
			Payload:    raw,
			ExpiresAt:  &expires,
			CreatedAt:  now,
		}
		select {
		case out <- sig:
		default:
			// Hub handles backpressure via fanout; collector should avoid blocking.
		}
	}

	if okCount > 0 {
		c.setHealth(now, "healthy", nil)
		return
	}
	if lastErr != nil {
		c.setHealth(now, "down", stringPtr(lastErr.Error()))
		return
	}
	c.setHealth(now, "degraded", stringPtr("no successful weather fetch"))
}

func (c *WeatherAPICollector) setHealth(ts time.Time, status string, errStr *string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastPoll = &ts
	c.status = status
	c.lastError = errStr
}

func (c *WeatherAPICollector) fetchWeightedForecastTempF(ctx context.Context, city string) (float64, map[string]any, error) {
	type srcResult struct {
		name string
		temp float64
		w    float64
	}
	results := make([]srcResult, 0, len(c.Sources))
	details := map[string]any{
		"sources": []any{},
	}
	var errs []string
	for _, s := range c.Sources {
		key := ""
		if strings.TrimSpace(s.APIKeyEnv) != "" {
			key = strings.TrimSpace(os.Getenv(strings.TrimSpace(s.APIKeyEnv)))
		}
		temp, err := c.fetchOpenWeatherForecastTempF(ctx, s.Endpoint, key, city)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", s.Name, err))
			continue
		}
		w := s.Weight
		if w <= 0 {
			w = 1.0
		}
		results = append(results, srcResult{name: s.Name, temp: temp, w: w})
	}
	srcItems := make([]any, 0, len(results))
	sumW := 0.0
	sum := 0.0
	for _, r := range results {
		sumW += r.w
		sum += r.temp * r.w
		srcItems = append(srcItems, map[string]any{"name": r.name, "temp_f": r.temp, "weight": r.w})
	}
	details["sources"] = srcItems
	if len(errs) > 0 {
		details["errors"] = errs
	}
	if sumW <= 0 {
		return 0, details, fmt.Errorf("no successful sources: %s", strings.Join(errs, "; "))
	}
	return sum / sumW, details, nil
}

// OpenWeather compatible endpoint:
// - endpoint should point to /data/2.5/forecast or similar.
// - query params: q, appid, units=imperial.
func (c *WeatherAPICollector) fetchOpenWeatherForecastTempF(ctx context.Context, endpoint string, apiKey string, city string) (float64, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return 0, fmt.Errorf("empty endpoint")
	}
	qCity := strings.ReplaceAll(strings.TrimSpace(city), "-", " ")
	url := endpoint
	sep := "?"
	if strings.Contains(url, "?") {
		sep = "&"
	}
	url = url + sep + "q=" + urlQueryEscape(qCity) + "&units=imperial"
	if strings.TrimSpace(apiKey) != "" {
		url += "&appid=" + urlQueryEscape(apiKey)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("http %d", resp.StatusCode)
	}
	var parsed struct {
		List []struct {
			Main struct {
				Temp float64 `json:"temp"`
			} `json:"main"`
		} `json:"list"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return 0, err
	}
	if len(parsed.List) == 0 {
		return 0, fmt.Errorf("no forecast items")
	}
	// MVP: take the nearest forecast temp.
	return parsed.List[0].Main.Temp, nil
}

func cleanCityList(items []string) []string {
	out := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, raw := range items {
		val := strings.ToLower(strings.TrimSpace(raw))
		val = strings.ReplaceAll(val, "_", "-")
		if val == "" {
			continue
		}
		if _, ok := seen[val]; ok {
			continue
		}
		seen[val] = struct{}{}
		out = append(out, val)
	}
	return out
}

func urlQueryEscape(s string) string {
	// Minimal escaping without pulling net/url into hot collector loop.
	repl := strings.NewReplacer(" ", "%20", "+", "%2B", "#", "%23", "%", "%25", "?", "%3F", "&", "%26", "=", "%3D")
	return repl.Replace(s)
}

func stringPtr(s string) *string {
	v := strings.TrimSpace(s)
	if v == "" {
		return nil
	}
	return &v
}
