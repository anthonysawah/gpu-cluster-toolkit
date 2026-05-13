package probe

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// echoHandler is an HTTP handler that reads the request body and writes it back.
// It's the simplest possible echo server for testing probes.
func echoHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, r.Body)
}

func TestProbeOnce_Succeeds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(echoHandler))
	defer server.Close()

	payload := MakePayload(1024)
	duration, err := ProbeOnce(context.Background(), server.URL, payload)
	if err != nil {
		t.Fatalf("ProbeOnce returned error: %v", err)
	}
	if duration <= 0 {
		t.Errorf("expected positive duration, got %v", duration)
	}
}

func TestProbeOnce_RespectsContext(t *testing.T) {
	// A handler that sleeps longer than our context allows.
	slow := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewServer(slow)
	defer server.Close()

	// Context that cancels after 50ms.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := ProbeOnce(ctx, server.URL, MakePayload(64))
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
}

func TestProbeOnce_BadStatus(t *testing.T) {
	bad := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	server := httptest.NewServer(bad)
	defer server.Close()

	_, err := ProbeOnce(context.Background(), server.URL, MakePayload(64))
	if err == nil {
		t.Fatal("expected error for 500 status, got nil")
	}
}

func TestMakePayload(t *testing.T) {
	p := MakePayload(100)
	if len(p) != 100 {
		t.Errorf("expected length 100, got %d", len(p))
	}
}