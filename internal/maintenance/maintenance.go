package maintenance

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

var backupNamePattern = regexp.MustCompile(`^coc-[0-9]{8}T[0-9]{6}\.[0-9]{9}Z\.(?:db|tar\.gz)$`)

type Service struct {
	db                    *sql.DB
	databasePath          string
	backupDir             string
	startedAt             time.Time
	assetDir              string
	customOccupationsPath string
}

type Status struct {
	StartedAt      time.Time  `json:"startedAt"`
	UptimeSeconds  int64      `json:"uptimeSeconds"`
	DatabaseBytes  int64      `json:"databaseBytes"`
	AccountCount   int        `json:"accountCount"`
	ActiveSessions int        `json:"activeSessions"`
	BackupCount    int        `json:"backupCount"`
	LatestBackupAt *time.Time `json:"latestBackupAt"`
}

type Backup struct {
	Name      string    `json:"name"`
	SizeBytes int64     `json:"sizeBytes"`
	CreatedAt time.Time `json:"createdAt"`
}
type Manifest struct {
	Version   int               `json:"version"`
	CreatedAt string            `json:"createdAt"`
	Files     map[string]string `json:"files"`
}
type ValidationReport struct {
	Valid                bool   `json:"valid"`
	DatabaseIntegrity    string `json:"databaseIntegrity"`
	AssetCount           int    `json:"assetCount"`
	HasCustomOccupations bool   `json:"hasCustomOccupations"`
	FileCount            int    `json:"fileCount"`
}
type RestoreResult struct {
	DatabasePath          string `json:"databasePath"`
	AssetDir              string `json:"assetDir"`
	CustomOccupationsPath string `json:"customOccupationsPath"`
	AssetCount            int    `json:"assetCount"`
}
type restoreSwap struct {
	target, prepared, old string
	optional              bool
}

func New(db *sql.DB, databasePath, backupDir string, startedAt time.Time, paths ...string) *Service {
	assetDir, custom := ".data/assets", ".data/rules/coc7/occupations.custom.json"
	if len(paths) > 0 {
		assetDir = paths[0]
	}
	if len(paths) > 1 {
		custom = paths[1]
	}
	return &Service{db: db, databasePath: databasePath, backupDir: backupDir, startedAt: startedAt, assetDir: assetDir, customOccupationsPath: custom}
}

func (s *Service) Status(ctx context.Context) (Status, error) {
	result := Status{StartedAt: s.startedAt, UptimeSeconds: int64(time.Since(s.startedAt).Seconds())}
	if info, err := os.Stat(s.databasePath); err == nil {
		result.DatabaseBytes = info.Size()
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM accounts`).Scan(&result.AccountCount); err != nil {
		return Status{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE expires_at > ?`, now).Scan(&result.ActiveSessions); err != nil {
		return Status{}, err
	}
	backups, err := s.ListBackups()
	if err != nil {
		return Status{}, err
	}
	result.BackupCount = len(backups)
	if len(backups) > 0 {
		latest := backups[0].CreatedAt
		result.LatestBackupAt = &latest
	}
	return result, nil
}

