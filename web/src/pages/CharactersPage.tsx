import { useEffect, useState, type FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { APIError, api, type Character, type CharacterSummary } from "../api/client";

export function CharactersPage() {
  const navigate = useNavigate();
  const [items, setItems] = useState<CharacterSummary[]>([]);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState("");
  useEffect(() => {
    let active = true;
    api<{ items: CharacterSummary[] }>("/characters")
      .then((result) => { if (active) setItems(result.items ?? []); })
      .catch((reason: unknown) => {
        if (active) setError(reason instanceof APIError ? reason.message : "读取人物卡失败");
      });
    return () => { active = false; };
  }, []);

  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    try {
      const created = await api<Character>("/characters", {
        method: "POST",
        body: JSON.stringify({ name: form.get("name"), kind: form.get("kind") }),
      });
      navigate(`/characters/${created.id}`);
    } catch (reason) {
      setError(reason instanceof APIError ? reason.message : "创建人物卡失败");
    }
  }

  return <section className="page-stack">
    <header className="page-header">
      <div><p className="eyebrow">角色档案</p><h1>人物卡</h1></div>
      <button type="button" onClick={() => setCreating((value) => !value)}>{creating ? "取消" : "创建人物卡"}</button>
    </header>
    {creating && <form className="inline-form character-create" onSubmit={(event) => void create(event)}>
      <label>人物姓名<input name="name" required autoFocus /></label>
      <label>类型<select name="kind" defaultValue="investigator"><option value="investigator">调查员</option><option value="npc">NPC</option></select></label>
      <button>创建</button>
    </form>}
    {error && <p className="form-error">{error}</p>}
    <div className="character-grid">
      {items.map((item) => <Link className="character-card" to={`/characters/${item.id}`} key={item.id}>
        <span className="status">{item.kind === "npc" ? "NPC" : "调查员"} · {statusNames[item.status]}</span>
        <h2>{item.name}</h2><p>{item.occupation || "尚未填写职业"}</p>
        <small>{new Date(item.updatedAt).toLocaleString()}</small>
      </Link>)}
    </div>
    {items.length === 0 && !creating && <div className="empty-state">还没有人物卡，先创建第一位调查员吧。</div>}
  </section>;
}

const statusNames: Record<CharacterSummary["status"], string> = {
  draft: "草稿", active: "启用", retired: "退役", deceased: "死亡", archived: "归档",
};
