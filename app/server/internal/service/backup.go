package service

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"orinforum/internal/db"
	"orinforum/internal/metrics"
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/repository"
	"orinforum/internal/storage"
)

type backupService struct {
	repo     repository.BackupRepository
	storage  storage.Provider
	store    db.Store
	settings SettingService
}

func NewBackupService(repo repository.BackupRepository, storage storage.Provider, store db.Store, settings SettingService) BackupService {
	return &backupService{repo: repo, storage: storage, store: store, settings: settings}
}

func (s *backupService) Create(ctx context.Context, actorID, name string) (result *model.Backup, err error) {
	// Backups fail silently until the day a restore is needed: a failed create
	// leaves the previous (still "ready") row in the list, so the only evidence
	// was a log line. Observe the outcome at the single service entry point.
	defer func() { observeBackup(metrics.BackupKindCreate, time.Now(), err) }()
	if name == "" {
		name = "snapshot-" + time.Now().Format("20060102-150405")
	}
	id := pkg.NewID()
	fileKey := "backups/" + id + ".json"
	snapshot, err := buildBackupSnapshot(ctx, s.store, id, name)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "backup snapshot failed: "+err.Error())
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "backup encode failed: "+err.Error())
	}
	reader := newBytesReader(payload)
	if err := s.storage.Put(ctx, fileKey, reader, int64(len(payload)), "application/json"); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "backup storage failed: "+err.Error())
	}
	sum := sha256.Sum256(payload)
	uploadsKey := "backups/" + id + ".uploads.tar.gz"
	uploadsArchive, uploadsSum, uploadsSize, err := buildUploadsArchive(ctx, s.storage)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "backup uploads failed: "+err.Error())
	}
	if err := s.storage.Put(ctx, uploadsKey, newBytesReader(uploadsArchive), int64(len(uploadsArchive)), "application/gzip"); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "backup uploads storage failed: "+err.Error())
	}
	backup := &model.Backup{
		ID:              id,
		Name:            name,
		FileKey:         fileKey,
		Size:            int64(len(payload)),
		Checksum:        hex.EncodeToString(sum[:]),
		UploadsKey:      uploadsKey,
		UploadsSize:     uploadsSize,
		UploadsChecksum: uploadsSum,
		Status:          "ready",
		CreatedBy:       actorID,
		CreatedAt:       time.Now(),
	}
	if err := s.repo.Create(ctx, backup); err != nil {
		return nil, err
	}
	return backup, nil
}

func (s *backupService) Restore(ctx context.Context, id, actorID string) (err error) {
	// A failing restore is the most urgent backup outcome: the operator thinks
	// they have a snapshot and does not.
	defer func() { observeBackup(metrics.BackupKindRestore, time.Now(), err) }()
	b, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if b.Status != "ready" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "只能恢复 ready 状态的备份")
	}
	reader, err := s.storage.Get(ctx, b.FileKey)
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, "read backup failed: "+err.Error())
	}
	defer reader.Close()
	payload, err := io.ReadAll(reader)
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, "read backup failed: "+err.Error())
	}
	sum := sha256.Sum256(payload)
	if hex.EncodeToString(sum[:]) != b.Checksum {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "备份校验和不匹配，已拒绝恢复")
	}
	var snapshot backupSnapshot
	if err := json.Unmarshal(payload, &snapshot); err != nil {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "备份文件格式不正确")
	}
	if snapshot.Backend != s.store.Type() {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "备份后端与当前数据库不一致")
	}
	if err := restoreBackupSnapshot(ctx, s.store, &snapshot); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, "restore failed: "+err.Error())
	}
	if b.UploadsKey != "" && b.UploadsChecksum != "" {
		if err := restoreUploadsArchive(ctx, s.storage, b.UploadsKey, b.UploadsChecksum); err != nil {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, "restore uploads failed: "+err.Error())
		}
	}
	return nil
}

func (s *backupService) List(ctx context.Context, page, pageSize int) ([]model.Backup, int, error) {
	page, pageSize = pkg.NormalizePagination(page, pageSize)
	return s.repo.List(ctx, (page-1)*pageSize, pageSize)
}

