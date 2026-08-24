import { useMemo, useState } from "react";
import { APIError, api, type DiceRoll, type ExpressionResult } from "../api/client";

const diceSides = [2, 3, 4, 6, 8, 10, 12, 20, 100] as const;

export function QuickDiceDrawer({ open, onClose, characterID }: {
  open: boolean;
  onClose: () => void;
  characterID: string;
}) {
  const [selectedDice, setSelectedDice] = useState<number[]>([]);
  const [result, setResult] = useState<DiceRoll<ExpressionResult> | null>(null);
  const [error, setError] = useState("");
  const [rolling, setRolling] = useState(false);
  const expression = useMemo(() => {
    const counts = new Map<number, number>();
    for (const sides of selectedDice) counts.set(sides, (counts.get(sides) ?? 0) + 1);
    return diceSides.filter((sides) => counts.has(sides)).map((sides) => `${counts.get(sides)}d${sides}`).join("+");
  }, [selectedDice]);

  function addDie(sides: number) {
    if (selectedDice.length >= 200) return setError("一次最多投掷 200 枚骰子");
    setSelectedDice((current) => [...current, sides]);
    setResult(null);
    setError("");
  }
  function undo() {
    setSelectedDice((current) => current.slice(0, -1));
    setResult(null);
    setError("");
  }
  function clear() {
    setSelectedDice([]);
    setResult(null);
    setError("");
  }
  async function roll() {
    if (!expression) return;
    setRolling(true);
    setError("");
    try {
      setResult(await api<DiceRoll<ExpressionResult>>("/rolls/expression", {
        method: "POST",
        body: JSON.stringify({
          requestId: crypto.randomUUID(), characterId: characterID,
          campaignId: null, visibility: "public", expression,
          label: expression.toUpperCase(),
        }),
      }));
    } catch (reason) {
      setError(reason instanceof APIError ? reason.message : "投骰失败");
    } finally {
      setRolling(false);
    }
  }

  if (!open) return null;
  return <>
    <button className="drawer-backdrop" type="button" aria-label="关闭快速骰子" onClick={onClose} />
    <aside className="dice-drawer" aria-label="组合骰" aria-modal="true">
      <div className="drawer-heading">
        <div><p className="eyebrow">QUICK DICE</p><h2>组合骰</h2></div>
        <button className="text-button" type="button" onClick={onClose}>关闭</button>
      </div>
      <p className="muted dice-context-note">投骰结果将自动记录到当前人物卡。</p>
      <div className="dice-expression-preview" aria-live="polite">
        {expression ? expression.toUpperCase().replaceAll("+", " + ") : "点击下方骰子组成表达式"}
      </div>
      <div className="dice-button-grid">
        {diceSides.map((sides) => {
          const count = selectedDice.filter((item) => item === sides).length;
          return <button className={count ? "dice-choice dice-choice--selected" : "dice-choice"} type="button" key={sides} onClick={() => addDie(sides)}>
            <strong>D{sides}</strong>{count > 0 && <small>× {count}</small>}
          </button>;
        })}
      </div>
      <div className="dice-builder-actions">
        <button className="secondary" type="button" disabled={!selectedDice.length} onClick={undo}>撤销</button>
        <button className="secondary" type="button" disabled={!selectedDice.length} onClick={clear}>清空</button>
      </div>
      {error && <p className="error-banner">{error}</p>}
      <button className="primary-button drawer-roll-button" type="button" disabled={rolling || !expression} onClick={() => void roll()}>{rolling ? "投掷中……" : "投掷"}</button>
      {result && <QuickResult roll={result} />}
    </aside>
  </>;
}

function QuickResult({ roll }: { roll: DiceRoll<ExpressionResult> }) {
  return <div className="quick-result">
    <span>{roll.label}</span><strong>{roll.result.total}</strong>
    <small>{roll.result.terms.map((term) => term.rolls ? `${term.expression}[${term.rolls.join(",")}]` : String(term.value)).join(" · ")}</small>
  </div>;
}
