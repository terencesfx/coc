package campaign

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrNotFound  = errors.New("campaign not found")
	ErrForbidden = errors.New("campaign keeper required")
)

type Campaign struct {
	ID              string  `json:"id"`
	KeeperAccountID string  `json:"keeperAccountId"`
	KeeperName      string  `json:"keeperName"`
	Title           string  `json:"title"`
	Summary         string  `json:"summary"`
	Status          string  `json:"status"`
	CoverAssetID    *string `json:"coverAssetId"`
	CreatedAt       string  `json:"createdAt"`
	UpdatedAt       string  `json:"updatedAt"`
	CanManage       bool    `json:"canManage"`
}

type Block struct {
	ID          string  `json:"id"`
	CampaignID  string  `json:"campaignId"`
	Type        string  `json:"type"`
	Title       string  `json:"title"`
	Content     string  `json:"content"`
	Visibility  string  `json:"visibility"`
	Position    int     `json:"position"`
	AssetID     *string `json:"assetId"`
	PublishedAt *string `json:"publishedAt"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   string  `json:"updatedAt"`
}

type Asset struct {
	ID           string `json:"id"`
	CampaignID   string `json:"campaignId"`
	OriginalName string `json:"originalName"`
	MimeType     string `json:"mimeType"`
	ByteSize     int64  `json:"byteSize"`
	Width        *int   `json:"width"`
	Height       *int   `json:"height"`
	CreatedAt    string `json:"createdAt"`
	StoragePath  string `json:"-"`
}
type CharacterLink struct {
	CharacterID    string `json:"characterId"`
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	OwnerAccountID string `json:"ownerAccountId"`
	OwnerName      string `json:"ownerName"`
	Role           string `json:"role"`
	Visibility     string `json:"visibility"`
	JoinedAt       string `json:"joinedAt"`
}

type Store struct {
	db       *sql.DB
	assetDir string
}

func NewStore(db *sql.DB, assetDir ...string) *Store {
	dir := ".data/assets"
	if len(assetDir) > 0 {
		dir = assetDir[0]
	}
	return &Store{db: db, assetDir: dir}
}

func (s *Store) Create(ctx context.Context, keeperID, title, summary string) (Campaign, error) {
	title = strings.TrimSpace(title)
	summary = strings.TrimSpace(summary)
	if title == "" || len(title) > 120 || len(summary) > 1000 {
		return Campaign{}, fmt.Errorf("invalid campaign")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	id := randomID()
	if _, err := s.db.ExecContext(ctx, `INSERT INTO campaigns(id, keeper_account_id, title, summary, status, created_at, updated_at) VALUES (?, ?, ?, ?, 'preparing', ?, ?)`, id, keeperID, title, summary, now, now); err != nil {
		return Campaign{}, err
	}
	return s.Get(ctx, id, keeperID)
}

func (s *Store) List(ctx context.Context, viewerID string) ([]Campaign, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT c.id, c.keeper_account_id, a.display_name, c.title, c.summary, c.status, c.cover_asset_id, c.created_at, c.updated_at, c.keeper_account_id = ? FROM campaigns c JOIN accounts a ON a.id = c.keeper_account_id WHERE c.archived_at IS NULL ORDER BY c.updated_at DESC`, viewerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Campaign{}
	for rows.Next() {
		var item Campaign
		if err := rows.Scan(&item.ID, &item.KeeperAccountID, &item.KeeperName, &item.Title, &item.Summary, &item.Status, &item.CoverAssetID, &item.CreatedAt, &item.UpdatedAt, &item.CanManage); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListForCharacter(ctx context.Context, characterID, viewerID string) ([]Campaign, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT cp.id, cp.keeper_account_id, a.display_name, cp.title, cp.summary, cp.status, cp.cover_asset_id, cp.created_at, cp.updated_at, cp.keeper_account_id = ? FROM campaign_characters cc JOIN campaigns cp ON cp.id = cc.campaign_id JOIN accounts a ON a.id = cp.keeper_account_id JOIN characters ch ON ch.id = cc.character_id WHERE cc.character_id = ? AND cc.left_at IS NULL AND cp.archived_at IS NULL AND (ch.owner_account_id = ? OR cp.keeper_account_id = ?) ORDER BY cp.updated_at DESC`, viewerID, characterID, viewerID, viewerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Campaign{}
	for rows.Next() {
		var item Campaign
		if err := rows.Scan(&item.ID, &item.KeeperAccountID, &item.KeeperName, &item.Title, &item.Summary, &item.Status, &item.CoverAssetID, &item.CreatedAt, &item.UpdatedAt, &item.CanManage); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) Get(ctx context.Context, id, viewerID string) (Campaign, error) {
	var item Campaign
	err := s.db.QueryRowContext(ctx, `SELECT c.id, c.keeper_account_id, a.display_name, c.title, c.summary, c.status, c.cover_asset_id, c.created_at, c.updated_at, c.keeper_account_id = ? FROM campaigns c JOIN accounts a ON a.id = c.keeper_account_id WHERE c.id = ? AND c.archived_at IS NULL`, viewerID, id).Scan(&item.ID, &item.KeeperAccountID, &item.KeeperName, &item.Title, &item.Summary, &item.Status, &item.CoverAssetID, &item.CreatedAt, &item.UpdatedAt, &item.CanManage)
	if errors.Is(err, sql.ErrNoRows) {
		return Campaign{}, ErrNotFound
	}
	return item, err
}

func (s *Store) Update(ctx context.Context, id, keeperID, title, summary, status string) (Campaign, error) {
	title, summary = strings.TrimSpace(title), strings.TrimSpace(summary)
	if title == "" || len(title) > 120 || len(summary) > 1000 || !validStatus(status) {
		return Campaign{}, fmt.Errorf("invalid campaign")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(ctx, `UPDATE campaigns SET title = ?, summary = ?, status = ?, updated_at = ? WHERE id = ? AND keeper_account_id = ? AND archived_at IS NULL`, title, summary, status, now, id, keeperID)
	if err != nil {
		return Campaign{}, err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		if _, err := s.Get(ctx, id, keeperID); errors.Is(err, ErrNotFound) {
			return Campaign{}, ErrNotFound
		}
		return Campaign{}, ErrForbidden
	}
	return s.Get(ctx, id, keeperID)
}

func (s *Store) Archive(ctx context.Context, id, keeperID string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE campaigns SET archived_at = ?, updated_at = ? WHERE id = ? AND keeper_account_id = ? AND archived_at IS NULL`, now, now, id, keeperID)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return ErrForbidden
	}
	if _, err := tx.ExecContext(ctx, `UPDATE campaign_characters SET left_at = ? WHERE campaign_id = ? AND left_at IS NULL`, now, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) SetCover(ctx context.Context, id, keeperID string, assetID *string) (Campaign, error) {
	if err := s.requireKeeper(ctx, id, keeperID); err != nil {
		return Campaign{}, err
	}
	if assetID != nil {
		var found int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM campaign_assets WHERE id = ? AND campaign_id = ?`, *assetID, id).Scan(&found); err != nil {
			return Campaign{}, err
		}
		if found != 1 {
			return Campaign{}, ErrNotFound
		}
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE campaigns SET cover_asset_id = ?, updated_at = ? WHERE id = ?`, assetID, time.Now().UTC().Format(time.RFC3339Nano), id); err != nil {
		return Campaign{}, err
	}
	return s.Get(ctx, id, keeperID)
}

func (s *Store) ListBlocks(ctx context.Context, campaignID, viewerID string) ([]Block, error) {
	item, err := s.Get(ctx, campaignID, viewerID)
	if err != nil {
		return nil, err
	}
	query := `SELECT id, campaign_id, block_type, title, content, visibility, position, asset_id, published_at, created_at, updated_at FROM campaign_blocks WHERE campaign_id = ?`
	args := []any{campaignID}
	if !item.CanManage {
		query += ` AND visibility = 'public'`
	}
	query += ` ORDER BY position`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Block{}
	for rows.Next() {
		var block Block
		if err := rows.Scan(&block.ID, &block.CampaignID, &block.Type, &block.Title, &block.Content, &block.Visibility, &block.Position, &block.AssetID, &block.PublishedAt, &block.CreatedAt, &block.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, block)
	}
	return items, rows.Err()
}

func (s *Store) CreateBlock(ctx context.Context, campaignID, keeperID, blockType, title, content, visibility string, assetID *string) (Block, error) {
	if err := s.requireKeeper(ctx, campaignID, keeperID); err != nil {
		return Block{}, err
	}
	if err := validateBlock(blockType, title, content, visibility); err != nil {
		return Block{}, err
	}
	if err := s.validateAsset(ctx, campaignID, blockType, assetID); err != nil {
		return Block{}, err
	}
	var position int
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(position), 0) + 1000 FROM campaign_blocks WHERE campaign_id = ?`, campaignID).Scan(&position); err != nil {
		return Block{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	block := Block{ID: randomID(), CampaignID: campaignID, Type: blockType, Title: strings.TrimSpace(title), Content: content, Visibility: visibility, Position: position, AssetID: assetID, CreatedAt: now, UpdatedAt: now}
	if visibility == "public" {
		block.PublishedAt = &now
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO campaign_blocks(id, campaign_id, block_type, title, content, visibility, position, asset_id, published_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, block.ID, campaignID, block.Type, block.Title, block.Content, block.Visibility, block.Position, block.AssetID, block.PublishedAt, now, now)
	return block, err
}

func (s *Store) UpdateBlock(ctx context.Context, campaignID, blockID, keeperID, blockType, title, content, visibility string, assetID *string) (Block, error) {
	if err := s.requireKeeper(ctx, campaignID, keeperID); err != nil {
		return Block{}, err
	}
	if err := validateBlock(blockType, title, content, visibility); err != nil {
		return Block{}, err
	}
	if err := s.validateAsset(ctx, campaignID, blockType, assetID); err != nil {
		return Block{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(ctx, `UPDATE campaign_blocks SET block_type = ?, title = ?, content = ?, visibility = ?, asset_id = ?, published_at = CASE WHEN ? = 'public' THEN COALESCE(published_at, ?) ELSE published_at END, updated_at = ? WHERE id = ? AND campaign_id = ?`, blockType, strings.TrimSpace(title), content, visibility, assetID, visibility, now, now, blockID, campaignID)
	if err != nil {
		return Block{}, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return Block{}, ErrNotFound
	}
	return s.getBlock(ctx, campaignID, blockID)
}

func (s *Store) DeleteBlock(ctx context.Context, campaignID, blockID, keeperID string) error {
	if err := s.requireKeeper(ctx, campaignID, keeperID); err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM campaign_blocks WHERE id = ? AND campaign_id = ?`, blockID, campaignID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) MoveBlock(ctx context.Context, campaignID, blockID, keeperID, direction string) error {
	if err := s.requireKeeper(ctx, campaignID, keeperID); err != nil {
		return err
	}
	if direction != "up" && direction != "down" {
		return fmt.Errorf("invalid direction")
	}
	operator, order := "<", "DESC"
	if direction == "down" {
		operator, order = ">", "ASC"
	}
	var current, adjacent int
	if err := s.db.QueryRowContext(ctx, `SELECT position FROM campaign_blocks WHERE id = ? AND campaign_id = ?`, blockID, campaignID).Scan(&current); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	query := fmt.Sprintf(`SELECT position FROM campaign_blocks WHERE campaign_id = ? AND position %s ? ORDER BY position %s LIMIT 1`, operator, order)
	if err := s.db.QueryRowContext(ctx, query, campaignID, current).Scan(&adjacent); errors.Is(err, sql.ErrNoRows) {
		return nil
	} else if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE campaign_blocks SET position = -1 WHERE id = ?`, blockID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE campaign_blocks SET position = ? WHERE campaign_id = ? AND position = ?`, current, campaignID, adjacent); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE campaign_blocks SET position = ? WHERE id = ?`, adjacent, blockID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) getBlock(ctx context.Context, campaignID, blockID string) (Block, error) {
	var block Block
	err := s.db.QueryRowContext(ctx, `SELECT id, campaign_id, block_type, title, content, visibility, position, asset_id, published_at, created_at, updated_at FROM campaign_blocks WHERE id = ? AND campaign_id = ?`, blockID, campaignID).Scan(&block.ID, &block.CampaignID, &block.Type, &block.Title, &block.Content, &block.Visibility, &block.Position, &block.AssetID, &block.PublishedAt, &block.CreatedAt, &block.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Block{}, ErrNotFound
	}
	return block, err
}

func (s *Store) requireKeeper(ctx context.Context, campaignID, accountID string) error {
	item, err := s.Get(ctx, campaignID, accountID)
	if err != nil {
		return err
	}
	if !item.CanManage {
		return ErrForbidden
	}
	return nil
}

func validateBlock(blockType, title, content, visibility string) error {
	if (blockType != "heading" && blockType != "text" && blockType != "clue" && blockType != "image") || (visibility != "public" && visibility != "keeper") || len(title) > 200 || len(content) > 100_000 {
		return fmt.Errorf("invalid block")
	}
	if blockType == "heading" && strings.TrimSpace(title) == "" {
		return fmt.Errorf("heading title required")
	}
	return nil
}

func (s *Store) SaveAsset(ctx context.Context, campaignID, keeperID, originalName, mimeType, extension string, data []byte, width, height *int) (Asset, error) {
	if err := s.requireKeeper(ctx, campaignID, keeperID); err != nil {
		return Asset{}, err
	}
	id := randomID()
	relative := filepath.Join(campaignID, id+extension)
	directory := filepath.Join(s.assetDir, campaignID)
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return Asset{}, err
	}
	path := filepath.Join(s.assetDir, relative)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return Asset{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	asset := Asset{ID: id, CampaignID: campaignID, OriginalName: filepath.Base(originalName), MimeType: mimeType, ByteSize: int64(len(data)), Width: width, Height: height, CreatedAt: now, StoragePath: relative}
	_, err := s.db.ExecContext(ctx, `INSERT INTO campaign_assets(id, campaign_id, uploader_account_id, storage_path, original_name, mime_type, byte_size, width, height, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, id, campaignID, keeperID, relative, asset.OriginalName, mimeType, len(data), width, height, now)
	if err != nil {
		_ = os.Remove(path)
		return Asset{}, err
	}
	return asset, nil
}

