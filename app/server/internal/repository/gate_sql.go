package repository

import (
	"context"

	"orinforum/internal/db"
	"orinforum/internal/model"
)

type sqlGateRepo struct {
	db db.SQLStore
}

func (r *sqlGateRepo) ListRules(ctx context.Context) ([]model.GateRule, error) {
	var rules []model.GateRule
	err := r.db.DB().WithContext(ctx).Order("priority DESC, id ASC").Find(&rules).Error
	return rules, err
}

func (r *sqlGateRepo) GetRule(ctx context.Context, id string) (*model.GateRule, error) {
	var rule model.GateRule
	if err := r.db.DB().WithContext(ctx).Where("id = ?", id).First(&rule).Error; err != nil {
		return nil, WrapDBErr(err)
	}
	return &rule, nil
}

func (r *sqlGateRepo) CreateRule(ctx context.Context, rule *model.GateRule) error {
	return r.db.DB().WithContext(ctx).Create(rule).Error
}

func (r *sqlGateRepo) UpdateRule(ctx context.Context, rule *model.GateRule) error {
	return r.db.DB().WithContext(ctx).Save(rule).Error
}

func (r *sqlGateRepo) DeleteRule(ctx context.Context, id string) error {
	res := r.db.DB().WithContext(ctx).Where("id = ?", id).Delete(&model.GateRule{})
	if res.Error != nil {
		return WrapDBErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *sqlGateRepo) ListDomainDefaults(ctx context.Context) ([]model.GateDomainDefault, error) {
	var defs []model.GateDomainDefault
	err := r.db.DB().WithContext(ctx).Order("domain ASC").Find(&defs).Error
	return defs, err
}

func (r *sqlGateRepo) UpsertDomainDefault(ctx context.Context, def *model.GateDomainDefault) error {
	return r.db.DB().WithContext(ctx).Save(def).Error
}
