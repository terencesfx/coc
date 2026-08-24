package maintenance

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mushan/coc/internal/database"
)

func TestCreateBackup(t *testing.T) {
	dataDir := t.TempDir()
	databasePath := filepath.Join(dataDir, "coc.db")
	db, err := database.Open(context.Background(), databasePath)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	assetDir := filepath.Join(dataDir, "assets")
	customPath := filepath.Join(dataDir, "rules", "occupations.custom.json")
	if err := os.MkdirAll(filepath.Join(assetDir, "campaign"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetDir, "campaign", "image.png"), []byte("image"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(customPath), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(customPath, []byte(`{"schemaVersion":1,"occupations":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	service := New(db, databasePath, filepath.Join(dataDir, "backups"), time.Now().UTC(), assetDir, customPath)

	created, err := service.CreateBackup(context.Background())
	if err != nil {
		t.Fatalf("create backup: %v", err)
	}
	if created.SizeBytes == 0 {
		t.Fatal("expected non-empty backup")
	}
	backups, err := service.ListBackups()
	if err != nil {
		t.Fatalf("list backups: %v", err)
	}
	if len(backups) != 1 || backups[0].Name != created.Name {
		t.Fatalf("expected created backup, got %+v", backups)
	}
	if _, err := service.BackupPath("../coc.db"); err == nil {
		t.Fatal("expected unsafe backup name to be rejected")
	}
	backupPath, err := service.BackupPath(created.Name)
	if err != nil {
		t.Fatalf("resolve backup path: %v", err)
	}
	report, err := service.ValidateBundle(backupPath)
	if err != nil {
		t.Fatalf("validate bundle: %v", err)
	}
	if !report.Valid || report.DatabaseIntegrity != "ok" || report.AssetCount != 1 || !report.HasCustomOccupations {
		t.Fatalf("unexpected validation report: %#v", report)
	}
	if _, err := db.Exec(`CREATE TABLE after_backup(value TEXT)`); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetDir, "campaign", "image.png"), []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(customPath, []byte(`{"changed":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.RestoreBundle(backupPath); err != nil {
		t.Fatalf("restore bundle: %v", err)
	}
	assetData, _ := os.ReadFile(filepath.Join(assetDir, "campaign", "image.png"))
	if string(assetData) != "image" {
		t.Fatalf("asset not restored: %q", assetData)
	}
	customData, _ := os.ReadFile(customPath)
	if string(customData) != `{"schemaVersion":1,"occupations":[]}` {
		t.Fatalf("custom occupations not restored: %q", customData)
	}
	restoredDB, err := sql.Open("sqlite", "file:"+databasePath+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer restoredDB.Close()
	if err := restoredDB.QueryRow(`SELECT COUNT(*) FROM after_backup`).Scan(new(int)); err == nil {
		t.Fatal("database was not restored to backup state")
	}
}