func (s *Service) CreateBackup(ctx context.Context) (Backup, error) {
	if err := os.MkdirAll(s.backupDir, 0o750); err != nil {
		return Backup{}, fmt.Errorf("create backup directory: %w", err)
	}
	name := "coc-" + time.Now().UTC().Format("20060102T150405.000000000Z") + ".tar.gz"
	path := filepath.Join(s.backupDir, name)
	tempDB := path + ".sqlite.tmp"
	defer os.Remove(tempDB)
	if _, err := s.db.ExecContext(ctx, `VACUUM INTO ?`, tempDB); err != nil {
		return Backup{}, fmt.Errorf("create sqlite backup: %w", err)
	}
	if err := s.writeBundle(path, tempDB); err != nil {
		_ = os.Remove(path)
		return Backup{}, err
	}
	if _, err := s.ValidateBundle(path); err != nil {
		_ = os.Remove(path)
		return Backup{}, fmt.Errorf("validate created backup: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return Backup{}, fmt.Errorf("secure backup permissions: %w", err)
	}
	return backupInfo(path)
}

func (s *Service) writeBundle(outputPath, databaseBackup string) error {
	output, err := os.OpenFile(outputPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer output.Close()
	gzipWriter := gzip.NewWriter(output)
	defer gzipWriter.Close()
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()
	manifest := Manifest{Version: 1, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Files: map[string]string{}}
	add := func(source, archiveName string) error {
		data, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		manifest.Files[archiveName] = hex.EncodeToString(sum[:])
		header := &tar.Header{Name: archiveName, Mode: 0o600, Size: int64(len(data)), ModTime: time.Now().UTC()}
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		_, err = tarWriter.Write(data)
		return err
	}
	if err := add(databaseBackup, "database/coc.db"); err != nil {
		return err
	}
	if _, err := os.Stat(s.customOccupationsPath); err == nil {
		if err := add(s.customOccupationsPath, "rules/occupations.custom.json"); err != nil {
			return err
		}
	}
	if err := filepath.WalkDir(s.assetDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(s.assetDir, path)
		if err != nil {
			return err
		}
		return add(path, filepath.ToSlash(filepath.Join("assets", relative)))
	}); err != nil && !os.IsNotExist(err) {
		return err
	}
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	if err := tarWriter.WriteHeader(&tar.Header{Name: "manifest.json", Mode: 0o600, Size: int64(len(manifestData)), ModTime: time.Now().UTC()}); err != nil {
		return err
	}
	_, err = tarWriter.Write(manifestData)
	return err
}

func (s *Service) ValidateBundle(bundlePath string) (ValidationReport, error) {
	file, err := os.Open(bundlePath)
	if err != nil {
		return ValidationReport{}, err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return ValidationReport{}, fmt.Errorf("invalid gzip: %w", err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	calculated := map[string]string{}
	var manifest Manifest
	report := ValidationReport{}
	tempDB, err := os.CreateTemp(s.backupDir, "validate-*.db")
	if err != nil {
		return report, err
	}
	tempDBPath := tempDB.Name()
	_ = tempDB.Close()
	defer os.Remove(tempDBPath)
	databaseFound := false
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return report, err
		}
		name := pathpkg.Clean(header.Name)
		if header.Typeflag != tar.TypeReg || name == "." || name == ".." || len(name) == 0 || name[0] == '/' || (len(name) >= 3 && name[:3] == "../") || header.Size < 0 || header.Size > 2<<30 {
			return report, fmt.Errorf("unsafe archive entry")
		}
		data, err := io.ReadAll(io.LimitReader(tarReader, header.Size+1))
		if err != nil || int64(len(data)) != header.Size {
			return report, fmt.Errorf("invalid archive entry")
		}
		if name == "manifest.json" {
			if err := json.Unmarshal(data, &manifest); err != nil {
				return report, fmt.Errorf("invalid manifest")
			}
			continue
		}
		sum := sha256.Sum256(data)
		calculated[name] = hex.EncodeToString(sum[:])
		report.FileCount++
		switch {
		case name == "database/coc.db":
			databaseFound = true
			if err := os.WriteFile(tempDBPath, data, 0o600); err != nil {
				return report, err
			}
		case name == "rules/occupations.custom.json":
			if !json.Valid(data) {
				return report, fmt.Errorf("invalid custom occupations")
			}
			report.HasCustomOccupations = true
		case len(name) > len("assets/") && name[:len("assets/")] == "assets/":
			report.AssetCount++
		default:
			return report, fmt.Errorf("unexpected archive entry %s", name)
		}
	}
	if !databaseFound || manifest.Version != 1 || len(manifest.Files) != len(calculated) {
		return report, fmt.Errorf("incomplete backup bundle")
	}
	for name, checksum := range manifest.Files {
		if calculated[name] != checksum {
			return report, fmt.Errorf("checksum mismatch for %s", name)
		}
	}
	backupDB, err := sql.Open("sqlite", "file:"+tempDBPath+"?mode=ro")
	if err != nil {
		return report, err
	}
	defer backupDB.Close()
	if err := backupDB.QueryRow(`PRAGMA integrity_check`).Scan(&report.DatabaseIntegrity); err != nil || report.DatabaseIntegrity != "ok" {
		return report, fmt.Errorf("database integrity check failed")
	}
	report.Valid = true
	return report, nil
}

func (s *Service) ValidateUpload(reader io.Reader) (ValidationReport, error) {
	if err := os.MkdirAll(s.backupDir, 0o750); err != nil {
		return ValidationReport{}, err
	}
	temp, err := os.CreateTemp(s.backupDir, "upload-*.tar.gz")
	if err != nil {
		return ValidationReport{}, err
	}
	path := temp.Name()
	defer os.Remove(path)
	defer temp.Close()
	written, err := io.Copy(temp, io.LimitReader(reader, (1<<30)+1))
	if err != nil {
		return ValidationReport{}, err
	}
	if written == 0 || written > 1<<30 {
		return ValidationReport{}, fmt.Errorf("backup bundle size invalid")
	}
	if err := temp.Close(); err != nil {
		return ValidationReport{}, err
	}
	return s.ValidateBundle(path)
}

func (s *Service) RestoreBundle(bundlePath string) (RestoreResult, error) {
	report, err := s.ValidateBundle(bundlePath)
	if err != nil {
		return RestoreResult{}, err
	}
	for _, directory := range []string{filepath.Dir(s.databasePath), filepath.Dir(s.assetDir), filepath.Dir(s.customOccupationsPath)} {
		if err := os.MkdirAll(directory, 0o750); err != nil {
			return RestoreResult{}, err
		}
	}
	databaseFile, err := os.CreateTemp(filepath.Dir(s.databasePath), ".coc-restore-*.db")
	if err != nil {
		return RestoreResult{}, err
	}
	databaseTemp := databaseFile.Name()
	_ = databaseFile.Close()
	defer os.Remove(databaseTemp)
	assetTemp, err := os.MkdirTemp(filepath.Dir(s.assetDir), ".assets-restore-*")
	if err != nil {
		return RestoreResult{}, err
	}
	defer os.RemoveAll(assetTemp)
	customFile, err := os.CreateTemp(filepath.Dir(s.customOccupationsPath), ".occupations-restore-*.json")
	if err != nil {
		return RestoreResult{}, err
	}
	customTemp := customFile.Name()
	_ = customFile.Close()
	defer os.Remove(customTemp)
	hasCustom := false
	file, err := os.Open(bundlePath)
	if err != nil {
		return RestoreResult{}, err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return RestoreResult{}, err
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return RestoreResult{}, err
		}
		name := pathpkg.Clean(header.Name)
		if name == "manifest.json" {
			continue
		}
		switch {
		case name == "database/coc.db":
			err = writeArchiveFile(databaseTemp, tarReader, header.Size)
		case name == "rules/occupations.custom.json":
			hasCustom = true
			err = writeArchiveFile(customTemp, tarReader, header.Size)
		case len(name) > len("assets/") && name[:len("assets/")] == "assets/":
			target := filepath.Join(assetTemp, filepath.FromSlash(name[len("assets/"):]))
			if mkdirErr := os.MkdirAll(filepath.Dir(target), 0o750); mkdirErr != nil {
				return RestoreResult{}, mkdirErr
			}
			err = writeArchiveFile(target, tarReader, header.Size)
		}
		if err != nil {
			return RestoreResult{}, err
		}
	}
	stamp := time.Now().UTC().Format("20060102T150405.000000000Z")
	swaps := []restoreSwap{{s.databasePath, databaseTemp, s.databasePath + ".before-restore-" + stamp, false}, {s.assetDir, assetTemp, s.assetDir + ".before-restore-" + stamp, false}, {s.customOccupationsPath, customTemp, s.customOccupationsPath + ".before-restore-" + stamp, !hasCustom}}
	applied := 0
	for index, item := range swaps {
		if _, err := os.Stat(item.target); err == nil {
			if err := os.Rename(item.target, item.old); err != nil {
				rollbackSwaps(swaps, applied)
				return RestoreResult{}, err
			}
		}
		if !item.optional {
			if err := os.Rename(item.prepared, item.target); err != nil {
				if _, statErr := os.Stat(item.old); statErr == nil {
					_ = os.Rename(item.old, item.target)
				}
				rollbackSwaps(swaps, applied)
				return RestoreResult{}, err
			}
		}
		applied = index + 1
	}
	_ = os.Remove(s.databasePath + "-wal")
	_ = os.Remove(s.databasePath + "-shm")
	for _, item := range swaps {
		_ = os.RemoveAll(item.old)
	}
	return RestoreResult{DatabasePath: s.databasePath, AssetDir: s.assetDir, CustomOccupationsPath: s.customOccupationsPath, AssetCount: report.AssetCount}, nil
}

