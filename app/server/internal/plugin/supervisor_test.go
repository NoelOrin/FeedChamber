package plugin

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// requireNode 在 node 不在 PATH 时跳过子进程测试。
//
// 刻意用 exec.LookPath 而不是探测 /usr/bin/node、/usr/local/bin/node：
// 用 nvm / vite-plus 之类工具装的 node 只出现在 PATH 上，探测固定路径会让
// 这些机器上的测试静默 SKIP——测试全绿但一个字节的 supervisor 行为都没验到。
func requireNode(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node 不在 PATH，跳过子进程测试")
	}
}

// buildPluginEnv 必须白名单重建，不得继承宿主环境。
func TestBuildPluginEnvDoesNotInheritHost(t *testing.T) {
	t.Setenv("DB_DSN", "postgres://secret")
	t.Setenv("ADMIN_PASSWORD", "hunter2")

	env := buildPluginEnv([]string{"PATH", "NODE_ENV"}, map[string]string{"PLUGIN_SECRET": "ok"})

	forbidden := []string{"DB_DSN=", "ADMIN_PASSWORD="}
	for _, f := range forbidden {
		for _, e := range env {
			require.NotContains(t, e, f, "子进程环境不得包含 %s", f)
		}
	}
	require.Contains(t, env, "PLUGIN_SECRET=ok")
}

func TestBuildPluginEnvOmitsUnsetWhitelistEntries(t *testing.T) {
	os.Unsetenv("FC_TEST_ABSENT")
	env := buildPluginEnv([]string{"FC_TEST_ABSENT", "PATH"}, nil)
	for _, e := range env {
		require.NotContains(t, e, "FC_TEST_ABSENT=")
	}
}

// TestBuildPluginEnvRejectsEmptyKey 钉住「白名单名字存在但宿主未设置时跳过」的
// 反面：即便宿主真的设了个空值，也不能把 "K=" 下发出去——node 会把空 PATH
// 当成有效值，之后子进程里连 `node` 自己都找不到。
func TestBuildPluginEnvDropsHostEmptyValues(t *testing.T) {
	t.Setenv("FC_TEST_EMPTY", "")
	env := buildPluginEnv([]string{"FC_TEST_EMPTY"}, nil)
	for _, e := range env {
		require.NotContains(t, e, "FC_TEST_EMPTY=")
	}
}

// TestEnvWhitelistHasNoSensitiveNames 钉住白名单本身的内容：spec 要求白名单
// MUST NOT 包含数据库连接串、管理员密码、会话密钥一类变量。buildPluginEnv
// 的逻辑再对，只要有人往白名单里塞一个 DB_DSN，这条防线就整个失效。
func TestEnvWhitelistHasNoSensitiveNames(t *testing.T) {
	exact := map[string]struct{}{
		"DB_DSN":         {},
		"ADMIN_PASSWORD": {},
		"JWT_SECRET":     {},
		"DB_TYPE":        {},
	}
	for _, name := range envWhitelist {
		_, banned := exact[name]
		require.False(t, banned, "白名单不得包含 %s", name)
		require.False(t,
			strings.HasSuffix(name, "_SECRET") || strings.HasSuffix(name, "_KEY"),
			"白名单不得包含敏感变量 %s", name)
	}
	// 反向：白名单必须真的含有 PATH，否则 node 起不来（防止有人把白名单清空
	// 当作「更安全」的改法——那会让所有插件都启动失败）。
	require.Contains(t, envWhitelist, "PATH")
}

// TestBackoffSequence 钉住退避阶梯 1s→2s→4s→8s→16s。
//
// 用纯函数断言而不是真等 31 秒跑满五级：真跑满需要 1+2+4+8+16=31s，任何
// CI 都不该为一条退避规则付这个时间。真实重启链路由
// TestSupervisorRestartsWithBackoff 用 MaxRestarts=2 覆盖前两级。
func TestBackoffSequence(t *testing.T) {
	want := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
	}
	for i, w := range want {
		require.Equal(t, w, backoffDelay(i), "第 %d 次退避", i+1)
	}
	// 超过阶梯后必须封顶在 16s，且不得因移位溢出变成负数。
	for _, i := range []int{5, 6, 32, 63, 64, 1000} {
		require.Equal(t, 16*time.Second, backoffDelay(i), "restarts=%d 应封顶", i)
	}
	// 阴性输入不得 panic（防御调用方算错）。
	require.Equal(t, 1*time.Second, backoffDelay(-1))
}

// TestNewSupervisorDefaults 钉住默认值：MaxRestarts 缺省 5 正好对应五级阶梯，
// Entry 缺省 index.js 与 ParseManifest 的默认值一致。
func TestNewSupervisorDefaults(t *testing.T) {
	s := NewSupervisor(SupervisorOptions{PluginDir: "/tmp/p"})
	require.Equal(t, 5, s.opts.MaxRestarts)
	require.Equal(t, "index.js", s.opts.Entry)
	require.Equal(t, "node", s.opts.NodePath)
}

