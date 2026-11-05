package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

// envWhitelist 是允许下发给插件子进程的宿主变量。
//
// 这是本方案唯一可执行的数据访问防线：插件与后端同用户，能读文件系统、
// 也能自己连数据库，但至少不该从 process.env 里直接拿到 DB_DSN。
// 因此启动子进程时 Env 必须由白名单重建（见 buildPluginEnv），
// 绝不能 os.Environ() 起底再删几个——那是黑名单，漏一个就全漏。
//
// 加变量前必须自问：它会不会带上数据库连接串、密码、密钥、OAuth 凭证？
// TestEnvWhitelistHasNoSensitiveNames 会拦住 * 与 _SECRET/_KEY 形态的误加。
var envWhitelist = []string{
	"PATH",
	"NODE_ENV",
	"NODE_OPTIONS",
	"TZ",
	"LANG",
}

// buildPluginEnv 按白名单重建子进程环境，并追加插件专属变量。
//
// 白名单里名字存在但宿主未设置（或设为空值）时跳过，不写 "K="：
// 空值对 node 是「有效值」，一个空的 PATH 会让子进程连 node 自己都找不到。
//
// extra 的键排序后追加，输出不受 map 迭代随机序影响——同样的输入永远得到
// 同样的环境，测试与日志都好对照。
func buildPluginEnv(whitelist []string, extra map[string]string) []string {
	out := make([]string, 0, len(whitelist)+len(extra))
	for _, k := range whitelist {
		if v, ok := os.LookupEnv(k); ok && v != "" {
			out = append(out, k+"="+v)
		}
	}
	keys := make([]string, 0, len(extra))
	for k := range extra {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out = append(out, k+"="+extra[k])
	}
	return out
}

// backoffDelay 返回第 n 次（从 0 起）重启前的退避时长：1s → 2s → 4s → 8s → 16s。
//
// 阶梯封顶在 16s：MaxRestarts 默认 5，正好用完五级；即便有人把 MaxRestarts
// 调大，也不该让退避时长继续翻倍到分钟级（那等于插件永久停止工作却仍占着
// 重启名额）。负数按 0 处理，防止调用方算错时移位出负数。
func backoffDelay(restarts int) time.Duration {
	const (
		first = time.Second
		last  = 16 * time.Second
	)
	if restarts < 0 {
		restarts = 0
	}
	if restarts > 4 {
		return last
	}
	return first << uint(restarts)
}

// stopGrace 是 Stop 里 SIGTERM 到 SIGKILL 的宽限期，以及等待进程被回收的上限。
//
// 分成两段各 2s：正常退出的 node 在毫秒级就没了，这个常量只在插件挂死时才被
// 用到，因此不影响关停速度。
const stopGrace = 2 * time.Second

// SupervisorOptions 是拉起一个插件所需的配置。
type SupervisorOptions struct {
	// NodePath 是 node 可执行文件路径，空则用 PATH 里的 "node"。
	NodePath string
	// PluginDir 是插件目录，子进程的 cwd 也设在这里。
	PluginDir string
	// Entry 是入口文件名，来自 Manifest.Entry（已由 validateEntry 校验过），
	// 空则用 "index.js"。
	Entry string
	// ExtraEnv 是追加给子进程的插件专属变量（如 manifest 声明的配置项）。
	// 它们与白名单变量一起构成子进程的**全部**环境。
	ExtraEnv map[string]string
	// MaxRestarts 是连续崩溃后的最大重启次数，0 视为默认 5。
	// 崩溃总次数为 MaxRestarts+1（首发一次 + 每次重启各一次）。
	MaxRestarts int
	// OnCrash 在每次崩溃时回调，用于更新落库状态。
	//
	// 它在 supervisor 自己的 goroutine 上**同步**执行，且不持有 supervisor 的锁
	// （回调里再调 Start/Stop 不会死锁，但**同步调用 Stop 会**——Stop 要等这个
	// goroutine 退出）。回调应当只做记录或转发，耗时动作请自行开 goroutine。
	OnCrash func(reason string)
}

