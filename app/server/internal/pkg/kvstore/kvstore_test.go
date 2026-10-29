package kvstore_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"orinforum/internal/pkg/kvstore"
	"orinforum/internal/pkg/testnats"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStore 是所有 Store 实现都要通过的通用测试套件。
// 使用表驱动子测试，每个实现都能复用。
func TestStore(t *testing.T) {
	url := testnats.Start(t)
	setup := func(t *testing.T) kvstore.Store {
		bucket := strings.ReplaceAll(t.Name(), "/", "_")
		store, err := kvstore.New(kvstore.Config{
			NATSURL:    url,
			NATSBucket: bucket,
		})
		require.NoError(t, err)
		t.Cleanup(func() { store.Close() })
		return store
	}

	ctx := context.Background()

	t.Run("Get_Set_Delete", func(t *testing.T) {
		s := setup(t)

		// 不存在的 key
		_, err := s.Get(ctx, "missing")
		assert.ErrorIs(t, err, kvstore.ErrKeyNotFound)

		// Set + Get
		require.NoError(t, s.Set(ctx, "k1", "v1", 0))
		val, err := s.Get(ctx, "k1")
		assert.NoError(t, err)
		assert.Equal(t, "v1", val)

		// Delete
		require.NoError(t, s.Delete(ctx, "k1"))
		_, err = s.Get(ctx, "k1")
		assert.ErrorIs(t, err, kvstore.ErrKeyNotFound)
	})

	t.Run("Set_with_TTL", func(t *testing.T) {
		s := setup(t)

		require.NoError(t, s.Set(ctx, "ttl-key", "val", 1*time.Second))
		val, err := s.Get(ctx, "ttl-key")
		assert.NoError(t, err)
		assert.Equal(t, "val", val)

		// Poll until key expires, up to 3 seconds
		deadline := time.Now().Add(3 * time.Second)
		var getErr error
		for time.Now().Before(deadline) {
			time.Sleep(100 * time.Millisecond)
			_, getErr = s.Get(ctx, "ttl-key")
			if errors.Is(getErr, kvstore.ErrKeyNotFound) {
				break
			}
		}
		assert.True(t, errors.Is(getErr, kvstore.ErrKeyNotFound),
			"expected ErrKeyNotFound after TTL expired, got: %v", getErr)
	})

	t.Run("SetNX", func(t *testing.T) {
		s := setup(t)

		ok, err := s.SetNX(ctx, "nx", "first", 0)
		require.NoError(t, err)
		assert.True(t, ok)

		// 已存在时应返回 false
		ok, err = s.SetNX(ctx, "nx", "second", 0)
		require.NoError(t, err)
		assert.False(t, ok)

		// 值应为第一次设置的
		val, _ := s.Get(ctx, "nx")
		assert.Equal(t, "first", val)
	})

	t.Run("Exists", func(t *testing.T) {
		s := setup(t)

		exists, err := s.Exists(ctx, "nope")
		assert.NoError(t, err)
		assert.False(t, exists)

		require.NoError(t, s.Set(ctx, "exists", "1", 0))
		exists, err = s.Exists(ctx, "exists")
		assert.NoError(t, err)
		assert.True(t, exists)

		// TTL expiry: poll until expired, up to 3 seconds
		require.NoError(t, s.Set(ctx, "exp", "1", 1*time.Second))
		deadline := time.Now().Add(3 * time.Second)
		var expired bool
		for time.Now().Before(deadline) {
			time.Sleep(100 * time.Millisecond)
			exists, err = s.Exists(ctx, "exp")
			if !exists && err == nil {
				expired = true
				break
			}
		}
		assert.True(t, expired, "expected key to expire within 3s")
		assert.NoError(t, err)
	})

	t.Run("PrefixScan", func(t *testing.T) {
		s := setup(t)

		require.NoError(t, s.Set(ctx, "vc:post1", "10", 0))
		require.NoError(t, s.Set(ctx, "vc:post2", "20", 0))
		require.NoError(t, s.Set(ctx, "other:key", "30", 0))

		result, err := s.PrefixScan(ctx, "vc:")
		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, "10", result["vc:post1"])
		assert.Equal(t, "20", result["vc:post2"])
	})

	t.Run("DeletePrefix", func(t *testing.T) {
		s := setup(t)

		require.NoError(t, s.Set(ctx, "bl:a", "1", 0))
		require.NoError(t, s.Set(ctx, "bl:b", "1", 0))
		require.NoError(t, s.Set(ctx, "other", "1", 0))

		err := s.DeletePrefix(ctx, "bl:")
		assert.NoError(t, err)

		_, err = s.Get(ctx, "bl:a")
		assert.ErrorIs(t, err, kvstore.ErrKeyNotFound)
		_, err = s.Get(ctx, "bl:b")
		assert.ErrorIs(t, err, kvstore.ErrKeyNotFound)

		val, err := s.Get(ctx, "other")
		assert.NoError(t, err)
		assert.Equal(t, "1", val)
	})

	t.Run("Incr_and_IncrBy", func(t *testing.T) {
		s := setup(t)

		n, err := s.Incr(ctx, "counter")
		assert.NoError(t, err)
		assert.Equal(t, int64(1), n)

		n, err = s.Incr(ctx, "counter")
		assert.NoError(t, err)
		assert.Equal(t, int64(2), n)

		n, err = s.IncrBy(ctx, "counter", 10)
		assert.NoError(t, err)
		assert.Equal(t, int64(12), n)

		n, err = s.IncrBy(ctx, "counter", -5)
		assert.NoError(t, err)
		assert.Equal(t, int64(7), n)
	})

	t.Run("PubSub", func(t *testing.T) {
		s := setup(t)

		ctx2, cancel := context.WithCancel(ctx)
		defer cancel()

		ch, err := s.Subscribe(ctx2, "test-channel")
		require.NoError(t, err)

		require.NoError(t, s.Publish(ctx, "test-channel", []byte("hello")))
		require.NoError(t, s.Publish(ctx, "test-channel", []byte("world")))

		select {
		case msg := <-ch:
			assert.Equal(t, "hello", string(msg))
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for first message")
		}

		select {
		case msg := <-ch:
			assert.Equal(t, "world", string(msg))
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for second message")
		}

		// Cancel subscription and wait for channel close (with timeout)
		cancel()
		closedOk := false
		deadline := time.After(2 * time.Second)
		for {
			select {
			case _, open := <-ch:
				if !open {
					closedOk = true
				}
				// keep looping to drain buffered items
			case <-deadline:
				t.Fatal("timeout: channel not closed after cancel")
			}
			if closedOk {
				break
			}
		}
	})
	t.Run("KVChangeNotification", func(t *testing.T) {
		s := setup(t)
		ctx2, cancel := context.WithCancel(ctx)
		defer cancel()

		ch, err := s.Subscribe(ctx2, kvstore.KVChangeChannel())
		require.NoError(t, err)

		// Set triggers insert/update
		require.NoError(t, s.Set(ctx, "chg:1", "hello", 0))

		select {
		case msg := <-ch:
			assert.Contains(t, string(msg), "chg:1")
		case <-time.After(2 * time.Second):
			t.Fatal("timeout")
		}

		// Delete triggers delete
		require.NoError(t, s.Delete(ctx, "chg:1"))

		select {
		case msg := <-ch:
			assert.Contains(t, string(msg), "chg:1")
			assert.Contains(t, string(msg), "delete")
		case <-time.After(2 * time.Second):
			t.Fatal("timeout")
		}
	})
}
