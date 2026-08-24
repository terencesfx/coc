package dice

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mushan/coc/internal/database"
)

func TestRollExpression(t *testing.T) {
	result, err := RollExpression("2d6 + 1d4 - 3")
	if err != nil {
		t.Fatalf("roll expression: %v", err)
	}
	if len(result.Terms) != 3 {
		t.Fatalf("expected 3 terms, got %d", len(result.Terms))
	}
	if len(result.Terms[0].Rolls) != 2 || len(result.Terms[1].Rolls) != 1 {
		t.Fatalf("unexpected dice results: %#v", result.Terms)
	}
	if result.Total < 0 || result.Total > 13 {
		t.Fatalf("total outside possible range: %d", result.Total)
	}
}

func TestRejectsInvalidExpressions(t *testing.T) {
	for _, expression := range []string{"", "d1", "0d6", "201d6", "2d6x", "1d6++2", "(1d6)"} {
		if _, err := RollExpression(expression); err == nil {
			t.Errorf("expected %q to be rejected", expression)
		}
	}
}

func TestOutcomeBoundaries(t *testing.T) {
	tests := []struct {
		value, target int
		want          string
	}{
		{1, 60, "critical"}, {12, 60, "extreme"}, {30, 60, "hard"},
		{60, 60, "regular"}, {61, 60, "failure"}, {96, 40, "fumble"},
		{99, 60, "failure"}, {100, 60, "fumble"},
	}
	for _, test := range tests {
		if got := outcome(test.value, test.target); got != test.want {
			t.Errorf("outcome(%d, %d) = %q, want %q", test.value, test.target, got, test.want)
		}
	}
}

func TestCheckUsesValidCandidates(t *testing.T) {
	for _, bonusPenalty := range []int{-2, -1, 0, 1, 2} {
		result, err := RollCheck(55, bonusPenalty)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Candidates) != 1+abs(bonusPenalty) {
			t.Fatalf("unexpected candidates: %#v", result.Candidates)
		}
		for _, value := range result.Candidates {
			if value < 1 || value > 100 {
				t.Fatalf("candidate outside percentile range: %d", value)
			}
		}
	}
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func TestListVisibleFiltersPrivateRolls(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "dice.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, account := range []struct{ id, name string }{{"a", "甲"}, {"b", "乙"}} {
		_, err = db.Exec(`INSERT INTO accounts(id, username, display_name, password_hash, role, status, must_change_password, password_changed_at, created_at, updated_at) VALUES (?, ?, ?, 'hash', 'user', 'active', 0, 'now', 'now', 'now')`, account.id, account.id, account.name)
		if err != nil {
			t.Fatal(err)
		}
	}
	store := NewStore(db)
	for _, roll := range []struct{ requestID, actor, visibility string }{
		{"public-a", "a", "public"}, {"keeper-a", "a", "keeper"},
		{"test-a", "a", "test"}, {"keeper-b", "b", "keeper"},
	} {
		if _, err := store.Save(ctx, roll.requestID, roll.actor, nil, nil, roll.visibility, "expression", "test", "1d6", roll, ExpressionResult{Total: 1}); err != nil {
			t.Fatal(err)
		}
	}
	items, err := store.ListVisible(ctx, "a", nil, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("viewer a should see public and own rolls, got %#v", items)
	}
	for _, item := range items {
		if item.RequestID == "keeper-b" {
			t.Fatal("private roll from another actor leaked")
		}
	}
	_, err = db.Exec(`INSERT INTO campaigns(id, keeper_account_id, title, summary, status, created_at, updated_at) VALUES ('campaign-1', 'b', '测试团', '', 'active', 'now', 'now')`)
	if err != nil {
		t.Fatal(err)
	}
	campaignID := "campaign-1"
	if _, err := store.Save(ctx, "campaign-secret", "a", nil, &campaignID, "keeper", "expression", "暗骰", "1d6", map[string]string{"id": "campaign-secret"}, ExpressionResult{Total: 3}); err != nil {
		t.Fatal(err)
	}
	keeperItems, err := store.ListVisible(ctx, "b", &campaignID, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(keeperItems) != 1 || keeperItems[0].RequestID != "campaign-secret" {
		t.Fatalf("campaign keeper cannot see secret roll: %#v", keeperItems)
	}
}

func TestSaveRerollKeepsOriginalRelation(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "reroll.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`INSERT INTO accounts(id, username, display_name, password_hash, role, status, must_change_password, password_changed_at, created_at, updated_at) VALUES ('a', 'a', '甲', 'hash', 'user', 'active', 0, 'now', 'now', 'now')`)
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore(db)
	original, err := store.Save(ctx, "original", "a", nil, nil, "public", "check", "侦查", "1d100", map[string]string{"id": "original"}, CheckResult{Outcome: "failure"})
	if err != nil {
		t.Fatal(err)
	}
	pushed, err := store.SaveRelated(ctx, "pushed", "a", nil, nil, "public", "check", "孤注一掷 · 侦查", "1d100", map[string]string{"id": "pushed"}, CheckResult{Outcome: "regular"}, original.ID, "push")
	if err != nil {
		t.Fatal(err)
	}
	if pushed.RerollOfID == nil || *pushed.RerollOfID != original.ID {
		t.Fatalf("reroll relation missing: %#v", pushed)
	}
	if pushed.RerollKind == nil || *pushed.RerollKind != "push" {
		t.Fatalf("reroll kind missing: %#v", pushed)
	}
	requestID, hasReroll, err := store.RerollRequestID(ctx, original.ID)
	if err != nil || !hasReroll || requestID != "pushed" {
		t.Fatalf("reroll not found: %v", err)
	}
}