// TestSupervisorStartFailsWhenNodeMissing 覆盖 spawn 的同步失败分支：
// 这条不需要 node（LookPath 在 Start 进程之前就失败），因此在任何平台都真跑。
func TestSupervisorStartFailsWhenNodeMissing(t *testing.T) {
	s := NewSupervisor(SupervisorOptions{
		NodePath:  "feedchamber-no-such-binary-xyz",
		PluginDir: t.TempDir(),
		Entry:     "index.js",
	})
	conn, err := s.Start(context.Background())
	require.Error(t, err)
	require.Nil(t, conn)
	require.False(t, s.Running())
	require.NoError(t, s.Stop(), "启动失败后 Stop 也必须安全")
}

// writeTestPlugin 在临时目录里写一个 index.js，返回目录。
func writeTestPlugin(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.js"), []byte(body), 0o644))
	return dir
}

// ndjsonPrelude 是插件侧的 NDJSON 分帧样板：按行切、逐行 JSON.parse、
// 坏行跳过，然后把帧交给 handle()。
const ndjsonPrelude = `
process.stdin.setEncoding('utf8');
let buf = '';
const send = (obj) => process.stdout.write(JSON.stringify(obj) + '\n');
process.stdin.on('data', (d) => {
  buf += d;
  let i;
  while ((i = buf.indexOf('\n')) >= 0) {
    const line = buf.slice(0, i); buf = buf.slice(i + 1);
    if (!line.trim()) continue;
    let f;
    try { f = JSON.parse(line); } catch (e) { continue; }
    handle(f);
  }
});
function handle(f) {
%s
}
`

// writeTestPluginWithHandler 写一个「只会 NDJSON 分帧 + 调 handle」的插件，
// 测试只需要给出 handle 的请求分支。
func writeTestPluginWithHandler(t *testing.T, handlerJS string) string {
	t.Helper()
	return writeTestPlugin(t, strings.Replace(ndjsonPrelude, "%s", handlerJS, 1))
}

func TestSupervisorStartsAndHandshakes(t *testing.T) {
	requireNode(t)

	dir := writeTestPluginWithHandler(t, `
  if (f.method === 'hook.cron.daily') {
    send({ id: f.id, result: { ok: true } });
  }`)

	s := NewSupervisor(SupervisorOptions{
		NodePath:    "node",
		PluginDir:   dir,
		Entry:       "index.js",
		MaxRestarts: 2,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := s.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Stop() })

	raw, err := conn.Call(ctx, "hook.cron.daily", nil)
	require.NoError(t, err)
	require.Contains(t, string(raw), "ok")
}

// TestSupervisorChildEnvIsWhitelisted 是白名单的端到端证据。
//
// TestBuildPluginEnvDoesNotInheritHost 只证明那个纯函数的行为；只要 spawn 里
// 少写一行 cmd.Env 赋值，exec 会回退到继承宿主环境，纯函数测试依然全绿。
// 这条测试真的把 DB_DSN 放进宿主环境、真的拉起 node、再问子进程要
// process.env 的键名清单。
func TestSupervisorChildEnvIsWhitelisted(t *testing.T) {
	requireNode(t)

	t.Setenv("DB_DSN", "postgres://user:pw@db/feedchamber")
	t.Setenv("ADMIN_PASSWORD", "hunter2")
	t.Setenv("SOME_API_KEY", "sk-live-should-not-leak")

	dir := writeTestPluginWithHandler(t, `
  if (f.method === 'env.dump') {
    send({ id: f.id, result: Object.keys(process.env).sort().join(',') });
  }`)

	s := NewSupervisor(SupervisorOptions{
		NodePath:    "node",
		PluginDir:   dir,
		Entry:       "index.js",
		ExtraEnv:    map[string]string{"PLUGIN_EXTRA": "1"},
		MaxRestarts: 2,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := s.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Stop() })

	raw, err := conn.Call(ctx, "env.dump", nil)
	require.NoError(t, err)

	var joined string
	require.NoError(t, json.Unmarshal(raw, &joined))
	keys := strings.Split(joined, ",")

	require.NotContains(t, keys, "DB_DSN")
	require.NotContains(t, keys, "ADMIN_PASSWORD")
	require.NotContains(t, keys, "SOME_API_KEY")
	require.Contains(t, keys, "PATH", "白名单里的路径变量必须下发，否则 node 找不到可执行文件")
	require.Contains(t, keys, "PLUGIN_EXTRA", "ExtraEnv 必须到达子进程")
}

