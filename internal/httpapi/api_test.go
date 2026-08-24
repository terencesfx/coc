package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/mushan/coc/internal/account"
	"github.com/mushan/coc/internal/campaign"
	"github.com/mushan/coc/internal/character"
	"github.com/mushan/coc/internal/database"
	"github.com/mushan/coc/internal/dice"
	"github.com/mushan/coc/internal/maintenance"
	"github.com/mushan/coc/internal/notification"
)

func TestLive(t *testing.T) {
	handler, _ := setupAPI(t)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/health/live", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	if response.Header().Get("X-Request-ID") == "" {
		t.Fatal("expected response request ID")
	}

	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected ok status, got %v", body["status"])
	}
}

func TestUnknownRoute(t *testing.T) {
	handler, _ := setupAPI(t)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/unknown", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, response.Code)
	}
}

func setupAPI(t *testing.T) (http.Handler, *account.Store) {
	t.Helper()
	dataDir := t.TempDir()
	databasePath := filepath.Join(dataDir, "test.db")
	db, err := database.Open(context.Background(), databasePath)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store := account.NewStore(db)
	characters := character.NewStore(db)
	campaigns := campaign.NewStore(db)
	diceRolls := dice.NewStore(db)
	maintenanceService := maintenance.New(db, databasePath, filepath.Join(dataDir, "backups"), time.Now().UTC())
	return New(slog.New(slog.NewTextHandler(io.Discard, nil)), store, characters, campaigns, diceRolls, nil, maintenanceService, notification.NewStore(db), false), store
}