// Supervisor 托管单个插件的子进程生命周期：拉起、等它退出、按退避重启、停止。
//
// # 返回的 Conn 只对首个子进程有效
//
// Start 返回的是**第一次**拉起的那条连接。插件崩溃自动重启后，新进程有新的
// 管道与新的 Conn，调用方手里的那个已经随 Close 失效——新一轮的 Conn 不经
// Start 返回。宿主因此必须在 OnCrash 里把该插件标记为异常/待恢复，并在需要
// 时重新 StartPlugin，而不是继续用旧 Conn 发钩子（那只会拿到 ErrClosed）。
// 这个限制来自「Start 只返回一次」，不是实现取舍。
//
// # 与 os/exec 的管道协作（为什么不用 StdinPipe/StdoutPipe）
//
// cmd.StdinPipe/cmd.StdoutPipe 把**父进程端**的管道登记进 exec 的
// parentIOPipes，由 cmd.Wait 在所有读取结束之后关闭。而 Conn 按 ipc.go 的
// 约定也拥有并关闭自己的流——两者并发关闭同一条管道就是 Task 3 文档里点名的
// 数据竞争（假 Wait 错误 → 被误判成崩溃 → 白白的重启阶梯）。
//
// 这里改为自己 os.Pipe()，把**子进程端**作为 *os.File 交给 cmd.Stdin/
// cmd.Stdout。exec 对 *os.File 不做包装、不起拷贝 goroutine、也不登记关闭，
// 于是：
//
//   - cmd.Wait 不再关闭任何管道，与 Conn.Close 没有重叠的关闭动作；
//   - 也不存在 StdoutPipe 那条「读完再 Wait」的约束——Wait 只负责回收进程；
//   - 关停顺序仍是 ipc.go 要求的三步：conn.Close() → 结束进程 → cmd.Wait()。
//
// 代价：子进程端的另外两个 fd 由本文件在 Start 成功后负责关闭（见
// closeChildEnds）。父进程留着子进程的读端会让「写端拿到 EPIPE」和
// 「读端看到 EOF」都失灵，崩溃会变成永久静默。
type Supervisor struct {
	opts SupervisorOptions

	mu        sync.Mutex
	cmd       *exec.Cmd
	conn      *Conn
	restarts  int
	running   bool
	stopped   bool
	cancel    context.CancelFunc
	supervise sync.WaitGroup

	// stopCh 只用于唤醒退避中的 select（见 Stop）。不开在 ctx 上是因为
	// childCtx 的取消会让 exec.CommandContext 立刻 SIGKILL，先于优雅退出。
	stopCh   chan struct{}
	stopOnce sync.Once
}

// NewSupervisor 构造 supervisor，此时不拉起任何进程。
func NewSupervisor(opts SupervisorOptions) *Supervisor {
	if opts.MaxRestarts == 0 {
		opts.MaxRestarts = 5
	}
	if opts.NodePath == "" {
		opts.NodePath = "node"
	}
	if opts.Entry == "" {
		opts.Entry = "index.js"
	}
	return &Supervisor{opts: opts, stopCh: make(chan struct{})}
}

// Running 报告插件当前是否在运行。
//
// 崩溃后的退避窗口内它是 false（此刻没有进程）；重启成功后又变回 true。
func (s *Supervisor) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// Conn 返回**当前这一代**的连接；从未成功启动过时返回 nil。
//
// # 为什么必须有这个方法
//
// Start 只返回第一代 Conn（它只能返回一次）。插件崩溃自动重启后，新一代进程有
// 自己的管道与 Conn，若宿主只能用 Start 的返回值，它手里那个已经死了——但
// Running() 是 true。宿主于是会一边显示「运行中」，一边让每个钩子都拿到
// ErrClosed，这是最难排查的一类故障。
//
// 因此宿主的正确用法是：
//
//	sup.Start(ctx)                       // 首次拉起
//	sup.OnCrash = func(string) { ... }   // 收到崩溃信号，标记状态
//	conn := sup.Conn()                   // 任何时刻取「现在」那条，而不是缓存 Start 的返回值
//
// 注意返回的 *Conn 仍可能在你拿到它之后失效（对端恰好此刻崩溃）——这是分布式
// 生命周期的固有竞态，不是本方法的缺陷。正确做法是把 ErrClosed 当作
// 「这一代结束了，去 OnCrash 里等下一代」的信号，而不是重试同一条连接。
func (s *Supervisor) Conn() *Conn {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn
}

