package cmd

import (
	"context"
	"fmt"

	"orinforum/internal/config"
	"orinforum/internal/db"
	"orinforum/internal/repository"
	"orinforum/internal/service"

	"github.com/spf13/cobra"
)

// recomputeXPCmd 全量重算 XP / 等级。可重复执行：XP 由行为聚合派生，重算不会
// 重复计分。用途：存量迁移、改过 XP 规则/等级定义后的一次性对账。
var recomputeXPCmd = &cobra.Command{
	Use:   "recompute-xp",
	Short: "Recompute every user's XP and trust level from behaviour aggregates",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Load()
		store, err := db.New(cfg.DB)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer store.Close()

		svc := service.NewTrustService(repository.NewUserRepository(store), repository.NewTrustRepository(store), nil, nil)
		n, err := svc.RecomputeAll(context.Background())
		if err != nil {
			return err
		}
		fmt.Printf("recomputed %d users\n", n)
		return nil
	},
}

func init() {
	RootCmd.AddCommand(recomputeXPCmd)
}
