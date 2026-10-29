package handler

// Table-driven tests for normalizeBanRequest (banned.go), the pure business
// judgement behind ban creation: decide permanent vs temporary, parse the
// RFC3339 expiry, reject the past / malformed input, and fall back to a
// minute count with a lower bound of 1.
//
// Every timestamp here is built relative to time.Now() so the table never rots
// (a hard-coded future date would silently start failing the day it passes).

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeBanRequest(t *testing.T) {
	// RFC3339 is second-precision, so "now + d" can lose up to a second when
	// formatted. All assertions below tolerate that (ranges) except the
	// sub-minute clamp, which is robust by construction.
	expiryIn := func(d time.Duration) *string {
		s := time.Now().Add(d).Format(time.RFC3339)
		return &s
	}
	intPtr := func(v int) *int { return &v }

	tests := []struct {
		name    string
		req     banRequest
		wantErr string // substring; "" means no error expected
		want    func(t *testing.T, got *banRequest)
	}{
		{
			name: "all fields empty normalizes to permanent",
			req:  banRequest{Reason: "spam"},
			want: func(t *testing.T, got *banRequest) {
				t.Helper()
				if !got.IsPermanent {
					t.Fatalf("IsPermanent = false, want true")
				}
				if got.DurationMinutes != nil {
					t.Fatalf("DurationMinutes = %v, want nil", *got.DurationMinutes)
				}
			},
		},
		{
			name: "already permanent is returned untouched",
			req: banRequest{
				Reason:      "spam",
				IsPermanent: true,
			},
			want: func(t *testing.T, got *banRequest) {
				t.Helper()
				if !got.IsPermanent {
					t.Fatalf("IsPermanent = false, want true")
				}
				if got.DurationMinutes != nil {
					t.Fatalf("DurationMinutes = %v, want nil", *got.DurationMinutes)
				}
			},
		},
		{
			name: "permanent wins over a supplied duration",
			req: banRequest{
				Reason:          "spam",
				IsPermanent:     true,
				DurationMinutes: intPtr(30),
				ExpiresAt:       expiryIn(2 * time.Hour),
			},
			want: func(t *testing.T, got *banRequest) {
				t.Helper()
				if !got.IsPermanent {
					t.Fatalf("IsPermanent = false, want true")
				}
				if got.DurationMinutes == nil || *got.DurationMinutes != 30 {
					t.Fatalf("DurationMinutes = %v, want 30 (unchanged)", got.DurationMinutes)
				}
			},
		},
		{
			name: "positive duration is kept as-is",
			req: banRequest{
				Reason:          "spam",
				DurationMinutes: intPtr(60),
			},
			want: func(t *testing.T, got *banRequest) {
				t.Helper()
				if got.IsPermanent {
					t.Fatalf("IsPermanent = true, want false")
				}
				if got.DurationMinutes == nil || *got.DurationMinutes != 60 {
					t.Fatalf("DurationMinutes = %v, want 60", got.DurationMinutes)
				}
			},
		},
		{
			name: "positive duration wins over an expires_at",
			req: banRequest{
				Reason:          "spam",
				DurationMinutes: intPtr(15),
				ExpiresAt:       expiryIn(2 * time.Hour),
			},
			want: func(t *testing.T, got *banRequest) {
				t.Helper()
				if got.DurationMinutes == nil || *got.DurationMinutes != 15 {
					t.Fatalf("DurationMinutes = %v, want 15 (not overwritten)", got.DurationMinutes)
				}
			},
		},
		{
			name: "future expires_at is converted to minutes",
			req: banRequest{
				Reason:    "spam",
				ExpiresAt: expiryIn(2 * time.Hour),
			},
			want: func(t *testing.T, got *banRequest) {
				t.Helper()
				if got.IsPermanent {
					t.Fatalf("IsPermanent = true, want false")
				}
				if got.DurationMinutes == nil {
					t.Fatalf("DurationMinutes = nil, want ~120")
				}
				// 120m minus the sub-second dropped by RFC3339 formatting,
				// minus whatever elapsed during the test.
				if m := *got.DurationMinutes; m < 118 || m > 120 {
					t.Fatalf("DurationMinutes = %d, want 118..120", m)
				}
			},
		},
		{
			name: "past expires_at is rejected",
			req: banRequest{
				Reason:    "spam",
				ExpiresAt: expiryIn(-time.Hour),
			},
			wantErr: "晚于当前时间",
			want: func(t *testing.T, got *banRequest) {
				t.Helper()
				if got.DurationMinutes != nil {
					t.Fatalf("DurationMinutes = %v, want nil on error", *got.DurationMinutes)
				}
			},
		},
		{
			name: "malformed expires_at is rejected",
			req: banRequest{
				Reason:    "spam",
				ExpiresAt: strPtr("2026-01-02 15:04:05"),
			},
			wantErr: "RFC3339",
		},
		{
			name: "zero duration without expires_at is rejected",
			req: banRequest{
				Reason:          "spam",
				DurationMinutes: intPtr(0),
			},
			wantErr: "必须指定有效时长",
		},
		{
			name: "negative duration without expires_at is rejected",
			req: banRequest{
				Reason:          "spam",
				DurationMinutes: intPtr(-30),
			},
			wantErr: "必须指定有效时长",
		},
		{
			name: "expires_at under one minute is clamped to 1",
			req: banRequest{
				Reason:    "spam",
				ExpiresAt: expiryIn(30 * time.Second),
			},
			want: func(t *testing.T, got *banRequest) {
				t.Helper()
				if got.DurationMinutes == nil {
					t.Fatalf("DurationMinutes = nil, want 1")
				}
				if *got.DurationMinutes != 1 {
					t.Fatalf("DurationMinutes = %d, want 1 (clamped)", *got.DurationMinutes)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.req
			err := normalizeBanRequest(&req)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("normalizeBanRequest() = nil, want error containing %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("normalizeBanRequest() error = %q, want containing %q", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatalf("normalizeBanRequest() = %v, want nil", err)
			}

			if tt.want != nil {
				tt.want(t, &req)
			}
		})
	}
}

func strPtr(s string) *string { return &s }
