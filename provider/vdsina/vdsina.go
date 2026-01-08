package vdsina

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/custom-app/NeverForgetVPS/provider"
)

const (
	vdsinaAPIURL = "https://userapi.vdsina.com/v1"
)

// VdsinaProvider implements the Provider interface for VDSina
type VdsinaProvider struct {
	apiKey string
	client *http.Client
}

// New creates a new instance of VdsinaProvider
// If apiKey is empty, the provider is considered not configured
func New(apiKey string) provider.Provider {
	if apiKey == "" {
		return nil
	}
	return &VdsinaProvider{
		apiKey: apiKey,
		client: &http.Client{Timeout: 40 * time.Second},
	}
}

// GetName returns the provider name
func (v *VdsinaProvider) GetName() string {
	return "vdsina"
}

// IsConfigured checks if the provider is configured
func (v *VdsinaProvider) IsConfigured() bool {
	return v != nil && v.apiKey != ""
}

// accountResponse represents the API response from VDSina for account information
type accountResponse struct {
	Status    string `json:"status"`
	StatusMsg string `json:"status_msg"`
	Data      struct {
		Account struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"account"`
		Created  string  `json:"created"`
		Forecast *string `json:"forecast"` // The shutdown forecast date (nullable)
		Can      struct {
			AddUser       bool `json:"add_user"`
			AddService    bool `json:"add_service"`
			ConvertToCash bool `json:"convert_to_cash"`
		} `json:"can"`
	} `json:"data"`
}

// GetNextPaymentDate retrieves the next payment due date from VDSina
// Returns the forecast date (shutdown forecast) from account information
func (v *VdsinaProvider) GetNextPaymentDate(ctx context.Context) (*time.Time, error) {
	accountInfo, err := v.fetchAccount(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch account: %w", err)
	}

	// If forecast is nil or empty, consider payment as overdue (return past date)
	if accountInfo.Data.Forecast == nil || *accountInfo.Data.Forecast == "" {
		pastDate := time.Now().AddDate(0, 0, -1) // Yesterday - overdue
		return &pastDate, nil
	}

	// Parse forecast date (format: "2029-02-20") in UTC
	forecastDate, err := time.Parse("2006-01-02", *accountInfo.Data.Forecast)
	if err != nil {
		return nil, fmt.Errorf("failed to parse forecast date: %w", err)
	}

	// Convert to UTC (at midnight UTC)
	forecastDateUTC := forecastDate.UTC()

	return &forecastDateUTC, nil
}

// makeRequest creates an HTTP request to VDSina API
// method - HTTP method (GET, POST, etc.)
// path - API path (e.g., "/account")
// queryParams - query parameters, can be nil
// body - request body for POST requests, can be nil
func (v *VdsinaProvider) makeRequest(ctx context.Context, method, path string, queryParams map[string]string, body io.Reader) (*http.Request, error) {
	// Build full URL
	fullURL := vdsinaAPIURL + path
	u, err := url.Parse(fullURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	// Add query parameters if present
	if len(queryParams) > 0 {
		q := u.Query()
		for key, value := range queryParams {
			q.Set(key, value)
		}
		u.RawQuery = q.Encode()
	}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+v.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	return req, nil
}

// executeRequest executes an HTTP request and returns the response body
// Returns the response body as bytes or an error if the request fails
func (v *VdsinaProvider) executeRequest(req *http.Request) ([]byte, error) {
	// Execute request
	resp, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// fetchAccount fetches account information from VDSina API
func (v *VdsinaProvider) fetchAccount(ctx context.Context) (*accountResponse, error) {
	// Create request to get account information
	req, err := v.makeRequest(ctx, "GET", "/account", nil, nil)
	if err != nil {
		return nil, err
	}

	// Execute request
	body, err := v.executeRequest(req)
	if err != nil {
		return nil, err
	}

	// Parse JSON
	var apiResponse accountResponse
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Check for API error
	if apiResponse.Status == "error" {
		return nil, fmt.Errorf("API error: %s", apiResponse.StatusMsg)
	}

	return &apiResponse, nil
}
