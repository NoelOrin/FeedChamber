package pkg

import "testing"

// (page-1)*pageSize 在 page 取极大值时整型溢出为负数，
// PostgreSQL 会直接报 "OFFSET must not be negative"（500 而不是空列表）。
func TestOffsetNeverNegative(t *testing.T) {
	cases := []struct {
		page, pageSize int
		wantNegative   bool
	}{
		{1, 20, false},
		{2, 20, false},
		{0, 20, false},  // page<1 → 0
		{-5, 20, false}, // 负数 page → 0
		{1, 0, false},   // pageSize<1 → 0
		{1 << 40, 1000, false},
		{1<<62 - 1, 1000, false},
	}
	for _, tc := range cases {
		got := Offset(tc.page, tc.pageSize)
		if got < 0 {
			t.Fatalf("Offset(%d,%d) = %d，不得为负", tc.page, tc.pageSize, got)
		}
	}
}

func TestOffsetBasics(t *testing.T) {
	if got := Offset(3, 20); got != 40 {
		t.Fatalf("Offset(3,20) = %d, want 40", got)
	}
	if got := Offset(1, 20); got != 0 {
		t.Fatalf("Offset(1,20) = %d, want 0", got)
	}
}
