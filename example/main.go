package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	neverforgetvps "github.com/custom-app/NeverForgetVPS"
)

type MessageToSend struct {
	Receiver int64
	Text     string
	Mode     string
}

func main() {
	// Create context for cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create message channel (using string for simplicity, can be chan domain.MessageToSend)
	messageChan := make(chan MessageToSend, 100)

	// Start goroutine to read messages from channel
	go func() {
		for msg := range messageChan {
			fmt.Printf("Received message: %s\n", msg.Text)
		}
	}()

	// Create VPSMonitor configuration
	config := neverforgetvps.Config{
		VdsinaAPIKey:         os.Getenv("VDSINA_API_KEY"),         // Set via environment variable
		OneProviderAPIKey:    os.Getenv("ONEPROVIDER_API_KEY"),    // Set via environment variable
		OneProviderClientKey: os.Getenv("ONEPROVIDER_CLIENT_KEY"), // Set via environment variable
		CheckInterval:        1 * time.Minute,                     // Check every hour
	}

	// Create VPSMonitor instance with your typed channel and converter function
	monitor := neverforgetvps.NewVPSMonitor(ctx, config, messageChan, func(text string) MessageToSend {
		return MessageToSend{
			Text: text,
		}
	})

	// Start monitoring
	if err := monitor.Start(); err != nil {
		fmt.Printf("Failed to start monitor: %v\n", err)
		return
	}

	fmt.Println("VPSMonitor started. Press Ctrl+C to stop...")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nStopping monitor...")
	monitor.Stop()
	fmt.Println("Monitor stopped")
}
