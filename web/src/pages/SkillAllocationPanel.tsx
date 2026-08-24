import { useMemo, useState, type FormEvent } from "react";
import {
  APIError,
  api,
  type Character,
  type CharacterSheet,
} from "../api/client";

export function SkillAllocationPanel({
  character,
  sheet,
  onApplied,
}: {
  character: Character;
  sheet: CharacterSheet;
  onApplied: (updated: Character) => void;
}) {
  const occupation = sheet.creation.occupationSnapshot;
  const [occupationPoints, setOccupationPoints] = useState<
    Record<string, number>
  >({ ...sheet.creation.occupationAllocations });
  const [interestPoints, setInterestPoints] = useState<Record<string, number>>({
    ...sheet.creation.interestAllocations,
  });
  const [choiceSelections, setChoiceSelections] = useState<string[][]>(
    occupation?.choiceGroups.map(
      (group, index) =>
        sheet.creation.choiceSelections?.[index] ?? Array(group.count).fill(""),
    ) ?? [],
  );
  const [freeSkills, setFreeSkills] = useState<string[]>(
    sheet.creation.freeSkills?.length
      ? sheet.creation.freeSkills
      : Array(occupation?.freeChoiceCount ?? 0).fill(""),
  );
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const skillNames = useMemo(
    () =>
      Object.keys(sheet.skills)
        .filter((name) => name !== "克苏鲁神话")
        .sort((a, b) => a.localeCompare(b, "zh-CN")),
    [sheet.skills],
  );
  const allowed = useMemo(
    () =>
      new Set(
        [
          "信用评级",
          ...(occupation?.fixedSkills ?? []),
          ...choiceSelections.flat(),
          ...freeSkills,
        ].filter(Boolean),
      ),
    [occupation, choiceSelections, freeSkills],
  );
  const filteredOccupationPoints = Object.fromEntries(
    Object.entries(occupationPoints).filter(
      ([skill, value]) => allowed.has(skill) && value > 0,
    ),
  );
  const filteredInterestPoints = Object.fromEntries(
    Object.entries(interestPoints).filter(([, value]) => value > 0),
  );
  const occupationSpent = Object.values(filteredOccupationPoints).reduce(
    (sum, value) => sum + value,
    0,
  );
  const interestSpent = Object.values(interestPoints).reduce(
    (sum, value) => sum + (value || 0),
    0,
  );
  if (!occupation) return null;

  function setChoice(groupIndex: number, slotIndex: number, value: string) {
    setChoiceSelections((current) => {
      const next = current.map((group) => [...group]);
      next[groupIndex][slotIndex] = value;
      return next;
    });
  }
  async function submit(event: FormEvent) {
    event.preventDefault();
    setSaving(true);
    setError("");
    try {
      const updated = await api<Character>(
        `/characters/${character.id}/skill-allocation`,
        {
          method: "PUT",
          body: JSON.stringify({
            baseVersion: character.currentVersion,
            allocation: {
              occupation: filteredOccupationPoints,
              interest: filteredInterestPoints,
              choiceSelections,
              freeSkills,
            },
          }),
        },
      );
      onApplied(updated);
    } catch (reason) {
      setError(
        reason instanceof APIError ? reason.message : "保存技能分配失败",
      );
    } finally {
      setSaving(false);
    }
  }
  return (
    <form
      className="panel allocation-panel"
      onSubmit={(event) => void submit(event)}
    >
      <div className="section-heading">
        <h2>创建技能分配</h2>
        <div className="budget">
          <span>
            职业剩余{" "}
            <strong>{sheet.creation.occupationPoints - occupationSpent}</strong>
          </span>
          <span>
            兴趣剩余{" "}
            <strong>{sheet.creation.interestPoints - interestSpent}</strong>
          </span>
        </div>
      </div>
      {occupation.choiceGroups.map((group, groupIndex) => (
        <div className="choice-line" key={groupIndex}>
          <strong>
            {group.category || `职业可选技能组 ${groupIndex + 1}`}
          </strong>
          {Array.from({ length: group.count }, (_, slotIndex) => (
            <select
              key={slotIndex}
              value={choiceSelections[groupIndex]?.[slotIndex] ?? ""}
              onChange={(event) =>
                setChoice(groupIndex, slotIndex, event.target.value)
              }
              required
            >
              <option value="">请选择</option>
              {(group.skills?.length ? group.skills : skillNames).map(
                (skill) => (
                  <option value={skill} key={skill}>
                    {skill}
                  </option>
                ),
              )}
            </select>
          ))}
        </div>
      ))}
      {freeSkills.length > 0 && (
        <div className="choice-line">
          <strong>任意职业技能</strong>
          {freeSkills.map((value, index) => (
            <select
              key={index}
              value={value}
              onChange={(event) =>
                setFreeSkills((current) =>
                  current.map((item, itemIndex) =>
                    itemIndex === index ? event.target.value : item,
                  ),
                )
              }
              required
            >
              <option value="">请选择</option>
              {skillNames.map((skill) => (
                <option value={skill} key={skill}>
                  {skill}
                </option>
              ))}
            </select>
          ))}
        </div>
      )}
      <div className="allocation-table">
        <div className="allocation-head">
          <span>技能</span>
          <span>基础</span>
          <span>职业点</span>
          <span>兴趣点</span>
          <span>最终</span>
        </div>
        {skillNames.map((skill) => {
          const base =
            sheet.creation.baseSkills?.[skill] ?? sheet.skills[skill];
          const occupational = allowed.has(skill)
            ? (occupationPoints[skill] ?? 0)
            : 0;
          const interest = interestPoints[skill] ?? 0;
          return (
            <label className="allocation-row" key={skill}>
              <span>
                {skill}
                {allowed.has(skill) && <small>职业</small>}
              </span>
              <output>{base}</output>
              <input
                type="number"
                min="0"
                value={occupational}
                disabled={!allowed.has(skill)}
                onChange={(event) =>
                  setOccupationPoints((current) => ({
                    ...current,
                    [skill]: Number(event.target.value),
                  }))
                }
              />
              <input
                type="number"
                min="0"
                value={interest}
                onChange={(event) =>
                  setInterestPoints((current) => ({
                    ...current,
                    [skill]: Number(event.target.value),
                  }))
                }
              />
              <output>{base + occupational + interest}</output>
            </label>
          );
        })}
      </div>
      {error && <p className="form-error">{error}</p>}
      <button
        disabled={
          saving ||
          occupationSpent > sheet.creation.occupationPoints ||
          interestSpent > sheet.creation.interestPoints
        }
      >
        {saving ? "正在保存…" : "保存技能分配"}
      </button>
    </form>
  );
}
