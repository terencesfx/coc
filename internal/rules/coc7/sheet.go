package coc7

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
)

type Sheet struct {
	SchemaVersion        int               `json:"schemaVersion"`
	Profile              Profile           `json:"profile"`
	Attributes           map[string]int    `json:"attributes"`
	Status               Status            `json:"status"`
	Derived              Derived           `json:"derived"`
	Skills               map[string]int    `json:"skills"`
	SkillSpecializations map[string]string `json:"skillSpecializations"`
	CustomSkills         []string          `json:"customSkills"`
	Backstory            map[string]string `json:"backstory"`
	Possessions          []Possession      `json:"possessions"`
	Weapons              []Weapon          `json:"weapons"`
	Finances             Finances          `json:"finances"`
	Notes                string            `json:"notes"`
	Creation             Creation          `json:"creation"`
}

type Profile struct {
	Name         string `json:"name"`
	Occupation   string `json:"occupation"`
	OccupationID string `json:"occupationId"`
	Age          int    `json:"age"`
	Gender       string `json:"gender"`
	Residence    string `json:"residence"`
	Birthplace   string `json:"birthplace"`
}
type Creation struct {
	OccupationSnapshot    *Occupation          `json:"occupationSnapshot,omitempty"`
	FormulaIndex          int                  `json:"formulaIndex"`
	OccupationPoints      int                  `json:"occupationPoints"`
	InterestPoints        int                  `json:"interestPoints"`
	OccupationAllocations map[string]int       `json:"occupationAllocations"`
	InterestAllocations   map[string]int       `json:"interestAllocations"`
	GrowthAllocations     map[string]int       `json:"growthAllocations"`
	BaseSkills            map[string]int       `json:"baseSkills"`
	ChoiceSelections      [][]string           `json:"choiceSelections"`
	FreeSkills            []string             `json:"freeSkills"`
	AgeAdjustment         *AgeAdjustmentResult `json:"ageAdjustment,omitempty"`
}
type Points struct {
	Current int `json:"current"`
	Max     int `json:"max"`
}
type Status struct {
	HP                 Points `json:"hp"`
	MP                 Points `json:"mp"`
	SAN                Points `json:"san"`
	Luck               int    `json:"luck"`
	MajorWound         bool   `json:"majorWound"`
	Dying              bool   `json:"dying"`
	Unconscious        bool   `json:"unconscious"`
	TemporaryInsanity  bool   `json:"temporaryInsanity"`
	IndefiniteInsanity bool   `json:"indefiniteInsanity"`
	PermanentInsanity  bool   `json:"permanentInsanity"`
}
type Possession struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
	Notes    string `json:"notes"`
}
type Weapon struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Skill           string `json:"skill"`
	Damage          string `json:"damage"`
	Range           string `json:"range"`
	Attacks         int    `json:"attacks"`
	Ammo            int    `json:"ammo"`
	Malfunction     int    `json:"malfunction"`
	AttacksText     string `json:"attacksText,omitempty"`
	AmmoText        string `json:"ammoText,omitempty"`
	MalfunctionText string `json:"malfunctionText,omitempty"`
	Penetration     string `json:"penetration,omitempty"`
	Era             string `json:"era,omitempty"`
	Price           string `json:"price,omitempty"`
	Invention       string `json:"invention,omitempty"`
	Category        string `json:"category,omitempty"`
	Notes           string `json:"notes"`
}
type Finances struct {
	SpendingLevel string `json:"spendingLevel"`
	Cash          string `json:"cash"`
	Assets        string `json:"assets"`
}
type Derived struct {
	Move        int    `json:"move"`
	Build       int    `json:"build"`
	DamageBonus string `json:"damageBonus"`
}

var attributeKeys = []string{"str", "con", "siz", "dex", "app", "int", "pow", "edu"}

