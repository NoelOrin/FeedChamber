package service

import (
	"context"
	"errors"
	"testing"

	"feedchamber/internal/metrics"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// Login failure metric coverage (openspec 2026-10-13-build-observability §9.6).
//
// feedchamber_login_failures_total{reason} is bounded to exactly three reasons,
// each fed by one distinct branch of authService.Login. These tests pin that
// contract by driving the real Login path instead of inspecting the埋点 lines:
//
//   - unknown_user  → FindByIdentifier returns repository.ErrNotFound
//   - no_credential → user exists, credential row missing
//   - bad_password  → credential exists, bcrypt comparison fails
//
// Every assertion is a delta, never an absolute value: the CounterVec is
// process-global and other tests (in this package or any other) may already
// have incremented it, so only the before/after difference is meaningful.

const (
	loginFailureTestUsername = "metrics_user"
	loginFailureTestEmail    = "metrics_user@example.com"
	loginFailureTestUserID   = "u-login-metrics"
	loginFailureTestPassword = "correct-horse-123"
	loginFailureTestWrong    = "wrong-horse-456"

	loginFailureMetricFamily = "feedchamber_login_failures_total"
	loginFailureMetricLabel  = "reason"
)

// loginFailureCounterValue reads one reason's counter straight from the metrics
// registry. Deliberately not prometheus/testutil: that import would drag in a
// new transitive dependency and rewrite go.mod/go.sum (same rationale as
// internal/pkg/ratelimit/ratelimit_test.go and internal/pkg/eventbus/trace_test.go).
func loginFailureCounterValue(t *testing.T, reason string) float64 {
	t.Helper()
	families, err := metrics.Registry.Gather()
	require.NoError(t, err, "gather metrics registry")
	for _, family := range families {
		if family.GetName() != loginFailureMetricFamily {
			continue
		}
		for _, m := range family.GetMetric() {
			for _, lp := range m.GetLabel() {
				if lp.GetName() != loginFailureMetricLabel || lp.GetValue() != reason {
					continue
				}
				if c := m.GetCounter(); c != nil {
					return c.GetValue()
				}
			}
		}
	}
	t.Fatalf("%s{%s=%q} is missing from the registry (init() must seed all three reasons)",
		loginFailureMetricFamily, loginFailureMetricLabel, reason)
	return 0
}

// loginFailureSnapshot reads the current value of all three reason counters.
func loginFailureSnapshot(t *testing.T) map[string]float64 {
	t.Helper()
	return map[string]float64{
		metrics.LoginReasonUnknownUser:  loginFailureCounterValue(t, metrics.LoginReasonUnknownUser),
		metrics.LoginReasonNoCredential: loginFailureCounterValue(t, metrics.LoginReasonNoCredential),
		metrics.LoginReasonBadPassword:  loginFailureCounterValue(t, metrics.LoginReasonBadPassword),
	}
}

// assertLoginFailureDelta requires the named reason to have advanced by exactly
// one and the other two reasons to be untouched. The untouched half is the real
// "reasons are not merged" assertion: if two branches shared a label value, one
// of the zero deltas below would come out as one.
func assertLoginFailureDelta(t *testing.T, before, after map[string]float64, wantReason string) {
	t.Helper()
	require.Len(t, after, 3)
	for _, reason := range []string{
		metrics.LoginReasonUnknownUser,
		metrics.LoginReasonNoCredential,
		metrics.LoginReasonBadPassword,
	} {
		want := 0.0
		if reason == wantReason {
			want = 1.0
		}
		assert.Equal(t, want, after[reason]-before[reason],
			"%s{reason=%q} delta", loginFailureMetricFamily, reason)
	}
}

// newLoginFailureAuth builds an authService over the invite/register test
// doubles with one user seeded. withCredential also seeds a bcrypt hash for
// loginFailureTestPassword. bcrypt.MinCost keeps hashing cheap; comparison cost
// comes from the hash itself, independent of the settings-driven bcrypt_cost.
//
// secMgr / banSvc / mfaSvc / emailSvc / bus stay nil — Login nil-guards every
// one of them, so the metric埋点 is reachable without extra fakes.
func newLoginFailureAuth(t *testing.T, withCredential bool) *authService {
	t.Helper()
	ctx := context.Background()

	users := newInviteTestUserRepo()
	require.NoError(t, users.Create(ctx, &model.User{
		ID:       loginFailureTestUserID,
		Username: loginFailureTestUsername,
		Email:    loginFailureTestEmail,
		Role:     "user",
	}))

	creds := newInviteTestCredRepo()
	if withCredential {
		hash, err := bcrypt.GenerateFromPassword([]byte(loginFailureTestPassword), bcrypt.MinCost)
		require.NoError(t, err)
		require.NoError(t, creds.Upsert(ctx, &model.Credential{
			UserID:       loginFailureTestUserID,
			PasswordHash: string(hash),
		}))
	}

	// newTestAuth always builds its own credRepo; swap in the one we seeded.
	svc := newTestAuth("all", nil, users)
	svc.credRepo = creds
	return svc
}

func TestLoginFailures_ReasonIsCountedOncePerBranch(t *testing.T) {
	cases := []struct {
		name       string
		identifier string
		password   string
		withCred   bool
		wantCode   pkg.ErrCode
		wantReason string
	}{
		{
			name:       "unknown_user",
			identifier: "ghost@example.com", // no such user/email in the repo
			password:   loginFailureTestPassword,
			withCred:   false,
			wantCode:   pkg.USER_NOT_FOUND,
			wantReason: metrics.LoginReasonUnknownUser,
		},
		{
			name:       "no_credential",
			identifier: loginFailureTestUsername, // user exists, credential row does not
			password:   loginFailureTestPassword,
			withCred:   false,
			wantCode:   pkg.INVALID_PASSWORD,
			wantReason: metrics.LoginReasonNoCredential,
		},
		{
			name:       "bad_password",
			identifier: loginFailureTestUsername, // user + credential exist, password differs
			password:   loginFailureTestWrong,
			withCred:   true,
			wantCode:   pkg.INVALID_PASSWORD,
			wantReason: metrics.LoginReasonBadPassword,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newLoginFailureAuth(t, tc.withCred)
			before := loginFailureSnapshot(t)

			resp, err := svc.Login(context.Background(), &LoginRequest{
				Login:     tc.identifier,
				Password:  tc.password,
				IP:        "1.2.3.4",
				UserAgent: "test",
			})

			require.Error(t, err)
			assert.Nil(t, resp)
			var appErr *pkg.AppError
			require.True(t, errors.As(err, &appErr), "expected *pkg.AppError, got %T", err)
			assert.Equal(t, tc.wantCode, appErr.Code)

			assertLoginFailureDelta(t, before, loginFailureSnapshot(t), tc.wantReason)
		})
	}
}