func (s *Store) GetVisibleAsset(ctx context.Context, campaignID, assetID, viewerID string) (Asset, error) {
	var asset Asset
	err := s.db.QueryRowContext(ctx, `SELECT a.id, a.campaign_id, a.original_name, a.mime_type, a.byte_size, a.width, a.height, a.created_at, a.storage_path FROM campaign_assets a JOIN campaigns c ON c.id = a.campaign_id WHERE a.id = ? AND a.campaign_id = ? AND (c.keeper_account_id = ? OR c.cover_asset_id = a.id OR EXISTS (SELECT 1 FROM campaign_blocks b WHERE b.asset_id = a.id AND b.visibility = 'public'))`, assetID, campaignID, viewerID).Scan(&asset.ID, &asset.CampaignID, &asset.OriginalName, &asset.MimeType, &asset.ByteSize, &asset.Width, &asset.Height, &asset.CreatedAt, &asset.StoragePath)
	if errors.Is(err, sql.ErrNoRows) {
		return Asset{}, ErrNotFound
	}
	return asset, err
}

func (s *Store) AssetPath(asset Asset) string { return filepath.Join(s.assetDir, asset.StoragePath) }

func (s *Store) validateAsset(ctx context.Context, campaignID, blockType string, assetID *string) error {
	if blockType != "image" && assetID != nil {
		return fmt.Errorf("asset only valid for image")
	}
	if blockType == "image" && assetID == nil {
		return fmt.Errorf("image asset required")
	}
	if assetID == nil {
		return nil
	}
	var found int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM campaign_assets WHERE id = ? AND campaign_id = ?`, *assetID, campaignID).Scan(&found); err != nil {
		return err
	}
	if found != 1 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) AttachCharacter(ctx context.Context, campaignID, characterID, actorID string) (CharacterLink, error) {
	camp, err := s.Get(ctx, campaignID, actorID)
	if err != nil {
		return CharacterLink{}, err
	}
	var ownerID, kind string
	err = s.db.QueryRowContext(ctx, `SELECT owner_account_id, kind FROM characters WHERE id = ? AND archived_at IS NULL`, characterID).Scan(&ownerID, &kind)
	if errors.Is(err, sql.ErrNoRows) {
		return CharacterLink{}, ErrNotFound
	}
	if err != nil {
		return CharacterLink{}, err
	}
	if ownerID != actorID {
		return CharacterLink{}, ErrForbidden
	}
	if !camp.CanManage && kind != "investigator" {
		return CharacterLink{}, ErrForbidden
	}
	visibility, role := "public", "investigator"
	if kind == "npc" {
		visibility, role = "hidden", "npc"
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(ctx, `INSERT INTO campaign_characters(campaign_id, character_id, role, visibility, joined_at) VALUES (?, ?, ?, ?, ?) ON CONFLICT(campaign_id, character_id) DO UPDATE SET role = excluded.role, visibility = excluded.visibility, joined_at = excluded.joined_at, left_at = NULL`, campaignID, characterID, role, visibility, now)
	if err != nil {
		return CharacterLink{}, err
	}
	return s.getCharacterLink(ctx, campaignID, characterID)
}

func (s *Store) ListCharacters(ctx context.Context, campaignID, viewerID string) ([]CharacterLink, error) {
	camp, err := s.Get(ctx, campaignID, viewerID)
	if err != nil {
		return nil, err
	}
	query := `SELECT cc.character_id, c.name, c.kind, c.owner_account_id, a.display_name, cc.role, cc.visibility, cc.joined_at FROM campaign_characters cc JOIN characters c ON c.id = cc.character_id JOIN accounts a ON a.id = c.owner_account_id WHERE cc.campaign_id = ? AND cc.left_at IS NULL`
	if !camp.CanManage {
		query += ` AND cc.visibility = 'public'`
	}
	query += ` ORDER BY cc.joined_at`
	rows, err := s.db.QueryContext(ctx, query, campaignID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []CharacterLink{}
	for rows.Next() {
		var item CharacterLink
		if err := rows.Scan(&item.CharacterID, &item.Name, &item.Kind, &item.OwnerAccountID, &item.OwnerName, &item.Role, &item.Visibility, &item.JoinedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) SetCharacterVisibility(ctx context.Context, campaignID, characterID, keeperID, visibility string) (CharacterLink, error) {
	if err := s.requireKeeper(ctx, campaignID, keeperID); err != nil {
		return CharacterLink{}, err
	}
	if visibility != "public" && visibility != "hidden" {
		return CharacterLink{}, fmt.Errorf("invalid visibility")
	}
	var role string
	if err := s.db.QueryRowContext(ctx, `SELECT role FROM campaign_characters WHERE campaign_id = ? AND character_id = ? AND left_at IS NULL`, campaignID, characterID).Scan(&role); errors.Is(err, sql.ErrNoRows) {
		return CharacterLink{}, ErrNotFound
	} else if err != nil {
		return CharacterLink{}, err
	}
	if role == "investigator" {
		visibility = "public"
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE campaign_characters SET visibility = ? WHERE campaign_id = ? AND character_id = ?`, visibility, campaignID, characterID); err != nil {
		return CharacterLink{}, err
	}
	return s.getCharacterLink(ctx, campaignID, characterID)
}

