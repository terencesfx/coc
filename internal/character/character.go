package character

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mushan/coc/internal/rules/coc7"
)

var (
	ErrNotFound = errors.New("character not found")
	ErrConflict = errors.New("character version conflict")
)

type Character struct {
	ID             string          `json:"id"`
	OwnerAccountID string          `json:"ownerAccountId"`
	Kind           string          `json:"kind"`
	Ruleset        string          `json:"ruleset"`
	Status         string          `json:"status"`
	Name           string          `json:"name"`
	CurrentVersion int             `json:"currentVersion"`
	Sheet          json.RawMessage `json:"sheet"`
	CreatedAt      string          `json:"createdAt"`
	UpdatedAt      string          `json:"updatedAt"`
	CanEdit        bool            `json:"canEdit"`
}

type Summary struct {
	ID             string `json:"id"`
	Kind           string `json:"kind"`
	Status         string `json:"status"`
	Name           string `json:"name"`
	Occupation     string `json:"occupation"`
	CurrentVersion int    `json:"currentVersion"`
	UpdatedAt      string `json:"updatedAt"`
}

type Version struct {
	Version        int             `json:"version"`
	ParentVersion  *int            `json:"parentVersion"`
	ActorAccountID string          `json:"actorAccountId"`
	ActorName      string          `json:"actorName"`
	ChangeKind     string          `json:"changeKind"`
	Message        *string         `json:"message"`
	ChangedPaths   json.RawMessage `json:"changedPaths"`
	Snapshot       json.RawMessage `json:"snapshot,omitempty"`
	CreatedAt      string          `json:"createdAt"`
}
type Change struct {
	Path   string `json:"path"`
	Before any    `json:"before"`
	After  any    `json:"after"`
}
type VersionDetail struct {
	FromVersion *int            `json:"fromVersion"`
	ToVersion   int             `json:"toVersion"`
	Snapshot    json.RawMessage `json:"snapshot"`
	Changes     []Change        `json:"changes"`
}

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

func DefaultSheet(name string) json.RawMessage {
	sheet := coc7.NewSheet(name)
	encoded, _ := json.Marshal(sheet)
	return encoded
}

