package coc7

import (
	"encoding/json"
	"testing"
)

func TestGenerateUsesSeventhEditionRanges(t *testing.T) {
	base, _ := json.Marshal(NewSheet("调查员"))
	for range 100 {
		generated, err := Generate(base)
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		var sheet Sheet
		if err := json.Unmarshal(generated, &sheet); err != nil {
			t.Fatalf("decode: %v", err)
		}
		for _, key := range []string{"str", "con", "dex", "app", "pow"} {
			if value := sheet.Attributes[key]; value < 15 || value > 90 || value%5 != 0 {
				t.Fatalf("%s out of range: %d", key, value)
			}
		}
		for _, key := range []string{"siz", "int", "edu"} {
			if value := sheet.Attributes[key]; value < 40 || value > 90 || value%5 != 0 {
				t.Fatalf("%s out of range: %d", key, value)
			}
		}
		if sheet.Status.HP.Max != (sheet.Attributes["con"]+sheet.Attributes["siz"])/10 {
			t.Fatal("incorrect HP")
		}
		if sheet.Status.MP.Max != sheet.Attributes["pow"]/5 {
			t.Fatal("incorrect MP")
		}
		if sheet.Skills["闪避"] != sheet.Attributes["dex"]/2 {
			t.Fatal("incorrect dodge base")
		}
	}
}

func TestNormalizeRejectsIncompleteSheet(t *testing.T) {
	if _, err := Normalize(json.RawMessage(`{"schemaVersion":1}`)); err == nil {
		t.Fatal("expected incomplete sheet to be rejected")
	}
}

func TestNormalizeSupplementalCharacterData(t *testing.T) {
	sheet := NewSheet("调查员")
	sheet.Possessions = []Possession{{ID: "item-1", Name: "煤油灯", Quantity: 1}}
	sheet.Weapons = []Weapon{{ID: "weapon-1", Name: "左轮手枪", Skill: "射击（手枪）", Damage: "1d10", Attacks: 1, Ammo: 6, Malfunction: 100}}
	sheet.Status.MajorWound = true
	sheet.Finances.Cash = "20 美元"
	encoded, _ := json.Marshal(sheet)
	normalized, err := Normalize(encoded)
	if err != nil {
		t.Fatal(err)
	}
	var result Sheet
	if err := json.Unmarshal(normalized, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Weapons) != 1 || result.Weapons[0].Damage != "1d10" || !result.Status.MajorWound || result.Finances.Cash == "" {
		t.Fatalf("supplemental data lost: %#v", result)
	}
}

func TestNormalizeCustomSkills(t *testing.T) {
	sheet := NewSheet("调查员")
	sheet.CustomSkills = append(sheet.CustomSkills, "科学（化学）")
	sheet.Skills["科学（化学）"] = 31
	sheet.Creation.BaseSkills["科学（化学）"] = 1
	encoded, _ := json.Marshal(sheet)
	normalized, err := Normalize(encoded)
	if err != nil {
		t.Fatal(err)
	}
	var result Sheet
	if err := json.Unmarshal(normalized, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.CustomSkills) != 1 || result.CustomSkills[0] != "科学（化学）" || result.Skills["科学（化学）"] != 31 {
		t.Fatalf("custom skill lost: %#v", result.CustomSkills)
	}

	delete(sheet.Skills, "科学（化学）")
	broken, _ := json.Marshal(sheet)
	if _, err := Normalize(broken); err == nil {
		t.Fatal("expected missing custom skill to be rejected")
	}
}

func TestAllocateOccupationAndInterestSkills(t *testing.T) {
	base, _ := json.Marshal(NewSheet("调查员"))
	occupation := Occupation{
		ID: "official.test", Name: "测试职业", Eras: []string{"1920s"},
		CreditRating:       CreditRating{Min: 10, Max: 40},
		SkillPointFormulas: []SkillPointFormula{{Label: "EDU × 4", Terms: []FormulaTerm{{Attribute: "edu", Multiplier: 4}}}},
		FixedSkills:        []string{"图书馆使用"},
		ChoiceGroups:       []SkillChoiceGroup{{Count: 1, Skills: []string{"侦查", "聆听"}}},
		FreeChoiceCount:    1,
	}
	withOccupation, err := ApplyOccupation(base, occupation, 0)
	if err != nil {
		t.Fatalf("apply occupation: %v", err)
	}
	allocated, err := AllocateSkills(withOccupation, SkillAllocation{
		Occupation:       map[string]int{"信用评级": 20, "图书馆使用": 30, "侦查": 25, "急救": 20},
		Interest:         map[string]int{"心理学": 30},
		ChoiceSelections: [][]string{{"侦查"}}, FreeSkills: []string{"急救"},
	})
	if err != nil {
		t.Fatalf("allocate skills: %v", err)
	}
	var sheet Sheet
	if err := json.Unmarshal(allocated, &sheet); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if sheet.Skills["信用评级"] != 20 || sheet.Skills["图书馆使用"] != 50 || sheet.Skills["心理学"] != 40 {
		t.Fatalf("unexpected skills: %+v", sheet.Skills)
	}
	if _, err := AllocateSkills(withOccupation, SkillAllocation{Occupation: map[string]int{"侦查": 999}, ChoiceSelections: [][]string{{"侦查"}}, FreeSkills: []string{"急救"}}); err == nil {
		t.Fatal("expected excessive allocation to fail")
	}
}