func (s *Store) DetachCharacter(ctx context.Context, campaignID, characterID, actorID string) error {
	camp, err := s.Get(ctx, campaignID, actorID)
	if err != nil {
		return err
	}
	var ownerID string
	if err := s.db.QueryRowContext(ctx, `SELECT c.owner_account_id FROM campaign_characters cc JOIN characters c ON c.id = cc.character_id WHERE cc.campaign_id = ? AND cc.character_id = ? AND cc.left_at IS NULL`, campaignID, characterID).Scan(&ownerID); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if !camp.CanManage && ownerID != actorID {
		return ErrForbidden
	}
	_, err = s.db.ExecContext(ctx, `UPDATE campaign_characters SET left_at = ? WHERE campaign_id = ? AND character_id = ?`, time.Now().UTC().Format(time.RFC3339Nano), campaignID, characterID)
	return err
}

func (s *Store) getCharacterLink(ctx context.Context, campaignID, characterID string) (CharacterLink, error) {
	var item CharacterLink
	err := s.db.QueryRowContext(ctx, `SELECT cc.character_id, c.name, c.kind, c.owner_account_id, a.display_name, cc.role, cc.visibility, cc.joined_at FROM campaign_characters cc JOIN characters c ON c.id = cc.character_id JOIN accounts a ON a.id = c.owner_account_id WHERE cc.campaign_id = ? AND cc.character_id = ? AND cc.left_at IS NULL`, campaignID, characterID).Scan(&item.CharacterID, &item.Name, &item.Kind, &item.OwnerAccountID, &item.OwnerName, &item.Role, &item.Visibility, &item.JoinedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return CharacterLink{}, ErrNotFound
	}
	return item, err
}

func (s *Store) ValidateRollContext(ctx context.Context, campaignID string, characterID *string, actorID string) error {
	camp, err := s.Get(ctx, campaignID, actorID)
	if err != nil {
		return err
	}
	if characterID == nil {
		if !camp.CanManage {
			return ErrForbidden
		}
		return nil
	}
	var allowed int
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM campaign_characters cc JOIN characters c ON c.id = cc.character_id WHERE cc.campaign_id = ? AND cc.character_id = ? AND cc.left_at IS NULL AND (c.owner_account_id = ? OR ? = 1)`, campaignID, *characterID, actorID, camp.CanManage).Scan(&allowed)
	if err != nil {
		return err
	}
	if allowed != 1 {
		return ErrForbidden
	}
	return nil
}

func validStatus(value string) bool {
	return value == "preparing" || value == "active" || value == "finished" || value == "archived"
}

func randomID() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		panic(err)
	}
	return hex.EncodeToString(value)
}
