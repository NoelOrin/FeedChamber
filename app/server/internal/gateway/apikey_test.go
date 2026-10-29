package gateway

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"feedchamber/internal/model"

	"golang.org/x/crypto/bcrypt"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/httptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeAPIScopes(t *testing.T) {
	got := normalizeAPIScopes(" write,read,post:read,read, ")
	require.Equal(t, []string{"post:read", "read", "write"}, got)
}

func TestValidAPIScopes(t *testing.T) {
	require.True(t, validAPIScopes([]string{"read", "post:read"}))
	require.False(t, validAPIScopes(nil))
	require.False(t, validAPIScopes([]string{"*"}))
	require.False(t, validAPIScopes([]string{"noscope"}))
}

func TestAPIKeyHasPermission(t *testing.T) {
	require.True(t, apiKeyHasPermission("read", model.PermPostRead))
	require.False(t, apiKeyHasPermission("read", model.PermPostCreate))
	require.True(t, apiKeyHasPermission("write", model.PermPostCreate))
	require.True(t, apiKeyHasPermission("admin", model.PermRoleManage))
	require.True(t, apiKeyHasPermission("post:read,report:manage", model.PermReportManage))
	require.False(t, apiKeyHasPermission("", model.PermPostRead))
	require.False(t, apiKeyHasPermission("read", strings.ToUpper(model.PermPostRead)))
}

// mockAPIKeySettingReader implements SettingReader for tests.
type mockAPIKeySettingReader struct {
	settings map[string]string
}

func newMockSettingReader() *mockAPIKeySettingReader {
	return &mockAPIKeySettingReader{
		settings: map[string]string{
			"api_key_enabled": "true",
		},
	}
}

func (m *mockAPIKeySettingReader) Get(ctx context.Context, key string) (string, error) {
	return m.settings[key], nil
}

func TestAPIKeyManager_KeyFormat(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("ak_testkey123"), bcrypt.DefaultCost)
	require.NoError(t, err)

	err = bcrypt.CompareHashAndPassword(hash, []byte("ak_testkey123"))
	assert.NoError(t, err)

	err = bcrypt.CompareHashAndPassword(hash, []byte("ak_wrongkey"))
	assert.Error(t, err)

	// Verify "ak_" prefix is part of the hashed material
	hash2, err := bcrypt.GenerateFromPassword([]byte("ak_abcdef"), bcrypt.DefaultCost)
	require.NoError(t, err)

	err = bcrypt.CompareHashAndPassword(hash2, []byte("ak_abcdef"))
	assert.NoError(t, err)

	err = bcrypt.CompareHashAndPassword(hash2, []byte("abcdef"))
	assert.Error(t, err)
}

func TestAPIKeyManager_Middleware_NoHeader(t *testing.T) {
	mgr := NewAPIKeyManager(Deps{SettingService: newMockSettingReader()})
	app := iris.New()
	app.Use(func(ctx iris.Context) {
		if !mgr.Middleware(ctx) {
			return
		}
		ctx.Next()
	})
	app.Get("/test", func(ctx iris.Context) {
		ctx.JSON(iris.Map{"ok": true})
	})

	e := httptest.New(t, app)
	e.GET("/test").Expect().Status(http.StatusUnauthorized)
}

func TestAPIKeyManager_Middleware_InvalidKey(t *testing.T) {
	mgr := NewAPIKeyManager(Deps{SettingService: newMockSettingReader()})
	app := iris.New()
	app.Use(func(ctx iris.Context) {
		if !mgr.Middleware(ctx) {
			return
		}
		ctx.Next()
	})
	app.Get("/test", func(ctx iris.Context) {
		ctx.JSON(iris.Map{"ok": true})
	})

	e := httptest.New(t, app)
	e.GET("/test").WithHeader("Authorization", "Bearer ak_invalid").Expect().Status(http.StatusInternalServerError)
}

func TestAPIKeyManager_Middleware_WrongPrefix(t *testing.T) {
	mgr := NewAPIKeyManager(Deps{SettingService: newMockSettingReader()})
	app := iris.New()
	app.Use(func(ctx iris.Context) {
		if !mgr.Middleware(ctx) {
			return
		}
		ctx.Next()
	})
	app.Get("/test", func(ctx iris.Context) {
		ctx.JSON(iris.Map{"ok": true})
	})

	e := httptest.New(t, app)
	e.GET("/test").WithHeader("Authorization", "Bearer sk_test").Expect().Status(http.StatusUnauthorized)
}

func TestAPIKeyManager_Middleware_Disabled(t *testing.T) {
	mockReader := newMockSettingReader()
	mockReader.settings["api_key_enabled"] = "false"
	mgr := NewAPIKeyManager(Deps{SettingService: mockReader})

	app := iris.New()
	app.Use(func(ctx iris.Context) {
		if !mgr.Middleware(ctx) {
			return
		}
		ctx.Next()
	})
	app.Get("/test", func(ctx iris.Context) {
		ctx.JSON(iris.Map{"ok": true})
	})

	e := httptest.New(t, app)
	e.GET("/test").WithHeader("X-API-Key", "ak_something").Expect().Status(http.StatusForbidden)
}

func TestAPIKeyManager_Middleware_XAPIKeyHeader(t *testing.T) {
	mgr := NewAPIKeyManager(Deps{SettingService: newMockSettingReader()})
	app := iris.New()
	app.Use(func(ctx iris.Context) {
		if !mgr.Middleware(ctx) {
			return
		}
		ctx.Next()
	})
	app.Get("/test", func(ctx iris.Context) {
		ctx.JSON(iris.Map{"ok": true})
	})

	e := httptest.New(t, app)
	e.GET("/test").WithHeader("X-API-Key", "ak_something").Expect().Status(http.StatusInternalServerError)
}

func TestAPIKeyManager_Middleware_BearerHeader(t *testing.T) {
	mgr := NewAPIKeyManager(Deps{SettingService: newMockSettingReader()})
	app := iris.New()
	app.Use(func(ctx iris.Context) {
		if !mgr.Middleware(ctx) {
			return
		}
		ctx.Next()
	})
	app.Get("/test", func(ctx iris.Context) {
		ctx.JSON(iris.Map{"ok": true})
	})

	e := httptest.New(t, app)
	e.GET("/test").WithHeader("Authorization", "Bearer ak_something").Expect().Status(http.StatusInternalServerError)
}

func TestAPIKeyManager_Middleware_InvalidAuthFormat(t *testing.T) {
	mgr := NewAPIKeyManager(Deps{SettingService: newMockSettingReader()})
	app := iris.New()
	app.Use(func(ctx iris.Context) {
		if !mgr.Middleware(ctx) {
			return
		}
		ctx.Next()
	})
	app.Get("/test", func(ctx iris.Context) {
		ctx.JSON(iris.Map{"ok": true})
	})

	e := httptest.New(t, app)
	e.GET("/test").WithHeader("Authorization", "Basic dGVzdDp0ZXN0").Expect().Status(http.StatusUnauthorized)
}

func TestAPIKeyManager_Middleware_NoSettingService(t *testing.T) {
	mgr := NewAPIKeyManager(Deps{})
	app := iris.New()
	app.Use(func(ctx iris.Context) {
		if !mgr.Middleware(ctx) {
			return
		}
		ctx.Next()
	})
	app.Get("/test", func(ctx iris.Context) {
		ctx.JSON(iris.Map{"ok": true})
	})

	e := httptest.New(t, app)
	e.GET("/test").WithHeader("X-API-Key", "ak_something").Expect().Status(http.StatusForbidden)
}
