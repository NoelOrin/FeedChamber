package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"feedchamber/internal/db"

	"gorm.io/gorm"
)

const backupSchemaV2 = "feedchamber-backup-v2"

type backupSnapshot struct {
	Schema    string         `json:"schema"`
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	CreatedAt time.Time      `json:"created_at"`
	Backend   db.BackendType `json:"backend"`
	Tables    []backupTable  `json:"tables"`
}

type backupTable struct {
	Name string            `json:"name"`
	Rows []json.RawMessage `json:"rows"`
}

func buildBackupSnapshot(ctx context.Context, store db.Store, id, name string) (*backupSnapshot, error) {
	snap := &backupSnapshot{
		Schema:    backupSchemaV2,
		ID:        id,
		Name:      name,
		CreatedAt: time.Now().UTC(),
		Backend:   store.Type(),
	}
	var err error
	snap.Tables, err = exportSQLSnapshot(ctx, store.SQL())
	return snap, err
}

func exportSQLSnapshot(ctx context.Context, sql db.SQLStore) ([]backupTable, error) {
	db := sql.DB()
	tables, err := db.Migrator().GetTables()
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	result := make([]backupTable, 0, len(tables))
	for _, table := range tables {
		if table == "backups" {
			continue
		}
		var rows []map[string]interface{}
		if err := db.WithContext(ctx).Table(table).Find(&rows).Error; err != nil {
			return nil, fmt.Errorf("read table %s: %w", table, err)
		}
		encoded := make([]json.RawMessage, 0, len(rows))
		for _, row := range rows {
			raw, err := json.Marshal(row)
			if err != nil {
				return nil, fmt.Errorf("encode table %s: %w", table, err)
			}
			encoded = append(encoded, raw)
		}
		result = append(result, backupTable{Name: table, Rows: encoded})
	}
	return result, nil
}

func restoreBackupSnapshot(ctx context.Context, store db.Store, snap *backupSnapshot) error {
	if snap.Schema != backupSchemaV2 {
		return fmt.Errorf("unsupported backup schema: %s", snap.Schema)
	}
	if snap.Backend != store.Type() {
		return fmt.Errorf("backup backend %s does not match current backend %s", snap.Backend, store.Type())
	}
	return restoreSQLSnapshot(ctx, store.SQL(), store.Type(), snap.Tables)
}

func restoreSQLSnapshot(ctx context.Context, sql db.SQLStore, backend db.BackendType, tables []backupTable) error {
	gdb := sql.DB()
	return gdb.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		switch backend {
		case db.BackendSQLite:
			if err := tx.Exec("PRAGMA foreign_keys = OFF").Error; err != nil {
				return err
			}
		case db.BackendMySQL:
			if err := tx.Exec("SET FOREIGN_KEY_CHECKS = 0").Error; err != nil {
				return err
			}
		case db.BackendPostgres:
			if err := tx.Exec("SET session_replication_role = replica").Error; err != nil {
				return err
			}
		}

		current, err := tx.Migrator().GetTables()
		if err != nil {
			return err
		}
		currentSet := make(map[string]struct{}, len(current))
		for _, name := range current {
			currentSet[name] = struct{}{}
			if name != "backups" {
				if err := tx.Table(name).Where("1 = 1").Delete(nil).Error; err != nil {
					return fmt.Errorf("clear table %s: %w", name, err)
				}
			}
		}

		for _, table := range tables {
			if table.Name == "" || table.Name == "backups" {
				continue
			}
			if _, ok := currentSet[table.Name]; !ok {
				return fmt.Errorf("table %s is not present in current database", table.Name)
			}
			for _, raw := range table.Rows {
				if len(raw) == 0 || string(raw) == "null" {
					continue
				}
				var row map[string]interface{}
				if err := json.Unmarshal(raw, &row); err != nil {
					return fmt.Errorf("decode table %s: %w", table.Name, err)
				}
				if err := tx.Table(table.Name).Create(&row).Error; err != nil {
					return fmt.Errorf("restore table %s: %w", table.Name, err)
				}
			}
		}
		return nil
	})
}
