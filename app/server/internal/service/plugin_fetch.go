package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// PluginFetcher 是「把已审批来源的代码取到本地」的接缝。
//
// 抽成接口的原因有两个：
//   - 单测不该真的 git clone / npm install（网络、耗时长、不确定）；
//   - 不同来源类型（v1 只有 git）将来可以各自实现，而 service 只认这个契约。
//
// 安全约束（实现方 MUST 遵守）：
//   - 只接受已经过 `ValidateSourceURL` 的 http(s)/git 地址，绝不拼接进 shell；
//   - 命令 MUST 带超时，失败 MUST 清理半成品目录（否则下次安装会撞上脏目录）；
//   - 目标目录 MUST 在插件根目录之内（逃逸即拒绝）。
type PluginFetcher interface {
	// Fetch 把 src.URL 的代码检出到 destDir（不存在时创建）。
	Fetch(ctx context.Context, srcURL, destDir string) error
	// InstallDeps 在插件目录内执行依赖安装（npm install --omit=dev）。
	InstallDeps(ctx context.Context, dir string) error
}

// ValidateSourceURL 校验插件来源地址形态。
//
// 只放行 http(s) 与 git@host:path（scp-like）两种；拒绝 file:// / ssh 到
// 非标准形态 / 任何含空白的地址。这不是完整的 SSRF 防护（管理员本来就在
// 配置要信任什么），但它挡住「把任意本地路径当来源」和命令拼接。
func ValidateSourceURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("来源 URL 不能为空")
	}
	if strings.ContainsAny(raw, " \t\r\n\"'`$|&;<>()") {
		return fmt.Errorf("来源 URL 含非法字符")
	}
	if strings.HasPrefix(raw, "-") {
		return fmt.Errorf("来源 URL 不能以 - 开头")
	}
	if strings.HasPrefix(raw, "git@") {
		// scp-like：git@host:path
		if !strings.Contains(strings.SplitN(raw, ":", 2)[0], "@") {
			return fmt.Errorf("非法的 git 地址")
		}
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("来源 URL 不是合法地址: %w", err)
	}
	switch u.Scheme {
	case "http", "https", "git":
		if u.Host == "" {
			return fmt.Errorf("来源 URL 缺少主机名")
		}
		return nil
	default:
		return fmt.Errorf("不支持的来源协议 %q（只允许 http/https/git/git@）", u.Scheme)
	}
}

// execPluginFetcher 是 PluginFetcher 的真实实现（git + npm）。
type execPluginFetcher struct {
	GitPath string
	NpmPath string
	Timeout time.Duration
}

// NewExecPluginFetcher 构造真实 fetcher。Timeout 缺省 120s。
func NewExecPluginFetcher() PluginFetcher {
	return &execPluginFetcher{GitPath: "git", NpmPath: "npm", Timeout: 120 * time.Second}
}

func (f *execPluginFetcher) Fetch(ctx context.Context, srcURL, destDir string) error {
	if err := ValidateSourceURL(srcURL); err != nil {
		return err
	}
	// 目标目录必须为空或不存在：残留的半成品会让 git 拒绝克隆。
	if entries, err := os.ReadDir(destDir); err == nil && len(entries) > 0 {
		return fmt.Errorf("插件目录 %s 非空，拒绝覆盖", destDir)
	}
	if err := os.MkdirAll(filepath.Dir(destDir), 0o755); err != nil {
		return fmt.Errorf("创建父目录失败: %w", err)
	}

	timeout := f.Timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// --depth 1：只要当前版本，不要历史。--single-branch 同理。
	cmd := exec.CommandContext(cctx, f.GitPath, "clone", "--depth", "1", "--single-branch", "--", srcURL, destDir)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0") // 绝不交互式要密码
	out, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.RemoveAll(destDir)
		if cctx.Err() != nil {
			return fmt.Errorf("拉取代码超时（%s）", timeout)
		}
		return fmt.Errorf("git clone 失败: %v: %s", err, truncateOutput(string(out), 400))
	}
	return nil
}

func (f *execPluginFetcher) InstallDeps(ctx context.Context, dir string) error {
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err != nil {
		// 没有依赖清单是合法的：纯 JS 插件不需要 npm install。
		return nil
	}
	timeout := f.Timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, f.NpmPath, "install", "--omit=dev", "--no-audit", "--no-fund", "--ignore-scripts")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "npm_config_yes=true")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if cctx.Err() != nil {
			return fmt.Errorf("安装依赖超时（%s）", timeout)
		}
		return fmt.Errorf("npm install 失败: %v: %s", err, truncateOutput(string(out), 400))
	}
	return nil
}

// ReadDependencyList 解析 package.json 的 dependencies，供安装前展示。
//
// 只读 dependencies（插件运行时需要），不含 devDependencies——后者在
// `--omit=dev` 下根本不会装，展示它们只会让管理员多批一份不存在的东西。
func ReadDependencyList(dir string) ([]string, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var pkg struct {
		Dependencies map[string]string `json:"dependencies"`
	}
	if err := json.Unmarshal(raw, &pkg); err != nil {
		return nil, fmt.Errorf("package.json 不是合法 JSON: %w", err)
	}
	out := make([]string, 0, len(pkg.Dependencies))
	for name, ver := range pkg.Dependencies {
		out = append(out, name+"@"+ver)
	}
	sort.Strings(out)
	return out, nil
}

// HashPluginDir 计算插件目录的内容校验和，用于「装的是不是同一份代码」的可查性。
//
// 只纳入文件名与内容，跳过 node_modules（它由 npm install 生成，字节级不稳定）。
// 返回 sha256 十六进制串。
func HashPluginDir(dir string) (string, error) {
	h := sha256.New()
	root := filepath.Clean(dir)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if info.IsDir() {
			if info.Name() == "node_modules" || info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		// 只哈希普通文件；符号链接跳过（可指向目录外）。
		if !info.Mode().IsRegular() {
			return nil
		}
		if _, err := io.WriteString(h, rel+"\x00"); err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := io.Copy(h, f); err != nil {
			return err
		}
		_, _ = io.WriteString(h, "\x00")
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func truncateOutput(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
