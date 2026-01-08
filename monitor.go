package neverforgetvps

import (
	"context"
	"fmt"
	"time"

	"github.com/custom-app/NeverForgetVPS/provider"
	"github.com/custom-app/NeverForgetVPS/provider/oneprovider"
	"github.com/custom-app/NeverForgetVPS/provider/vdsina"
)

const (
	// DefaultCheckInterval is the default interval for checking payment dates
	DefaultCheckInterval = 12 * time.Hour
)

// VPSMonitor represents the main monitor for VPS providers
// T is the type of messages sent to the channel
type VPSMonitor[T any] struct {
	// Providers are optional - if nil, they are not configured
	Vdsina      provider.Provider
	OneProvider provider.Provider

	ctx              context.Context
	cancel           context.CancelFunc
	checkInterval    time.Duration
	messageChan      chan T         // Channel for sending messages to Telegram
	messageConverter func(string) T // Function to convert text string to message type T
}

// Config contains configuration for Monitor initialization
type Config struct {
	VdsinaAPIKey         string        // API key for VDSina (optional)
	OneProviderAPIKey    string        // API key for OneProvider (optional)
	OneProviderClientKey string        // Client key for OneProvider (optional)
	CheckInterval        time.Duration // Interval for checking payment dates (optional, default: 1 hour)
}

// NewVPSMonitor creates a new instance of VPSMonitor
// Providers are created only if corresponding API keys are provided
// Call Start() to begin periodic payment date checking
// messageChan is required - panic if nil
// messageConverter is a function that converts text string to message type T
// T is the type of messages (e.g., domain.MessageToSend, string, etc.)
func NewVPSMonitor[T any](ctx context.Context, config Config, messageChan chan T, messageConverter func(string) T) *VPSMonitor[T] {
	m := &VPSMonitor[T]{}

	if messageChan == nil {
		panic("messageChan is required")
	}

	if messageConverter == nil {
		panic("messageConverter is required")
	}

	if (config.OneProviderAPIKey == "" || config.OneProviderClientKey == "") && config.VdsinaAPIKey == "" {
		panic("OneProviderAPIKey and OneProviderClientKey or VdsinaAPIKey are required")
	}

	// Initialize providers only if credentials are provided
	if config.VdsinaAPIKey != "" {
		m.Vdsina = vdsina.New(config.VdsinaAPIKey)
	}

	if config.OneProviderAPIKey != "" && config.OneProviderClientKey != "" {
		m.OneProvider = oneprovider.New(config.OneProviderAPIKey, config.OneProviderClientKey)
	}

	// Set check interval (default: 12 hours)
	checkInterval := config.CheckInterval
	if checkInterval == 0 {
		checkInterval = DefaultCheckInterval
	}
	m.checkInterval = checkInterval

	// Set message channel and converter function
	m.messageChan = messageChan
	m.messageConverter = messageConverter

	// Create cancel context from provided context
	m.ctx, m.cancel = context.WithCancel(ctx)

	return m
}

// runPaymentDateCheck runs periodic checks of provider payment dates
func (m *VPSMonitor[T]) runPaymentDateCheck(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Perform initial check immediately
	m.checkPaymentDates()

	// Then check periodically
	for {
		select {
		case <-ticker.C:
			m.checkPaymentDates()
		case <-m.ctx.Done():
			return
		}
	}
}

// checkPaymentDates checks payment dates for all configured providers
func (m *VPSMonitor[T]) checkPaymentDates() {
	providers := []provider.Provider{}
	timeouts := []time.Duration{}
	if m.Vdsina != nil && m.Vdsina.IsConfigured() {
		providers = append(providers, m.Vdsina)
		timeouts = append(timeouts, 40*time.Second)
	}
	if m.OneProvider != nil && m.OneProvider.IsConfigured() {
		providers = append(providers, m.OneProvider)
		timeouts = append(timeouts, 30*time.Second)
	}

	for i, p := range providers {
		ctx, cancel := context.WithTimeout(m.ctx, timeouts[i])
		defer cancel()

		nextDate, err := p.GetNextPaymentDate(ctx)
		if err != nil {
			if m.messageChan != nil {
				m.sendMessage(fmt.Sprintf("Error checking payment date for provider %s: %v", p.GetName(), err))
			}
			continue
		}

		if nextDate != nil {
			message := m.formatPaymentMessage(p.GetName(), *nextDate)
			// Send notification via Telegram channel if configured
			m.sendMessage(message)

		} else {
			m.sendMessage(fmt.Sprintf("Provider %s: no payment due", p.GetName()))

		}
	}
}

// sendMessage sends a message to the channel using the converter function
func (m *VPSMonitor[T]) sendMessage(text string) {
	if m.messageChan == nil || m.messageConverter == nil {
		return
	}

	// Convert text to message type T using the converter function
	msg := m.messageConverter(text)

	// Send the message to channel
	select {
	case m.messageChan <- msg:
	default:
		// Channel is full, skip sending
	}
}

// formatPaymentMessage formats a payment notification message based on days until payment
func (m *VPSMonitor[T]) formatPaymentMessage(providerName string, paymentDate time.Time) string {
	now := time.Now().UTC()
	daysUntil := int(paymentDate.Sub(now).Hours() / 24)

	dateStr := paymentDate.Format("2006-01-02")

	switch {
	case daysUntil < 0:
		// Payment overdue - critical situation
		return fmt.Sprintf("🚨🚨🚨 CRITICAL: Provider %s - Payment overdue! Payment date was %s (%d days ago). Urgent action required!", providerName, dateStr, -daysUntil)
	case daysUntil <= 2:
		// 0-2 days left - urgent warning
		return fmt.Sprintf("🚨 WARNING: Provider %s - Urgent payment required! Payment due date: %s (%d day(s) left)", providerName, dateStr, daysUntil)
	case daysUntil <= 5:
		// 3-5 days left - attention
		return fmt.Sprintf("⚠️ ATTENTION: Provider %s - Payment due soon! Payment date: %s (%d days left)", providerName, dateStr, daysUntil)
	default:
		// More than 5 days left - informational
		return fmt.Sprintf("ℹ️ INFO: Provider %s - Next payment date: %s (%d days left)", providerName, dateStr, daysUntil)
	}
}

// Stop stops the monitoring goroutine
func (m *VPSMonitor[T]) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
}

// Start starts VPS monitoring
// Starts a goroutine for periodic payment date checking
func (m *VPSMonitor[T]) Start() error {
	// Start periodic checking goroutine
	go m.runPaymentDateCheck(m.checkInterval)
	return nil
}
