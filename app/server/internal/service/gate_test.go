package service

import (
	"context"
	"path/filepath"
	"testing"

	"orinforum/internal/config"
	"orinforum/internal/db"
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newGateTestService(t *testing.T) (GateService, db.Store) {
	t.Helper()
	store, err := db.New(config.DBConfig{Type: "sqlite", DSN: filepath.Join(t.TempDir(), "gate.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.AutoMigrate(&model.User{}, &model.GateRule{}, &model.GateDomainDefault{}))
	svc := NewGateService(
		repository.NewGateRepository(store),
		repository.NewUserRepository(store),
		repository.NewTrustRepository(store),
		nil, // 限流动作在单测里不触发（nil limiter 是 no-op）
	)
	return svc, store
}

func mustRule(t *testing.T, svc GateService, rule model.GateRule) {
	t.Helper()
	require.NoError(t, svc.SaveRule(context.Background(), &rule, "admin"))
}

// 首个匹配 + 高优先级例外放行。
func TestGate_FirstMatchAndPriorityException(t *testing.T) {
	svc, _ := newGateTestService(t)
	ctx := context.Background()

	mustRule(t, svc, model.GateRule{
		ID: "deny-tl0", Priority: 10, Enabled: true, Domain: model.GateDomainPostCreate,
		Conditions: `[{"field":"level","op":"eq","value":0}]`, Action: model.GateActionDeny, Note: "TL0 停发",
	})
	mustRule(t, svc, model.GateRule{
		ID: "allow-author", Priority: 100, Enabled: true, Domain: model.GateDomainPostCreate,
		// 同一批 TL0 用户里，作者本人放行 —— 证明是「首个匹配」而非「取最严」。
		Conditions: `[{"field":"level","op":"eq","value":0},{"field":"is_author","op":"eq","value":true}]`,
		Action:     model.GateActionAllow,
	})

	d, err := svc.Evaluate(ctx, model.GateDomainPostCreate, GateInput{UserID: "u1", Level: 0, IsAuthor: true})
	require.NoError(t, err)
	assert.Equal(t, model.GateActionAllow, d.Action)
	assert.Equal(t, "allow-author", d.RuleID)

	d, err = svc.Evaluate(ctx, model.GateDomainPostCreate, GateInput{UserID: "u2", Level: 0})
	require.NoError(t, err)
	assert.Equal(t, model.GateActionDeny, d.Action)
	assert.Equal(t, "deny-tl0", d.RuleID)

	// 无命中走域默认，并标注 is_default。
	d, err = svc.Evaluate(ctx, model.GateDomainPostCreate, GateInput{UserID: "u3", Level: 2})
	require.NoError(t, err)
	assert.Equal(t, model.GateActionAllow, d.Action)
	assert.True(t, d.IsDefault)
}

// 不同能力域可以有不同默认动作。
func TestGate_DomainDefaults(t *testing.T) {
	svc, _ := newGateTestService(t)
	ctx := context.Background()
	require.NoError(t, svc.SaveDomainDefault(ctx, model.GateDomainExternalLink, model.GateActionDeny))
	require.NoError(t, svc.SaveDomainDefault(ctx, model.GateDomainPostCreate, model.GateActionAllow))

	link, err := svc.Evaluate(ctx, model.GateDomainExternalLink, GateInput{Level: 3})
	require.NoError(t, err)
	assert.Equal(t, model.GateActionDeny, link.Action)
	assert.True(t, link.IsDefault)

	post, err := svc.Evaluate(ctx, model.GateDomainPostCreate, GateInput{Level: 3})
	require.NoError(t, err)
	assert.Equal(t, model.GateActionAllow, post.Action)
	assert.True(t, post.IsDefault)
}

// 规则校验：非法谓词/动作、空条件、未知字段、优先级冲突。
func TestGate_Validation(t *testing.T) {
	svc, _ := newGateTestService(t)
	ctx := context.Background()
	mustRule(t, svc, model.GateRule{
		ID: "base", Priority: 10, Enabled: true, Domain: model.GateDomainPostCreate,
		Conditions: `[{"field":"level","op":"eq","value":0}]`, Action: model.GateActionDeny,
	})

	cases := []struct {
		name string
		rule model.GateRule
	}{
		{"未知字段", model.GateRule{Domain: model.GateDomainPostCreate, Priority: 20, Enabled: true, Conditions: `[{"field":"nope","op":"eq","value":1}]`, Action: model.GateActionDeny}},
		{"未知动作", model.GateRule{Domain: model.GateDomainPostCreate, Priority: 20, Enabled: true, Conditions: `[{"field":"level","op":"eq","value":0}]`, Action: "hide"}},
		{"空条件", model.GateRule{Domain: model.GateDomainPostCreate, Priority: 20, Enabled: true, Conditions: `[]`, Action: model.GateActionDeny}},
		{"未知域", model.GateRule{Domain: "nope", Priority: 20, Enabled: true, Conditions: `[{"field":"level","op":"eq","value":0}]`, Action: model.GateActionDeny}},
		{"不支持的操作符", model.GateRule{Domain: model.GateDomainPostCreate, Priority: 20, Enabled: true, Conditions: `[{"field":"contains_external_link","op":"gt","value":1}]`, Action: model.GateActionDeny}},
		{"优先级冲突", model.GateRule{Domain: model.GateDomainPostCreate, Priority: 10, Enabled: true, Conditions: `[{"field":"level","op":"eq","value":1}]`, Action: model.GateActionAllow}},
		{"死条件", model.GateRule{Domain: model.GateDomainPostCreate, Priority: 20, Enabled: true, Conditions: `[{"field":"level","op":"eq","value":-1}]`, Action: model.GateActionDeny}},
		{"require_review 用错域", model.GateRule{Domain: model.GateDomainExternalLink, Priority: 20, Enabled: true, Conditions: `[{"field":"level","op":"eq","value":0}]`, Action: model.GateActionRequireReview}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rule := tc.rule
			err := svc.SaveRule(ctx, &rule, "admin")
			require.Error(t, err)
			appErr, ok := err.(*pkg.AppError)
			require.True(t, ok)
			assert.Equal(t, pkg.INVALID_PARAMS, appErr.Code)
		})
	}
}

// 试算与线上求值一致（同一个 Evaluate）。
func TestGate_SimulateMatchesEvaluate(t *testing.T) {
	svc, store := newGateTestService(t)
	ctx := context.Background()
	require.NoError(t, store.SQL().DB().Create(&model.User{
		ID: "u1", Username: "u1", Email: "u1@x.com", Role: "user", TrustLevel: 0,
	}).Error)
	mustRule(t, svc, model.GateRule{
		ID: "allow-author", Priority: 100, Enabled: true, Domain: model.GateDomainPostCreate,
		Conditions: `[{"field":"is_author","op":"eq","value":true}]`, Action: model.GateActionAllow,
	})
	mustRule(t, svc, model.GateRule{
		ID: "deny-tl0", Priority: 10, Enabled: true, Domain: model.GateDomainPostCreate,
		Conditions: `[{"field":"level","op":"eq","value":0}]`, Action: model.GateActionDeny,
	})

	sim, err := svc.Simulate(ctx, GateSimulateInput{UserID: "u1", Domain: model.GateDomainPostCreate, IsAuthor: true})
	require.NoError(t, err)
	assert.Equal(t, "allow-author", sim.RuleID)

	sim, err = svc.Simulate(ctx, GateSimulateInput{UserID: "u1", Domain: model.GateDomainPostCreate})
	require.NoError(t, err)
	assert.Equal(t, "deny-tl0", sim.RuleID)

	direct, err := svc.Evaluate(ctx, model.GateDomainPostCreate, GateInput{UserID: "u1", Level: 0})
	require.NoError(t, err)
	assert.Equal(t, direct.Action, sim.Action)
}

// 外链规则：TL0 命中 deny，等级 ≥1 由显式 allow 规则放行。
func TestGate_ExternalLinkSeedShape(t *testing.T) {
	svc, store := newGateTestService(t)
	ctx := context.Background()
	for i := range model.DefaultGateRules {
		require.NoError(t, store.SQL().DB().Create(&model.DefaultGateRules[i]).Error)
	}
	link := "see https://example.com"
	d, err := svc.Evaluate(ctx, model.GateDomainExternalLink, GateInput{UserID: "u1", Level: 0, ContainsExternalLink: ContainsExternalLink(link)})
	require.NoError(t, err)
	assert.Equal(t, model.GateActionDeny, d.Action)

	d, err = svc.Evaluate(ctx, model.GateDomainExternalLink, GateInput{UserID: "u2", Level: 1, ContainsExternalLink: ContainsExternalLink(link)})
	require.NoError(t, err)
	assert.Equal(t, model.GateActionAllow, d.Action)
}
