package service_test

import (
	"context"
	"path/filepath"
	"testing"

	"feedchamber/internal/config"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
	"feedchamber/internal/repository"
	"feedchamber/internal/service"

	"github.com/stretchr/testify/require"
)

// newSlotTestSvc 用真实 SQLite 建 service。
//
// 与 plugin_install_test.go 同一个理由：:memory: 是「每个连接一个库」，
// gorm 的连接池会开多条连接，AutoMigrate 建的表在后续查询的连接上可能不存在。
func newSlotTestSvc(t *testing.T) service.SlotService {
	t.Helper()
	store, err := db.New(config.DBConfig{Type: "sqlite", DSN: filepath.Join(t.TempDir(), "slot.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.AutoMigrate(&model.SlotEntry{}))
	return service.NewSlotService(repository.NewSlotEntryRepository(store), nil)
}

// 未知插槽名必须被拒绝：前端只认枚举，写进库也不会被渲染。
func TestCreateSlotEntryRejectsUnknownSlot(t *testing.T) {
	svc := newSlotTestSvc(t)
	_, err := svc.Create(context.Background(), service.SlotEntryInput{
		Slot: "not.a.slot", Kind: model.SlotKindText,
		Props: map[string]any{"text": "hi"},
	}, "admin")
	require.Error(t, err)
}

// 裸 HTML 必须被拒绝——这是 shell-slots 规范里唯一不可协商的类型限制。
func TestCreateSlotEntryRejectsHTMLKind(t *testing.T) {
	svc := newSlotTestSvc(t)
	_, err := svc.Create(context.Background(), service.SlotEntryInput{
		Slot: model.SlotSidebarTop, Kind: "html",
		Props: map[string]any{"html": "<script>alert(1)</script>"},
	}, "admin")
	require.Error(t, err)
	require.Contains(t, err.Error(), "html")
}

// 可见性过滤：TL1 用户看不到 TL2 门槛的条目。
func TestListForViewerFiltersByLevel(t *testing.T) {
	svc := newSlotTestSvc(t)
	ctx := context.Background()
	_, err := svc.Create(ctx, service.SlotEntryInput{
		Slot: model.SlotSidebarTop, Kind: model.SlotKindText,
		Props:      map[string]any{"text": "vip"},
		Visibility: model.NavVisibilityLevel, MinLevel: 2,
	}, "admin")
	require.NoError(t, err)

	guest, err := svc.ListForViewer(ctx, false, 0)
	require.NoError(t, err)
	require.Empty(t, guest, "游客不得看到等级门槛条目")

	tl1, err := svc.ListForViewer(ctx, true, 1)
	require.NoError(t, err)
	require.Empty(t, tl1, "TL1 不得看到 TL2 条目")

	tl2, err := svc.ListForViewer(ctx, true, 2)
	require.NoError(t, err)
	require.Len(t, tl2, 1)
}

// 排序按 sort 升序。
func TestListForViewerOrdersBySort(t *testing.T) {
	svc := newSlotTestSvc(t)
	ctx := context.Background()
	for _, pair := range []struct {
		text string
		sort int
	}{{"b", 20}, {"a", 10}} {
		_, err := svc.Create(ctx, service.SlotEntryInput{
			Slot: model.SlotSidebarTop, Kind: model.SlotKindText,
			Props: map[string]any{"text": pair.text}, Sort: pair.sort,
		}, "admin")
		require.NoError(t, err)
	}
	got, err := svc.ListForViewer(ctx, true, 0)
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, 10, got[0].Sort, "必须按 sort 升序")
}

// 停用的条目不下发到前台。
func TestListForViewerSkipsDisabled(t *testing.T) {
	svc := newSlotTestSvc(t)
	ctx := context.Background()
	off := false
	_, err := svc.Create(ctx, service.SlotEntryInput{
		Slot: model.SlotSidebarTop, Kind: model.SlotKindText,
		Props: map[string]any{"text": "off"}, Enabled: &off,
	}, "admin")
	require.NoError(t, err)

	got, err := svc.ListForViewer(ctx, true, 0)
	require.NoError(t, err)
	require.Empty(t, got)
}

// 插件写入是**覆盖**而不是追加：否则 cron.daily 每跑一次就多一条。
func TestSetForPluginOverwritesInsteadOfAppending(t *testing.T) {
	svc := newSlotTestSvc(t)
	ctx := context.Background()

	require.NoError(t, svc.SetForPlugin(ctx, "alpha", model.SlotSidebarTop, model.SlotKindText,
		map[string]any{"text": "first"}, 0))
	require.NoError(t, svc.SetForPlugin(ctx, "alpha", model.SlotSidebarTop, model.SlotKindText,
		map[string]any{"text": "second"}, 0))

	got, err := svc.ListForViewer(ctx, true, 0)
	require.NoError(t, err)
	require.Len(t, got, 1, "同一插件对同一插槽必须只有一条")
	require.Contains(t, got[0].Props, "second")
}

// 两个插件写同一插槽互不覆盖。
func TestSetForPluginKeepsOtherPluginsEntries(t *testing.T) {
	svc := newSlotTestSvc(t)
	ctx := context.Background()

	require.NoError(t, svc.SetForPlugin(ctx, "alpha", model.SlotSidebarTop, model.SlotKindText,
		map[string]any{"text": "a"}, 0))
	require.NoError(t, svc.SetForPlugin(ctx, "beta", model.SlotSidebarTop, model.SlotKindText,
		map[string]any{"text": "b"}, 0))

	got, err := svc.ListForViewer(ctx, true, 0)
	require.NoError(t, err)
	require.Len(t, got, 2, "不同插件的条目必须并存")
}

// 卸载插件时清掉它写入的全部条目。
func TestDeleteByOwnerClearsPluginEntries(t *testing.T) {
	svc := newSlotTestSvc(t)
	ctx := context.Background()

	require.NoError(t, svc.SetForPlugin(ctx, "alpha", model.SlotSidebarTop, model.SlotKindText,
		map[string]any{"text": "a"}, 0))
	require.NoError(t, svc.SetForPlugin(ctx, "alpha", model.SlotSidebarBottom, model.SlotKindText,
		map[string]any{"text": "b"}, 0))
	require.NoError(t, svc.DeleteByOwner(ctx, "alpha"))

	got, err := svc.ListForViewer(ctx, true, 0)
	require.NoError(t, err)
	require.Empty(t, got)
}

// 等级可见性必须指定门槛，否则会渲染出一个谁也看不到的条目。
func TestCreateSlotEntryRejectsLevelVisibilityWithoutMinLevel(t *testing.T) {
	svc := newSlotTestSvc(t)
	_, err := svc.Create(context.Background(), service.SlotEntryInput{
		Slot: model.SlotSidebarTop, Kind: model.SlotKindText,
		Props:      map[string]any{"text": "x"},
		Visibility: model.NavVisibilityLevel, MinLevel: 0,
	}, "admin")
	require.Error(t, err)
}
