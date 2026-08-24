package account

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUsernameTaken      = errors.New("username already exists")
	ErrAccountDisabled    = errors.New("account disabled")
	ErrSessionNotFound    = errors.New("session not found")
	ErrAccountNotFound    = errors.New("account not found")
	ErrLastActiveAdmin    = errors.New("cannot disable last active admin")
)

type Account struct {
	ID                 string  `json:"id"`
	Username           string  `json:"username"`
	DisplayName        string  `json:"displayName"`
	Role               string  `json:"role"`
	Status             string  `json:"status"`
	MustChangePassword bool    `json:"mustChangePassword"`
	CreatedAt          string  `json:"createdAt"`
	LastLoginAt        *string `json:"lastLoginAt"`
}

type AuditLog struct {
	ID               string  `json:"id"`
	ActorAccountID   string  `json:"actorAccountId"`
	ActorDisplayName string  `json:"actorDisplayName"`
	Action           string  `json:"action"`
	TargetAccountID  *string `json:"targetAccountId"`
	Details          string  `json:"details"`
	CreatedAt        string  `json:"createdAt"`
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Create(ctx context.Context, username, displayName, password, role string, mustChange bool) (Account, error) {
	username = strings.TrimSpace(username)
	displayName = strings.TrimSpace(displayName)
	if username == "" || displayName == "" || password == "" {
		return Account{}, fmt.Errorf("invalid account data")
	}
	if role != "admin" && role != "user" {
		return Account{}, fmt.Errorf("invalid role")
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return Account{}, fmt.Errorf("hash password: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	created := Account{
		ID:                 randomID(),
		Username:           username,
		DisplayName:        displayName,
		Role:               role,
		Status:             "active",
		MustChangePassword: mustChange,
		CreatedAt:          now,
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO accounts(
			id, username, display_name, password_hash, role, status,
			must_change_password, password_changed_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, 'active', ?, ?, ?, ?)`,
		created.ID, created.Username, created.DisplayName, string(passwordHash), created.Role,
		boolInt(mustChange), now, now, now,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return Account{}, ErrUsernameTaken
		}
		return Account{}, fmt.Errorf("insert account: %w", err)
	}
	return created, nil
}

func (s *Store) Authenticate(ctx context.Context, username, password string) (Account, error) {
	var result Account
	var passwordHash string
	var mustChange int
	err := s.db.QueryRowContext(ctx, `
		SELECT id, username, display_name, password_hash, role, status, must_change_password, created_at, last_login_at
		FROM accounts WHERE username = ?`, strings.TrimSpace(username),
	).Scan(&result.ID, &result.Username, &result.DisplayName, &passwordHash, &result.Role, &result.Status, &mustChange, &result.CreatedAt, &result.LastLoginAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrInvalidCredentials
	}
	if err != nil {
		return Account{}, fmt.Errorf("select account: %w", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) != nil {
		return Account{}, ErrInvalidCredentials
	}
	if result.Status != "active" {
		return Account{}, ErrAccountDisabled
	}
	result.MustChangePassword = mustChange == 1
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := s.db.ExecContext(ctx, `UPDATE accounts SET last_login_at = ?, updated_at = ? WHERE id = ?`, now, now, result.ID); err != nil {
		return Account{}, fmt.Errorf("update last login: %w", err)
	}
	result.LastLoginAt = &now
	return result, nil
}

func (s *Store) CreateSession(ctx context.Context, accountID string, ttl time.Duration) (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)
	tokenHash := sha256.Sum256([]byte(token))
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions(id, account_id, token_hash, expires_at, created_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		randomID(), accountID, tokenHash[:], now.Add(ttl).Format(time.RFC3339Nano),
		now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano),
	)
	if err != nil {
		return "", fmt.Errorf("insert session: %w", err)
	}
	return token, nil
}

func (s *Store) AccountBySession(ctx context.Context, token string) (Account, error) {
	tokenHash := sha256.Sum256([]byte(token))
	var result Account
	var mustChange int
	var expiresAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT a.id, a.username, a.display_name, a.role, a.status,
		       a.must_change_password, a.created_at, a.last_login_at, s.expires_at
		FROM sessions s JOIN accounts a ON a.id = s.account_id
		WHERE s.token_hash = ?`, tokenHash[:],
	).Scan(&result.ID, &result.Username, &result.DisplayName, &result.Role, &result.Status, &mustChange, &result.CreatedAt, &result.LastLoginAt, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrSessionNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("select session: %w", err)
	}
	expiry, err := time.Parse(time.RFC3339Nano, expiresAt)
	if err != nil || time.Now().UTC().After(expiry) || result.Status != "active" {
		_ = s.DeleteSession(ctx, token)
		return Account{}, ErrSessionNotFound
	}
	result.MustChangePassword = mustChange == 1
	return result, nil
}

func (s *Store) DeleteSession(ctx context.Context, token string) error {
	tokenHash := sha256.Sum256([]byte(token))
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash[:])
	return err
}

func (s *Store) ChangePassword(ctx context.Context, accountID, currentPassword, newPassword string) error {
	if newPassword == "" {
		return fmt.Errorf("new password is empty")
	}
	var currentHash string
	if err := s.db.QueryRowContext(ctx, `SELECT password_hash FROM accounts WHERE id = ?`, accountID).Scan(&currentHash); err != nil {
		return fmt.Errorf("select password: %w", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(currentPassword)) != nil {
		return ErrInvalidCredentials
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `
		UPDATE accounts SET password_hash = ?, must_change_password = 0,
		password_changed_at = ?, updated_at = ? WHERE id = ?`, string(newHash), now, now, accountID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE account_id = ?`, accountID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) List(ctx context.Context) ([]Account, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, username, display_name, role, status, must_change_password, created_at, last_login_at
		FROM accounts ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []Account
	for rows.Next() {
		var item Account
		var mustChange int
		if err := rows.Scan(&item.ID, &item.Username, &item.DisplayName, &item.Role, &item.Status, &mustChange, &item.CreatedAt, &item.LastLoginAt); err != nil {
			return nil, err
		}
		item.MustChangePassword = mustChange == 1
		accounts = append(accounts, item)
	}
	return accounts, rows.Err()
}

func (s *Store) ResetPassword(ctx context.Context, accountID, newPassword string) error {
	if newPassword == "" {
		return fmt.Errorf("new password is empty")
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx, `
		UPDATE accounts SET password_hash = ?, must_change_password = 1,
		password_changed_at = ?, updated_at = ? WHERE id = ?`, string(newHash), now, now, accountID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrAccountNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE account_id = ?`, accountID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) SetStatus(ctx context.Context, accountID, status string) error {
	if status != "active" && status != "disabled" {
		return fmt.Errorf("invalid status")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if status == "disabled" {
		var role, currentStatus string
		if err := tx.QueryRowContext(ctx, `SELECT role, status FROM accounts WHERE id = ?`, accountID).Scan(&role, &currentStatus); errors.Is(err, sql.ErrNoRows) {
			return ErrAccountNotFound
		} else if err != nil {
			return err
		}
		if role == "admin" && currentStatus == "active" {
			var activeAdmins int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM accounts WHERE role = 'admin' AND status = 'active'`).Scan(&activeAdmins); err != nil {
				return err
			}
			if activeAdmins <= 1 {
				return ErrLastActiveAdmin
			}
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx, `UPDATE accounts SET status = ?, updated_at = ? WHERE id = ?`, status, now, accountID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrAccountNotFound
	}
	if status == "disabled" {
		if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE account_id = ?`, accountID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) RevokeSessions(ctx context.Context, accountID string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE account_id = ?`, accountID)
	if err != nil {
		return err
	}
	_ = result
	return nil
}

func (s *Store) RecordAudit(ctx context.Context, actorID, action string, targetID *string, details string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO admin_audit_logs(id, actor_account_id, action, target_account_id, details, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`, randomID(), actorID, action, targetID, details, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) ListAudit(ctx context.Context, limit int) ([]AuditLog, error) {
	if limit < 1 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT l.id, l.actor_account_id, a.display_name, l.action, l.target_account_id, l.details, l.created_at
		FROM admin_audit_logs l JOIN accounts a ON a.id = l.actor_account_id
		ORDER BY l.created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var logs []AuditLog
	for rows.Next() {
		var item AuditLog
		if err := rows.Scan(&item.ID, &item.ActorAccountID, &item.ActorDisplayName, &item.Action, &item.TargetAccountID, &item.Details, &item.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, item)
	}
	return logs, rows.Err()
}

func randomID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		panic("system random source unavailable")
	}
	return hex.EncodeToString(bytes)
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
