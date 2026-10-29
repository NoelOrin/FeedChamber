package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"orinforum/internal/config"

	"github.com/spf13/cobra"
)

var backupOut string

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Export a local site backup (DB snapshot + uploads inventory)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(backupOut) == "" {
			return fmt.Errorf("--out is required")
		}
		cfg := config.Load()
		outDir, err := filepath.Abs(backupOut)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return err
		}

		// 1) Database snapshot
		dbType := strings.ToLower(strings.TrimSpace(cfg.DB.Type))
		dbPath := ""
		switch dbType {
		case "sqlite", "":
			dbType = "sqlite"
			dbPath = cfg.DB.DSN
			if dbPath == "" {
				// default local sqlite path used by dev
				dbPath = "db/app.db"
			}
			// strip sqlite DSN prefixes if any
			dbPath = strings.TrimPrefix(dbPath, "file:")
			if i := strings.Index(dbPath, "?"); i >= 0 {
				dbPath = dbPath[:i]
			}
			src, err := filepath.Abs(dbPath)
			if err != nil {
				return err
			}
			dst := filepath.Join(outDir, "app.db")
			if err := copyFile(src, dst); err != nil {
				return fmt.Errorf("copy sqlite db: %w (src=%s)", err, src)
			}
		default:
			// Non-sqlite: write a note file; full dump tools differ by engine.
			note := filepath.Join(outDir, "db-note.txt")
			content := fmt.Sprintf("DB_TYPE=%s\nDB_DSN is not dumped by this MVP backup command.\nUse engine-native dump tools (mysqldump/pg_dump) and place the dump next to this note.\n", dbType)
			if err := os.WriteFile(note, []byte(content), 0o644); err != nil {
				return err
			}
		}

		// 2) uploads inventory / optional copy of file list
		uploadsDir := "uploads"
		if cfg.Storage.LocalDir != "" {
			uploadsDir = cfg.Storage.LocalDir
		}
		manifest := []map[string]any{}
		_ = filepath.WalkDir(uploadsDir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d == nil || d.IsDir() {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			rel, _ := filepath.Rel(uploadsDir, path)
			manifest = append(manifest, map[string]any{
				"path": rel,
				"size": info.Size(),
				"mod":  info.ModTime().UTC().Format(time.RFC3339),
			})
			return nil
		})
		manBytes, _ := json.MarshalIndent(manifest, "", "  ")
		if err := os.WriteFile(filepath.Join(outDir, "uploads-manifest.json"), manBytes, 0o644); err != nil {
			return err
		}

		// 3) meta.json
		meta := map[string]any{
			"created_at":     time.Now().UTC().Format(time.RFC3339),
			"db_type":        dbType,
			"db_file":        filepath.Base(dbPath),
			"go_version":     runtime.Version(),
			"app":            "orinforum",
			"backup_version": 1,
			"uploads_dir":    uploadsDir,
			"uploads_count":  len(manifest),
		}
		metaBytes, _ := json.MarshalIndent(meta, "", "  ")
		if err := os.WriteFile(filepath.Join(outDir, "meta.json"), metaBytes, 0o644); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "backup written to %s\n", outDir)
		return nil
	},
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func init() {
	backupCmd.Flags().StringVar(&backupOut, "out", "", "output directory for backup artifacts")
	RootCmd.AddCommand(backupCmd)
}