// TestLoginFailures_SuccessDoesNotCount guards the successful path: a valid
// login issues a session credential and must leave all three reasons flat.
func TestLoginFailures_SuccessDoesNotCount(t *testing.T) {
	svc := newLoginFailureAuth(t, true)
	before := loginFailureSnapshot(t)

	resp, err := svc.Login(context.Background(), &LoginRequest{
		Login:     loginFailureTestUsername,
		Password:  loginFailureTestPassword,
		IP:        "1.2.3.4",
		UserAgent: "test",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.Token, "successful login must issue a session credential")

	// wantReason "" means every reason must stay put.
	assertLoginFailureDelta(t, before, loginFailureSnapshot(t), "")
}

// TestLoginFailureReasonsAreDistinct pins the label values themselves. If two
// branches collapsed onto one reason constant, the per-case deltas above would
// individually still pass while the series silently merged.
func TestLoginFailureReasonsAreDistinct(t *testing.T) {
	reasons := []string{
		metrics.LoginReasonUnknownUser,
		metrics.LoginReasonNoCredential,
		metrics.LoginReasonBadPassword,
	}
	seen := map[string]struct{}{}
	for _, r := range reasons {
		assert.NotEmpty(t, r)
		seen[r] = struct{}{}
	}
	assert.Len(t, seen, 3, "login failure reasons must be three distinct label values")
}