// Start 拉起子进程并建立 IPC。
//
// ctx 控制整个生命周期：它被取消时子进程会被结束，且不再重启。
// 重复调用 Start 会先停掉上一个进程，避免同一插件跑两份。
func (s *Supervisor) Start(ctx context.Context) (*Conn, error) {
	childCtx, cancel := context.WithCancel(ctx)

	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		cancel()
		return nil, fmt.Errorf("plugin: supervisor 已停止，不能再 Start")
	}
	prevCmd, prevConn := s.cmd, s.conn
	s.cancel = cancel
	cmd, conn, err := s.spawnLocked(childCtx)
	if err != nil {
		s.mu.Unlock()
		cancel()
		return nil, err
	}
	s.supervise.Add(1)
	s.mu.Unlock()

	// 重复 Start：旧进程交还给它的 watch goroutine 回收，这里只断开连接并
	// 让它退出（它会发现 s.cmd 已经不是自己，见 supervise）。
	if prevConn != nil {
		_ = prevConn.Close()
	}
	if prevCmd != nil && prevCmd.Process != nil {
		_ = prevCmd.Process.Signal(syscall.SIGTERM)
	}

	go s.superviseLoop(childCtx, cmd, conn)
	return conn, nil
}

// spawnLocked 拉起一个子进程并构造 Conn。调用方必须持有 s.mu。
func (s *Supervisor) spawnLocked(ctx context.Context) (*exec.Cmd, *Conn, error) {
	// 入口路径必须绝对：cmd.Dir 也设成插件目录，若 entry 还带相对前缀，
	// node 会把它当成「相对 cwd 的路径」，拼出 <dir>/<dir>/index.js 而报模块
	// 找不到（PluginRoot 为 "./plugins" 时在 Task 7 的装配里必然发生）。
	dir, err := filepath.Abs(s.opts.PluginDir)
	if err != nil {
		return nil, nil, fmt.Errorf("plugin: 解析插件目录 %q: %w", s.opts.PluginDir, err)
	}
	entry := filepath.Join(dir, s.opts.Entry)

	cmd := exec.CommandContext(ctx, s.opts.NodePath, entry)
	cmd.Dir = dir
	cmd.Env = buildPluginEnv(envWhitelist, s.opts.ExtraEnv)
	// stderr 直接继到宿主 stderr，插件日志才能在 docker logs 里看见。
	cmd.Stderr = os.Stderr

	inR, inW, err := os.Pipe()
	if err != nil {
		return nil, nil, fmt.Errorf("plugin: 建 stdin 管道: %w", err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		_ = inR.Close()
		_ = inW.Close()
		return nil, nil, fmt.Errorf("plugin: 建 stdout 管道: %w", err)
	}

	// 子进程端以 *os.File 交出：exec 直接继承 fd，不做包装也不登记回收。
	cmd.Stdin = inR
	cmd.Stdout = outW

	if err := cmd.Start(); err != nil {
		closeAll(inR, inW, outR, outW)
		return nil, nil, fmt.Errorf("plugin: 拉起 %s: %w", entry, err)
	}
	// 父进程必须放掉子进程的那两个 fd，否则：
	//   - inR 留在父进程 → 子进程读不到 EOF（我们只关得到自己的写端）；
	//   - outW 留在父进程 → 子进程退出后 outR 永远等不到 EOF。
	// 两者都会把「崩溃」变成「永久静默」。
	closeAll(inR, outW)

	conn := NewConn(outR, inW)
	if err := conn.Start(); err != nil {
		_ = conn.Close()
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		return nil, nil, err
	}

	s.cmd = cmd
	s.conn = conn
	s.running = true
	return cmd, conn, nil
}

// superviseLoop 等子进程退出，按退避重启，超过上限后停止。
//
// 它是唯一的回收者（cmd.Wait 只在这里调用），因此 supervisor 的关停第三步
// 就是「等这个 goroutine 结束」——见 Stop。
func (s *Supervisor) superviseLoop(ctx context.Context, cmd *exec.Cmd, conn *Conn) {
	defer s.supervise.Done()

	for {
		waitErr := cmd.Wait()

		// 进程已经没了，先断开连接：解开读循环与所有在途 Call。
		// 这里 Close 与上面的 Wait 是**先后**关系（Wait 已返回），不是并发，
		// 所以不会碰到 ipc.go 文档里点名的关闭竞态。
		_ = conn.Close()

		s.mu.Lock()
		s.running = false
		stopped := s.stopped
		restarts := s.restarts
		// 重复 Start 后 s.cmd 已被换成新进程：旧进程的收尾到此为止，
		// 重启阶梯由新进程自己的 goroutine 接管。
		replaced := s.cmd != cmd
		s.mu.Unlock()

		if stopped || replaced || ctx.Err() != nil {
			return
		}

		reason := "exited"
		if waitErr != nil {
			reason = waitErr.Error()
		}
		// 回调在锁外执行：OnCrash 里再碰 supervisor 的其它方法不会死锁。
		if s.opts.OnCrash != nil {
			s.opts.OnCrash(reason)
		}

		if restarts >= s.opts.MaxRestarts {
			logrus.Errorf("plugin: %s 连续崩溃 %d 次，达到上限 %d，停止重启",
				s.opts.PluginDir, restarts+1, s.opts.MaxRestarts+1)
			return
		}

		backoff := backoffDelay(restarts)
		logrus.Warnf("plugin: %s 退出（%s），%s 后重启（第 %d/%d 次）",
			s.opts.PluginDir, reason, backoff, restarts+1, s.opts.MaxRestarts)

		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		}

		// 检查与拉起必须在同一把锁里完成，否则 Stop 可能插在中间：
		// 它关掉旧进程、拿到「已停止」，而这里紧接着又拉起一个新进程，
		// 那个进程就再也没人管了（Stop 已经走完）。
		s.mu.Lock()
		if s.stopped || s.cmd != cmd {
			s.mu.Unlock()
			return
		}
		newCmd, newConn, err := s.spawnLocked(ctx)
		if err != nil {
			s.mu.Unlock()
			logrus.Errorf("plugin: %s 重启失败，放弃: %v", s.opts.PluginDir, err)
			return
		}
		s.restarts++
		s.mu.Unlock()

		cmd, conn = newCmd, newConn
	}
}