func TestSupervisorRestartsWithBackoff(t *testing.T) {
	requireNode(t)

	// 插件启动后立刻退出，触发重启并最终达到上限。
	dir := writeTestPlugin(t, `process.exit(1);`)

	crashes := make(chan int, 16)
	s := NewSupervisor(SupervisorOptions{
		NodePath: "node", PluginDir: dir, Entry: "index.js",
		MaxRestarts: 2,
		OnCrash:     func(string) { crashes <- 1 },
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, _ = s.Start(ctx)

	select {
	case <-crashes:
	case <-time.After(8 * time.Second):
		t.Fatal("未观测到崩溃回调")
	}
	_ = s.Stop()
	require.False(t, s.Running(), "超过重启上限后必须停止")
}

// TestSupervisorStopsAfterMaxRestarts 钉住「超限后不再重启」，且崩溃回调次数
// 恰为 MaxRestarts+1（首发 + 每次重启各一次）。只看 Running() 不够：
// 一个把 restarts 条件写成 < 而不是 <= 的实现，Running() 也是 false，
// 但它会多重启一次。
func TestSupervisorStopsAfterMaxRestarts(t *testing.T) {
	requireNode(t)

	const maxRestarts = 2
	dir := writeTestPlugin(t, `process.exit(1);`)

	var (
		mu       int
		crashCnt = make(chan struct{}, 16)
	)
	s := NewSupervisor(SupervisorOptions{
		NodePath: "node", PluginDir: dir, Entry: "index.js",
		MaxRestarts: maxRestarts,
		OnCrash:     func(string) { crashCnt <- struct{}{} },
	})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, _ = s.Start(ctx)

	// 退避 1s + 2s，加上 node 起停开销，10s 足够跑完全部 3 次崩溃。
	deadline := time.After(12 * time.Second)
	for {
		select {
		case <-crashCnt:
			mu++
		case <-deadline:
			t.Fatalf("崩溃回调只有 %d 次，期望 %d 次（首发 + %d 次重启）", mu, maxRestarts+1, maxRestarts)
		}
		if mu >= maxRestarts+1 {
			break
		}
	}

	// 再等一轮退避窗口，确认没有第 4 次（若实现仍在重启，这个窗口内必然崩溃）。
	select {
	case <-crashCnt:
		t.Fatalf("超过重启上限后仍在重启：崩溃次数 > %d", maxRestarts+1)
	case <-time.After(4 * time.Second):
	}
	require.False(t, s.Running(), "超过重启上限后必须停止")
	_ = s.Stop()
}

// TestSupervisorStopUnblocksWedgedWrite 钉住 Task 3 转来的硬约束：
// 插件不读 stdin 时，Notify 会永久阻塞在 writeMu/Write 上，唯一能解开它的是
// conn.Close() / 杀进程——而这两件事都归 supervisor 管。若哪天有人把
// Stop 里的 conn.Close() 删掉（觉得「关管道是 os/exec 的事」），这条会红。
func TestSupervisorStopUnblocksWedgedWrite(t *testing.T) {
	requireNode(t)

	// 刻意不挂 data 监听：node 完全不消费 stdin，父进程写满管道缓冲后阻塞。
	dir := writeTestPlugin(t, `process.stdin.pause(); setInterval(() => {}, 1000);`)

	s := NewSupervisor(SupervisorOptions{
		NodePath: "node", PluginDir: dir, Entry: "index.js",
		MaxRestarts: 2,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	conn, err := s.Start(ctx)
	require.NoError(t, err)
	require.True(t, s.Running())

	notifyDone := make(chan error, 1)
	go func() {
		// 4MiB 远超任何平台默认管道缓冲（macOS 16–64KiB、Linux 64KiB），
		// 所以这条写一定会卡住。
		notifyDone <- conn.Notify("wedge", map[string]string{"p": strings.Repeat("x", 4<<20)})
	}()

	select {
	case <-notifyDone:
		t.Fatal("对端不读 stdin 时 Notify 不应返回——管道缓冲未被写满，测试构造失效")
	case <-time.After(500 * time.Millisecond):
	}

	require.NoError(t, s.Stop())

	select {
	case <-notifyDone:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop 未能解开阻塞在写上的 Notify：kill 路径失效")
	}

	require.False(t, s.Running())
}

// TestSupervisorStopIsIdempotent 钉住宿主关停路径：Stop 可以被重复调用，
// 且 Stop 之后 Running() 为 false。
func TestSupervisorStopIsIdempotent(t *testing.T) {
	requireNode(t)

	dir := writeTestPluginWithHandler(t, `
  if (f.method === 'hook.cron.daily') {
    send({ id: f.id, result: { ok: true } });
  }`)

	s := NewSupervisor(SupervisorOptions{
		NodePath: "node", PluginDir: dir, Entry: "index.js", MaxRestarts: 2,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := s.Start(ctx)
	require.NoError(t, err)

	require.NoError(t, s.Stop())
	require.False(t, s.Running())
	require.NoError(t, s.Stop(), "重复 Stop 必须安全")
	require.False(t, s.Running())
}
