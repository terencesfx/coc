package notification

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/mushan/coc/internal/database"
	"github.com/mushan/coc/internal/dice"
)

func TestPublicRollCreatesAndSendsConsoleDelivery(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "notification.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`INSERT INTO accounts(id, username, display_name, password_hash, role, status, must_change_password, password_changed_at, created_at, updated_at) VALUES ('keeper', 'keeper', '守秘人', 'hash', 'user', 'active', 0, 'now', 'now', 'now')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO campaigns(id, keeper_account_id, title, summary, status, created_at, updated_at) VALUES ('campaign', 'keeper', '测试团', '', 'active', 'now', 'now')`)
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore(db)
	if _, err := store.SetSetting(ctx, "campaign", "keeper", "console", "test-group"); err != nil {
		t.Fatal(err)
	}
	campaignID := "campaign"
	rollStore := dice.NewStore(db)
	if _, err := rollStore.Save(ctx, "public-roll", "keeper", nil, &campaignID, "public", "expression", "测试", "1d6", map[string]string{"requestId": "public-roll"}, dice.ExpressionResult{Total: 4}); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM notification_deliveries WHERE status = 'pending'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("pending delivery missing: %d %v", count, err)
	}
	worker := NewWorker(db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if !worker.processOne(ctx) {
		t.Fatal("worker did not process delivery")
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM notification_deliveries`).Scan(&status); err != nil || status != "sent" {
		t.Fatalf("delivery not sent: %s %v", status, err)
	}
	if _, err := rollStore.Save(ctx, "secret-roll", "keeper", nil, &campaignID, "keeper", "expression", "暗骰", "1d6", map[string]string{"requestId": "secret-roll"}, dice.ExpressionResult{Total: 2}); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM notification_deliveries`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("secret roll created notification: %d %v", count, err)
	}
}
