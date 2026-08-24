package notification

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

var ErrForbidden = errors.New("campaign keeper required")

type Setting struct {
	CampaignID      string `json:"campaignId"`
	Provider        string `json:"provider"`
	TargetReference string `json:"targetReference"`
	UpdatedAt       string `json:"updatedAt"`
}
type Delivery struct {
	ID        string `json:"id"`
	RollID    string `json:"rollId"`
	Provider  string `json:"provider"`
	Status    string `json:"status"`
	Attempts  int    `json:"attempts"`
	LastError string `json:"lastError"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}
type Event struct {
	DeliveryID    string
	Provider      string
	Target        string
	ActorName     string
	CharacterName *string
	Label, Kind   string
	Result        json.RawMessage
}

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

func (s *Store) GetSetting(ctx context.Context, campaignID, viewerID string) (Setting, bool, error) {
	var keeper bool
	if err := s.db.QueryRowContext(ctx, `SELECT keeper_account_id = ? FROM campaigns WHERE id = ? AND archived_at IS NULL`, viewerID, campaignID).Scan(&keeper); errors.Is(err, sql.ErrNoRows) {
		return Setting{}, false, sql.ErrNoRows
	} else if err != nil {
		return Setting{}, false, err
	}
	setting := Setting{CampaignID: campaignID, Provider: "disabled"}
	err := s.db.QueryRowContext(ctx, `SELECT provider, target_reference, updated_at FROM campaign_notification_settings WHERE campaign_id = ?`, campaignID).Scan(&setting.Provider, &setting.TargetReference, &setting.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return setting, keeper, nil
	}
	return setting, keeper, err
}

func (s *Store) SetSetting(ctx context.Context, campaignID, keeperID, provider, target string) (Setting, error) {
	if provider != "disabled" && provider != "console" {
		return Setting{}, fmt.Errorf("unsupported provider")
	}
	var allowed int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM campaigns WHERE id = ? AND keeper_account_id = ? AND archived_at IS NULL`, campaignID, keeperID).Scan(&allowed); err != nil {
		return Setting{}, err
	}
	if allowed != 1 {
		return Setting{}, ErrForbidden
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `INSERT INTO campaign_notification_settings(campaign_id, provider, target_reference, updated_at) VALUES (?, ?, ?, ?) ON CONFLICT(campaign_id) DO UPDATE SET provider = excluded.provider, target_reference = excluded.target_reference, updated_at = excluded.updated_at`, campaignID, provider, target, now)
	if err != nil {
		return Setting{}, err
	}
	return Setting{CampaignID: campaignID, Provider: provider, TargetReference: target, UpdatedAt: now}, nil
}

func (s *Store) ListDeliveries(ctx context.Context, campaignID, keeperID string) ([]Delivery, error) {
	var allowed int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM campaigns WHERE id = ? AND keeper_account_id = ?`, campaignID, keeperID).Scan(&allowed); err != nil {
		return nil, err
	}
	if allowed != 1 {
		return nil, ErrForbidden
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, roll_id, provider, status, attempts, last_error, created_at, updated_at FROM notification_deliveries WHERE campaign_id = ? ORDER BY created_at DESC LIMIT 50`, campaignID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Delivery{}
	for rows.Next() {
		var item Delivery
		if err := rows.Scan(&item.ID, &item.RollID, &item.Provider, &item.Status, &item.Attempts, &item.LastError, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

type Sender interface {
	Send(context.Context, Event) error
}
type ConsoleSender struct{ logger *slog.Logger }

func (s ConsoleSender) Send(_ context.Context, event Event) error {
	s.logger.Info("dice notification", "target", event.Target, "message", Format(event))
	return nil
}
func Format(event Event) string {
	character := ""
	if event.CharacterName != nil {
		character = "（" + *event.CharacterName + "）"
	}
	var result map[string]any
	_ = json.Unmarshal(event.Result, &result)
	if event.Kind == "check" {
		return fmt.Sprintf("%s%s %s检定：%v/%v（%s）", event.ActorName, character, event.Label, result["value"], result["target"], outcomeName(result["outcome"]))
	}
	return fmt.Sprintf("%s%s %s：%v", event.ActorName, character, event.Label, result["total"])
}
func outcomeName(value any) string {
	names := map[string]string{"critical": "大成功", "extreme": "极难成功", "hard": "困难成功", "regular": "成功", "failure": "失败", "fumble": "大失败"}
	if name := names[fmt.Sprint(value)]; name != "" {
		return name
	}
	return fmt.Sprint(value)
}

type Worker struct {
	db      *sql.DB
	logger  *slog.Logger
	senders map[string]Sender
}

func NewWorker(db *sql.DB, logger *slog.Logger) *Worker {
	return &Worker{db: db, logger: logger, senders: map[string]Sender{"console": ConsoleSender{logger}}}
}
func (w *Worker) Run(ctx context.Context) {
	_, _ = w.db.ExecContext(ctx, `UPDATE notification_deliveries SET status = 'pending' WHERE status = 'sending'`)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		for w.processOne(ctx) {
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (w *Worker) processOne(ctx context.Context) bool {
	var event Event
	var attempts int
	var characterName sql.NullString
	var resultData string
	err := w.db.QueryRowContext(ctx, `SELECT d.id, d.provider, s.target_reference, a.display_name, c.name, r.label, r.roll_kind, r.result_data, d.attempts FROM notification_deliveries d JOIN dice_rolls r ON r.id = d.roll_id JOIN accounts a ON a.id = r.actor_account_id LEFT JOIN characters c ON c.id = r.character_id JOIN campaign_notification_settings s ON s.campaign_id = d.campaign_id WHERE d.status IN ('pending','failed') AND d.next_attempt_at <= ? AND d.attempts < 5 ORDER BY d.created_at LIMIT 1`, time.Now().UTC().Format(time.RFC3339Nano)).Scan(&event.DeliveryID, &event.Provider, &event.Target, &event.ActorName, &characterName, &event.Label, &event.Kind, &resultData, &attempts)
	if errors.Is(err, sql.ErrNoRows) || errors.Is(err, context.Canceled) {
		return false
	}
	if err != nil {
		w.logger.Error("load notification failed", "error", err)
		return false
	}
	if characterName.Valid {
		event.CharacterName = &characterName.String
	}
	event.Result = json.RawMessage(resultData)
	now := time.Now().UTC()
	result, err := w.db.ExecContext(ctx, `UPDATE notification_deliveries SET status = 'sending', attempts = attempts + 1, updated_at = ? WHERE id = ? AND status IN ('pending','failed')`, now.Format(time.RFC3339Nano), event.DeliveryID)
	if err != nil {
		return false
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return true
	}
	sender := w.senders[event.Provider]
	if sender == nil {
		err = fmt.Errorf("provider %s is not available", event.Provider)
	} else {
		err = sender.Send(ctx, event)
	}
	if err == nil {
		_, _ = w.db.ExecContext(ctx, `UPDATE notification_deliveries SET status = 'sent', last_error = '', updated_at = ? WHERE id = ?`, time.Now().UTC().Format(time.RFC3339Nano), event.DeliveryID)
		return true
	}
	delay := time.Duration(1<<min(attempts, 5)) * time.Minute
	_, _ = w.db.ExecContext(ctx, `UPDATE notification_deliveries SET status = 'failed', last_error = ?, next_attempt_at = ?, updated_at = ? WHERE id = ?`, err.Error(), time.Now().UTC().Add(delay).Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano), event.DeliveryID)
	return true
}
