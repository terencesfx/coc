package database

import (
	"context"
	"path/filepath"
	"testing"
)

func TestAppliesAllMigrations(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if count != len(migrations()) {
		t.Fatalf("expected %d migrations, got %d", len(migrations()), count)
	}
	for _, table := range []string{"accounts", "admin_audit_logs", "characters", "character_versions", "dice_rolls", "campaigns", "campaign_blocks", "campaign_assets", "campaign_characters", "campaign_notification_settings", "notification_deliveries"} {
		var found int
		if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&found); err != nil || found != 1 {
			t.Fatalf("expected table %s, found=%d error=%v", table, found, err)
		}
	}
}