func NewSheet(name string) Sheet {
	attributes := map[string]int{"str": 50, "con": 50, "siz": 50, "dex": 50, "app": 50, "int": 50, "pow": 50, "edu": 50}
	sheet := Sheet{
		SchemaVersion: 1, Profile: Profile{Name: name, Age: 20}, Attributes: attributes,
		Status: Status{Luck: 50}, Skills: defaultSkills(attributes),
		CustomSkills: []string{},
		Backstory:    map[string]string{"description": "", "ideology": "", "significantPeople": "", "meaningfulLocations": "", "treasuredPossessions": "", "traits": ""},
		Possessions:  []Possession{}, Weapons: []Weapon{},
		Creation: Creation{OccupationAllocations: map[string]int{}, InterestAllocations: map[string]int{}, GrowthAllocations: map[string]int{}, BaseSkills: map[string]int{}},
	}
	applyDerived(&sheet, true)
	return sheet
}

func Normalize(data json.RawMessage) (json.RawMessage, error) {
	var sheet Sheet
	if err := json.Unmarshal(data, &sheet); err != nil {
		return nil, fmt.Errorf("decode coc7 sheet: %w", err)
	}
	if sheet.Creation.OccupationAllocations == nil {
		sheet.Creation.OccupationAllocations = map[string]int{}
	}
	if sheet.Creation.InterestAllocations == nil {
		sheet.Creation.InterestAllocations = map[string]int{}
	}
	growthAllocationsMissing := sheet.Creation.GrowthAllocations == nil
	if growthAllocationsMissing {
		sheet.Creation.GrowthAllocations = map[string]int{}
	}
	legacySkills := map[string]string{
		"艺术与手艺": "技艺①", "魅惑": "取悦", "斗殴": "格斗",
		"手枪": "射击", "步枪/霰弹枪": "射击②", "外语": "外语①",
		"领航": "导航", "重型操作": "操作重型机械", "科学": "科学①", "侦查": "侦察",
	}
	for oldName, newName := range legacySkills {
		if value, exists := sheet.Skills[oldName]; exists {
			if _, migrated := sheet.Skills[newName]; !migrated {
				sheet.Skills[newName] = value
			}
		}
		for _, allocations := range []map[string]int{sheet.Creation.BaseSkills, sheet.Creation.OccupationAllocations, sheet.Creation.InterestAllocations, sheet.Creation.GrowthAllocations} {
			if value, exists := allocations[oldName]; exists {
				if _, migrated := allocations[newName]; !migrated {
					allocations[newName] = value
				}
			}
		}
		if specialization, exists := sheet.SkillSpecializations[oldName]; exists {
			if _, migrated := sheet.SkillSpecializations[newName]; !migrated {
				sheet.SkillSpecializations[newName] = specialization
			}
		}
	}
	defaults := defaultSkills(sheet.Attributes)
	if sheet.Skills == nil {
		sheet.Skills = map[string]int{}
	}
	for skill, base := range defaults {
		if _, exists := sheet.Skills[skill]; !exists {
			sheet.Skills[skill] = base
		}
	}
	if len(sheet.Creation.BaseSkills) == 0 {
		sheet.Creation.BaseSkills = cloneSkills(sheet.Skills)
	} else {
		for skill, base := range defaults {
			if _, exists := sheet.Creation.BaseSkills[skill]; !exists {
				sheet.Creation.BaseSkills[skill] = base
			}
		}
	}
	if growthAllocationsMissing {
		for skill, total := range sheet.Skills {
			allocated := sheet.Creation.BaseSkills[skill] + sheet.Creation.OccupationAllocations[skill] + sheet.Creation.InterestAllocations[skill]
			if total > allocated {
				sheet.Creation.GrowthAllocations[skill] = total - allocated
			}
		}
	}
	if sheet.Backstory == nil {
		sheet.Backstory = map[string]string{}
	}
	if sheet.Possessions == nil {
		sheet.Possessions = []Possession{}
	}
	if sheet.Weapons == nil {
		sheet.Weapons = []Weapon{}
	}
	if sheet.CustomSkills == nil {
		sheet.CustomSkills = []string{}
	}
	if sheet.SkillSpecializations == nil {
		sheet.SkillSpecializations = map[string]string{}
	}
	if err := Validate(sheet); err != nil {
		return nil, err
	}
	applyDerived(&sheet, false)
	encoded, err := json.Marshal(sheet)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func Validate(sheet Sheet) error {
	if sheet.SchemaVersion != 1 {
		return fmt.Errorf("unsupported sheet schema")
	}
	if strings.TrimSpace(sheet.Profile.Name) == "" || sheet.Profile.Age < 15 || sheet.Profile.Age > 120 {
		return fmt.Errorf("invalid profile")
	}
	for _, key := range attributeKeys {
		value, ok := sheet.Attributes[key]
		if !ok || value < 0 || value > 500 {
			return fmt.Errorf("invalid attribute %s", key)
		}
	}
	if sheet.Status.Luck < 0 || sheet.Status.Luck > 500 {
		return fmt.Errorf("invalid luck")
	}
	if sheet.Skills == nil {
		return fmt.Errorf("skills are required")
	}
	for name, value := range sheet.Skills {
		if strings.TrimSpace(name) == "" || len(name) > 100 || value < 0 || value > 999 {
			return fmt.Errorf("invalid skill")
		}
	}
	for skill, specialization := range sheet.SkillSpecializations {
		if _, exists := sheet.Skills[skill]; !exists || strings.TrimSpace(specialization) == "" || len(specialization) > 100 {
			return fmt.Errorf("invalid skill specialization")
		}
	}
	customSeen := map[string]bool{}
	for _, name := range sheet.CustomSkills {
		if strings.TrimSpace(name) != name || name == "" || customSeen[name] {
			return fmt.Errorf("invalid custom skill")
		}
		if _, ok := sheet.Skills[name]; !ok {
			return fmt.Errorf("custom skill missing")
		}
		customSeen[name] = true
	}
	for _, item := range sheet.Possessions {
		if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.Name) == "" || item.Quantity < 0 || item.Quantity > 9999 || len(item.Notes) > 5000 {
			return fmt.Errorf("invalid possession")
		}
	}
	if len(sheet.Weapons) > 5 {
		return errors.New("武器最多填写 5 件")
	}
	for _, item := range sheet.Weapons {
		if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.Name) == "" || len(item.Damage) > 100 || item.Attacks < 0 || item.Attacks > 100 || item.Ammo < 0 || item.Ammo > 9999 || item.Malfunction < 0 || item.Malfunction > 100 {
			return fmt.Errorf("invalid weapon")
		}
	}
	return nil
}

