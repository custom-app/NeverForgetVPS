package oneprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/custom-app/NeverForgetVPS/provider"
)

const (
	oneProviderAPIURL = "https://api.oneprovider.com"
	userAgent         = "OneApi/1.0"
)

// OneProvider implements the Provider interface for OneProvider
type OneProvider struct {
	apiKey    string
	clientKey string
	client    *http.Client
}

// New creates a new instance of OneProvider
// If apiKey or clientKey is empty, the provider is considered not configured
func New(apiKey, clientKey string) provider.Provider {
	if apiKey == "" || clientKey == "" {
		return nil
	}
	return &OneProvider{
		apiKey:    apiKey,
		clientKey: clientKey,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

// GetName returns the provider name
func (o *OneProvider) GetName() string {
	return "oneprovider"
}

// IsConfigured checks if the provider is configured
func (o *OneProvider) IsConfigured() bool {
	return o != nil && o.apiKey != "" && o.clientKey != ""
}

// invoiceResponse represents the API response from OneProvider for invoice list
type invoiceResponse struct {
	Result   string `json:"result"`
	Response struct {
		CurrentPage          int64     `json:"current_page"`
		TotalPages           int64     `json:"total_pages"`
		NumberOfEntries      int64     `json:"number_of_entries"`
		TotalNumberOfEntries int64     `json:"total_number_of_entries"`
		Invoices             []invoice `json:"invoices"`
	} `json:"response"`
	Error *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// invoice represents an invoice from OneProvider API
type invoice struct {
	ID           string        `json:"id"`
	Status       string        `json:"status"`
	CreationDate string        `json:"creation_date"`
	DueDate      string        `json:"due_date"`
	PaidDate     string        `json:"paid_date"`
	SubTotal     string        `json:"sub_total"`
	Tax          string        `json:"tax"`
	Tax2         string        `json:"tax2"`
	Credit       string        `json:"credit"`
	Balance      string        `json:"balance"`
	Items        []invoiceItem `json:"items"`
}

// invoiceItem represents an invoice item
type invoiceItem struct {
	ID             string `json:"id"`
	Type           string `json:"type"`
	RelationshipID string `json:"relationship_id"`
	Description    string `json:"description"`
	Amount         string `json:"amount"`
}

// GetNextPaymentDate retrieves the next payment due date from OneProvider
// Returns the earliest due date from unpaid invoices, or nil if there are no unpaid invoices
func (o *OneProvider) GetNextPaymentDate(ctx context.Context) (*time.Time, error) {
	page := 1
	limit := 20 // Number of invoices per page

	invoices, _, err := o.fetchinvoicesPage(ctx, page, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch invoices: %w", err)
	}

	// Find the earliest due date from unpaid invoices
	if len(invoices) == 0 {
		return nil, nil
	}

	var earliestDate *time.Time
	for _, invoice := range invoices {
		if invoice.Status == "Unpaid" && invoice.DueDate != "" {
			dueDate, err := time.Parse("2006-01-02", invoice.DueDate)
			if err != nil {
				return nil, fmt.Errorf("failed to parse due date: %w", err)
			}
			if earliestDate == nil || dueDate.Before(*earliestDate) {
				earliestDate = &dueDate
			}
		}
	}

	return earliestDate, nil
}

// makeRequest creates an HTTP request to OneProvider API
// method - HTTP method (GET, POST, etc.)
// path - API path (e.g., "/invoices")
// queryParams - query parameters, can be nil
// body - request body for POST requests, can be nil
func (o *OneProvider) makeRequest(ctx context.Context, method, path string, queryParams map[string]string, body io.Reader) (*http.Request, error) {
	// Build full URL
	fullURL := oneProviderAPIURL + path
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
	req.Header.Set("Api-Key", o.apiKey)
	req.Header.Set("Client-Key", o.clientKey)
	req.Header.Set("User-Agent", userAgent)

	return req, nil
}

// executeRequest executes an HTTP request and returns the response body
// Returns the response body as bytes or an error if the request fails
func (o *OneProvider) executeRequest(req *http.Request) ([]byte, error) {
	// Execute request
	resp, err := o.client.Do(req)
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

// fetchinvoicesPage fetches one page of invoices
func (o *OneProvider) fetchinvoicesPage(ctx context.Context, page, limit int) ([]invoice, int, error) {
	// Build query parameters
	queryParams := map[string]string{
		"status": "Unpaid",
		"page":   strconv.Itoa(page),
		"limit":  strconv.Itoa(limit),
	}

	// Create request
	req, err := o.makeRequest(ctx, "GET", "/invoices", queryParams, nil)
	if err != nil {
		return nil, 0, err
	}

	// Execute request
	resp, err := o.executeRequest(req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to execute request: %w", err)
	}

	// Parse JSON
	var apiResponse invoiceResponse
	if err := json.Unmarshal(resp, &apiResponse); err != nil {
		return nil, 0, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Check for API error
	if apiResponse.Error != nil {
		return nil, 0, fmt.Errorf("API error: %s (code: %s)", apiResponse.Error.Message, apiResponse.Error.Code)
	}

	return apiResponse.Response.Invoices, int(apiResponse.Response.TotalPages), nil
}
