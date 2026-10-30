package gateway

import (
	"context"
	"net/http"
	"testing"

	"feedchamber/internal/authz"
	"feedchamber/internal/pkg"

	"golang.org/x/crypto/bcrypt"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/httptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// authz.CredentialFrom 把 iris context 的凭证信息翻译成判定输入。
// 两种机器凭证的 scope 来自不同的键，取错就会变成「空 scope 却放行」或「全拒」。
func TestCredentialOfReadsScopesPerAuthMethod(t *testing.T) {
	var got authz.Credential
	app := iris.New()
	app.Get("/probe", func(ctx iris.Context) {
		ctx.Values().Set(pkg.CtxAuthMethod, pkg.AuthMethodAPIKey)
		ctx.Values().Set(pkg.CtxRole, "admin")
		ctx.Values().Set(pkg.CtxAPIKeyScopes, "read,post:read")
		ctx.Values().Set(pkg.CtxClientScopes, "client-only")
		got = authz.CredentialFrom(ctx.Values())
	})
	app.Get("/probe-client", func(ctx iris.Context) {
		ctx.Values().Set(pkg.CtxAuthMethod, pkg.AuthMethodOAuthClient)
		ctx.Values().Set(pkg.CtxAPIKeyScopes, "should-not-be-used")
		ctx.Values().Set(pkg.CtxClientScopes, "read")
		got = authz.CredentialFrom(ctx.Values())
	})

	e := httptest.New(t, app)
	e.GET("/probe").Expect().Status(http.StatusOK)
	assert.Equal(t, authz.Credential{Method: pkg.AuthMethodAPIKey, Scopes: "read,post:read", Role: "admin"}, got)

	e.GET("/probe-client").Expect().Status(http.StatusOK)
	assert.Equal(t, authz.Credential{Method: pkg.AuthMethodOAuthClient, Scopes: "read"}, got)
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