func ApplyOccupation(data json.RawMessage, occupation Occupation, formulaIndex int) (json.RawMessage, error) {
	var sheet Sheet
	if err := json.Unmarshal(data, &sheet); err != nil {
		return nil, err
	}
	points, err := occupation.SkillPoints(formulaIndex, sheet.Attributes)
	if err != nil {
		return nil, err
	}
	sheet.Profile.Occupation = occupation.Name
	sheet.Profile.OccupationID = occupation.ID
	snapshot := occupation
	sheet.Creation.OccupationSnapshot = &snapshot
	sheet.Creation.FormulaIndex = formulaIndex
	sheet.Creation.OccupationPoints = points
	sheet.Creation.InterestPoints = sheet.Attributes["int"] * 2
	baseSkills := sheet.Creation.BaseSkills
	if len(baseSkills) == 0 {
		baseSkills = cloneSkills(sheet.Skills)
	}
	sheet.Skills = cloneSkills(baseSkills)
	sheet.Creation.OccupationAllocations = map[string]int{}
	sheet.Creation.InterestAllocations = map[string]int{}
	sheet.Creation.BaseSkills = cloneSkills(baseSkills)
	sheet.Creation.ChoiceSelections = make([][]string, len(occupation.ChoiceGroups))
	sheet.Creation.FreeSkills = []string{}
	encoded, err := json.Marshal(sheet)
	if err != nil {
		return nil, err
	}
	return Normalize(encoded)
}

type SkillAllocation struct {
	Occupation       map[string]int `json:"occupation"`
	Interest         map[string]int `json:"interest"`
	ChoiceSelections [][]string     `json:"choiceSelections"`
	FreeSkills       []string       `json:"freeSkills"`
}

type SkillGrowthResult struct {
	Items []SkillGrowthItem `json:"items"`
}

