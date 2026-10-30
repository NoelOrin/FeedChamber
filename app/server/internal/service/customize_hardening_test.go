package service

import (
	"context"
	"testing"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newCustomizeServiceForTest 组装一个全部依赖都是内存替身的 CustomizeService。
func newCustomizeServiceForTest() (CustomizeService, *memSnippetRepo, *memTokenRepo) {
	snips := newMemSnippetRepo()
	tokens := newMemTokenRepo()
	svc := NewCustomizeService(snips, tokens, newMemNavRepo(), newMemModuleRepo(), newMemPageRepo(), nil)
	return svc, snips, tokens
}

// 注入面：片段内容必须与它被内联的容器相容。
//
// 判定在写入侧（服务端是策略边界），渲染侧只负责按 kind 选容器。
// 这三条断言曾经全部不成立：JS 里带 </script> 会截断 <script> 标签、
// CSS 里带 </style> 会闭合 <style>、html 片段的 position 会被原样存成 head
// 而读取侧按 position 列过滤 → 内容永不渲染。

func TestCreateSnippetRejectsScriptTerminatorInJS(t *testing.T) {
	svc, _, _ := newCustomizeServiceForTest()
	_, _, err := svc.CreateSnippet(context.Background(), SnippetInput{
		Kind:    model.SnippetKindJS,
		Content: `console.log("</script><img src=x onerror=alert(1)>")`,
	}, WriteActor{UserID: "u1", Source: WriteSourceUI})

	require.Error(t, err)
	assert.Equal(t, pkg.INVALID_PARAMS, err.(*pkg.AppError).Code)
}

func TestCreateSnippetRejectsStyleTerminatorInCSS(t *testing.T) {
	svc, _, _ := newCustomizeServiceForTest()
	_, _, err := svc.CreateSnippet(context.Background(), SnippetInput{
		Kind:    model.SnippetKindCSS,
		Content: `body{color:red}</style><script>alert(1)</script>`,
	}, WriteActor{UserID: "u1", Source: WriteSourceUI})

	require.Error(t, err)
	assert.Equal(t, pkg.INVALID_PARAMS, err.(*pkg.AppError).Code)
}

func TestCreateSnippetNormalizesHTMLPositionToBodyEnd(t *testing.T) {
	svc, snippets, _ := newCustomizeServiceForTest()
	got, _, err := svc.CreateSnippet(context.Background(), SnippetInput{
		Kind:     model.SnippetKindHTML,
		Position: model.SnippetPositionHead, // 后台表单默认值，必须被纠正
		Content:  "<div>hi</div>",
		Enabled:  true,
	}, WriteActor{UserID: "u1", Source: WriteSourceUI})

	require.NoError(t, err)
	assert.Equal(t, model.SnippetPositionBodyEnd, got.Position)
	assert.Equal(t, model.SnippetPositionBodyEnd, snippets.items[got.ID].Position,
		"落库位置也要是归一化后的值，读取侧按列过滤")
}

// 历史数据可能已经把 html 存成 head；读取侧必须按 EffectivePosition 分组，
// 否则这些片段永远发不出去。
func TestActiveSnippetsGroupsByEffectivePosition(t *testing.T) {
	svc, snippets, _ := newCustomizeServiceForTest()
	ctx := context.Background()

	legacy := &model.Snippet{
		ID: "s-legacy", Kind: model.SnippetKindHTML, Position: model.SnippetPositionHead,
		Content: "<p>legacy</p>", Enabled: true,
	}
	require.NoError(t, snippets.Create(ctx, legacy))

	bodyEnd, err := svc.ActiveSnippets(ctx, model.SnippetPositionBodyEnd)
	require.NoError(t, err)
	require.Len(t, bodyEnd, 1)
	assert.Equal(t, "s-legacy", bodyEnd[0].ID)

	head, err := svc.ActiveSnippets(ctx, model.SnippetPositionHead)
	require.NoError(t, err)
	assert.Empty(t, head, "html 片段不得出现在 head 分组")
}

// 主题 token 是 CSS 自定义属性，不是任意 CSS：名字必须是 --x，值不能闭合声明块。
func TestSetThemeTokensValidatesNameAndValue(t *testing.T) {
	svc, _, _ := newCustomizeServiceForTest()
	ctx := context.Background()

	cases := []struct {
		name  string
		item  ThemeTokenInput
		valid bool
	}{
		{"正常", ThemeTokenInput{Token: "--primary", Value: "#7c3aed"}, true},
		{"值含分号", ThemeTokenInput{Token: "--primary", Value: "red;background:url(x)"}, false},
		{"值闭合 style", ThemeTokenInput{Token: "--primary", Value: "red</style><script>1</script>"}, false},
		{"值含花括号", ThemeTokenInput{Token: "--primary", Value: "red}"}, false},
		{"名不以 -- 开头", ThemeTokenInput{Token: "primary", Value: "red"}, false},
		{"名含注入字符", ThemeTokenInput{Token: "--a}", Value: "red"}, false},
		{"名含分号", ThemeTokenInput{Token: "--a;color:red", Value: "red"}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.SetThemeTokens(ctx, []ThemeTokenInput{tc.item},
				WriteActor{UserID: "u1", Source: WriteSourceUI})
			if tc.valid {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Equal(t, pkg.INVALID_PARAMS, err.(*pkg.AppError).Code)
		})
	}
}

// 导航 href 会被渲染进 <Link href>：`javascript:` 在点击时就地执行。
// 后台表单与 MCP 工具都能写它，因此只白名单站内路径与 http(s)。
func TestValidateNavHrefRejectsScriptSchemes(t *testing.T) {
	svc, _, _ := newCustomizeServiceForTest()
	ctx := context.Background()

	cases := map[string]bool{
		"/timeline":               true,
		"/spaces/go":              true,
		"https://example.com/x":   true,
		"http://example.com/x":    true,
		"javascript:alert(1)":     false,
		" JavaScript:alert(1)":    false,
		"data:text/html,<script>": false,
		"//evil.example.com/x":    false,
		"timeline":                false,
	}

	for href, valid := range cases {
		_, _, err := svc.CreateNavItem(ctx, NavItemInput{Label: "x", Href: href},
			WriteActor{UserID: "u1", Source: WriteSourceUI})
		if valid {
			assert.NoError(t, err, "href=%q 应放行", href)
			continue
		}
		require.Error(t, err, "href=%q 应被拒", href)
		assert.Equal(t, pkg.INVALID_PARAMS, err.(*pkg.AppError).Code)
	}
}
