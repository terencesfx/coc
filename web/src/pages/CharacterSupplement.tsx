import { useState } from "react";
import {
  APIError,
  api,
  type Character,
  type CharacterSheet,
  type CheckResult,
  type DiceRoll,
  type ExpressionResult,
} from "../api/client";
import weaponCatalog from "../data/coc7-weapons.json";

type WeaponPreset = (typeof weaponCatalog)[number];

export function CharacterSupplement({
  character,
  sheet,
  change,
  campaignID,
  section = "all",
}: {
  character: Character;
  sheet: CharacterSheet;
  change: (mutator: (next: CharacterSheet) => void) => void;
  campaignID: string;
  section?: "all" | "combat" | "assets" | "details";
}) {
  const [damageResult, setDamageResult] =
    useState<DiceRoll<ExpressionResult> | null>(null);
  const [hitResult, setHitResult] = useState<DiceRoll<CheckResult> | null>(
    null,
  );
  const [bonusPenalty, setBonusPenalty] = useState(0);
  const [error, setError] = useState("");
  const [weaponPickerIndex, setWeaponPickerIndex] = useState<number | null>(null);
  const [weaponSearch, setWeaponSearch] = useState("");
  function addPresetWeapon(preset: WeaponPreset) {
    if (weaponPickerIndex === null) return;
    change((next) => {
      const weapon = {
        id: crypto.randomUUID(),
        name: preset.name,
        skill: preset.skill,
        damage: preset.damage,
        range: preset.range,
        attacks: Number.parseInt(preset.attacksText, 10) || 0,
        ammo: Number.parseInt(preset.ammoText, 10) || 0,
        malfunction: Number.parseInt(preset.malfunctionText, 10) || 0,
        attacksText: preset.attacksText,
        ammoText: preset.ammoText,
        malfunctionText: preset.malfunctionText,
        penetration: preset.penetration,
        era: preset.era,
        price: preset.price,
        invention: preset.invention,
        category: preset.category,
        notes: preset.notes,
      };
      if (weaponPickerIndex < next.weapons.length) {
        next.weapons[weaponPickerIndex] = weapon;
      } else if (next.weapons.length < 5) {
        next.weapons.push(weapon);
      }
    });
    setWeaponPickerIndex(null);
    setWeaponSearch("");
  }
  async function rollDamage(name: string, expression: string) {
    try {
      const result = await api<DiceRoll<ExpressionResult>>(
        "/rolls/expression",
        {
          method: "POST",
          body: JSON.stringify({
            requestId: crypto.randomUUID(),
            characterId: character.id,
            expression,
            label: `${name}伤害`,
            campaignId: campaignID || null,
            visibility: "public",
          }),
        },
      );
      setDamageResult(result);
      setError("");
    } catch (reason) {
      setError(reason instanceof APIError ? reason.message : "伤害投骰失败");
    }
  }
  async function rollHit(name: string, skill: string) {
    try {
      const result = await api<DiceRoll<CheckResult>>("/rolls/check", {
        method: "POST",
        body: JSON.stringify({
          requestId: crypto.randomUUID(),
          characterId: character.id,
          skill,
          bonusPenalty,
          campaignId: campaignID || null,
          visibility: "public",
        }),
      });
      setHitResult({ ...result, label: `${name}命中（${skill}）` });
      setError("");
    } catch (reason) {
      setError(
        reason instanceof APIError ? reason.message : "武器命中检定失败",
      );
    }
  }
  return (
    <>
      {(section === "all" || section === "details") && <section className="panel">
        <h2>人物背景</h2>
        <div className="backstory-grid">
          {backstoryFields.map(([key, label]) => (
            <label key={key}>
              {label}
              <textarea
                rows={4}
                value={sheet.backstory[key] ?? ""}
                disabled={!character.canEdit}
                onChange={(event) =>
                  change((next) => {
                    next.backstory[key] = event.target.value;
                  })
                }
              />
            </label>
          ))}
        </div>
      </section>}
      {(section === "all" || section === "combat") && <div className="character-combat-layout">
      <section className="panel combat-weapons-panel">
        <div className="section-heading">
          <h2>武器</h2>
          <div className="weapon-tools">
            <select
              aria-label="武器检定奖励骰或惩罚骰"
              value={bonusPenalty}
              onChange={(event) => setBonusPenalty(Number(event.target.value))}
            >
              <option value={2}>2 奖励骰</option>
              <option value={1}>1 奖励骰</option>
              <option value={0}>普通检定</option>
              <option value={-1}>1 惩罚骰</option>
              <option value={-2}>2 惩罚骰</option>
            </select>
          </div>
        </div>
        {error && <p className="error-banner">{error}</p>}
        {hitResult && (
          <p className={`roll-result outcome-${hitResult.result.outcome}`}>
            <span>{hitResult.label}</span>
            <strong>{hitResult.result.value}</strong>
            <span>
              / {hitResult.result.target} ·{" "}
              {outcomeNames[hitResult.result.outcome]}
            </span>
          </p>
        )}
        {damageResult && (
          <p className="roll-result">
            <span>{damageResult.label}</span>
            <strong>{damageResult.result.total}</strong>
          </p>
        )}
        <div className="weapon-table">
          <div className="weapon-head">
            <span>名称</span>
            <span>技能</span>
            <span>伤害表达式</span>
            <span>射程</span>
            <span>每轮</span>
            <span>装弹量</span>
            <span>故障值</span>
            <span />
          </div>
          {Array.from({ length: 5 }, (_, index) => sheet.weapons[index]).map((weapon, index) => (
            <div className={`weapon-row${weapon ? "" : " weapon-row--empty"}`} key={weapon?.id ?? `empty-${index}`}>
              {character.canEdit ? (
                <button className="weapon-name-button secondary" type="button" onClick={() => setWeaponPickerIndex(index)}>
                  {weapon?.name || "选择武器"}
                </button>
              ) : <span className="weapon-name-readonly">{weapon?.name || "—"}</span>}
              <input
                value={weapon?.skill ?? ""}
                disabled={!character.canEdit || !weapon}
                onChange={(event) =>
                  change((next) => {
                    next.weapons[index].skill = event.target.value;
                  })
                }
              />
              <input
                value={weapon?.damage ?? ""}
                disabled={!character.canEdit || !weapon}
                onChange={(event) =>
                  change((next) => {
                    next.weapons[index].damage = event.target.value;
                  })
                }
              />
              <input
                value={weapon?.range ?? ""}
                disabled={!character.canEdit || !weapon}
                onChange={(event) =>
                  change((next) => {
                    next.weapons[index].range = event.target.value;
                  })
                }
              />
              <input
                value={weapon ? (weapon.attacksText ?? String(weapon.attacks)) : ""}
                disabled={!character.canEdit || !weapon}
                onChange={(event) =>
                  change((next) => {
                    next.weapons[index].attacksText = event.target.value;
                  })
                }
              />
              <input
                value={weapon ? (weapon.ammoText ?? String(weapon.ammo)) : ""}
                disabled={!character.canEdit || !weapon}
                onChange={(event) =>
                  change((next) => {
                    next.weapons[index].ammoText = event.target.value;
                  })
                }
              />
              <input
                value={weapon ? (weapon.malfunctionText ?? String(weapon.malfunction)) : ""}
                disabled={!character.canEdit || !weapon}
                onChange={(event) =>
                  change((next) => {
                    next.weapons[index].malfunctionText = event.target.value;
                  })
                }
              />
              <div className="row-actions">
                {character.canEdit && weapon && (
                  <>
                    <button
                      type="button"
                      onClick={() => void rollHit(weapon.name, weapon.skill)}
                    >
                      命中
                    </button>
                    <button
                      type="button"
                      onClick={() =>
                        void rollDamage(weapon.name, weapon.damage)
                      }
                    >
                      伤害
                    </button>
                    <button
                      type="button"
                      onClick={() =>
                        change((next) => {
                          next.weapons.splice(index, 1);
                        })
                      }
                    >
                      清除
                    </button>
                  </>
                )}
              </div>
            </div>
          ))}
        </div>
      </section>
      <aside className="panel combat-summary-panel">
        <h2>战斗</h2>
        <dl className="combat-summary">
          <div><dt>伤害加值</dt><dd>{sheet.derived.damageBonus || "0"}</dd></div>
          <div><dt>体格</dt><dd>{sheet.derived.build}</dd></div>
          <div><dt>移动力</dt><dd>{sheet.derived.move}</dd></div>
          <div><dt>闪避</dt><dd>{sheet.skills["闪避"] ?? 0}</dd></div>
        </dl>
        <p className="muted">这些数值随人物属性与技能自动更新。</p>
      </aside>
      </div>}
      {weaponPickerIndex !== null && (
        <div className="modal-backdrop" role="presentation" onMouseDown={(event) => {
          if (event.target === event.currentTarget) setWeaponPickerIndex(null);
        }}>
          <section className="weapon-picker" role="dialog" aria-modal="true" aria-label="选择武器">
            <header className="section-heading">
              <div><p className="eyebrow">COC7 武器库</p><h2>选择武器</h2></div>
              <button className="secondary" type="button" onClick={() => setWeaponPickerIndex(null)}>关闭</button>
            </header>
            <input autoFocus value={weaponSearch} onChange={(event) => setWeaponSearch(event.target.value)} placeholder="搜索名称、技能、时代或类别" />
            <div className="weapon-picker-list">
              {weaponCatalog.filter((weapon) => `${weapon.name} ${weapon.skill} ${weapon.era} ${weapon.category}`.toLowerCase().includes(weaponSearch.trim().toLowerCase())).map((weapon) => (
                <button className="weapon-preset" type="button" key={weapon.name} onClick={() => addPresetWeapon(weapon)}>
                  <strong>{weapon.name}</strong>
                  <span>{weapon.skill} · {weapon.damage} · {weapon.range}</span>
                  <small>每轮 {weapon.attacksText}　装弹量 {weapon.ammoText}　故障 {weapon.malfunctionText}　贯穿 {weapon.penetration}</small>
                </button>
              ))}
            </div>
          </section>
        </div>
      )}
      {(section === "all" || section === "assets") && <div className="character-assets-layout">
      <section className="panel">
        <h2>装备与财产</h2>
        <div className="finance-grid">
          <label>
            消费水平
            <input
              value={sheet.finances?.spendingLevel ?? ""}
              disabled={!character.canEdit}
              onChange={(event) =>
                change((next) => {
                  next.finances.spendingLevel = event.target.value;
                })
              }
            />
          </label>
          <label>
            现金
            <input
              value={sheet.finances?.cash ?? ""}
              disabled={!character.canEdit}
              onChange={(event) =>
                change((next) => {
                  next.finances.cash = event.target.value;
                })
              }
            />
          </label>
          <label>
            资产
            <textarea
              rows={12}
              value={sheet.finances?.assets ?? ""}
              disabled={!character.canEdit}
              onChange={(event) =>
                change((next) => {
                  next.finances.assets = event.target.value;
                })
              }
            />
          </label>
        </div>
      </section>
      <aside className="panel mythos-panel">
        <h2>克苏鲁神话</h2>
        <div className="mythos-fields">
          {mythosFields.map(([key, label, legacyKey]) => (
            <label key={key}>
              {label}
              <textarea
                rows={5}
                value={sheet.backstory[key] ?? (legacyKey ? sheet.backstory[legacyKey] ?? "" : "")}
                disabled={!character.canEdit}
                onChange={(event) => change((next) => {
                  next.backstory[key] = event.target.value;
                })}
              />
            </label>
          ))}
        </div>
      </aside>
      </div>}
    </>
  );
}

const backstoryFields = [
  ["description", "外貌描述"],
  ["ideology", "思想与信念"],
  ["significantPeople", "重要之人"],
  ["meaningfulLocations", "意义非凡之地"],
  ["treasuredPossessions", "宝贵之物"],
  ["traits", "特质"],
  ["injuriesScars", "伤口与疤痕"],
  ["phobiasManias", "恐惧与躁狂症"],
] as const;

const mythosFields = [
  ["magicItemsAndTomes", "魔法物品与典籍", "tomesSpellsArtifacts"],
  ["spells", "法术", ""],
  ["thirdKindEncounters", "第三类接触", "strangeEncounters"],
] as const;
const outcomeNames: Record<CheckResult["outcome"], string> = {
  critical: "大成功",
  extreme: "极难成功",
  hard: "困难成功",
  regular: "成功",
  failure: "失败",
  fumble: "大失败",
};