// Stop 停止子进程且不再重启。可重复调用。
//
// 关停顺序严格按 ipc.go 的约定：
//
//  1. conn.Close() —— 先让本地不再有在途读写（解开阻塞的 Write/Scan）。
//     注意对一个**已经卡在写**的调用，Close 只是让它的 fd 在写返回后才真正
//     释放（阻塞 fd 的 poll.FD 会等 refcount 归零），所以单靠它解不开。
//  2. 结束子进程 —— 先 SIGTERM 让它自己退，宽限 stopGrace 后 SIGKILL。
//     这一步才是卡死写的真正解药：子进程一死，它持有的读端关闭，阻塞中的
//     write(2) 立刻拿到 EPIPE 返回。
//  3. cmd.Wait() —— 由 superviseLoop 里的唯一回收者完成，Stop 只是等它结束。
//     因此绝不存在「Close 与 Wait 并发关同一条管道」。
func (s *Supervisor) Stop() error {
	s.mu.Lock()
	s.stopped = true
	conn, cmd, cancel := s.conn, s.cmd, s.cancel
	s.mu.Unlock()

	// 唤醒退避中的 select，否则 Stop 要陪它睡到退避结束（最长 16s）。
	s.stopOnce.Do(func() { close(s.stopCh) })

	// 1. 断开连接：解开读循环与在途 Call。
	if conn != nil {
		_ = conn.Close()
	}
	// 2. 结束进程：先礼后兵。
	if cmd != nil && cmd.Process != nil {
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			// 进程已经退出（ErrProcessDone）或平台不支持 SIGTERM（Windows），
			// 直接强杀。已经退出的进程再 Kill 也只是返回错误，无害。
			_ = cmd.Process.Kill()
		}
	}

	// 3. 等唯一的回收者走完 cmd.Wait()。
	done := make(chan struct{})
	go func() {
		s.supervise.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(stopGrace):
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		select {
		case <-done:
		case <-time.After(stopGrace):
			return fmt.Errorf("plugin: %s 停止超时，进程未被回收", s.opts.PluginDir)
		}
	}

	// 进程已回收，此时取消 childCtx 不会让 exec 提前 SIGKILL（那会绕过上面的
	// 优雅退出），只负责释放 ctx 资源。
	if cancel != nil {
		cancel()
	}
	return nil
}

// closeAll 关闭一组管道端，忽略错误（多半是「已经关了」）。
func closeAll(files ...*os.File) {
	for _, f := range files {
		if f != nil {
			_ = f.Close()
		}
	}
}
