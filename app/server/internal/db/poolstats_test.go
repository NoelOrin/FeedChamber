package db

import (
	"testing"
)

func TestPoolStatsInterface(t *testing.T) {
	var _ Store = (*mockPoolStatsStore)(nil)
}

type mockPoolStatsStore struct {
	poolStats *PoolStats
}

func (m *mockPoolStatsStore) AutoMigrate(models ...any) error { return nil }
func (m *mockPoolStatsStore) SQL() SQLStore                   { return nil }
func (m *mockPoolStatsStore) Type() BackendType               { return BackendSQLite }
func (m *mockPoolStatsStore) Close() error                    { return nil }
func (m *mockPoolStatsStore) PoolStats() *PoolStats           { return m.poolStats }

func TestMockPoolStatsReturns(t *testing.T) {
	expected := &PoolStats{MaxOpenConnections: 25, OpenConnections: 10}
	mock := &mockPoolStatsStore{poolStats: expected}
	got := mock.PoolStats()
	if got != expected {
		t.Errorf("PoolStats() = %v, want %v", got, expected)
	}
}

func TestNilPoolStats(t *testing.T) {
	mock := &mockPoolStatsStore{poolStats: nil}
	got := mock.PoolStats()
	if got != nil {
		t.Errorf("expected nil PoolStats, got %v", got)
	}
}