func (s *backupService) Cleanup(ctx context.Context) (n int, err error) {
	defer func() { observeBackup(metrics.BackupKindCleanup, time.Now(), err) }()
	retention := pkg.GetSettingInt(ctx, s.settings, model.SettingBackupRetentionDays, "", 30)
	if retention <= 0 {
		return 0, nil
	}
	cutoff := time.Now().Add(-time.Duration(retention) * 24 * time.Hour)
	items, err := s.repo.ListAll(ctx)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, b := range items {
		if b.CreatedAt.After(cutoff) || b.CreatedAt.Equal(cutoff) {
			continue
		}
		if err := s.repo.Delete(ctx, b.ID); err != nil {
			return removed, err
		}
		_ = s.storage.Delete(ctx, b.FileKey)
		if b.UploadsKey != "" {
			_ = s.storage.Delete(ctx, b.UploadsKey)
		}
		removed++
	}
	return removed, nil
}

func (s *backupService) Download(ctx context.Context, id string) (*model.Backup, io.ReadCloser, error) {
	b, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	reader, err := s.storage.Get(ctx, b.FileKey)
	if err != nil {
		return nil, nil, err
	}
	return b, reader, nil
}

func (s *backupService) Delete(ctx context.Context, id string) error {
	b, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	_ = s.storage.Delete(ctx, b.FileKey)
	if b.UploadsKey != "" {
		_ = s.storage.Delete(ctx, b.UploadsKey)
	}
	return nil
}

func buildUploadsArchive(ctx context.Context, st storage.Provider) ([]byte, string, int64, error) {
	objects, err := st.List(ctx, "uploads/")
	if err != nil {
		return nil, "", 0, err
	}
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for _, obj := range objects {
		reader, err := st.Get(ctx, obj.Key)
		if err != nil {
			tw.Close()
			gw.Close()
			return nil, "", 0, fmt.Errorf("read %s: %w", obj.Key, err)
		}
		content, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			tw.Close()
			gw.Close()
			return nil, "", 0, err
		}
		hdr := &tar.Header{Name: obj.Key, Mode: 0o600, Size: int64(len(content)), ModTime: obj.ModifiedAt}
		if err := tw.WriteHeader(hdr); err != nil {
			tw.Close()
			gw.Close()
			return nil, "", 0, err
		}
		if _, err := tw.Write(content); err != nil {
			tw.Close()
			gw.Close()
			return nil, "", 0, err
		}
	}
	if err := tw.Close(); err != nil {
		gw.Close()
		return nil, "", 0, err
	}
	if err := gw.Close(); err != nil {
		return nil, "", 0, err
	}
	sum := sha256.Sum256(buf.Bytes())
	return buf.Bytes(), hex.EncodeToString(sum[:]), int64(buf.Len()), nil
}

func restoreUploadsArchive(ctx context.Context, st storage.Provider, key, expectedChecksum string) error {
	reader, err := st.Get(ctx, key)
	if err != nil {
		return err
	}
	defer reader.Close()
	content, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(content)
	if hex.EncodeToString(sum[:]) != expectedChecksum {
		return fmt.Errorf("uploads checksum mismatch")
	}
	gz, err := gzip.NewReader(bytes.NewReader(content))
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg || !strings.HasPrefix(hdr.Name, "uploads/") {
			continue
		}
		clean := path.Clean(hdr.Name)
		if clean != hdr.Name || strings.Contains(clean, "..") {
			return fmt.Errorf("invalid upload key in archive: %s", hdr.Name)
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return err
		}
		if err := st.Put(ctx, clean, bytes.NewReader(data), int64(len(data)), "application/octet-stream"); err != nil {
			return err
		}
	}
	return nil
}

type byteReader struct {
	data []byte
	pos  int
}

func newBytesReader(data []byte) io.Reader {
	return &byteReader{data: data}
}

func (r *byteReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// observeBackup records one backup job's outcome and duration.
//
// Kept as a single helper so all three entry points classify the same way:
// outcome is derived from err, never passed in, so a caller cannot forget to
// report a failure as one.
func observeBackup(kind string, start time.Time, err error) {
	outcome := metrics.BackupOutcomeSucceeded
	if err != nil {
		outcome = metrics.BackupOutcomeFailed
	}
	metrics.BackupJobsTotal.WithLabelValues(kind, outcome).Inc()
	metrics.BackupJobDuration.WithLabelValues(kind).Observe(time.Since(start).Seconds())
}