func writeArchiveFile(path string, reader io.Reader, size int64) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.CopyN(file, reader, size)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
func rollbackSwaps(swaps []restoreSwap, applied int) {
	for index := applied - 1; index >= 0; index-- {
		item := swaps[index]
		_ = os.RemoveAll(item.target)
		if _, err := os.Stat(item.old); err == nil {
			_ = os.Rename(item.old, item.target)
		}
	}
}

func (s *Service) ListBackups() ([]Backup, error) {
	entries, err := os.ReadDir(s.backupDir)
	if os.IsNotExist(err) {
		return []Backup{}, nil
	}
	if err != nil {
		return nil, err
	}
	backups := make([]Backup, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !backupNamePattern.MatchString(entry.Name()) {
			continue
		}
		item, err := backupInfo(filepath.Join(s.backupDir, entry.Name()))
		if err != nil {
			return nil, err
		}
		backups = append(backups, item)
	}
	sort.Slice(backups, func(i, j int) bool { return backups[i].CreatedAt.After(backups[j].CreatedAt) })
	return backups, nil
}

func (s *Service) BackupPath(name string) (string, error) {
	if !backupNamePattern.MatchString(name) || filepath.Base(name) != name {
		return "", fmt.Errorf("invalid backup name")
	}
	path := filepath.Join(s.backupDir, name)
	if _, err := os.Stat(path); err != nil {
		return "", err
	}
	return path, nil
}

func backupInfo(path string) (Backup, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Backup{}, err
	}
	return Backup{Name: filepath.Base(path), SizeBytes: info.Size(), CreatedAt: info.ModTime().UTC()}, nil
}