type SkillGrowthItem struct {
	Skill     string `json:"skill"`
	Roll      int    `json:"roll"`
	Succeeded bool   `json:"succeeded"`
	Increase  int    `json:"increase"`
	Before    int    `json:"before"`
	After     int    `json:"after"`
}

type AgeAdjustmentResult struct {
	Age                 int              `json:"age"`
	PhysicalReductions  map[string]int   `json:"physicalReductions"`
	AppearanceReduction int              `json:"appearanceReduction"`
	EducationReduction  int              `json:"educationReduction"`
	LuckReroll          int              `json:"luckReroll"`
	EducationChecks     []EducationCheck `json:"educationChecks"`
}

type EducationCheck struct {
	Roll     int `json:"roll"`
	Increase int `json:"increase"`
}

func ApplyAgeAdjustment(data json.RawMessage, reductions map[string]int) (json.RawMessage, AgeAdjustmentResult, error) {
	return applyAgeAdjustment(data, reductions, func(sides int) int { return rollDice(1, sides) })
}

func applyAgeAdjustment(data json.RawMessage, reductions map[string]int, roll func(int) int) (json.RawMessage, AgeAdjustmentResult, error) {
	var sheet Sheet
	if err := json.Unmarshal(data, &sheet); err != nil {
		return nil, AgeAdjustmentResult{}, err
	}
	if sheet.Creation.AgeAdjustment != nil || sheet.Creation.OccupationSnapshot != nil {
		return nil, AgeAdjustmentResult{}, fmt.Errorf("age adjustment must be applied once before occupation")
	}
	if len(sheet.Creation.BaseSkills) == 0 {
		sheet.Creation.BaseSkills = cloneSkills(sheet.Skills)
	}
	age := sheet.Profile.Age
	physicalTotal, appearanceReduction, educationChecks := ageAdjustmentRequirements(age)
	allowed := map[string]bool{"str": true, "siz": true}
	if age >= 40 {
		allowed = map[string]bool{"str": true, "con": true, "dex": true}
	}
	result := AgeAdjustmentResult{Age: age, PhysicalReductions: map[string]int{}, AppearanceReduction: appearanceReduction, EducationChecks: []EducationCheck{}}
	total := 0
	for key, value := range reductions {
		if !allowed[key] || value < 0 || value > sheet.Attributes[key] {
			return nil, AgeAdjustmentResult{}, fmt.Errorf("invalid age reduction")
		}
		result.PhysicalReductions[key] = value
		total += value
	}
	if total != physicalTotal || sheet.Attributes["app"] < appearanceReduction {
		return nil, AgeAdjustmentResult{}, fmt.Errorf("age reductions do not match requirements")
	}
	for key, value := range result.PhysicalReductions {
		sheet.Attributes[key] -= value
	}
	sheet.Attributes["app"] -= appearanceReduction
	if age < 20 {
		if sheet.Attributes["edu"] < 5 {
			return nil, AgeAdjustmentResult{}, fmt.Errorf("education too low")
		}
		result.EducationReduction = 5
		sheet.Attributes["edu"] -= 5
		result.LuckReroll = (roll(6) + roll(6) + roll(6)) * 5
		sheet.Status.Luck = max(sheet.Status.Luck, result.LuckReroll)
	}
	for range educationChecks {
		check := roll(100)
		item := EducationCheck{Roll: check}
		if check > sheet.Attributes["edu"] {
			item.Increase = roll(10)
			sheet.Attributes["edu"] = min(99, sheet.Attributes["edu"]+item.Increase)
		}
		result.EducationChecks = append(result.EducationChecks, item)
	}
	updateBaseSkill(&sheet, "闪避", sheet.Attributes["dex"]/2)
	updateBaseSkill(&sheet, "母语", sheet.Attributes["edu"])
	applyDerived(&sheet, false)
	sheet.Creation.AgeAdjustment = &result
	encoded, err := json.Marshal(sheet)
	if err != nil {
		return nil, AgeAdjustmentResult{}, err
	}
	normalized, err := Normalize(encoded)
	return normalized, result, err
}

