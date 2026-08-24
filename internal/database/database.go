package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type migration struct {
	version int
	apply   func(context.Context, *sql.Tx) error
}

func Open(ctx context.Context, path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if err := migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL) STRICT`); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}
	for _, item := range migrations() {
		var applied int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, item.version).Scan(&applied); err != nil {
			return err
		}
		if applied != 0 {
			continue
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", item.version, err)
		}
		if err := item.apply(ctx, tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %d: %w", item.version, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES (?, ?)`, item.version, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %d: %w", item.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", item.version, err)
		}
	}
	return nil
}

func migrations() []migration {
	return []migration{
		{1, func(ctx context.Context, tx *sql.Tx) error {
			return execAll(ctx, tx,
				`CREATE TABLE IF NOT EXISTS accounts (
				id TEXT PRIMARY KEY, username TEXT NOT NULL COLLATE NOCASE UNIQUE, display_name TEXT NOT NULL,
				password_hash TEXT NOT NULL, role TEXT NOT NULL CHECK (role IN ('admin', 'user')),
				status TEXT NOT NULL CHECK (status IN ('active', 'disabled')),
				must_change_password INTEGER NOT NULL CHECK (must_change_password IN (0, 1)),
				password_changed_at TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
			) STRICT`,
				`CREATE TABLE IF NOT EXISTS sessions (
				id TEXT PRIMARY KEY, account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
				token_hash BLOB NOT NULL UNIQUE, expires_at TEXT NOT NULL, created_at TEXT NOT NULL, last_seen_at TEXT NOT NULL
			) STRICT`,
				`CREATE INDEX IF NOT EXISTS sessions_account_id_idx ON sessions(account_id)`,
				`CREATE INDEX IF NOT EXISTS sessions_expires_at_idx ON sessions(expires_at)`,
			)
		}},
		{2, func(ctx context.Context, tx *sql.Tx) error {
			if err := ensureColumn(ctx, tx, "accounts", "last_login_at", "TEXT"); err != nil {
				return err
			}
			return execAll(ctx, tx,
				`CREATE TABLE IF NOT EXISTS admin_audit_logs (
					id TEXT PRIMARY KEY, actor_account_id TEXT NOT NULL REFERENCES accounts(id), action TEXT NOT NULL,
					target_account_id TEXT REFERENCES accounts(id), details TEXT NOT NULL DEFAULT '{}', created_at TEXT NOT NULL
				) STRICT`,
				`CREATE INDEX IF NOT EXISTS admin_audit_logs_created_at_idx ON admin_audit_logs(created_at DESC)`,
			)
		}},
		{3, func(ctx context.Context, tx *sql.Tx) error {
			return execAll(ctx, tx,
				`CREATE TABLE IF NOT EXISTS characters (
				id TEXT PRIMARY KEY, owner_account_id TEXT NOT NULL REFERENCES accounts(id),
				kind TEXT NOT NULL CHECK (kind IN ('investigator', 'npc')), ruleset TEXT NOT NULL CHECK (ruleset = 'coc7'),
				status TEXT NOT NULL CHECK (status IN ('draft', 'active', 'retired', 'deceased', 'archived')),
				name TEXT NOT NULL, current_version INTEGER NOT NULL, sheet_data TEXT NOT NULL CHECK (json_valid(sheet_data)),
				created_at TEXT NOT NULL, updated_at TEXT NOT NULL, archived_at TEXT
			) STRICT`,
				`CREATE INDEX IF NOT EXISTS characters_owner_idx ON characters(owner_account_id, updated_at DESC)`,
				`CREATE TABLE IF NOT EXISTS character_versions (
				id TEXT PRIMARY KEY, character_id TEXT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
				version INTEGER NOT NULL, parent_version INTEGER, actor_account_id TEXT NOT NULL REFERENCES accounts(id),
				source_campaign_id TEXT, change_kind TEXT NOT NULL CHECK (change_kind IN ('edit', 'generation', 'restore', 'import', 'system')),
				message TEXT, changed_paths TEXT NOT NULL CHECK (json_valid(changed_paths)),
				snapshot_data TEXT NOT NULL CHECK (json_valid(snapshot_data)), created_at TEXT NOT NULL,
				UNIQUE(character_id, version)
			) STRICT`,
				`CREATE INDEX IF NOT EXISTS character_versions_character_idx ON character_versions(character_id, version DESC)`,
			)
		}},
		{4, func(ctx context.Context, tx *sql.Tx) error {
			return execAll(ctx, tx,
				`CREATE TABLE IF NOT EXISTS dice_rolls (
				id TEXT PRIMARY KEY, request_id TEXT NOT NULL UNIQUE,
				actor_account_id TEXT NOT NULL REFERENCES accounts(id), character_id TEXT REFERENCES characters(id),
				campaign_id TEXT, visibility TEXT NOT NULL CHECK (visibility IN ('public', 'keeper', 'test')),
				roll_kind TEXT NOT NULL CHECK (roll_kind IN ('check', 'expression', 'damage', 'attribute_generation')),
				label TEXT NOT NULL, expression TEXT NOT NULL, request_data TEXT NOT NULL CHECK (json_valid(request_data)),
				result_data TEXT NOT NULL CHECK (json_valid(result_data)), reroll_of_id TEXT REFERENCES dice_rolls(id), created_at TEXT NOT NULL
			) STRICT`,
				`CREATE INDEX IF NOT EXISTS dice_rolls_created_at_idx ON dice_rolls(created_at DESC)`,
				`CREATE INDEX IF NOT EXISTS dice_rolls_character_idx ON dice_rolls(character_id, created_at DESC)`,
			)
		}},
		{5, func(ctx context.Context, tx *sql.Tx) error {
			return execAll(ctx, tx,
				`CREATE TABLE IF NOT EXISTS campaigns (
					id TEXT PRIMARY KEY, keeper_account_id TEXT NOT NULL REFERENCES accounts(id),
					title TEXT NOT NULL, summary TEXT NOT NULL DEFAULT '', status TEXT NOT NULL
					CHECK (status IN ('preparing', 'active', 'finished', 'archived')),
					cover_asset_id TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, archived_at TEXT
				) STRICT`,
				`CREATE INDEX IF NOT EXISTS campaigns_updated_idx ON campaigns(updated_at DESC)`,
				`CREATE TABLE IF NOT EXISTS campaign_assets (
					id TEXT PRIMARY KEY, campaign_id TEXT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
					uploader_account_id TEXT NOT NULL REFERENCES accounts(id), storage_path TEXT NOT NULL UNIQUE,
					original_name TEXT NOT NULL, mime_type TEXT NOT NULL, byte_size INTEGER NOT NULL,
					width INTEGER, height INTEGER, created_at TEXT NOT NULL
				) STRICT`,
				`CREATE TABLE IF NOT EXISTS campaign_blocks (
					id TEXT PRIMARY KEY, campaign_id TEXT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
					block_type TEXT NOT NULL CHECK (block_type IN ('heading', 'text', 'image', 'clue')),
					title TEXT NOT NULL DEFAULT '', content TEXT NOT NULL DEFAULT '',
					visibility TEXT NOT NULL CHECK (visibility IN ('public', 'keeper')),
					position INTEGER NOT NULL, asset_id TEXT REFERENCES campaign_assets(id),
					published_at TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
					UNIQUE(campaign_id, position)
				) STRICT`,
				`CREATE INDEX IF NOT EXISTS campaign_blocks_order_idx ON campaign_blocks(campaign_id, position)`,
				`CREATE TABLE IF NOT EXISTS campaign_characters (
					campaign_id TEXT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
					character_id TEXT NOT NULL REFERENCES characters(id), role TEXT NOT NULL CHECK (role IN ('investigator', 'npc')),
					visibility TEXT NOT NULL CHECK (visibility IN ('hidden', 'summary', 'public')),
					joined_at TEXT NOT NULL, left_at TEXT, PRIMARY KEY(campaign_id, character_id)
				) STRICT`,
			)
		}},
		{6, func(ctx context.Context, tx *sql.Tx) error {
			return execAll(ctx, tx,
				`CREATE TABLE IF NOT EXISTS campaign_notification_settings (
					campaign_id TEXT PRIMARY KEY REFERENCES campaigns(id) ON DELETE CASCADE,
					provider TEXT NOT NULL CHECK (provider IN ('disabled', 'console', 'qq_official')),
					target_reference TEXT NOT NULL DEFAULT '', event_settings TEXT NOT NULL DEFAULT '{}'
					CHECK (json_valid(event_settings)), secret_reference TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL
				) STRICT`,
				`CREATE TABLE IF NOT EXISTS notification_deliveries (
					id TEXT PRIMARY KEY, campaign_id TEXT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
					roll_id TEXT NOT NULL REFERENCES dice_rolls(id) ON DELETE CASCADE, provider TEXT NOT NULL,
					status TEXT NOT NULL CHECK (status IN ('pending', 'sending', 'sent', 'failed', 'skipped')),
					attempts INTEGER NOT NULL DEFAULT 0, next_attempt_at TEXT NOT NULL, last_error TEXT NOT NULL DEFAULT '',
					created_at TEXT NOT NULL, updated_at TEXT NOT NULL, UNIQUE(roll_id, provider)
				) STRICT`,
				`CREATE INDEX IF NOT EXISTS notification_pending_idx ON notification_deliveries(status, next_attempt_at)`,
			)
		}},
		{7, func(ctx context.Context, tx *sql.Tx) error {
			return ensureColumn(ctx, tx, "dice_rolls", "reroll_kind", "TEXT CHECK (reroll_kind IN ('push', 'reroll'))")
		}},
	}
}

func execAll(ctx context.Context, tx *sql.Tx, statements ...string) error {
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func ensureColumn(ctx context.Context, tx *sql.Tx, table, column, definition string) error {
	rows, err := tx.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return fmt.Errorf("inspect table %s: %w", table, err)
	}
	found := false
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, dataType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			rows.Close()
			return err
		}
		if name == column {
			found = true
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if found {
		return nil
	}
	if _, err := tx.ExecContext(ctx, "ALTER TABLE "+table+" ADD COLUMN "+column+" "+definition); err != nil {
		return fmt.Errorf("add column %s.%s: %w", table, column, err)
	}
	return nil
}
