package pkg

import (
	"net/http"
	"testing"
	"time"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/httptest"
)

func TestCookieNames(t *testing.T) {
	if AuthFlagCookie != "master_auth" {
		t.Errorf("AuthFlagCookie = %q, want %q", AuthFlagCookie, "master_auth")
	}
	if SessionCookie != "master_session" {
		t.Errorf("SessionCookie = %q, want %q", SessionCookie, "master_session")
	}
}

// do writes/clears/reads the session cookie inside a handler and returns the
// resulting Set-Cookie headers (and the value read back from the request).
func do(t *testing.T, mutate func(iris.Context), read func(iris.Context) string) (*http.Response, string) {
	t.Helper()
	app := iris.New()
	var got string
	app.Get("/", func(ctx iris.Context) {
		if mutate != nil {
			mutate(ctx)
		}
		got = read(ctx)
		ctx.StatusCode(http.StatusOK)
	})
	e := httptest.New(t, app)
	resp := e.GET("/").Expect().Status(http.StatusOK).Raw()
	return resp, got
}

func TestSetSessionCookie(t *testing.T) {
	resp, _ := do(t,
		func(ctx iris.Context) { SetSessionCookie(ctx, "abc123", time.Hour) },
		func(iris.Context) string { return "" })

	cookies := resp.Cookies()
	var session, flag *http.Cookie
	for _, c := range cookies {
		switch c.Name {
		case SessionCookie:
			session = c
		case AuthFlagCookie:
			flag = c
		}
	}
	if session == nil || session.Value != "abc123" {
		t.Fatalf("master_session cookie missing or wrong: %#v", session)
	}
	if !session.HttpOnly {
		t.Error("master_session must be HttpOnly")
	}
	if session.Path != "/" {
		t.Errorf("Path = %q, want /", session.Path)
	}
	if session.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", session.SameSite)
	}
	if flag == nil || flag.Value != "1" || flag.HttpOnly {
		t.Errorf("master_auth flag cookie wrong: %#v", flag)
	}
}

func TestClearSessionCookie(t *testing.T) {
	resp, _ := do(t, ClearSessionCookie, func(iris.Context) string { return "" })
	for _, c := range resp.Cookies() {
		if c.Name == SessionCookie && c.Value != "" {
			t.Errorf("master_session should be cleared, got %q", c.Value)
		}
	}
}

func TestReadSessionCookie(t *testing.T) {
	_, got := do(t, nil, ReadSessionCookie)
	if got != "" {
		t.Errorf("ReadSessionCookie on empty request = %q, want empty", got)
	}
}