func ageAdjustmentRequirements(age int) (physical, appearance, educationChecks int) {
	switch {
	case age < 20:
		return 5, 0, 0
	case age < 40:
		return 0, 0, 1
	case age < 50:
		return 5, 5, 2
	case age < 60:
		return 10, 10, 3
	case age < 70:
		return 20, 15, 4
	case age < 80:
		return 40, 20, 4
	default:
		return 80, 25, 4
	}
}

func updateBaseSkill(sheet *Sheet, skill string, base int) {
	oldBase, ok := sheet.Creation.BaseSkills[skill]
	if !ok {
		oldBase = sheet.Skills[skill]
	}
	sheet.Skills[skill] += base - oldBase
	sheet.Creation.BaseSkills[skill] = base
}

// GrowSkills performs CoC 7th edition investigator development checks. A skill
// improves when d100 is greater than its current value or the roll is 96–100.
func GrowSkills(data json.RawMessage, skills []string) (json.RawMessage, SkillGrowthResult, error) {
	return growSkills(data, skills, func(sides int) int { return rollDice(1, sides) })
}

func growSkills(data json.RawMessage, skills []string, roll func(int) int) (json.RawMessage, SkillGrowthResult, error) {
	var sheet Sheet
	if err := json.Unmarshal(data, &sheet); err != nil {
		return nil, SkillGrowthResult{}, err
	}
	if len(skills) == 0 || len(skills) > len(sheet.Skills) {
		return nil, SkillGrowthResult{}, fmt.Errorf("select at least one skill")
	}
	seen := map[string]bool{}
	result := SkillGrowthResult{Items: make([]SkillGrowthItem, 0, len(skills))}
	for _, skill := range skills {
		before, exists := sheet.Skills[skill]
		if !exists || skill == "克苏鲁神话" || seen[skill] {
			return nil, SkillGrowthResult{}, fmt.Errorf("invalid growth skill %s", skill)
		}
		seen[skill] = true
		check := roll(100)
		item := SkillGrowthItem{Skill: skill, Roll: check, Before: before, After: before}
		if check > before || check >= 96 {
			item.Succeeded = true
			item.Increase = roll(10)
			item.After += item.Increase
			sheet.Skills[skill] = item.After
		}
		result.Items = append(result.Items, item)
	}
	encoded, err := json.Marshal(sheet)
	if err != nil {
		return nil, SkillGrowthResult{}, err
	}
	normalized, err := Normalize(encoded)
	return normalized, result, err
}

