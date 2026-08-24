package campaign

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/mushan/coc/internal/character"
	"github.com/mushan/coc/internal/database"
)

func TestCampaignVisibilityAndKeeperPermission(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "campaign.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, account := range []string{"keeper", "player", "spectator"} {
		if _, err := db.Exec(`INSERT INTO accounts(id, username, display_name, password_hash, role, status, must_change_password, password_changed_at, created_at, updated_at) VALUES (?, ?, ?, 'hash', 'user', 'active', 0, 'now', 'now', 'now')`, account, account, account); err != nil {
			t.Fatal(err)
		}
	}
	store := NewStore(db)
	created, err := store.Create(ctx, "keeper", "无名祭祀书", "朋友团测试")
	if err != nil {
		t.Fatal(err)
	}
	seenByPlayer, err := store.Get(ctx, created.ID, "player")
	if err != nil {
		t.Fatal(err)
	}
	if seenByPlayer.CanManage {
		t.Fatal("player unexpectedly received keeper permission")
	}
	if _, err := store.Update(ctx, created.ID, "player", "篡改", "", "active"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden update, got %v", err)
	}
	updated, err := store.Update(ctx, created.ID, "keeper", "无名祭祀书", "开团", "active")
	if err != nil {
		t.Fatal(err)
	}
	if !updated.CanManage || updated.Status != "active" {
		t.Fatalf("unexpected updated campaign: %#v", updated)
	}
	if _, err := store.CreateBlock(ctx, created.ID, "keeper", "text", "公开信息", "玩家可见", "public", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateBlock(ctx, created.ID, "keeper", "clue", "幕后真相", "不可泄露", "keeper", nil); err != nil {
		t.Fatal(err)
	}
	playerBlocks, err := store.ListBlocks(ctx, created.ID, "player")
	if err != nil {
		t.Fatal(err)
	}
	if len(playerBlocks) != 1 || playerBlocks[0].Title != "公开信息" {
		t.Fatalf("private blocks leaked to player: %#v", playerBlocks)
	}
	if _, err := store.CreateBlock(ctx, created.ID, "player", "text", "篡改", "", "public", nil); !errors.Is(err, ErrForbidden) {
		t.Fatalf("player created block: %v", err)
	}
	asset, err := store.SaveAsset(ctx, created.ID, "keeper", "secret.png", "image/png", ".png", []byte("test"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetVisibleAsset(ctx, created.ID, asset.ID, "player"); !errors.Is(err, ErrNotFound) {
		t.Fatal("unpublished asset leaked to player")
	}
	if _, err := store.CreateBlock(ctx, created.ID, "keeper", "image", "公开图片", "", "public", &asset.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetVisibleAsset(ctx, created.ID, asset.ID, "player"); err != nil {
		t.Fatalf("public image not visible: %v", err)
	}
	cover, err := store.SaveAsset(ctx, created.ID, "keeper", "cover.png", "image/png", ".png", []byte("cover"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetCover(ctx, created.ID, "keeper", &cover.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetVisibleAsset(ctx, created.ID, cover.ID, "spectator"); err != nil {
		t.Fatalf("campaign cover not visible: %v", err)
	}
	characters := character.NewStore(db)
	investigator, err := characters.Create(ctx, "player", "player", "调查员甲", "investigator")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AttachCharacter(ctx, created.ID, investigator.ID, "player"); err != nil {
		t.Fatal(err)
	}
	links, err := store.ListCharacters(ctx, created.ID, "spectator")
	if err != nil || len(links) != 1 {
		t.Fatalf("public investigator missing: links=%#v err=%v", links, err)
	}
	if editable, err := characters.GetEditable(ctx, investigator.ID, "keeper"); err != nil || !editable.CanEdit {
		t.Fatalf("keeper cannot edit attached investigator: %#v %v", editable, err)
	}
	for viewer, expected := range map[string]int{"player": 1, "keeper": 1, "spectator": 0} {
		campaigns, err := store.ListForCharacter(ctx, investigator.ID, viewer)
		if err != nil || len(campaigns) != expected {
			t.Fatalf("character campaigns for %s: %#v %v", viewer, campaigns, err)
		}
	}
	if err := store.DetachCharacter(ctx, created.ID, investigator.ID, "player"); err != nil {
		t.Fatal(err)
	}
	if _, err := characters.GetEditable(ctx, investigator.ID, "keeper"); !errors.Is(err, character.ErrNotFound) {
		t.Fatal("keeper retained edit permission after detach")
	}
}
