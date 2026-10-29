package model

import "testing"

func TestNewUser_DefaultsDisplayNameToUsername(t *testing.T) {
	u := NewUser("alice", "alice@example.com")
	if u.DisplayName != "alice" {
		t.Fatalf("expected DisplayName to default to username %q, got %q", "alice", u.DisplayName)
	}
	if u.Username != "alice" {
		t.Fatalf("expected Username %q, got %q", "alice", u.Username)
	}
}
