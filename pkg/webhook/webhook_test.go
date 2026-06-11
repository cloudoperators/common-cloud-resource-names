// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Greenhouse contributors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/cloudoperators/common-cloud-resource-names/pkg/apis"
)

// mockBackend implements apis.ValidationBackend for testing
type mockBackend struct {
	refreshErr error
}

func (m *mockBackend) GetCRD(ccrnVersion string) (*apis.CRDInfo, error) {
	return nil, nil
}

func (m *mockBackend) ValidateResource(namespace string, parsedCCRN *apis.ParsedResource) error {
	return nil
}

func (m *mockBackend) GetURNTemplate(ccrnName string, ccrnVersion string) (string, error) {
	return "", nil
}

func (m *mockBackend) Refresh() error {
	return m.refreshErr
}

func (m *mockBackend) IsResourceTypeSupported(ccrnVersion string) bool {
	return false
}

func TestHealthzReturns503BeforeReady(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	server, err := NewWebhookServer(log, &mockBackend{})
	if err != nil {
		t.Fatalf("failed to create webhook server: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	server.healthz(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503 before ready, got %d", w.Code)
	}
}

func TestHealthzReturns200AfterReady(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	server, err := NewWebhookServer(log, &mockBackend{})
	if err != nil {
		t.Fatalf("failed to create webhook server: %v", err)
	}

	// Mark server as ready
	server.SetReady()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	server.healthz(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 after ready, got %d", w.Code)
	}
}

func TestShutdownMethodExists(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	server, err := NewWebhookServer(log, &mockBackend{})
	if err != nil {
		t.Fatalf("failed to create webhook server: %v", err)
	}

	// Shutdown should be callable even without a running server
	// It should return nil or an error (not panic)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// When no http server has been started, Shutdown should return nil gracefully
	err = server.Shutdown(ctx)
	if err != nil {
		t.Errorf("expected nil error from Shutdown when server not started, got: %v", err)
	}
}

func TestShutdownStopsServer(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	server, err := NewWebhookServer(log, &mockBackend{})
	if err != nil {
		t.Fatalf("failed to create webhook server: %v", err)
	}

	// Bind to loopback on an ephemeral port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	// Start serving
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeHTTP(ln)
	}()

	// Give the server a moment to start accepting
	time.Sleep(50 * time.Millisecond)

	// Shutdown should succeed
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = server.Shutdown(ctx)
	if err != nil {
		t.Errorf("expected nil error from Shutdown, got: %v", err)
	}

	// Server should have returned from Serve
	select {
	case serveErr := <-errCh:
		if serveErr != nil && serveErr != http.ErrServerClosed {
			t.Errorf("expected ErrServerClosed or nil, got: %v", serveErr)
		}
	case <-time.After(5 * time.Second):
		t.Error("server did not shut down within timeout")
	}
}

func TestReadyFieldIsAtomicBool(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	server, err := NewWebhookServer(log, &mockBackend{})
	if err != nil {
		t.Fatalf("failed to create webhook server: %v", err)
	}

	// Test that concurrent access is safe (race detector will catch issues)
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			server.SetReady()
		}
		close(done)
	}()

	for i := 0; i < 1000; i++ {
		server.IsReady()
	}
	<-done
}

func TestInFlightRequestsCompleteDuringShutdown(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	server, err := NewWebhookServer(log, &mockBackend{})
	if err != nil {
		t.Fatalf("failed to create webhook server: %v", err)
	}
	server.SetReady()

	// Bind to loopback on an ephemeral port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	addr := ln.Addr().String()

	// Start serving
	go func() {
		_ = server.ServeHTTP(ln)
	}()

	// Give the server a moment to start accepting
	time.Sleep(50 * time.Millisecond)

	// Start a long-running request to /healthz (simulates in-flight request)
	var wg sync.WaitGroup
	wg.Add(1)
	requestCompleted := make(chan struct{})
	go func() {
		defer wg.Done()
		resp, err := http.Get(fmt.Sprintf("http://%s/healthz", addr))
		if err != nil {
			t.Errorf("in-flight request failed: %v", err)
			return
		}
		resp.Body.Close()
		close(requestCompleted)
	}()

	// Wait for the request to complete before shutting down
	// (healthz is fast, so it completes immediately)
	<-requestCompleted

	// Now start another request and initiate shutdown simultaneously
	wg.Add(1)
	go func() {
		defer wg.Done()
		// This request should complete because shutdown waits for in-flight requests
		resp, err := http.Get(fmt.Sprintf("http://%s/healthz", addr))
		if err != nil {
			// Connection may be refused after shutdown starts - this is acceptable
			return
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200 from in-flight request, got %d", resp.StatusCode)
		}
	}()

	// Initiate graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = server.Shutdown(ctx)
	if err != nil {
		t.Errorf("shutdown failed: %v", err)
	}

	wg.Wait()
}
