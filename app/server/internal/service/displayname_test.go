package service

import "testing"

func TestValidateDisplayName(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"ok ascii", "alice", false},
		{"ok chinese", "爱丽丝", false},
		{"ok emoji", "alice🚀", false},
		{"empty", "", true},
		{"space inside", "al ice", true},
		{"leading space", " alice", true},
		{"trailing space", "alice ", true},
		{"tab", "al\tice", true},
		{"too long 33", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", true}, // 33 'a'
		{"max 32", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", false},      // 32 'a'
	}
	for _, c := range cases {
		err := validateDisplayName(c.input)
		if c.wantErr && err == nil {
			t.Errorf("%s: expected error, got nil", c.name)
		}
		if !c.wantErr && err != nil {
			t.Errorf("%s: expected no error, got %v", c.name, err)
		}
	}
}
