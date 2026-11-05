package plugin

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

// ErrClosed 表示连接已关闭（本地 Close 或对端断开）。
//
// 调用方应当用 errors.Is(err, ErrClosed) 判断，而不是匹配错误文案。
//
// 注意：**关闭后的错误身份是不确定的**。若 Close 与某次在途写并发，那次
// 写可能先把底层管道写失败（net.ErrClosed / io.ErrClosedPipe / EPIPE），
// Call 就返回那个 I/O 错误而不是 ErrClosed。两者都只意味着「连接没了」，
// 调用方不该依赖具体是哪一个——需要区分的是「有响应的失败」与「连接没了」，
// 而那应当由 ctx 的取消状态另行判断。
var ErrClosed = errors.New("ipc closed")

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
//
// # 流的所有权（调用方必读）
//
// **Conn 接管这两个流的所有权，并会在 Close 时主动关闭它们**
// （只要它们实现了 io.Closer——os/exec 的 StdinPipe/StdoutPipe 都实现了）。
// 这是刻意的：对端卡死、不再读 stdin 时，阻塞在 Write 上的 goroutine 只有
// 关掉底层流才能解开（见 Close）。
//
// 因此，**调用方 MUST NOT 在 Close 之后、或与 Close 并发地，再自行关闭同一对管道**。
// 与 os/exec 一起使用时这一点尤其重要：
//
//   - cmd.StdinPipe / cmd.StdoutPipe 返回的管道**由 cmd.Wait 负责关闭**；
//   - 若 Conn.Close 与 cmd.Wait 并发关闭同一条管道，属于数据竞争，在部分平台上
//     会让 Wait 返回一个伪错误（"file already closed" / EPIPE 一类）。
//     supervisor 若把它当插件崩溃，就会无端触发 1s→2s→4s 的重启阶梯。
//
// # supervisor 推荐的关停顺序
//
//  1. conn.Close()            —— 先让本地不再有在途读写（解开所有阻塞的 Write/Scan）
//  2. 结束子进程               —— SIGTERM，宽限期后 SIGKILL，或直接 Process.Kill()
//  3. cmd.Wait()              —— 回收进程；此时管道由 Wait 关闭，与 1 不冲突
//
// 也就是说：**永远不要自己关 StdoutPipe/StdinPipe**，交给 step 1 与 step 3
// 各关一次；两者之间隔着进程的结束，不会并发。
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

// Close 关闭连接；在途调用会返回 ErrClosed（或底层 I/O 错误）。
//
// 除了关闭 closed 信号，还关掉底层流：子进程卡死、不再读 stdin 时，阻塞在
// Write 上的调用方（见 Call、Notify、dispatch）只有这样才能被解开。
//
// 与 os/exec 共用管道时的关停顺序见 NewConn 的文档——不要与 cmd.Wait 并发。
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

	// 写单独放一个 goroutine：对端若不读 stdin（管道建立但没人消费），
	// Write 会一直阻塞，直接写在调用路径上等于让 ctx 取消失效。
	//
	// 注意这**不能保证**写 goroutine 一定会退出：它只在 Write 返回后才结束，
	// 而对端永久卡住且 w 不是 io.Closer 时，Write 永远不返回，这个 goroutine
	// 就被永久 pin 住。缓冲容量 1 只保证它发结果时不阻塞、不泄漏 channel。
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
		return nil, ErrClosed
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
		return nil, ErrClosed
	}
}

// Notify 发一条不需要响应的请求。
//
// **写是同步的**：它在调用方 goroutine 上直接获取 writeMu 并写下去，没有
// ctx、没有超时。对端停止消费 stdin 时，Notify 会一直阻塞在 writeMu/Write 上；
// 此时唯一能解开它的是 Close（关闭底层流）或杀掉对端进程——见 NewConn 的
// 关停顺序。supervisor 的超时→杀进程是最终兜底。
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
			// 响应。非阻塞投递：同一个 id 的第二条响应（写错的或恶意的插件）
			// 必须被丢弃。阻塞式 `ch <- f` 会在缓冲已满、而 pending 条目已被
			// Call 删除时把整个读循环永久卡住——连接看着还活着，之后每个
			// Call 都只会超时。
			c.mu.Lock()
			ch := c.pending[f.ID]
			c.mu.Unlock()
			if ch != nil {
				select {
				case ch <- f:
				default: // 重复响应，丢弃（也可能是对端乱填的 id）
				}
			}
			continue
		}
		// 每个入站请求单独一个 goroutine：handler 里回调对端是合法用法，
		// 读循环一旦阻塞在这里，那类回调就永远等不到自己的响应。
		go c.dispatch(f)
	}
	_ = c.Close()
}

// callHandlerSafely 调用 handler 并把 panic 转成 error。
//
// 这是宿主进程的存活边界，不是「防御性代码」：handler 在 dispatch
// goroutine 里同步执行，任何未捕获的 panic 都会终结进程。
func callHandlerSafely(fn Handler, params json.RawMessage) (res any, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("handler panic: %v", r)
		}
	}()
	return fn(context.Background(), params)
}

func (c *Conn) dispatch(f Frame) {
	c.mu.Lock()
	fn, ok := c.handlers[f.Method]
	c.mu.Unlock()

	// 下面的写都在本 goroutine 上同步进行（没有 ctx、没有超时）。对端不再
	// 消费 stdin 时会一起卡在 writeMu 上，只能靠 Close / 杀进程解开。
	if !ok {
		_ = c.write(Frame{ID: f.ID, Error: &FrameError{Message: "unknown method: " + f.Method}})
		return
	}
	// 锁已在上面释放：handler 内回调对端不会死锁。
	//
	// handler 必须带 panic 边界：它在 ipc 的 dispatch goroutine 里跑，
	// 未捕获的 panic 会直接终止整个宿主进程——一个插件的畸形参数就能
	// 打挂站点。这里 recover 后回错误帧，对端只会看到一次失败调用。
	res, err := callHandlerSafely(fn, f.Params)
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
