package dice

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	ErrInvalidExpression = errors.New("invalid dice expression")
	ErrRequestConflict   = errors.New("request id already used with different roll")
)

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

type Roll struct {
	ID             string          `json:"id"`
	RequestID      string          `json:"requestId"`
	ActorAccountID string          `json:"actorAccountId"`
	ActorName      string          `json:"actorName,omitempty"`
	CharacterID    *string         `json:"characterId"`
	CharacterName  *string         `json:"characterName,omitempty"`
	CampaignID     *string         `json:"campaignId"`
	CampaignTitle  *string         `json:"campaignTitle,omitempty"`
	Visibility     string          `json:"visibility"`
	Kind           string          `json:"kind"`
	Label          string          `json:"label"`
	Expression     string          `json:"expression"`
	Result         json.RawMessage `json:"result"`
	RerollOfID     *string         `json:"rerollOfId"`
	RerollKind     *string         `json:"rerollKind"`
	CreatedAt      string          `json:"createdAt"`
}

func (s *Store) ListVisible(ctx context.Context, viewerID string, campaignID *string, limit int) ([]Roll, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	query := `
		SELECT r.id, r.request_id, r.actor_account_id, a.display_name,
		       r.character_id, c.name, r.campaign_id, cp.title, r.visibility, r.roll_kind, r.label,
		       r.expression, r.result_data, r.reroll_of_id, r.reroll_kind, r.created_at
		FROM dice_rolls r
		JOIN accounts a ON a.id = r.actor_account_id
		LEFT JOIN characters c ON c.id = r.character_id
		LEFT JOIN campaigns cp ON cp.id = r.campaign_id
		WHERE (r.visibility = 'public' OR r.actor_account_id = ? OR (r.visibility = 'keeper' AND cp.keeper_account_id = ?))`
	args := []any{viewerID, viewerID}
	if campaignID != nil {
		query += ` AND r.campaign_id = ?`
		args = append(args, *campaignID)
	}
	query += `
		ORDER BY r.created_at DESC, r.id DESC
		LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Roll{}
	for rows.Next() {
		var item Roll
		var result string
		if err := rows.Scan(&item.ID, &item.RequestID, &item.ActorAccountID, &item.ActorName, &item.CharacterID, &item.CharacterName, &item.CampaignID, &item.CampaignTitle, &item.Visibility, &item.Kind, &item.Label, &item.Expression, &result, &item.RerollOfID, &item.RerollKind, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.Result = json.RawMessage(result)
		items = append(items, item)
	}
	return items, rows.Err()
}

type ExpressionResult struct {
	Terms []TermResult `json:"terms"`
	Total int          `json:"total"`
}
type TermResult struct {
	Sign       int    `json:"sign"`
	Expression string `json:"expression"`
	Rolls      []int  `json:"rolls,omitempty"`
	Value      int    `json:"value"`
}
type CheckResult struct {
	Target       int    `json:"target"`
	BonusPenalty int    `json:"bonusPenalty"`
	Units        int    `json:"units"`
	Tens         []int  `json:"tens"`
	Candidates   []int  `json:"candidates"`
	Value        int    `json:"value"`
	Outcome      string `json:"outcome"`
	Hard         int    `json:"hard"`
	Extreme      int    `json:"extreme"`
}

var termPattern = regexp.MustCompile(`(?i)([+-]?)(?:(\d*)d(\d+)|(\d+))`)

func RollExpression(expression string) (ExpressionResult, error) {
	compact := strings.ReplaceAll(strings.TrimSpace(expression), " ", "")
	if compact == "" || len(compact) > 100 {
		return ExpressionResult{}, ErrInvalidExpression
	}
	matches := termPattern.FindAllStringSubmatchIndex(compact, -1)
	if len(matches) == 0 {
		return ExpressionResult{}, ErrInvalidExpression
	}
	position, diceTotal := 0, 0
	result := ExpressionResult{}
	for _, indices := range matches {
		if indices[0] != position {
			return ExpressionResult{}, ErrInvalidExpression
		}
		position = indices[1]
		raw := compact[indices[0]:indices[1]]
		sign := 1
		if strings.HasPrefix(raw, "-") {
			sign = -1
		}
		term := TermResult{Sign: sign, Expression: strings.TrimLeft(raw, "+-")}
		if indices[4] >= 0 {
			count := 1
			if indices[4] != indices[5] {
				count, _ = strconv.Atoi(compact[indices[4]:indices[5]])
			}
			sides, _ := strconv.Atoi(compact[indices[6]:indices[7]])
			if count < 1 || count > 100 || sides < 2 || sides > 10000 || diceTotal+count > 200 {
				return ExpressionResult{}, ErrInvalidExpression
			}
			diceTotal += count
			for range count {
				value := secureInt(sides) + 1
				term.Rolls = append(term.Rolls, value)
				term.Value += value
			}
		} else {
			value, err := strconv.Atoi(compact[indices[8]:indices[9]])
			if err != nil || value > 1_000_000 {
				return ExpressionResult{}, ErrInvalidExpression
			}
			term.Value = value
		}
		result.Total += sign * term.Value
		result.Terms = append(result.Terms, term)
	}
	if position != len(compact) {
		return ExpressionResult{}, ErrInvalidExpression
	}
	return result, nil
}

func RollCheck(target, bonusPenalty int) (CheckResult, error) {
	if target < 0 || target > 999 || bonusPenalty < -2 || bonusPenalty > 2 {
		return CheckResult{}, fmt.Errorf("invalid check")
	}
	count := 1
	if bonusPenalty < 0 {
		count += -bonusPenalty
	} else {
		count += bonusPenalty
	}
	units := secureInt(10)
	result := CheckResult{Target: target, BonusPenalty: bonusPenalty, Units: units, Hard: target / 2, Extreme: target / 5}
	for range count {
		tens := secureInt(10)
		result.Tens = append(result.Tens, tens)
		value := tens*10 + units
		if value == 0 {
			value = 100
		}
		result.Candidates = append(result.Candidates, value)
	}
	result.Value = result.Candidates[0]
	for _, value := range result.Candidates[1:] {
		if bonusPenalty > 0 && value < result.Value {
			result.Value = value
		}
		if bonusPenalty < 0 && value > result.Value {
			result.Value = value
		}
	}
	result.Outcome = outcome(result.Value, target)
	return result, nil
}

func outcome(value, target int) string {
	if value == 1 {
		return "critical"
	}
	if (target < 50 && value >= 96) || value == 100 {
		return "fumble"
	}
	if value <= target/5 {
		return "extreme"
	}
	if value <= target/2 {
		return "hard"
	}
	if value <= target {
		return "regular"
	}
	return "failure"
}

func (s *Store) Save(ctx context.Context, requestID, actorID string, characterID, campaignID *string, visibility, kind, label, expression string, request, result any) (Roll, error) {
	return s.save(ctx, requestID, actorID, characterID, campaignID, visibility, kind, label, expression, request, result, nil, nil)
}

func (s *Store) SaveReroll(ctx context.Context, requestID, actorID string, characterID, campaignID *string, visibility, kind, label, expression string, request, result any, rerollOfID string) (Roll, error) {
	rerollKind := "reroll"
	return s.save(ctx, requestID, actorID, characterID, campaignID, visibility, kind, label, expression, request, result, &rerollOfID, &rerollKind)
}

func (s *Store) SaveRelated(ctx context.Context, requestID, actorID string, characterID, campaignID *string, visibility, kind, label, expression string, request, result any, rerollOfID, relationKind string) (Roll, error) {
	if relationKind != "push" && relationKind != "reroll" {
		return Roll{}, fmt.Errorf("invalid reroll relation")
	}
	return s.save(ctx, requestID, actorID, characterID, campaignID, visibility, kind, label, expression, request, result, &rerollOfID, &relationKind)
}

func (s *Store) save(ctx context.Context, requestID, actorID string, characterID, campaignID *string, visibility, kind, label, expression string, request, result any, rerollOfID, rerollKind *string) (Roll, error) {
	requestJSON, _ := json.Marshal(request)
	resultJSON, _ := json.Marshal(result)
	var existing Roll
	var existingRequest, existingResult string
	err := s.db.QueryRowContext(ctx, `SELECT id, request_id, actor_account_id, character_id, campaign_id, visibility, roll_kind, label, expression, request_data, result_data, reroll_of_id, reroll_kind, created_at FROM dice_rolls WHERE request_id = ?`, requestID).Scan(&existing.ID, &existing.RequestID, &existing.ActorAccountID, &existing.CharacterID, &existing.CampaignID, &existing.Visibility, &existing.Kind, &existing.Label, &existing.Expression, &existingRequest, &existingResult, &existing.RerollOfID, &existing.RerollKind, &existing.CreatedAt)
	if err == nil {
		if existing.ActorAccountID != actorID || existingRequest != string(requestJSON) {
			return Roll{}, ErrRequestConflict
		}
		existing.Result = json.RawMessage(existingResult)
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Roll{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	item := Roll{ID: randomID(), RequestID: requestID, ActorAccountID: actorID, CharacterID: characterID, CampaignID: campaignID, Visibility: visibility, Kind: kind, Label: label, Expression: expression, Result: resultJSON, RerollOfID: rerollOfID, RerollKind: rerollKind, CreatedAt: now}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Roll{}, err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO dice_rolls(id, request_id, actor_account_id, character_id, campaign_id, visibility, roll_kind, label, expression, request_data, result_data, reroll_of_id, reroll_kind, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, item.ID, requestID, actorID, characterID, campaignID, visibility, kind, label, expression, string(requestJSON), string(resultJSON), rerollOfID, rerollKind, now)
	if err != nil {
		return Roll{}, err
	}
	if campaignID != nil && visibility == "public" {
		_, err = tx.ExecContext(ctx, `INSERT INTO notification_deliveries(id, campaign_id, roll_id, provider, status, next_attempt_at, created_at, updated_at) SELECT ?, campaign_id, ?, provider, 'pending', ?, ?, ? FROM campaign_notification_settings WHERE campaign_id = ? AND provider != 'disabled'`, randomID(), item.ID, now, now, now, *campaignID)
		if err != nil {
			return Roll{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Roll{}, err
	}
	return item, nil
}

func (s *Store) GetForReroll(ctx context.Context, id, actorID string) (Roll, json.RawMessage, error) {
	var item Roll
	var requestData, resultData string
	err := s.db.QueryRowContext(ctx, `SELECT id, request_id, actor_account_id, character_id, campaign_id, visibility, roll_kind, label, expression, request_data, result_data, reroll_of_id, reroll_kind, created_at FROM dice_rolls WHERE id = ? AND actor_account_id = ?`, id, actorID).Scan(&item.ID, &item.RequestID, &item.ActorAccountID, &item.CharacterID, &item.CampaignID, &item.Visibility, &item.Kind, &item.Label, &item.Expression, &requestData, &resultData, &item.RerollOfID, &item.RerollKind, &item.CreatedAt)
	if err != nil {
		return Roll{}, nil, err
	}
	item.Result = json.RawMessage(resultData)
	return item, json.RawMessage(requestData), nil
}

func (s *Store) RerollRequestID(ctx context.Context, id string) (string, bool, error) {
	var requestID string
	err := s.db.QueryRowContext(ctx, `SELECT request_id FROM dice_rolls WHERE reroll_of_id = ? AND (reroll_kind = 'push' OR (reroll_kind IS NULL AND label LIKE '孤注一掷 · %')) LIMIT 1`, id).Scan(&requestID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return requestID, err == nil, err
}

func secureInt(max int) int {
	value, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		panic("system random source unavailable")
	}
	return int(value.Int64())
}
func randomID() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		panic(err)
	}
	return hex.EncodeToString(value)
}
