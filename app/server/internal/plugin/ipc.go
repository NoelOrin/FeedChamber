package plugin

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

// Frame 是 IPC 的一帧（NDJSON：一行一个 JSON 对象）。
//
// 请求与响应共用这个结构：有 Method 的是请求，有 Result/Error 且无 Method 的是响应。
type Frame struct {
	ID     int64           `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *FrameError     `json:"error,omitempty"`
}

// FrameError 是远端返回的错误。
type FrameError struct {
	Message string `json:"message"`
}

func (e *FrameError) Error() string { return e.Message }

// Handler 处理一条来自对端的请求。
type Handler func(ctx context.Context, params json.RawMessage) (any, error)

// Conn 是一条双向 JSON-RPC 连接（通常挂在子进程的 stdin/stdout 上）。
//
// 读写各占一个 goroutine：读循环只把响应塞回等待者、把请求交给 handler；
// 调用方用 Call 阻塞等结果。所有写操作串行化，避免半行交错的帧。
type Conn struct {
	r io.Reader
	w io.Writer

	mu       sync.Mutex
	writeMu  sync.Mutex
	handlers map[string]Handler
	pending  map[int64]chan Frame
	nextID   atomic.Int64

	closed  chan struct{}
	closeOn sync.Once
}

// NewConn 用一对读写流构造连接。子进程场景下 r 与 w 是同一个管道。
func NewConn(r io.Reader, w io.Writer) *Conn {
	return &Conn{
		r:        r,
		w:        w,
		handlers: make(map[string]Handler),
		pending:  make(map[int64]chan Frame),
		closed:   make(chan struct{}),
	}
}

// Handle 注册对端可调用的方法。
func (c *Conn) Handle(method string, fn Handler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers[method] = fn
}

// Start 启动读循环。
func (c *Conn) Start() error {
	go c.readLoop()
	return nil
}

// Close 关闭连接；在途调用会收到错误。
//
// 除了关闭 closed 信号，还关掉底层流：子进程卡死、不再读 stdin 时，
// 阻塞在 Write 上的调用方（见 Call）只有这样才能被解开。
func (c *Conn) Close() error {
	c.closeOn.Do(func() {
		close(c.closed)
		if closer, ok := c.w.(io.Closer); ok {
			_ = closer.Close()
		}
		if closer, ok := c.r.(io.Closer); ok {
			_ = closer.Close()
		}
	})
	return nil
}

// Call 发一条请求并等待响应。ctx 取消时返回 ctx.Err()。
func (c *Conn) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := c.nextID.Add(1)
	ch := make(chan Frame, 1)

	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	raw, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("encode params: %w", err)
	}

	// 写单独放一个 goroutine：对端若不读 stdin（子进程已崩溃但管道未关），
	// Write 会无限阻塞，直接写在调用路径上等于让 ctx 取消失效。
	writeErr := make(chan error, 1)
	go func() { writeErr <- c.write(Frame{ID: id, Method: method, Params: raw}) }()

	select {
	case err := <-writeErr:
		if err != nil {
			return nil, err
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.closed:
		return nil, fmt.Errorf("ipc closed")
	}

	select {
	case f := <-ch:
		if f.Error != nil {
			return nil, f.Error
		}
		return f.Result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.closed:
		return nil, fmt.Errorf("ipc closed")
	}
}

// Notify 发一条不需要响应的请求。
func (c *Conn) Notify(method string, params any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("encode params: %w", err)
	}
	return c.write(Frame{Method: method, Params: raw})
}

func (c *Conn) write(f Frame) error {
	data, err := json.Marshal(f)
	if err != nil {
		return fmt.Errorf("encode frame: %w", err)
	}
	data = append(data, '\n')
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err = c.w.Write(data)
	return err
}

func (c *Conn) readLoop() {
	sc := bufio.NewScanner(c.r)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		var f Frame
		if err := json.Unmarshal(sc.Bytes(), &f); err != nil {
			continue // 坏帧跳过，不让单个脏行拖垮连接
		}
		if f.Method == "" {
			// 响应
			c.mu.Lock()
			ch := c.pending[f.ID]
			c.mu.Unlock()
			if ch != nil {
				ch <- f
			}
			continue
		}
		// 每个入站请求单独一个 goroutine：handler 里回调对端是合法用法，
		// 读循环一旦阻塞在这里，那类回调就永远等不到自己的响应。
		go c.dispatch(f)
	}
	_ = c.Close()
}

func (c *Conn) dispatch(f Frame) {
	c.mu.Lock()
	fn, ok := c.handlers[f.Method]
	c.mu.Unlock()

	if !ok {
		_ = c.write(Frame{ID: f.ID, Error: &FrameError{Message: "unknown method: " + f.Method}})
		return
	}
	// 锁已在上面释放：handler 内回调对端不会死锁。
	res, err := fn(context.Background(), f.Params)
	if err != nil {
		_ = c.write(Frame{ID: f.ID, Error: &FrameError{Message: err.Error()}})
		return
	}
	raw, err := json.Marshal(res)
	if err != nil {
		_ = c.write(Frame{ID: f.ID, Error: &FrameError{Message: "encode result: " + err.Error()}})
		return
	}
	_ = c.write(Frame{ID: f.ID, Result: raw})
}
