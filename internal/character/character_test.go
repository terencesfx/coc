package character

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/mushan/coc/internal/account"
	"github.com/mushan/coc/internal/database"
)

func TestCharacterHistoryAndRestore(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	owner, err := account.NewStore(db).Create(ctx, "player", "玩家", "player-password", "user", false)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	store := NewStore(db)
	created, err := store.Create(ctx, owner.ID, owner.ID, "调查员", "investigator")
	if err != nil {
		t.Fatalf("create character: %v", err)
	}

	var sheet map[string]any
	if err := json.Unmarshal(created.Sheet, &sheet); err != nil {
		t.Fatalf("decode sheet: %v", err)
	}
	profile := sheet["profile"].(map[string]any)
	profile["occupation"] = "私家侦探"
	updatedSheet, _ := json.Marshal(sheet)
	updated, err := store.Update(ctx, created.ID, owner.ID, owner.ID, 1, updatedSheet, "选择职业")
	if err != nil {
		t.Fatalf("update character: %v", err)
	}
	if updated.CurrentVersion != 2 {
		t.Fatalf("expected version 2, got %d", updated.CurrentVersion)
	}
	if _, err := store.Update(ctx, created.ID, owner.ID, owner.ID, 1, updatedSheet, "stale"); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}

	versions, err := store.Versions(ctx, created.ID, owner.ID)
	if err != nil || len(versions) != 2 {
		t.Fatalf("expected two versions, got %+v error=%v", versions, err)
	}
	from := 1
	detail, err := store.VersionDetail(ctx, created.ID, owner.ID, &from, 2)
	if err != nil {
		t.Fatalf("compare versions: %v", err)
	}
	foundOccupation := false
	for _, change := range detail.Changes {
		if change.Path == "/profile/occupation" && change.After == "私家侦探" {
			foundOccupation = true
		}
	}
	if !foundOccupation {
		t.Fatalf("occupation change missing: %#v", detail.Changes)
	}
	restored, err := store.Restore(ctx, created.ID, owner.ID, owner.ID, 1, 2, "恢复创建状态")
	if err != nil {
		t.Fatalf("restore character: %v", err)
	}
	if restored.CurrentVersion != 3 {
		t.Fatalf("expected version 3, got %d", restored.CurrentVersion)
	}
	retired, err := store.SetStatusOwned(ctx, created.ID, owner.ID, owner.ID, "retired")
	if err != nil || retired.Status != "retired" || retired.CurrentVersion != 4 {
		t.Fatalf("set lifecycle status: %#v %v", retired, err)
	}
	copied, err := store.CopyOwned(ctx, created.ID, owner.ID, owner.ID, "调查员副本")
	if err != nil {
		t.Fatal(err)
	}
	if copied.ID == created.ID || copied.Name != "调查员副本" || copied.Status != "draft" || copied.CurrentVersion != 1 {
		t.Fatalf("unexpected copy: %#v", copied)
	}
}
