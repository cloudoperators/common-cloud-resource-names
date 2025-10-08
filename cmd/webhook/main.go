// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"

	"github.com/cloudoperators/common-cloud-resource-names/pkg/webhook"
)

func main() {
	// Define command line flags
	var (
		port      int
		certFile  string
		keyFile   string
		logLevel  string
		ccrnGroup string
	)

	flag.IntVar(&port, "port", 8443, "Port to listen on")
	flag.StringVar(&certFile, "cert-file", "/etc/webhook/certs/tls.crt", "Path to the TLS certificate file")
	flag.StringVar(&keyFile, "key-file", "/etc/webhook/certs/tls.key", "Path to the TLS key file")
	flag.StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	flag.StringVar(&ccrnGroup, "ccrn-group", "ccrn.example.com", "The CCRN CRD group used for all CCRN CRDs")
	flag.Parse()

	// Configure logger
	log := logrus.New()
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// Set log level
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		log.Warnf("Invalid log level %s, using info", logLevel)
		level = logrus.InfoLevel
	}
	log.SetLevel(level)

	// Create webhook server using the refactored structure
	// This maintains backward compatibility by using the Kubernetes backend
	server, err := webhook.NewWebhookServerFromConfig(log, ccrnGroup)
	if err != nil {
		log.Fatalf("Failed to create webhook server: %v", err)
	}

	// Set up signal handling for graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// Start the webhook server in a goroutine
	errCh := make(chan error)
	go func() {
		errCh <- server.Serve(port, certFile, keyFile)
	}()

	// Wait for shutdown signal or error
	select {
	case err := <-errCh:
		log.Fatalf("Webhook server failed: %v", err)
	case <-stop:
		log.Info("Received shutdown signal, exiting...")
	}
}