func TestGrowSkills(t *testing.T) {
	sheet := NewSheet("调查员")
	sheet.Skills["侦查"] = 60
	sheet.Skills["聆听"] = 70
	encoded, _ := json.Marshal(sheet)
	rolls := []int{61, 7, 40}
	index := 0
	updated, result, err := growSkills(encoded, []string{"侦查", "聆听"}, func(int) int {
		value := rolls[index]
		index++
		return value
	})
	if err != nil {
		t.Fatal(err)
	}
	var grown Sheet
	if err := json.Unmarshal(updated, &grown); err != nil {
		t.Fatal(err)
	}
	if grown.Skills["侦查"] != 67 || grown.Skills["聆听"] != 70 || !result.Items[0].Succeeded || result.Items[1].Succeeded {
		t.Fatalf("unexpected growth: %+v, %+v", grown.Skills, result)
	}
	if _, _, err := growSkills(encoded, []string{"克苏鲁神话"}, func(int) int { return 100 }); err == nil {
		t.Fatal("expected mythos growth to be rejected")
	}
}

func TestApplyAgeAdjustment(t *testing.T) {
	sheet := NewSheet("调查员")
	sheet.Profile.Age = 45
	encoded, _ := json.Marshal(sheet)
	rolls := []int{90, 4, 20}
	index := 0
	updated, result, err := applyAgeAdjustment(encoded, map[string]int{"str": 2, "con": 1, "dex": 2}, func(int) int {
		value := rolls[index]
		index++
		return value
	})
	if err != nil {
		t.Fatal(err)
	}
	var adjusted Sheet
	if err := json.Unmarshal(updated, &adjusted); err != nil {
		t.Fatal(err)
	}
	if adjusted.Attributes["str"] != 48 || adjusted.Attributes["con"] != 49 || adjusted.Attributes["dex"] != 48 || adjusted.Attributes["app"] != 45 {
		t.Fatalf("incorrect physical adjustment: %+v", adjusted.Attributes)
	}
	if adjusted.Attributes["edu"] != 54 || len(result.EducationChecks) != 2 || result.EducationChecks[0].Increase != 4 {
		t.Fatalf("incorrect education adjustment: %+v", result)
	}
	if len(adjusted.Skills) < 10 || len(adjusted.Creation.BaseSkills) != len(adjusted.Skills) {
		t.Fatalf("age adjustment lost base skills: %d/%d", len(adjusted.Creation.BaseSkills), len(adjusted.Skills))
	}
	if _, _, err := applyAgeAdjustment(updated, map[string]int{"str": 5}, func(int) int { return 1 }); err == nil {
		t.Fatal("expected repeated age adjustment to fail")
	}
}

func TestYoungInvestigatorAgeAdjustment(t *testing.T) {
	sheet := NewSheet("年轻调查员")
	sheet.Profile.Age = 18
	sheet.Status.Luck = 40
	encoded, _ := json.Marshal(sheet)
	rolls := []int{4, 5, 6}
	index := 0
	updated, result, err := applyAgeAdjustment(encoded, map[string]int{"str": 3, "siz": 2}, func(int) int {
		value := rolls[index]
		index++
		return value
	})
	if err != nil {
		t.Fatal(err)
	}
	var adjusted Sheet
	_ = json.Unmarshal(updated, &adjusted)
	if adjusted.Attributes["edu"] != 45 || adjusted.Status.Luck != 75 || result.LuckReroll != 75 {
		t.Fatalf("incorrect young adjustment: %+v", result)
	}
}
