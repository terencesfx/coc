package coc7

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type OccupationCatalogFile struct {
	SchemaVersion int          `json:"schemaVersion"`
	Occupations   []Occupation `json:"occupations"`
}

type Occupation struct {
	ID                 string              `json:"id"`
	Name               string              `json:"name"`
	Eras               []string            `json:"eras"`
	CreditRating       CreditRating        `json:"creditRating"`
	SkillPointFormulas []SkillPointFormula `json:"skillPointFormulas"`
	FixedSkills        []string            `json:"fixedSkills"`
	ChoiceGroups       []SkillChoiceGroup  `json:"choiceGroups"`
	FreeChoiceCount    int                 `json:"freeChoiceCount"`
	Description        string              `json:"description,omitempty"`
	Source             string              `json:"source,omitempty"`
}

type CreditRating struct {
	Min int `json:"min"`
	Max int `json:"max"`
}
type SkillPointFormula struct {
	Label string        `json:"label"`
	Terms []FormulaTerm `json:"terms"`
}
type FormulaTerm struct {
	Attribute  string `json:"attribute"`
	Multiplier int    `json:"multiplier"`
}
type SkillChoiceGroup struct {
	Count    int      `json:"count"`
	Skills   []string `json:"skills,omitempty"`
	Category string   `json:"category,omitempty"`
}

type OccupationCatalog struct {
	items []Occupation
	byID  map[string]Occupation
}

func LoadOccupationCatalog(officialPath, customPath string) (*OccupationCatalog, error) {
	if err := ensureCustomOccupationFile(customPath); err != nil {
		return nil, err
	}
	official, err := readOccupationFile(officialPath, "official")
	if err != nil {
		return nil, err
	}
	custom, err := readOccupationFile(customPath, "custom")
	if err != nil {
		return nil, err
	}
	catalog := &OccupationCatalog{byID: map[string]Occupation{}}
	for _, item := range append(official, custom...) {
		if _, exists := catalog.byID[item.ID]; exists {
			return nil, fmt.Errorf("duplicate occupation id %q", item.ID)
		}
		catalog.byID[item.ID] = item
		catalog.items = append(catalog.items, item)
	}
	sort.Slice(catalog.items, func(i, j int) bool { return catalog.items[i].Name < catalog.items[j].Name })
	return catalog, nil
}

func (c *OccupationCatalog) List() []Occupation {
	result := make([]Occupation, len(c.items))
	copy(result, c.items)
	return result
}

func (c *OccupationCatalog) Find(id string) (Occupation, bool) {
	item, ok := c.byID[id]
	return item, ok
}

func (o Occupation) SkillPoints(formulaIndex int, attributes map[string]int) (int, error) {
	if formulaIndex < 0 || formulaIndex >= len(o.SkillPointFormulas) {
		return 0, fmt.Errorf("invalid formula index")
	}
	total := 0
	for _, term := range o.SkillPointFormulas[formulaIndex].Terms {
		value, ok := attributes[term.Attribute]
		if !ok {
			return 0, fmt.Errorf("missing attribute %s", term.Attribute)
		}
		total += value * term.Multiplier
	}
	return total, nil
}

func readOccupationFile(path, namespace string) ([]Occupation, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s occupations: %w", namespace, err)
	}
	var file OccupationCatalogFile
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil {
		return nil, fmt.Errorf("decode %s occupations: %w", namespace, err)
	}
	if file.SchemaVersion != 1 {
		return nil, fmt.Errorf("unsupported %s occupation schema version %d", namespace, file.SchemaVersion)
	}
	for index := range file.Occupations {
		file.Occupations[index].Source = namespace
		if err := validateOccupation(file.Occupations[index], namespace); err != nil {
			return nil, fmt.Errorf("%s occupation %d: %w", namespace, index, err)
		}
	}
	return file.Occupations, nil
}

func validateOccupation(item Occupation, namespace string) error {
	if !strings.HasPrefix(item.ID, namespace+".") {
		return fmt.Errorf("id %q must start with %s.", item.ID, namespace)
	}
	if strings.TrimSpace(item.Name) == "" || len(item.Eras) == 0 {
		return fmt.Errorf("name and eras are required")
	}
	if item.CreditRating.Min < 0 || item.CreditRating.Max > 99 || item.CreditRating.Min > item.CreditRating.Max {
		return fmt.Errorf("invalid credit rating range")
	}
	if len(item.SkillPointFormulas) == 0 {
		return fmt.Errorf("at least one skill point formula is required")
	}
	validAttributes := map[string]bool{"str": true, "con": true, "siz": true, "dex": true, "app": true, "int": true, "pow": true, "edu": true}
	for _, formula := range item.SkillPointFormulas {
		if len(formula.Terms) == 0 {
			return fmt.Errorf("formula terms are required")
		}
		for _, term := range formula.Terms {
			if !validAttributes[term.Attribute] || term.Multiplier < 1 || term.Multiplier > 10 {
				return fmt.Errorf("invalid formula term")
			}
		}
	}
	if item.FreeChoiceCount < 0 || item.FreeChoiceCount > 20 {
		return fmt.Errorf("invalid free choice count")
	}
	for _, group := range item.ChoiceGroups {
		if group.Count < 1 || (len(group.Skills) == 0 && strings.TrimSpace(group.Category) == "") {
			return fmt.Errorf("invalid skill choice group")
		}
	}
	return nil
}

func ensureCustomOccupationFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create custom occupation directory: %w", err)
	}
	data := []byte("{\n  \"schemaVersion\": 1,\n  \"occupations\": []\n}\n")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("create custom occupation file: %w", err)
	}
	return nil
}
