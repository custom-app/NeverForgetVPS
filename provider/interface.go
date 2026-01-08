package provider

import (
	"context"
	"time"
)

// Provider defines the interface for working with VPS providers
type Provider interface {
	// GetName returns the provider name
	GetName() string

	// GetNextPaymentDate retrieves the next payment due date from the provider
	// Returns the next payment date or nil if there's no payment due, and an error if something went wrong
	GetNextPaymentDate(ctx context.Context) (*time.Time, error)

	// IsConfigured checks if the provider is configured (credentials provided)
	IsConfigured() bool
}
