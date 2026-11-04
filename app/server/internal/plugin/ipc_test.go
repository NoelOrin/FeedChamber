package plugin_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
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

// TestConnCallReturnsOnClose 钉住「连接已关闭」这条路径：Close 之后在途的
// Call 必须立刻返回 ErrClosed，而不是一直等到自己的 ctx 到期。
//
// 这条与 TestConnCallTimesOut 互补：那个测「对端还在但不回话」→ DeadlineExceeded，
// 这个测「连接已关闭」→ 立即 ErrClosed。两者都靠 Call 里的 select，缺一条
// 就说明那条分支写歪了。
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
		// 关闭后的错误身份不确定（Close 与在途写并发时可能先拿到 I/O 错误），
		// 所以这里只断言「不是靠 ctx 超时解开的」。见 ErrClosed 的文档。
		require.Error(t, err)
		require.NotErrorIs(t, err, context.DeadlineExceeded)
		if !errors.Is(err, plugin.ErrClosed) {
			// 不是 ErrClosed 就应该是底层管道错误，不能是别的什么。
			require.ErrorIs(t, err, net.ErrClosed)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close 未能解除在途 Call")
	}

	// 关闭之后新起的 Call 必须立刻拿到 ErrClosed，而不是挂起。
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := a.Call(ctx, "any.method", nil)
	require.ErrorIs(t, err, plugin.ErrClosed)
}

// TestConnSkipsMalformedLines 钉住「坏行跳过，不致命」：插件用 console.log
// 往同一个 stdout 打日志是常事，一行脏数据不能把整条连接拆掉。
func TestConnSkipsMalformedLines(t *testing.T) {
	// c1 由测试自己扮演对端；c2 交给被测的 Conn。
	c1, c2 := net.Pipe()
	conn := plugin.NewConn(c2, c2)
	require.NoError(t, conn.Start())
	t.Cleanup(func() {
		_ = conn.Close()
		_ = c1.Close()
	})

	conn.Handle("ping", func(ctx context.Context, params json.RawMessage) (any, error) {
		return "pong", nil
	})

	// 三条坏行（非 JSON、合法 JSON 但不是对象、空行）夹着一条合法请求。
	_, err := io.WriteString(c1,
		"this is not json\n"+
			`"just a string"`+"\n"+
			"\n"+
			"{\"id\":1,\"method\":\"ping\"}\n")
	require.NoError(t, err)

	// 合法请求必须仍被处理并回包：读循环没被前面的脏行带走。
	rd := bufio.NewReader(c1)
	line, err := rd.ReadString('\n')
	require.NoError(t, err, "坏行之后连接应仍然可用")

	var f plugin.Frame
	require.NoError(t, json.Unmarshal([]byte(line), &f))
	require.EqualValues(t, 1, f.ID)
	require.Nil(t, f.Error)
	require.JSONEq(t, `"pong"`, string(f.Result))
}

// TestConnDuplicateResponseDoesNotWedge 钉住重复响应的处理：同一个 id 的第二条
// 响应必须被丢弃，而不是把读循环永久堵死。
//
// 卡死的确切条件（读循环是单 goroutine，所以必须这样构造）：
//  1. 对端不读 stdin，于是 Call 的写一直阻塞在第一段 select 上，还没开始收响应；
//  2. 对端接连写两条同 id 的响应：第一条填满容量 1 的 chan，第二条阻塞在 `ch <- f`；
//  3. Call 的 ctx 到期返回，defer 删除 pending —— 但读循环手里已经握着那个 chan，
//     且没有任何接收者再会来取，第二条发送永远等不到空间。
//
// 症状是连接看起来还活着，之后每个 Call 都只会超时。任何写错或恶意的插件都能触发。
func TestConnDuplicateResponseDoesNotWedge(t *testing.T) {
	c1, c2 := net.Pipe()
	conn := plugin.NewConn(c2, c2)
	require.NoError(t, conn.Start())
	t.Cleanup(func() {
		_ = conn.Close()
		_ = c1.Close()
	})

	// 先算出一个 id：Call 从 1 开始分配，这里直接用 1。
	// 对端一开始**不读** stdin，制造「Call 卡在写」的状态。
	_, err := io.WriteString(c1,
		"{\"id\":1,\"result\":\"first\"}\n"+
			"{\"id\":1,\"result\":\"second\"}\n")
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, callErr := conn.Call(ctx, "ping", nil)
	// 写被阻塞，所以这里拿到的是 ctx 超时，而不是某条响应。
	require.ErrorIs(t, callErr, context.DeadlineExceeded)

	// 关键断言：读循环必须还活着。把剩下的输入读完（drain），再让对端正常
	// 服务一次调用；若第二条响应把读循环堵死了，这里会超时。
	go func() {
		rd := bufio.NewReader(c1)
		for {
			line, err := rd.ReadString('\n')
			if err != nil {
				return
			}
			var f plugin.Frame
			if json.Unmarshal([]byte(line), &f) != nil || f.Method == "" {
				continue
			}
			_, _ = fmt.Fprintf(c1, "{\"id\":%d,\"result\":\"still-here\"}\n", f.ID)
		}
	}()

	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	raw, err := conn.Call(ctx2, "after", nil)
	require.NoError(t, err, "读循环被重复响应卡死了")
	require.JSONEq(t, `"still-here"`, string(raw))
}

// TestConnWritesAreSerialized 钉住「写必须串行」：N 个 goroutine 并发 Notify，
// 对端读到的每一行都必须是完整合法的 JSON，且条数一条不差。
//
// 少了 writeMu 时两个 goroutine 会各写半行，对端读到的是拼接出的坏行——
// 而坏行在读侧被静默跳过，表现为「帧凭空消失」，所以只能这样测。
func TestConnWritesAreSerialized(t *testing.T) {
	pr, pw := io.Pipe()

	// 扮演对端：把每一行都解出来，解不出就说明写交错了。
	var (
		mu      sync.Mutex
		frames  []plugin.Frame
		garbage []string
		done    = make(chan struct{})
	)
	go func() {
		defer close(done)
		sc := bufio.NewScanner(pr)
		for sc.Scan() {
			var f plugin.Frame
			if err := json.Unmarshal(sc.Bytes(), &f); err != nil {
				mu.Lock()
				garbage = append(garbage, sc.Text())
				mu.Unlock()
				continue
			}
			mu.Lock()
			frames = append(frames, f)
			mu.Unlock()
		}
	}()

	conn := plugin.NewConn(bytes.NewReader(nil), pw)
	t.Cleanup(func() { _ = pw.Close() })

	const writers = 16
	const perWriter = 8

	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < perWriter; j++ {
				require.NoError(t, conn.Notify("evt", map[string]int{"w": i, "j": j}))
			}
		}(i)
	}
	wg.Wait()
	require.NoError(t, pw.Close())
	<-done

	mu.Lock()
	defer mu.Unlock()
	require.Empty(t, garbage, "写交错了：对端读到了拼在一起的坏行")
	require.Len(t, frames, writers*perWriter, "每一行都必须被完整读到")
	for _, f := range frames {
		require.Equal(t, "evt", f.Method)
	}
}