func (s *Store) Create(ctx context.Context, ownerID, actorID, name, kind string) (Character, error) {
	name = strings.TrimSpace(name)
	if name == "" || (kind != "investigator" && kind != "npc") {
		return Character{}, fmt.Errorf("invalid character")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	item := Character{ID: randomID(), OwnerAccountID: ownerID, Kind: kind, Ruleset: "coc7", Status: "draft", Name: name, CurrentVersion: 1, Sheet: DefaultSheet(name), CreatedAt: now, UpdatedAt: now}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Character{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO characters(id, owner_account_id, kind, ruleset, status, name, current_version, sheet_data, created_at, updated_at) VALUES (?, ?, ?, 'coc7', 'draft', ?, 1, ?, ?, ?)`, item.ID, ownerID, kind, name, string(item.Sheet), now, now); err != nil {
		return Character{}, err
	}
	paths, _ := json.Marshal([]string{"/"})
	if _, err := tx.ExecContext(ctx, `INSERT INTO character_versions(id, character_id, version, actor_account_id, change_kind, changed_paths, snapshot_data, created_at) VALUES (?, ?, 1, ?, 'system', ?, ?, ?)`, randomID(), item.ID, actorID, string(paths), string(item.Sheet), now); err != nil {
		return Character{}, err
	}
	if err := tx.Commit(); err != nil {
		return Character{}, err
	}
	return item, nil
}

func (s *Store) CopyOwned(ctx context.Context, id, ownerID, actorID, name string) (Character, error) {
	source, err := s.GetOwned(ctx, id, ownerID)
	if err != nil {
		return Character{}, err
	}
	var sheet coc7.Sheet
	if err := json.Unmarshal(source.Sheet, &sheet); err != nil {
		return Character{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = source.Name + "（副本）"
	}
	if len(name) > 120 {
		return Character{}, fmt.Errorf("invalid character name")
	}
	sheet.Profile.Name = name
	encoded, err := json.Marshal(sheet)
	if err != nil {
		return Character{}, err
	}
	normalized, err := coc7.Normalize(encoded)
	if err != nil {
		return Character{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	item := Character{ID: randomID(), OwnerAccountID: ownerID, Kind: source.Kind, Ruleset: "coc7", Status: "draft", Name: name, CurrentVersion: 1, Sheet: normalized, CreatedAt: now, UpdatedAt: now, CanEdit: true}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Character{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO characters(id, owner_account_id, kind, ruleset, status, name, current_version, sheet_data, created_at, updated_at) VALUES (?, ?, ?, 'coc7', 'draft', ?, 1, ?, ?, ?)`, item.ID, ownerID, item.Kind, name, string(normalized), now, now); err != nil {
		return Character{}, err
	}
	paths, _ := json.Marshal([]string{"/"})
	message := "复制自人物卡：" + source.Name
	if _, err := tx.ExecContext(ctx, `INSERT INTO character_versions(id, character_id, version, actor_account_id, change_kind, message, changed_paths, snapshot_data, created_at) VALUES (?, ?, 1, ?, 'import', ?, ?, ?, ?)`, randomID(), item.ID, actorID, message, string(paths), string(normalized), now); err != nil {
		return Character{}, err
	}
	if err := tx.Commit(); err != nil {
		return Character{}, err
	}
	return item, nil
}

func (s *Store) SetStatusOwned(ctx context.Context, id, ownerID, actorID, status string) (Character, error) {
	if !validCharacterStatus(status) {
		return Character{}, fmt.Errorf("invalid status")
	}
	current, err := s.GetOwned(ctx, id, ownerID)
	if err != nil {
		return Character{}, err
	}
	if current.Status == status {
		return current, nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	nextVersion := current.CurrentVersion + 1
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Character{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE characters SET status = ?, current_version = ?, updated_at = ? WHERE id = ? AND owner_account_id = ? AND current_version = ?`, status, nextVersion, now, id, ownerID, current.CurrentVersion)
	if err != nil {
		return Character{}, err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return Character{}, ErrConflict
	}
	paths, _ := json.Marshal([]string{"/@status"})
	message := "人物状态变更为" + status
	if _, err := tx.ExecContext(ctx, `INSERT INTO character_versions(id, character_id, version, parent_version, actor_account_id, change_kind, message, changed_paths, snapshot_data, created_at) VALUES (?, ?, ?, ?, ?, 'edit', ?, ?, ?, ?)`, randomID(), id, nextVersion, current.CurrentVersion, actorID, message, string(paths), string(current.Sheet), now); err != nil {
		return Character{}, err
	}
	if err := tx.Commit(); err != nil {
		return Character{}, err
	}
	return s.GetOwned(ctx, id, ownerID)
}

func validCharacterStatus(value string) bool {
	return value == "draft" || value == "active" || value == "retired" || value == "deceased" || value == "archived"
}

func (s *Store) ListOwned(ctx context.Context, ownerID string) ([]Summary, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, kind, status, name, COALESCE(json_extract(sheet_data, '$.profile.occupation'), ''), current_version, updated_at FROM characters WHERE owner_account_id = ? AND archived_at IS NULL ORDER BY updated_at DESC`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Summary{}
	for rows.Next() {
		var item Summary
		if err := rows.Scan(&item.ID, &item.Kind, &item.Status, &item.Name, &item.Occupation, &item.CurrentVersion, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetOwned(ctx context.Context, id, ownerID string) (Character, error) {
	var item Character
	var sheet string
	err := s.db.QueryRowContext(ctx, `SELECT id, owner_account_id, kind, ruleset, status, name, current_version, sheet_data, created_at, updated_at FROM characters WHERE id = ? AND owner_account_id = ? AND archived_at IS NULL`, id, ownerID).Scan(&item.ID, &item.OwnerAccountID, &item.Kind, &item.Ruleset, &item.Status, &item.Name, &item.CurrentVersion, &sheet, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Character{}, ErrNotFound
	}
	item.Sheet = json.RawMessage(sheet)
	if err == nil {
		item.Sheet, err = coc7.Normalize(item.Sheet)
	}
	item.CanEdit = err == nil
	return item, err
}

func (s *Store) ArchiveOwned(ctx context.Context, id, ownerID string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE characters SET archived_at = ?, updated_at = ? WHERE id = ? AND owner_account_id = ? AND archived_at IS NULL`, now, now, id, ownerID)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `UPDATE campaign_characters SET left_at = ? WHERE character_id = ? AND left_at IS NULL`, now, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) GetVisible(ctx context.Context, id, viewerID string) (Character, error) {
	var item Character
	var sheet string
	err := s.db.QueryRowContext(ctx, `SELECT c.id, c.owner_account_id, c.kind, c.ruleset, c.status, c.name, c.current_version, c.sheet_data, c.created_at, c.updated_at,
		(c.owner_account_id = ? OR EXISTS (SELECT 1 FROM campaign_characters cc JOIN campaigns cp ON cp.id = cc.campaign_id WHERE cc.character_id = c.id AND cc.left_at IS NULL AND cp.keeper_account_id = ?))
		FROM characters c WHERE c.id = ? AND c.archived_at IS NULL AND (c.owner_account_id = ? OR EXISTS (SELECT 1 FROM campaign_characters cc JOIN campaigns cp ON cp.id = cc.campaign_id WHERE cc.character_id = c.id AND cc.left_at IS NULL AND (cc.visibility = 'public' OR cp.keeper_account_id = ?)))`, viewerID, viewerID, id, viewerID, viewerID).Scan(&item.ID, &item.OwnerAccountID, &item.Kind, &item.Ruleset, &item.Status, &item.Name, &item.CurrentVersion, &sheet, &item.CreatedAt, &item.UpdatedAt, &item.CanEdit)
	if errors.Is(err, sql.ErrNoRows) {
		return Character{}, ErrNotFound
	}
	item.Sheet = json.RawMessage(sheet)
	if err == nil {
		item.Sheet, err = coc7.Normalize(item.Sheet)
	}
	return item, err
}

func (s *Store) GetEditable(ctx context.Context, id, editorID string) (Character, error) {
	item, err := s.GetVisible(ctx, id, editorID)
	if err != nil || !item.CanEdit {
		return Character{}, ErrNotFound
	}
	return item, nil
}

func (s *Store) Update(ctx context.Context, id, ownerID, actorID string, baseVersion int, sheet json.RawMessage, message string) (Character, error) {
	return s.update(ctx, id, ownerID, actorID, baseVersion, sheet, message, "edit")
}

func (s *Store) update(ctx context.Context, id, ownerID, actorID string, baseVersion int, sheet json.RawMessage, message, changeKind string) (Character, error) {
	normalized, err := coc7.Normalize(sheet)
	if err != nil {
		return Character{}, err
	}
	sheet = normalized
	current, err := s.GetOwned(ctx, id, ownerID)
	if err != nil {
		return Character{}, err
	}
	if current.CurrentVersion != baseVersion {
		return Character{}, ErrConflict
	}
	var profile struct {
		Profile struct {
			Name string `json:"name"`
		} `json:"profile"`
	}
	if err := json.Unmarshal(sheet, &profile); err != nil || strings.TrimSpace(profile.Profile.Name) == "" {
		return Character{}, fmt.Errorf("invalid profile")
	}
	paths := changedPaths(current.Sheet, sheet)
	if len(paths) == 0 {
		return current, nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	nextVersion := baseVersion + 1
	pathsJSON, _ := json.Marshal(paths)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Character{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE characters SET name = ?, sheet_data = ?, current_version = ?, updated_at = ? WHERE id = ? AND owner_account_id = ? AND current_version = ?`, strings.TrimSpace(profile.Profile.Name), string(sheet), nextVersion, now, id, ownerID, baseVersion)
	if err != nil {
		return Character{}, err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return Character{}, ErrConflict
	}
	var messageValue any
	if strings.TrimSpace(message) != "" {
		messageValue = strings.TrimSpace(message)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO character_versions(id, character_id, version, parent_version, actor_account_id, change_kind, message, changed_paths, snapshot_data, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, randomID(), id, nextVersion, baseVersion, actorID, changeKind, messageValue, string(pathsJSON), string(sheet), now); err != nil {
		return Character{}, err
	}
	if err := tx.Commit(); err != nil {
		return Character{}, err
	}
	return s.GetOwned(ctx, id, ownerID)
}

func (s *Store) Versions(ctx context.Context, id, ownerID string) ([]Version, error) {
	if _, err := s.GetOwned(ctx, id, ownerID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT v.version, v.parent_version, v.actor_account_id, a.display_name, v.change_kind, v.message, v.changed_paths, v.created_at FROM character_versions v JOIN accounts a ON a.id = v.actor_account_id WHERE v.character_id = ? ORDER BY v.version DESC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Version{}
	for rows.Next() {
		var item Version
		var changedPaths string
		if err := rows.Scan(&item.Version, &item.ParentVersion, &item.ActorAccountID, &item.ActorName, &item.ChangeKind, &item.Message, &changedPaths, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.ChangedPaths = json.RawMessage(changedPaths)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) VersionDetail(ctx context.Context, id, ownerID string, fromVersion *int, toVersion int) (VersionDetail, error) {
	if _, err := s.GetOwned(ctx, id, ownerID); err != nil {
		return VersionDetail{}, err
	}
	toSnapshot, err := s.versionSnapshot(ctx, id, toVersion)
	if err != nil {
		return VersionDetail{}, err
	}
	var before any = map[string]any{}
	if fromVersion != nil {
		fromSnapshot, err := s.versionSnapshot(ctx, id, *fromVersion)
		if err != nil {
			return VersionDetail{}, err
		}
		if err := json.Unmarshal(fromSnapshot, &before); err != nil {
			return VersionDetail{}, err
		}
	}
	var after any
	if err := json.Unmarshal(toSnapshot, &after); err != nil {
		return VersionDetail{}, err
	}
	changes := []Change{}
	diffValues("", before, after, &changes)
	return VersionDetail{FromVersion: fromVersion, ToVersion: toVersion, Snapshot: toSnapshot, Changes: changes}, nil
}

func (s *Store) versionSnapshot(ctx context.Context, id string, version int) (json.RawMessage, error) {
	var snapshot string
	err := s.db.QueryRowContext(ctx, `SELECT snapshot_data FROM character_versions WHERE character_id = ? AND version = ?`, id, version).Scan(&snapshot)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return json.RawMessage(snapshot), err
}

func diffValues(path string, before, after any, changes *[]Change) {
	left, leftOK := before.(map[string]any)
	right, rightOK := after.(map[string]any)
	if leftOK && rightOK {
		keys := map[string]bool{}
		for key := range left {
			keys[key] = true
		}
		for key := range right {
			keys[key] = true
		}
		names := make([]string, 0, len(keys))
		for key := range keys {
			names = append(names, key)
		}
		sort.Strings(names)
		for _, key := range names {
			diffValues(path+"/"+escapePointer(key), left[key], right[key], changes)
		}
		return
	}
	leftList, leftListOK := before.([]any)
	rightList, rightListOK := after.([]any)
	if leftListOK && rightListOK {
		length := max(len(leftList), len(rightList))
		for index := 0; index < length; index++ {
			var leftValue, rightValue any
			if index < len(leftList) {
				leftValue = leftList[index]
			}
			if index < len(rightList) {
				rightValue = rightList[index]
			}
			diffValues(fmt.Sprintf("%s/%d", path, index), leftValue, rightValue, changes)
		}
		return
	}
	leftJSON, _ := json.Marshal(before)
	rightJSON, _ := json.Marshal(after)
	if string(leftJSON) != string(rightJSON) {
		*changes = append(*changes, Change{Path: path, Before: before, After: after})
	}
}

func escapePointer(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "~", "~0"), "/", "~1")
}

func (s *Store) Restore(ctx context.Context, id, ownerID, actorID string, version, baseVersion int, message string) (Character, error) {
	current, err := s.GetOwned(ctx, id, ownerID)
	if err != nil {
		return Character{}, err
	}
	if current.CurrentVersion != baseVersion {
		return Character{}, ErrConflict
	}
	var snapshotData string
	if err := s.db.QueryRowContext(ctx, `SELECT snapshot_data FROM character_versions WHERE character_id = ? AND version = ?`, id, version).Scan(&snapshotData); errors.Is(err, sql.ErrNoRows) {
		return Character{}, ErrNotFound
	} else if err != nil {
		return Character{}, err
	}
	return s.update(ctx, id, ownerID, actorID, baseVersion, json.RawMessage(snapshotData), message, "restore")
}

func (s *Store) GenerateAttributes(ctx context.Context, id, ownerID, actorID string, baseVersion int) (Character, error) {
	current, err := s.GetOwned(ctx, id, ownerID)
	if err != nil {
		return Character{}, err
	}
	if current.CurrentVersion != baseVersion {
		return Character{}, ErrConflict
	}
	generated, err := coc7.Generate(current.Sheet)
	if err != nil {
		return Character{}, err
	}
	return s.update(ctx, id, ownerID, actorID, baseVersion, generated, "随机生成人物属性", "generation")
}

func (s *Store) ApplyOccupation(ctx context.Context, id, ownerID, actorID string, baseVersion int, occupation coc7.Occupation, formulaIndex int) (Character, error) {
	current, err := s.GetOwned(ctx, id, ownerID)
	if err != nil {
		return Character{}, err
	}
	if current.CurrentVersion != baseVersion {
		return Character{}, ErrConflict
	}
	updated, err := coc7.ApplyOccupation(current.Sheet, occupation, formulaIndex)
	if err != nil {
		return Character{}, err
	}
	return s.update(ctx, id, ownerID, actorID, baseVersion, updated, "选择职业："+occupation.Name, "edit")
}

func (s *Store) AllocateSkills(ctx context.Context, id, ownerID, actorID string, baseVersion int, allocation coc7.SkillAllocation) (Character, error) {
	current, err := s.GetOwned(ctx, id, ownerID)
	if err != nil {
		return Character{}, err
	}
	if current.CurrentVersion != baseVersion {
		return Character{}, ErrConflict
	}
	updated, err := coc7.AllocateSkills(current.Sheet, allocation)
	if err != nil {
		return Character{}, err
	}
	return s.update(ctx, id, ownerID, actorID, baseVersion, updated, "分配职业与兴趣技能点", "edit")
}

func (s *Store) GrowSkills(ctx context.Context, id, ownerID, actorID string, baseVersion int, skills []string) (Character, coc7.SkillGrowthResult, error) {
	current, err := s.GetOwned(ctx, id, ownerID)
	if err != nil {
		return Character{}, coc7.SkillGrowthResult{}, err
	}
	if current.CurrentVersion != baseVersion {
		return Character{}, coc7.SkillGrowthResult{}, ErrConflict
	}
	updated, result, err := coc7.GrowSkills(current.Sheet, skills)
	if err != nil {
		return Character{}, coc7.SkillGrowthResult{}, err
	}
	item, err := s.update(ctx, id, ownerID, actorID, baseVersion, updated, "进行幕间技能成长", "edit")
	return item, result, err
}

func (s *Store) ApplyAgeAdjustment(ctx context.Context, id, ownerID, actorID string, baseVersion int, reductions map[string]int) (Character, coc7.AgeAdjustmentResult, error) {
	current, err := s.GetOwned(ctx, id, ownerID)
	if err != nil {
		return Character{}, coc7.AgeAdjustmentResult{}, err
	}
	if current.CurrentVersion != baseVersion {
		return Character{}, coc7.AgeAdjustmentResult{}, ErrConflict
	}
	updated, result, err := coc7.ApplyAgeAdjustment(current.Sheet, reductions)
	if err != nil {
		return Character{}, coc7.AgeAdjustmentResult{}, err
	}
	item, err := s.update(ctx, id, ownerID, actorID, baseVersion, updated, "应用年龄修正", "generation")
	return item, result, err
}

func changedPaths(before, after json.RawMessage) []string {
	var left, right any
	_ = json.Unmarshal(before, &left)
	_ = json.Unmarshal(after, &right)
	paths := []string{}
	walkDiff("", left, right, &paths)
	sort.Strings(paths)
	return paths
}
func walkDiff(path string, left, right any, paths *[]string) {
	lm, lok := left.(map[string]any)
	rm, rok := right.(map[string]any)
	if lok && rok {
		keys := map[string]bool{}
		for key := range lm {
			keys[key] = true
		}
		for key := range rm {
			keys[key] = true
		}
		for key := range keys {
			walkDiff(path+"/"+key, lm[key], rm[key], paths)
		}
		return
	}
	lb, _ := json.Marshal(left)
	rb, _ := json.Marshal(right)
	if string(lb) != string(rb) {
		if path == "" {
			path = "/"
		}
		*paths = append(*paths, path)
	}
}
func randomID() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		panic(err)
	}
	return hex.EncodeToString(value)
}