func AllocateSkills(data json.RawMessage, allocation SkillAllocation) (json.RawMessage, error) {
	var sheet Sheet
	if err := json.Unmarshal(data, &sheet); err != nil {
		return nil, err
	}
	occupation := sheet.Creation.OccupationSnapshot
	if occupation == nil {
		return nil, fmt.Errorf("occupation is required")
	}
	if len(allocation.ChoiceSelections) != len(occupation.ChoiceGroups) {
		return nil, fmt.Errorf("occupation choices are incomplete")
	}
	allowed := map[string]bool{"信用评级": true}
	for _, skill := range occupation.FixedSkills {
		allowed[skill] = true
	}
	chosen := map[string]bool{}
	for index, group := range occupation.ChoiceGroups {
		selection := allocation.ChoiceSelections[index]
		if len(selection) != group.Count {
			return nil, fmt.Errorf("choice group %d requires %d skills", index, group.Count)
		}
		options := stringSet(group.Skills)
		for _, skill := range selection {
			if skill == "克苏鲁神话" || chosen[skill] {
				return nil, fmt.Errorf("invalid occupation skill choice")
			}
			if len(options) > 0 && !options[skill] {
				return nil, fmt.Errorf("skill %s is not in choice group", skill)
			}
			if _, exists := sheet.Skills[skill]; !exists {
				return nil, fmt.Errorf("unknown skill %s", skill)
			}
			chosen[skill], allowed[skill] = true, true
		}
	}
	if len(allocation.FreeSkills) != occupation.FreeChoiceCount {
		return nil, fmt.Errorf("free skill choices are incomplete")
	}
	for _, skill := range allocation.FreeSkills {
		if skill == "克苏鲁神话" || chosen[skill] {
			return nil, fmt.Errorf("invalid free skill choice")
		}
		if _, exists := sheet.Skills[skill]; !exists {
			return nil, fmt.Errorf("unknown skill %s", skill)
		}
		chosen[skill], allowed[skill] = true, true
	}
	if len(sheet.Creation.BaseSkills) == 0 {
		sheet.Creation.BaseSkills = cloneSkills(sheet.Skills)
	}
	// Repair sheets created by older versions which only snapshotted the two
	// attribute-derived skills during generation. No allocation has been saved
	// yet, so the current skill values are still the correct base values.
	if len(sheet.Creation.OccupationAllocations) == 0 && len(sheet.Creation.InterestAllocations) == 0 && len(sheet.Creation.BaseSkills) < len(sheet.Skills) {
		sheet.Creation.BaseSkills = cloneSkills(sheet.Skills)
	}
	occupationSpent, interestSpent := 0, 0
	for skill, points := range allocation.Occupation {
		if !allowed[skill] || points < 0 {
			return nil, fmt.Errorf("invalid occupation allocation for %s", skill)
		}
		occupationSpent += points
	}
	for skill, points := range allocation.Interest {
		if skill == "克苏鲁神话" || points < 0 {
			return nil, fmt.Errorf("invalid interest allocation for %s", skill)
		}
		if _, exists := sheet.Skills[skill]; !exists {
			return nil, fmt.Errorf("unknown skill %s", skill)
		}
		interestSpent += points
	}
	if occupationSpent > sheet.Creation.OccupationPoints || interestSpent > sheet.Creation.InterestPoints {
		return nil, fmt.Errorf("skill point budget exceeded")
	}
	nextSkills := cloneSkills(sheet.Creation.BaseSkills)
	for skill, points := range allocation.Occupation {
		nextSkills[skill] += points
	}
	for skill, points := range allocation.Interest {
		nextSkills[skill] += points
	}
	for skill, value := range nextSkills {
		if value > 99 {
			return nil, fmt.Errorf("skill %s exceeds 99", skill)
		}
	}
	credit := nextSkills["信用评级"]
	if credit < occupation.CreditRating.Min || credit > occupation.CreditRating.Max {
		return nil, fmt.Errorf("credit rating must be between %d and %d", occupation.CreditRating.Min, occupation.CreditRating.Max)
	}
	sheet.Skills = nextSkills
	sheet.Creation.OccupationAllocations = allocation.Occupation
	sheet.Creation.InterestAllocations = allocation.Interest
	sheet.Creation.ChoiceSelections = allocation.ChoiceSelections
	sheet.Creation.FreeSkills = allocation.FreeSkills
	encoded, err := json.Marshal(sheet)
	if err != nil {
		return nil, err
	}
	return Normalize(encoded)
}

