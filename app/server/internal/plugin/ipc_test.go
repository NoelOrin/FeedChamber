package plugin_test

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"testing"
	"time"

	"feedchamber/internal/plugin"

	"github.com/stretchr/testify/require"
)

// newPipePair 返回一对接好的 Conn，用于双向通信测试。
func newPipePair(t *testing.T) (*plugin.Conn, *plugin.Conn) {
	t.Helper()
	c1, c2 := net.Pipe()
	a := plugin.NewConn(c1, c1)
	b := plugin.NewConn(c2, c2)
	require.NoError(t, a.Start())
	require.NoError(t, b.Start())
	t.Cleanup(func() {
		_ = a.Close()
		_ = b.Close()
	})
	return a, b
}

func TestConnCallReturnsResult(t *testing.T) {
	a, b := newPipePair(t)

	b.Handle("hook.post.created", func(ctx context.Context, params json.RawMessage) (any, error) {
		var in struct {
			ID string `json:"id"`
		}
		require.NoError(t, json.Unmarshal(params, &in))
		return map[string]string{"seen": in.ID}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	raw, err := a.Call(ctx, "hook.post.created", map[string]string{"id": "p1"})
	require.NoError(t, err)

	var out struct {
		Seen string `json:"seen"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	require.Equal(t, "p1", out.Seen)
}

func TestConnCallPropagatesHandlerError(t *testing.T) {
	a, b := newPipePair(t)

	b.Handle("boom", func(ctx context.Context, params json.RawMessage) (any, error) {
		return nil, io.ErrUnexpectedEOF
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := a.Call(ctx, "boom", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unexpected EOF")
}

func TestConnCallTimesOut(t *testing.T) {
	// 对端注册一个永远不返回的 handler：Call 必须在 ctx 到期时返回
	// context.DeadlineExceeded，而不是一直等到对端回话。
	//
	// 这里刻意不用「对端不注册 handler」来制造超时：那样对端会立刻回一条
	// unknown method 错误帧，Call 拿到的是那个错误——测的是未知方法，
	// 不是 ctx 取消（后者由 TestConnUnknownMethodReturnsError 覆盖）。
	a, b := newPipePair(t)
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	b.Handle("nobody.home", func(ctx context.Context, params json.RawMessage) (any, error) {
		<-release
		return nil, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err := a.Call(ctx, "nobody.home", nil)
	require.Error(t, err)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestConnUnknownMethodReturnsError(t *testing.T) {
	a, _ := newPipePair(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := a.Call(ctx, "no.such.method", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown method")
}

func TestConnNotifyDoesNotWaitForResponse(t *testing.T) {
	a, b := newPipePair(t)

	got := make(chan struct{}, 1)
	b.Handle("event", func(ctx context.Context, params json.RawMessage) (any, error) {
		got <- struct{}{}
		return nil, nil
	})

	require.NoError(t, a.Notify("event", map[string]string{"k": "v"}))
	select {
	case <-got:
	case <-time.After(2 * time.Second):
		t.Fatal("notify 未被处理")
	}
}

// TestConnCallReturnsOnClose 钉住「对端彻底没了」这条路径：连接关闭后在途的
// Call 必须立刻拿到错误，而不是一直等到自己的 ctx 到期。
//
// 这条与 TestConnCallTimesOut 互补：那个测「对端还在但不回话」→ DeadlineExceeded，
// 这个测「对端已关闭」→ 立即返回。两者都靠 Call 里的 select，缺一条就说明
// 那条分支写歪了。
func TestConnCallReturnsOnClose(t *testing.T) {
	a, b := newPipePair(t)

	// 对端 handler 永不返回，保证 Call 一定停在等待分支上。
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	b.Handle("nobody.home", func(ctx context.Context, params json.RawMessage) (any, error) {
		<-release
		return nil, nil
	})

	errCh := make(chan error, 1)
	go func() {
		// ctx 给得足够长：若关闭不能解开 Call，这里会等到 10s 才返回。
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, err := a.Call(ctx, "nobody.home", nil)
		errCh <- err
	}()

	// 给 Call 一点时间把请求发出去并进入等待，再关连接。
	time.Sleep(100 * time.Millisecond)
	require.NoError(t, a.Close())

	select {
	case err := <-errCh:
		require.Error(t, err)
		require.NotErrorIs(t, err, context.DeadlineExceeded)
	case <-time.After(5 * time.Second):
		t.Fatal("Close 未能解除在途 Call")
	}
}
