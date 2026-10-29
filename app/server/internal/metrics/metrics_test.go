package metrics

import (
	"testing"
)

func TestMetricsRegistered(t *testing.T) {
	mfs, err := Registry.Gather()
	if err != nil {
		t.Fatalf("Registry.Gather() error: %v", err)
	}

	names := make(map[string]bool)
	for _, mf := range mfs {
		names[mf.GetName()] = true
	}

	expected := []string{
		"feedchamber_http_request_duration_seconds",
		"feedchamber_http_requests_total",
		"feedchamber_http_request_size_bytes",
		"feedchamber_http_response_size_bytes",
		"go_goroutines", // from GoCollector
	}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("expected metric %q not found in registry", name)
		}
	}
}