func cloneSkills(source map[string]int) map[string]int {
	result := make(map[string]int, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
func stringSet(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}

func Generate(data json.RawMessage) (json.RawMessage, error) {
	var sheet Sheet
	if err := json.Unmarshal(data, &sheet); err != nil {
		return nil, err
	}
	sheet.Creation.AgeAdjustment = nil
	for _, key := range []string{"str", "con", "dex", "app", "pow"} {
		sheet.Attributes[key] = rollDice(3, 6) * 5
	}
	for _, key := range []string{"siz", "int", "edu"} {
		sheet.Attributes[key] = (rollDice(2, 6) + 6) * 5
	}
	sheet.Status.Luck = rollDice(3, 6) * 5
	if sheet.Skills == nil {
		sheet.Skills = defaultSkills(sheet.Attributes)
	}
	sheet.Skills["闪避"] = sheet.Attributes["dex"] / 2
	sheet.Skills["母语"] = sheet.Attributes["edu"]
	if len(sheet.Creation.BaseSkills) == 0 {
		sheet.Creation.BaseSkills = cloneSkills(sheet.Skills)
	} else {
		sheet.Creation.BaseSkills["闪避"] = sheet.Skills["闪避"]
		sheet.Creation.BaseSkills["母语"] = sheet.Skills["母语"]
	}
	applyDerived(&sheet, true)
	encoded, err := json.Marshal(sheet)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func applyDerived(sheet *Sheet, resetCurrent bool) {
	con, siz, pow := sheet.Attributes["con"], sheet.Attributes["siz"], sheet.Attributes["pow"]
	hp, mp := (con+siz)/10, pow/5
	sheet.Status.HP.Max, sheet.Status.MP.Max = hp, mp
	mythos := sheet.Skills["克苏鲁神话"]
	sheet.Status.SAN.Max = max(0, 99-mythos)
	if resetCurrent {
		sheet.Status.HP.Current, sheet.Status.MP.Current, sheet.Status.SAN.Current = hp, mp, pow
	}
	sheet.Derived.Move = movement(sheet.Attributes["str"], siz, sheet.Attributes["dex"], sheet.Profile.Age)
	sheet.Derived.Build, sheet.Derived.DamageBonus = buildAndDamage(sheet.Attributes["str"] + siz)
	sheet.Creation.InterestPoints = sheet.Attributes["int"] * 2
	if sheet.Creation.OccupationSnapshot != nil {
		if points, err := sheet.Creation.OccupationSnapshot.SkillPoints(sheet.Creation.FormulaIndex, sheet.Attributes); err == nil {
			sheet.Creation.OccupationPoints = points
		}
	}
}

func movement(strength, size, dexterity, age int) int {
	move := 8
	if strength < size && dexterity < size {
		move = 7
	} else if strength > size && dexterity > size {
		move = 9
	}
	if age >= 40 {
		reduction := (age - 30) / 10
		if reduction > 5 {
			reduction = 5
		}
		move -= reduction
	}
	return max(1, move)
}

func buildAndDamage(total int) (int, string) {
	switch {
	case total <= 64:
		return -2, "-2"
	case total <= 84:
		return -1, "-1"
	case total <= 124:
		return 0, "0"
	case total <= 164:
		return 1, "+1d4"
	case total <= 204:
		return 2, "+1d6"
	default:
		dice := 2 + (total-205)/80
		return dice + 1, fmt.Sprintf("+%dd6", dice)
	}
}

func defaultSkills(attributes map[string]int) map[string]int {
	return map[string]int{
		"会计": 5, "人类学": 1, "估价": 5, "考古学": 1,
		"技艺①": 5, "技艺②": 5, "技艺③": 5, "取悦": 15, "攀爬": 20,
		"计算机使用": 5, "信用评级": 0, "克苏鲁神话": 0, "乔装": 5,
		"闪避": attributes["dex"] / 2, "汽车驾驶": 20, "电气维修": 10, "电子学": 1, "话术": 5,
		"格斗": 25, "格斗①": 25, "格斗②": 25, "格斗③": 25,
		"射击": 20, "射击①": 20, "射击②": 25, "射击③": 25,
		"急救": 30, "历史": 5, "恐吓": 15, "跳跃": 20,
		"外语①": 1, "外语②": 1, "外语③": 1, "母语": attributes["edu"],
		"法律": 5, "图书馆使用": 20, "聆听": 20, "锁匠": 1, "机械维修": 10,
		"医学": 1, "博物学": 10, "导航": 10, "神秘学": 5, "操作重型机械": 1,
		"说服": 10, "驾驶": 1, "精神分析": 1, "心理学": 10, "骑术": 5,
		"科学①": 1, "科学②": 1, "科学③": 1, "妙手": 10, "侦察": 25,
		"潜行": 20, "生存": 10, "游泳": 20, "投掷": 20, "追踪": 10,
		"驯兽": 5, "潜水": 1, "爆破": 1, "读唇": 1, "催眠": 1, "炮术": 1, "学识": 1,
		// Legacy aliases remain readable for existing snapshots and API clients;
		// the web sheet only renders the Excel-ordered keys above.
		"艺术与手艺": 5, "魅惑": 15, "斗殴": 25, "手枪": 20, "步枪/霰弹枪": 25,
		"外语": 1, "领航": 10, "重型操作": 1, "科学": 1, "侦查": 25,
	}
}

func rollDice(count, sides int) int {
	total := 0
	for range count {
		value, err := rand.Int(rand.Reader, big.NewInt(int64(sides)))
		if err != nil {
			panic("system random source unavailable")
		}
		total += int(value.Int64()) + 1
	}
	return total
}
