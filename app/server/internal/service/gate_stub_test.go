package service

import (
	"context"

	"feedchamber/internal/model"
)

// stubGateService 只实现测试用到的求值路径：固定返回一个动作。
type stubGateService struct {
	decision GateDecision
}

func (s *stubGateService) Evaluate(context.Context, string, GateInput) (GateDecision, error) {
	return s.decision, nil
}
func (s *stubGateService) EnforceRateLimit(context.Context, GateDecision, GateInput) error {
	return nil
}
func (s *stubGateService) ListRules(context.Context) ([]model.GateRule, error)     { return nil, nil }
func (s *stubGateService) SaveRule(context.Context, *model.GateRule, string) error { return nil }
func (s *stubGateService) DeleteRule(context.Context, string) error                { return nil }
func (s *stubGateService) ListDomainDefaults(context.Context) ([]model.GateDomainDefault, error) {
	return nil, nil
}
func (s *stubGateService) SaveDomainDefault(context.Context, string, string) error { return nil }
func (s *stubGateService) Simulate(context.Context, GateSimulateInput) (GateDecision, error) {
	return s.decision, nil
}
func (s *stubGateService) StartSyncSubscriber(context.Context) error { return nil }
