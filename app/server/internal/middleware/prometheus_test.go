package middleware

import (
	"testing"
)

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Static paths pass through unchanged
		{"/api/v1/health", "/api/v1/health"},
		{"/api/v1/auth/login", "/api/v1/auth/login"},
		{"/api/v1/posts/list", "/api/v1/posts/list"},

		// Numeric IDs are collapsed
		{"/api/v1/users/123", "/api/v1/users/:id"},
		{"/api/v1/posts/456/detail", "/api/v1/posts/:id/detail"},

		// UUID-like hex IDs are collapsed
		{"/api/v1/users/abc123def456", "/api/v1/users/:id"},
		{"/api/v1/posts/a1b2c3d4e5f6/tags", "/api/v1/posts/:id/tags"},

		// Multiple ID segments
		{"/api/v1/users/123/posts/456", "/api/v1/users/:id/posts/:id"},

		// Short non-ID segments are preserved
		{"/api/v1/admin/stats", "/api/v1/admin/stats"},

		// Mixed
		{"/api/v1/oauth/providers", "/api/v1/oauth/providers"},
	}

	for _, tt := range tests {
		got := normalizePath(tt.input)
		if got != tt.expected {
			t.Errorf("normalizePath(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestIsLikelyID(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"123", false},
		{"99999", true},
		{"abc123def456", true},
		{"admin", false},
		{"stats", false},
		{"health", false},
		{"a1b2c3d4e5f6a7b8", true},
		{"", false},
	}

	for _, tt := range tests {
		got := isLikelyID(tt.input)
		if got != tt.expected {
			t.Errorf("isLikelyID(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}
