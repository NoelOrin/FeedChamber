package service_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"feedchamber/internal/model"
	"feedchamber/internal/service"

	"github.com/stretchr/testify/require"
)

// 来源地址是「管理员填什么、服务端就去执行什么」的面，形状校验是唯一的
// 命令注入防线（命令不走 shell，但参数以 - 开头仍会被 git 当选项）。

func TestValidateSourceURL_Accepts(t *testing.T) {
	for _, u := range []string{
		"https://github.com/x/y.git",
		"http://example.com/p",
		"git://example.com/r.git",
		"git@github.com:x/y.git",
	} {
		require.NoError(t, service.ValidateSourceURL(u), u)
	}
}

func TestValidateSourceURL_Rejects(t *testing.T) {
	for _, u := range []string{
		"",
		"file:///etc/passwd",
		"ftp://example.com/x",
		"https://exa mple.com/x", // 空白
		"https://x/y;rm -rf /",   // shell 元字符
		"--upload-pack=evil",     // 以 - 开头会被当 git 选项
		"ssh://example.com/repo", // 未支持的协议
		"https:///nohost",
	} {
		require.Error(t, service.ValidateSourceURL(u), "should reject: %q", u)
	}
}

// 目录校验和必须可重复、且不因 node_modules 变化而漂移。
func TestHashPluginDir_StableAndIgnoresNodeModules(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.js"), []byte("console.log(1)"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "node_modules", "dep"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "node_modules", "dep", "i.js"), []byte("x"), 0o644))

	h1, err := service.HashPluginDir(dir)
	require.NoError(t, err)
	h2, err := service.HashPluginDir(dir)
	require.NoError(t, err)
	require.Equal(t, h1, h2, "同一份代码两次哈希必须相同")

	// node_modules 内容变化不影响结果。
	require.NoError(t, os.WriteFile(filepath.Join(dir, "node_modules", "dep", "i.js"), []byte("changed"), 0o644))
	h3, err := service.HashPluginDir(dir)
	require.NoError(t, err)
	require.Equal(t, h1, h3, "node_modules 不应计入校验和")

	// 源码变化必须改变结果。
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.js"), []byte("console.log(2)"), 0o644))
	h4, err := service.HashPluginDir(dir)
	require.NoError(t, err)
	require.NotEqual(t, h1, h4, "源码变化必须改变校验和")
}

func TestReadDependencyList(t *testing.T) {
	dir := t.TempDir()
	// 无 package.json → 空列表、无错误。
	deps, err := service.ReadDependencyList(dir)
	require.NoError(t, err)
	require.Empty(t, deps)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{
		"dependencies": {"lodash": "^4.0.0", "left-pad": "1.3.0"},
		"devDependencies": {"jest": "^29"}
	}`), 0o644))
	deps, err = service.ReadDependencyList(dir)
	require.NoError(t, err)
	require.Equal(t, []string{"left-pad@1.3.0", "lodash@^4.0.0"}, deps,
		"只列 dependencies，且排序稳定")
}

// fakeFetcher 记录调用并按需失败。
type fakeFetcher struct {
	fetched  []string
	depsDir  string
	fetchErr error
	depsErr  error
}

func (f *fakeFetcher) Fetch(_ context.Context, srcURL, destDir string) error {
	f.fetched = append(f.fetched, srcURL)
	if f.fetchErr != nil {
		return f.fetchErr
	}
	return os.MkdirAll(destDir, 0o755)
}

func (f *fakeFetcher) InstallDeps(_ context.Context, dir string) error {
	f.depsDir = dir
	return f.depsErr
}

// Install 必须经 fetcher 拉代码并装依赖；拉取失败不得留下登记。
func TestInstallFetchesCodeAndDeps(t *testing.T) {
	svc, store := newPluginTestSvc(t)
	ff := &fakeFetcher{}
	svc.SetFetcher(ff)
	ctx := context.Background()
	src, err := svc.CreateSource(ctx, "s", "git", "https://example.com/x.git", "admin")
	require.NoError(t, err)
	require.NoError(t, svc.ApproveSource(ctx, src.ID, "admin"))

	_, err = svc.Install(ctx, service.PluginInstallInput{
		PluginID: "fetched", SourceID: src.ID, Manifest: validManifestJSON,
	})
	require.NoError(t, err)
	require.Equal(t, []string{"https://example.com/x.git"}, ff.fetched)
	require.NotEmpty(t, ff.depsDir, "必须执行依赖安装")

	var plugins []model.Plugin
	require.NoError(t, store.SQL().DB().Find(&plugins).Error)
	require.Len(t, plugins, 1)
}

func TestInstallCleansUpWhenFetchFails(t *testing.T) {
	svc, store := newPluginTestSvc(t)
	ff := &fakeFetcher{fetchErr: os.ErrPermission}
	svc.SetFetcher(ff)
	ctx := context.Background()
	src, err := svc.CreateSource(ctx, "s", "git", "https://example.com/x.git", "admin")
	require.NoError(t, err)
	require.NoError(t, svc.ApproveSource(ctx, src.ID, "admin"))

	_, err = svc.Install(ctx, service.PluginInstallInput{
		PluginID: "fetched", SourceID: src.ID, Manifest: validManifestJSON,
	})
	require.Error(t, err)

	var plugins []model.Plugin
	var installs []model.PluginInstall
	require.NoError(t, store.SQL().DB().Find(&plugins).Error)
	require.NoError(t, store.SQL().DB().Find(&installs).Error)
	require.Empty(t, plugins, "拉取失败不得留下插件登记")
	require.Empty(t, installs, "拉取失败不得留下安装记录")
}
