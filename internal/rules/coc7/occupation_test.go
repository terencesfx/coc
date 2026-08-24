package coc7

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestOccupationCatalogMergesNamespaces(t *testing.T) {
	dir := t.TempDir()
	officialPath := filepath.Join(dir, "official.json")
	customPath := filepath.Join(dir, "custom.json")
	writeCatalog(t, officialPath, `{"schemaVersion":1,"occupations":[{"id":"official.test","name":"官方职业","eras":["1920s"],"creditRating":{"min":10,"max":50},"skillPointFormulas":[{"label":"EDU × 4","terms":[{"attribute":"edu","multiplier":4}]}],"fixedSkills":[],"choiceGroups":[],"freeChoiceCount":0}]}`)
	writeCatalog(t, customPath, `{"schemaVersion":1,"occupations":[{"id":"custom.test","name":"自定义职业","eras":["modern"],"creditRating":{"min":0,"max":99},"skillPointFormulas":[{"label":"EDU × 2 + INT × 2","terms":[{"attribute":"edu","multiplier":2},{"attribute":"int","multiplier":2}]}],"fixedSkills":[],"choiceGroups":[],"freeChoiceCount":4}]}`)

	catalog, err := LoadOccupationCatalog(officialPath, customPath)
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	if len(catalog.List()) != 2 {
		t.Fatalf("expected two occupations, got %d", len(catalog.List()))
	}
	occupation, ok := catalog.Find("custom.test")
	if !ok {
		t.Fatal("expected custom occupation")
	}
	points, err := occupation.SkillPoints(0, map[string]int{"edu": 60, "int": 70})
	if err != nil || points != 260 {
		t.Fatalf("expected 260 points, got %d error=%v", points, err)
	}
	sheetData, _ := json.Marshal(NewSheet("调查员"))
	applied, err := ApplyOccupation(sheetData, occupation, 0)
	if err != nil {
		t.Fatalf("apply occupation: %v", err)
	}
	var sheet Sheet
	if err := json.Unmarshal(applied, &sheet); err != nil {
		t.Fatalf("decode applied sheet: %v", err)
	}
	if sheet.Profile.OccupationID != "custom.test" || sheet.Creation.OccupationPoints != 200 {
		t.Fatalf("unexpected occupation snapshot: %+v", sheet.Creation)
	}
}

func TestOccupationCatalogCreatesCustomFile(t *testing.T) {
	dir := t.TempDir()
	officialPath := filepath.Join(dir, "official.json")
	customPath := filepath.Join(dir, "nested", "custom.json")
	writeCatalog(t, officialPath, `{"schemaVersion":1,"occupations":[]}`)
	if _, err := LoadOccupationCatalog(officialPath, customPath); err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	if info, err := os.Stat(customPath); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("expected private custom file, info=%v error=%v", info, err)
	}
}

func TestCustomCannotUseOfficialNamespace(t *testing.T) {
	dir := t.TempDir()
	officialPath := filepath.Join(dir, "official.json")
	customPath := filepath.Join(dir, "custom.json")
	writeCatalog(t, officialPath, `{"schemaVersion":1,"occupations":[]}`)
	writeCatalog(t, customPath, `{"schemaVersion":1,"occupations":[{"id":"official.bad","name":"错误","eras":["modern"],"creditRating":{"min":0,"max":1},"skillPointFormulas":[{"label":"EDU","terms":[{"attribute":"edu","multiplier":1}]}],"fixedSkills":[],"choiceGroups":[],"freeChoiceCount":0}]}`)
	if _, err := LoadOccupationCatalog(officialPath, customPath); err == nil {
		t.Fatal("expected namespace validation error")
	}
}

func writeCatalog(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write catalog: %v", err)
	}
}
