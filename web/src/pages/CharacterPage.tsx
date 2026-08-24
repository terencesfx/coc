import { useEffect, useMemo, useRef, useState } from "react";
import {
  Link,
  useNavigate,
  useParams,
} from "react-router-dom";
import {
  APIError,
  api,
  type AgeAdjustmentResult,
  type Character,
  type CharacterSheet,
  type CharacterVersion,
  type CharacterVersionDetail,
  type CheckResult,
  type DiceRoll,
  type ExpressionResult,
  type Occupation,
} from "../api/client";
import { CharacterSupplement } from "./CharacterSupplement";
import { useAuth } from "../auth/AuthContext";
import skillDescriptionCatalog from "../data/coc7-skill-descriptions.json";

type SkillDescription = (typeof skillDescriptionCatalog)[number];
const skillDescriptionByName = new Map(skillDescriptionCatalog.map((item) => [item.name, item]));
const skillDescriptionAliases: Record<string, string> = {
  技艺: "艺术与手艺", 绘画: "美术", 烹饪: "厨艺", 徒手: "斗殴",
  电子学: "电子学 Ω", 驯兽: "动物驯养", 侦察: "侦查",
};
const skillDescriptionSections = [
  ["difficulty", "难度依据"],
  ["description", "技能解释"],
  ["regularExample", "普通难度示例"],
  ["hardExample", "困难难度示例"],
  ["pushExamples", "孤注一掷示例"],
  ["pushFailure", "孤注一掷失败"],
  ["insaneFailure", "疯狂调查员孤注一掷失败"],
] as const;

const attributes: [string, string][] = [
  ["str", "力量"],
  ["con", "体质"],
  ["siz", "体型"],
  ["dex", "敏捷"],
  ["app", "外貌"],
  ["int", "智力"],
  ["pow", "意志"],
  ["edu", "教育"],
];
const statusResourceNames = { hp: "生命", mp: "魔法", san: "理智" } as const;
const skillAllocationNames = {
  occupationAllocations: "职业点",
  interestAllocations: "兴趣点",
  growthAllocations: "成长点",
} as const;
const skillSpecializationOptions: Record<string, string[]> = {
  "技艺①": ["表演", "摄影", "写作", "烹饪", "绘画", "雕塑", "书法", "乐器"],
  "技艺②": ["表演", "摄影", "写作", "烹饪", "绘画", "雕塑", "书法", "乐器"],
  "技艺③": ["表演", "摄影", "写作", "烹饪", "绘画", "雕塑", "书法", "乐器"],
  "格斗": ["徒手", "斧", "剑", "绞索", "链锯", "矛", "鞭"],
  "格斗①": ["徒手", "斧", "剑", "绞索", "链锯", "矛", "鞭"],
  "格斗②": ["徒手", "斧", "剑", "绞索", "链锯", "矛", "鞭"],
  "格斗③": ["徒手", "斧", "剑", "绞索", "链锯", "矛", "鞭"],
  "射击": ["手枪", "左轮手枪", "自动手枪", "步枪", "霰弹枪", "来复枪", "弓"],
  "射击①": ["手枪", "左轮手枪", "自动手枪", "步枪", "霰弹枪", "来复枪", "弓"],
  "射击②": ["手枪", "左轮手枪", "自动手枪", "步枪", "霰弹枪", "来复枪", "弓"],
  "射击③": ["手枪", "左轮手枪", "自动手枪", "步枪", "霰弹枪", "来复枪", "弓"],
  "母语": ["汉语", "英语", "日语", "法语", "德语", "西班牙语", "俄语", "拉丁语"],
  "外语①": ["汉语", "英语", "日语", "法语", "德语", "西班牙语", "俄语", "拉丁语"],
  "外语②": ["汉语", "英语", "日语", "法语", "德语", "西班牙语", "俄语", "拉丁语"],
  "外语③": ["汉语", "英语", "日语", "法语", "德语", "西班牙语", "俄语", "拉丁语"],
  "驾驶": ["飞机", "船", "马车", "摩托车"],
  "科学①": ["生物学", "化学", "地质学", "数学", "物理学", "天文学", "药学", "植物学"],
  "科学②": ["生物学", "化学", "地质学", "数学", "物理学", "天文学", "药学", "植物学"],
  "科学③": ["生物学", "化学", "地质学", "数学", "物理学", "天文学", "药学", "植物学"],
  "生存": ["森林", "山地", "沙漠", "海洋", "极地", "沼泽"],
  "学识": ["宗教", "神话", "民俗", "历史", "地域"],
};
const skillDefinitions = "会计 人类学 估价 考古学 技艺① 技艺② 技艺③ 取悦 攀爬 计算机使用 信用评级 克苏鲁神话 乔装 闪避 汽车驾驶 电气维修 电子学 话术 格斗 格斗① 格斗② 格斗③ 射击 射击① 射击② 射击③ 急救 历史 恐吓 跳跃 外语① 外语② 外语③ 母语 法律 图书馆使用 聆听 锁匠 机械维修 医学 博物学 导航 神秘学 操作重型机械 说服 驾驶 精神分析 心理学 骑术 科学① 科学② 科学③ 妙手 侦察 潜行 生存 游泳 投掷 追踪 驯兽 潜水 爆破 读唇 催眠 炮术 学识"
  .split(" ")
  .map((key) => ({ key, label: key.replace(/[①②③]$/, "") }));
const skillCategories = ["全部技能", "特殊", "探索", "社交", "战斗", "医疗", "运动", "知识", "技术", "操纵", "其他"] as const;
type SkillCategory = (typeof skillCategories)[number];
const skillCategoryMembers: Record<Exclude<SkillCategory, "全部技能">, Set<string>> = {
  特殊: new Set(["信用评级", "克苏鲁神话", "催眠", "学识"]),
  探索: new Set(["图书馆使用", "聆听", "导航", "侦察", "潜行", "生存", "追踪", "读唇"]),
  社交: new Set(["取悦", "话术", "恐吓", "说服", "心理学"]),
  战斗: new Set(["闪避", "格斗", "射击", "炮术"]),
  医疗: new Set(["急救", "医学", "精神分析"]),
  运动: new Set(["攀爬", "跳跃", "骑术", "游泳", "投掷", "潜水"]),
  知识: new Set(["会计", "人类学", "考古学", "历史", "外语", "母语", "法律", "博物学", "神秘学", "科学"]),
  技术: new Set(["技艺", "计算机使用", "电气维修", "电子学", "锁匠", "机械维修", "爆破"]),
  操纵: new Set(["汽车驾驶", "驾驶", "操作重型机械", "驯兽"]),
  其他: new Set(["估价", "乔装", "妙手"]),
};
const statusConditionFields = [
  ["majorWound", "重伤"],
  ["dying", "濒死"],
  ["unconscious", "昏迷"],
  ["temporaryInsanity", "临时疯狂"],
  ["indefiniteInsanity", "不定性疯狂"],
  ["permanentInsanity", "永久疯狂"],
] as const;

export function CharacterPage() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const campaignID = "";
  const { account } = useAuth();
  const [character, setCharacter] = useState<Character | null>(null);
  const [sheet, setSheet] = useState<CharacterSheet | null>(null);
  const [versions, setVersions] = useState<CharacterVersion[]>([]);
  const [historyDetail, setHistoryDetail] =
    useState<CharacterVersionDetail | null>(null);
  const [compareFrom, setCompareFrom] = useState("");
  const [compareTo, setCompareTo] = useState("");
  const [editMessage, setEditMessage] = useState("");
  const [historyActor, setHistoryActor] = useState("");
  const [historyKind, setHistoryKind] = useState("");
  const [historySearch, setHistorySearch] = useState("");
  const [occupations, setOccupations] = useState<Occupation[]>([]);
  const [selectedOccupationID, setSelectedOccupationID] = useState("");
  const [occupationPickerOpen, setOccupationPickerOpen] = useState(false);
  const [occupationSearch, setOccupationSearch] = useState("");
  const [previewOccupationID, setPreviewOccupationID] = useState("");
  const [ageReductions, setAgeReductions] = useState<Record<string, number>>(
    {},
  );
  const [ageResult, setAgeResult] = useState<AgeAdjustmentResult | null>(null);
  const [skillCategory, setSkillCategory] = useState<SkillCategory>("全部技能");
  const [bonusPenalty, setBonusPenalty] = useState(0);
  const [latestRoll, setLatestRoll] = useState<DiceRoll<CheckResult> | null>(
    null,
  );
  const [quickCheckRoll, setQuickCheckRoll] =
    useState<DiceRoll<CheckResult> | null>(null);
  const [attributeCheckOpen, setAttributeCheckOpen] = useState(false);
  const [sanSuccessLoss, setSanSuccessLoss] = useState("0");
  const [sanFailureLoss, setSanFailureLoss] = useState("1d6");
  const [sanLossRoll, setSanLossRoll] =
    useState<DiceRoll<ExpressionResult> | null>(null);
  const [rollingSkill, setRollingSkill] = useState("");
  const [specializationSkill, setSpecializationSkill] = useState("");
  const [skillDescription, setSkillDescription] = useState<SkillDescription | null>(null);
  const [diceExpression, setDiceExpression] = useState("1d100");
  const [expressionRoll, setExpressionRoll] =
    useState<DiceRoll<ExpressionResult> | null>(null);
  const [saveState, setSaveState] = useState<
    "saved" | "dirty" | "saving" | "error" | "conflict"
  >("saved");
  const [saveError, setSaveError] = useState("");
  const [serverVersion, setServerVersion] = useState<number | null>(null);
  const [error, setError] = useState("");
  const sheetRef = useRef<CharacterSheet | null>(null);
  const versionRef = useRef(0);

  async function load() {
    const [item, history] = await Promise.all([
      api<Character>(`/characters/${id}`),
      api<{ items: CharacterVersion[] }>(`/characters/${id}/versions`),
    ]);
    setCharacter(item);
    setSheet(item.sheet);
    setSelectedOccupationID(item.sheet.profile.occupationId ?? "");
    sheetRef.current = item.sheet;
    versionRef.current = item.currentVersion;
    setVersions(history.items ?? []);
    setSaveState("saved");
    setSaveError("");
    setServerVersion(null);
  }
  useEffect(() => {
    let active = true;
    Promise.all([
      api<Character>(`/characters/${id}`),
      api<{ items: Occupation[] }>("/rules/coc7/occupations"),
    ])
      .then(([item, occupationResult]) => {
        if (!active) return;
        setCharacter(item);
        setSheet(item.sheet);
        sheetRef.current = item.sheet;
        versionRef.current = item.currentVersion;
        setOccupations(occupationResult.items ?? []);
        setSelectedOccupationID(item.sheet.profile.occupationId ?? "");
        setSaveError("");
        setServerVersion(null);
      })
      .catch((reason: unknown) => {
        if (active)
          setError(
            reason instanceof APIError ? reason.message : "读取人物卡失败",
          );
      });
    return () => {
      active = false;
    };
  }, [id]);
  const hasUnsavedChanges = saveState !== "saved";
  useEffect(() => {
    if (!hasUnsavedChanges) return;
    const beforeUnload = (event: BeforeUnloadEvent) => {
      event.preventDefault();
      event.returnValue = true;
    };
    const protectLinks = (event: MouseEvent) => {
      if (event.defaultPrevented || event.button !== 0) return;
      const target = event.target;
      if (!(target instanceof Element)) return;
      const anchor = target.closest("a[href]") as HTMLAnchorElement | null;
      if (
        !anchor ||
        anchor.target === "_blank" ||
        anchor.origin !== location.origin
      )
        return;
      if (!window.confirm("人物卡仍有未保存内容，确定离开并放弃这些修改吗？")) {
        event.preventDefault();
        event.stopPropagation();
      }
    };
    window.addEventListener("beforeunload", beforeUnload);
    document.addEventListener("click", protectLinks, true);
    return () => {
      window.removeEventListener("beforeunload", beforeUnload);
      document.removeEventListener("click", protectLinks, true);
    };
  }, [hasUnsavedChanges]);

  function validateSheet(current: CharacterSheet): string {
    if (!current.profile.name.trim()) return "调查员姓名不能为空";
    if (!current.profile.occupationId) return "请选择职业";
    for (const [key, label] of attributes) {
      const value = current.attributes[key];
      if (!Number.isInteger(value) || value < 0 || value > 500)
        return `${label}必须是 0～500 的整数`;
    }
    for (const [name, value] of Object.entries(current.skills)) {
      if (!Number.isInteger(value) || value < 0 || value > 100)
        return `技能“${name}”必须是 0～100 的整数`;
    }
    const occupation = occupations.find(
      (item) => item.id === current.profile.occupationId,
    );
    const credit = current.skills["信用评级"] ?? 0;
    if (
      occupation &&
      (credit < occupation.creditRating.min ||
        credit > occupation.creditRating.max)
    )
      return `职业“${occupation.name}”的信用评级必须在 ${occupation.creditRating.min}～${occupation.creditRating.max} 之间`;
    const occupationSpent = allocatedSkillPoints(
      current.creation.occupationAllocations,
    );
    const interestSpent = allocatedSkillPoints(
      current.creation.interestAllocations,
    );
    if (occupation && occupationSpent > occupationSkillPoints(occupation, current))
      return "职业技能点超过了当前职业可用点数";
    if (interestSpent > current.attributes.int * 2)
      return "兴趣技能点超过了可用点数";
    if (
      current.status.hp.current < 0 ||
      current.status.hp.current > current.status.hp.max
    )
      return "当前生命必须在 0 和生命上限之间";
    if (
      current.status.mp.current < 0 ||
      current.status.mp.current > current.status.mp.max
    )
      return "当前魔法必须在 0 和魔法上限之间";
    if (
      current.status.san.current < 0 ||
      current.status.san.current > current.status.san.max
    )
      return "当前理智必须在 0 和理智上限之间";
    return "";
  }

  async function saveSheet() {
    if (!sheet || !character?.canEdit || saveState === "saving") return;
    const validationError = validateSheet(sheet);
    if (validationError) {
      setSaveError(validationError);
      setSaveState("error");
      return;
    }
    setSaveState("saving");
    setSaveError("");
    try {
      const updated = await api<Character>(`/characters/${id}`, {
        method: "PATCH",
        body: JSON.stringify({
          baseVersion: versionRef.current,
          sheet,
          message: editMessage.trim() || "手动保存人物卡",
        }),
      });
      versionRef.current = updated.currentVersion;
      sheetRef.current = updated.sheet;
      setCharacter(updated);
      setSheet(updated.sheet);
      setSaveState("saved");
      setEditMessage("");
    } catch (reason) {
      const conflict =
        reason instanceof APIError && reason.code === "version_conflict";
      setSaveState(conflict ? "conflict" : "error");
      setSaveError(
        reason instanceof APIError ? reason.message : "网络异常，人物卡尚未保存",
      );
      if (conflict) {
        try {
          setServerVersion((await api<Character>(`/characters/${id}`)).currentVersion);
        } catch {
          setServerVersion(null);
        }
      }
    }
  }

  async function deleteCurrentCharacter() {
    if (!character || account?.id !== character.ownerAccountId) return;
    const unsavedWarning = saveState === "dirty" ? "当前尚未保存的修改也会丢失。\n" : "";
    if (!window.confirm(`${unsavedWarning}确定删除人物卡“${character.name}”吗？`)) return;
    try {
      await api(`/characters/${id}`, { method: "DELETE" });
      navigate("/characters");
    } catch (reason) {
      setError(reason instanceof APIError ? reason.message : "删除人物卡失败");
    }
  }

  async function loadServerVersion() {
    if (!window.confirm("确定丢弃当前未保存内容并载入服务器最新版本吗？"))
      return;
    try {
      await load();
    } catch (reason) {
      setSaveError(
        reason instanceof APIError ? reason.message : "读取服务器版本失败",
      );
    }
  }

  function change(mutator: (next: CharacterSheet) => void) {
    if (!sheet || !character?.canEdit) return;
    const next = structuredClone(sheet);
    next.backstory ??= {};
    next.weapons ??= [];
    next.possessions ??= [];
    next.finances ??= { spendingLevel: "", cash: "", assets: "" };
    next.customSkills ??= [];
    next.skillSpecializations ??= {};
    next.creation.baseSkills ??= {};
    next.creation.occupationAllocations ??= {};
    next.creation.interestAllocations ??= {};
    next.creation.growthAllocations ??= {};
    mutator(next);
    sheetRef.current = next;
    setSheet(next);
    setSaveState("dirty");
  }

  const visibleSkills = useMemo(
    () =>
      skillDefinitions
        .filter(({ label }) => skillCategory === "全部技能" || skillCategoryMembers[skillCategory].has(label))
        .map(({ key, label }) => [key, label, sheet?.skills[key] ?? 0] as const),
    [sheet, skillCategory],
  );
  const selectedOccupation = useMemo(
    () => occupations.find((item) => item.id === selectedOccupationID),
    [occupations, selectedOccupationID],
  );
  const previewOccupation = useMemo(
    () =>
      occupations.find(
        (item) => item.id === (previewOccupationID || selectedOccupationID),
      ),
    [occupations, previewOccupationID, selectedOccupationID],
  );
  const filteredOccupations = useMemo(() => {
    const keyword = occupationSearch.trim().toLocaleLowerCase("zh-CN");
    if (!keyword) return occupations;
    return occupations.filter((item) =>
      `${item.name} ${item.description ?? ""}`
        .toLocaleLowerCase("zh-CN")
        .includes(keyword),
    );
  }, [occupations, occupationSearch]);
  const visibleVersions = useMemo(() => {
    const keyword = historySearch.trim().toLocaleLowerCase("zh-CN");
    return versions.filter((item) => {
      if (historyActor && item.actorName !== historyActor) return false;
      if (historyKind && item.changeKind !== historyKind) return false;
      if (!keyword) return true;
      const fields = item.changedPaths.map(changePathName).join(" ");
      return `${item.message ?? ""} ${fields}`
        .toLocaleLowerCase("zh-CN")
        .includes(keyword);
    });
  }, [versions, historyActor, historyKind, historySearch]);

  async function restore(version: number) {
    const reason = window.prompt(
      "请填写恢复原因（会记录在历史中）",
      "纠正误修改",
    );
    if (!reason?.trim()) return;
    if (!window.confirm(`确定恢复到版本 ${version} 吗？当前历史不会被删除。`))
      return;
    try {
      await api<Character>(`/characters/${id}/restore/${version}`, {
        method: "POST",
        body: JSON.stringify({
          baseVersion: versionRef.current,
          message: `恢复到版本 ${version}：${reason.trim()}`,
        }),
      });
      await load();
    } catch (reason) {
      setError(reason instanceof APIError ? reason.message : "恢复版本失败");
    }
  }

  async function viewVersion(version: number) {
    try {
      setHistoryDetail(
        await api<CharacterVersionDetail>(
          `/characters/${id}/versions/${version}`,
        ),
      );
    } catch (reason) {
      setError(
        reason instanceof APIError ? reason.message : "读取版本详情失败",
      );
    }
  }

  async function compareVersions() {
    if (!compareFrom || !compareTo || compareFrom === compareTo) return;
    try {
      setHistoryDetail(
        await api<CharacterVersionDetail>(
          `/characters/${id}/compare?from=${compareFrom}&to=${compareTo}`,
        ),
      );
    } catch (reason) {
      setError(reason instanceof APIError ? reason.message : "比较版本失败");
    }
  }

  function generateAttributes() {
    change((next) => {
      for (const key of ["str", "con", "dex", "app", "pow"])
        next.attributes[key] = rollLocal(3, 6) * 5;
      for (const key of ["siz", "int", "edu"])
        next.attributes[key] = (rollLocal(2, 6) + 6) * 5;
      next.status.luck = rollLocal(3, 6) * 5;
      next.skills["闪避"] = Math.floor(next.attributes.dex / 2);
      next.skills["母语"] = next.attributes.edu;
      next.creation.baseSkills["闪避"] = next.skills["闪避"];
      next.creation.baseSkills["母语"] = next.skills["母语"];
      delete next.creation.ageAdjustment;
      recalculateStatusLimits(next);
      next.status.hp.current = next.status.hp.max;
      next.status.mp.current = next.status.mp.max;
      next.status.san.current = next.attributes.pow;
      recalculateDerived(next);
    });
  }

  async function applyAgeAdjustment() {
    if (!sheet || saveState !== "saved") return;
    const reductions = Object.fromEntries(
      ageRequirements(sheet.profile.age).keys.map(([key]) => [
        key,
        ageReductions[key] ?? 0,
      ]),
    );
    try {
      const response = await api<{
        character: Character;
        result: AgeAdjustmentResult;
      }>(`/characters/${id}/age-adjustment`, {
        method: "POST",
        body: JSON.stringify({
          baseVersion: versionRef.current,
          reductions,
        }),
      });
      setCharacter(response.character);
      setSheet(response.character.sheet);
      sheetRef.current = response.character.sheet;
      versionRef.current = response.character.currentVersion;
      setAgeResult(response.result);
      setSaveState("saved");
      setVersions(
        (await api<{ items: CharacterVersion[] }>(`/characters/${id}/versions`))
          .items ?? [],
      );
    } catch (reason) {
      setError(
        reason instanceof APIError ? reason.message : "应用年龄修正失败",
      );
    }
  }

  async function rollSkill(skill: string) {
    setRollingSkill(skill);
    try {
      const roll = await api<DiceRoll<CheckResult>>("/rolls/check", {
        method: "POST",
        body: JSON.stringify({
          requestId: crypto.randomUUID(),
          characterId: id,
          skill,
          bonusPenalty,
          campaignId: campaignID || null,
          visibility: "public",
        }),
      });
      setLatestRoll(roll);
    } catch (reason) {
      setError(reason instanceof APIError ? reason.message : "技能检定失败");
    } finally {
      setRollingSkill("");
    }
  }

  async function pushCheck(
    original: DiceRoll<CheckResult>,
    destination: "skill" | "quick",
  ) {
    if (!window.confirm("进行孤注一掷吗？失败时通常会带来更严重的后果。"))
      return;
    try {
      const roll = await api<DiceRoll<CheckResult>>("/rolls/push", {
        method: "POST",
        body: JSON.stringify({
          requestId: crypto.randomUUID(),
          originalRollId: original.id,
        }),
      });
      if (destination === "skill") setLatestRoll(roll);
      else setQuickCheckRoll(roll);
    } catch (reason) {
      setError(reason instanceof APIError ? reason.message : "孤注一掷失败");
    }
  }

  async function rollAttribute(attribute: string) {
    try {
      const roll = await api<DiceRoll<CheckResult>>("/rolls/check", {
        method: "POST",
        body: JSON.stringify({
          requestId: crypto.randomUUID(),
          characterId: id,
          attribute,
          bonusPenalty,
          campaignId: campaignID || null,
          visibility: "public",
        }),
      });
      setQuickCheckRoll(roll);
    } catch (reason) {
      setError(reason instanceof APIError ? reason.message : "属性检定失败");
    }
  }

  async function rollExpression() {
    try {
      const roll = await api<DiceRoll<ExpressionResult>>("/rolls/expression", {
        method: "POST",
        body: JSON.stringify({
          requestId: crypto.randomUUID(),
          characterId: id,
          expression: diceExpression,
          label: "自由组合骰",
          campaignId: campaignID || null,
          visibility: "public",
        }),
      });
      setExpressionRoll(roll);
    } catch (reason) {
      setError(reason instanceof APIError ? reason.message : "组合骰投掷失败");
    }
  }

  async function rollSanityLoss() {
    if (!quickCheckRoll || quickCheckRoll.label !== "理智") return;
    const succeeded = ["critical", "extreme", "hard", "regular"].includes(
      quickCheckRoll.result.outcome,
    );
    const expression = succeeded ? sanSuccessLoss : sanFailureLoss;
    try {
      const roll = await api<DiceRoll<ExpressionResult>>("/rolls/expression", {
        method: "POST",
        body: JSON.stringify({
          requestId: crypto.randomUUID(),
          characterId: id,
          expression,
          label: `理智损失（${succeeded ? "成功" : "失败"}）`,
          campaignId: campaignID || null,
          visibility: "public",
        }),
      });
      if (roll.result.total < 0) {
        setError("理智损失不能为负数");
        return;
      }
      setSanLossRoll(roll);
      change((next) => {
        next.status.san.current = Math.max(
          0,
          next.status.san.current - roll.result.total,
        );
      });
    } catch (reason) {
      setError(
        reason instanceof APIError ? reason.message : "理智损失投骰失败",
      );
    }
  }

  function selectOccupation(occupationID: string) {
    setSelectedOccupationID(occupationID);
    const occupation = occupations.find((item) => item.id === occupationID);
    change((next) => {
      next.profile.occupationId = occupation?.id ?? "";
      next.profile.occupation = occupation?.name ?? "";
      next.creation.occupationSnapshot = occupation;
      next.creation.formulaIndex = 0;
      next.creation.occupationAllocations = {};
      next.creation.interestAllocations = {};
      next.creation.choiceSelections = [];
      next.creation.freeSkills = [];
      for (const skill of Object.keys(next.skills))
        next.skills[skill] =
          (next.creation.baseSkills[skill] ?? 0) +
          (next.creation.growthAllocations[skill] ?? 0);
    });
  }

  function setSkillAllocation(
    skill: string,
    kind: "occupationAllocations" | "interestAllocations" | "growthAllocations",
    value: number,
  ) {
    if (!sheet || !Number.isInteger(value) || value < 0) return;
    const base = sheet.creation.baseSkills[skill] ?? 0;
    const currentKindValue = sheet.creation[kind]?.[skill] ?? 0;
    const currentTotal =
      base +
      (sheet.creation.occupationAllocations?.[skill] ?? 0) +
      (sheet.creation.interestAllocations?.[skill] ?? 0) +
      (sheet.creation.growthAllocations?.[skill] ?? 0);
    const nextTotal = currentTotal - currentKindValue + value;
    if (nextTotal > 100) return;
    change((next) => {
      next.creation[kind][skill] = value;
      next.skills[skill] = nextTotal;
      if (skill === "克苏鲁神话") recalculateStatusLimits(next);
    });
  }

  function setSkillSpecialization(skill: string, specialization: string) {
    change((next) => {
      if (specialization) next.skillSpecializations[skill] = specialization;
      else delete next.skillSpecializations[skill];
    });
    setSpecializationSkill("");
  }
  function showSkillDescription(skill: string) {
    const baseName = skill.replace(/[①②③]$/, "");
    const specialization = sheet?.skillSpecializations?.[skill];
    const requestedName = specialization || baseName;
    const item = skillDescriptionByName.get(requestedName)
      ?? skillDescriptionByName.get(skillDescriptionAliases[requestedName])
      ?? skillDescriptionByName.get(skillDescriptionAliases[baseName])
      ?? skillDescriptionByName.get(baseName);
    if (item) setSkillDescription(item);
  }
  async function setLifecycleStatus(status: Character["status"]) {
    try {
      const updated = await api<Character>(`/characters/${id}/status`, {
        method: "PATCH",
        body: JSON.stringify({ status }),
      });
      setCharacter(updated);
      versionRef.current = updated.currentVersion;
      const history = await api<{ items: CharacterVersion[] }>(
        `/characters/${id}/versions`,
      );
      setVersions(history.items ?? []);
    } catch (reason) {
      setError(
        reason instanceof APIError ? reason.message : "修改人物状态失败",
      );
    }
  }
  async function copyCharacter() {
    const name = window.prompt(
      "新人物卡名称",
      `${character?.name ?? "人物"}（副本）`,
    );
    if (!name) return;
    try {
      const copied = await api<Character>(`/characters/${id}/copy`, {
        method: "POST",
        body: JSON.stringify({ name }),
      });
      navigate(`/characters/${copied.id}`);
    } catch (reason) {
      setError(reason instanceof APIError ? reason.message : "复制人物卡失败");
    }
  }
  if (error) return <p className="form-error">{error}</p>;
  if (!character || !sheet)
    return <div className="center-message">正在加载人物卡……</div>;
  const showLegacyPanels: boolean = false;

  return (
    <section className="page-stack character-editor">
      {!character.canEdit && (
        <p className="muted read-only-banner">
          只读查看：只有人物卡所有者和挂靠团本的 KP 可以修改。
        </p>
      )}
      <header className="character-header">
        <div>
          <Link to="/characters">← 返回档案</Link>
          <h1>{sheet.profile.name || "未命名调查员"}</h1>
        </div>
        <div className="character-lifecycle">
          {character.canEdit && <div className="page-save-actions">
            <button type="button" disabled={saveState === "saved" || saveState === "saving"} onClick={() => void saveSheet()}>
              {saveState === "saving" ? "正在保存…" : "保存人物卡"}
            </button>
            {account?.id === character.ownerAccountId && (
              <button className="danger-action" type="button" onClick={() => void deleteCurrentCharacter()}>删除人物卡</button>
            )}
          </div>}
        </div>
      </header>
      {(saveState === "error" || saveState === "conflict") && (
        <div className="save-problem" role="alert">
          <div>
            <strong>
              {saveState === "conflict"
                ? "人物卡发生版本冲突"
                : "人物卡保存失败"}
            </strong>
            <span>{saveError || "当前修改仍保留在此页面中。"}</span>
            {saveState === "conflict" && (
              <small>
                当前编辑基于 v{character.currentVersion}
                {serverVersion ? `，服务器已经是 v${serverVersion}` : ""}。
              </small>
            )}
          </div>
          {saveState === "error" ? (
            <button type="button" onClick={() => void saveSheet()}>
              检查并重试
            </button>
          ) : (
            <button type="button" onClick={() => void loadServerVersion()}>
              丢弃本地修改并载入新版
            </button>
          )}
        </div>
      )}
      <div className="character-overview">
        <section className="panel basic-info-panel">
          <h2>基础信息</h2>
          <div className="basic-info-grid">
            <label>
              姓名
              <input
                value={sheet.profile.name}
                onChange={(event) =>
                  change((next) => {
                    next.profile.name = event.target.value;
                  })
                }
              />
            </label>
            <label>
              职业
              <span className="occupation-input">
                <input
                  value={sheet.profile.occupation}
                  placeholder="点击右侧图标选择职业"
                  readOnly
                />
                <button
                  type="button"
                  aria-label="选择职业"
                  title="选择职业"
                  onClick={() => {
                    setPreviewOccupationID(selectedOccupationID);
                    setOccupationPickerOpen(true);
                  }}
                >
                  ⌕
                </button>
              </span>
            </label>
            <NumberField
              label="年龄"
              value={sheet.profile.age}
              disabled={Boolean(sheet.creation.ageAdjustment)}
              onChange={(value) =>
                change((next) => {
                  next.profile.age = value;
                  recalculateDerived(next);
                })
              }
            />
            <label>
              性别
              <input
                value={sheet.profile.gender ?? ""}
                onChange={(event) =>
                  change((next) => {
                    next.profile.gender = event.target.value;
                  })
                }
              />
            </label>
            <label>
              居住地
              <input
                value={sheet.profile.residence}
                onChange={(event) =>
                  change((next) => {
                    next.profile.residence = event.target.value;
                  })
                }
              />
            </label>
            <label>
              出生地
              <input
                value={sheet.profile.birthplace}
                onChange={(event) =>
                  change((next) => {
                    next.profile.birthplace = event.target.value;
                  })
                }
              />
            </label>
          </div>
        </section>
        <section className="panel core-attributes-panel">
          <div className="section-heading">
            <h2>属性</h2>
            <div className="attribute-actions">
              <button
                className="icon-button"
                type="button"
                disabled={!character.canEdit}
                onClick={generateAttributes}
                aria-label="随机生成属性"
                title="随机生成属性"
              >
                ↻
              </button>
              <button
                className="icon-button"
                type="button"
                onClick={() => setAttributeCheckOpen(true)}
                aria-label="进行属性检定"
                title="进行属性检定"
              >
                <span aria-hidden="true">◈</span>
              </button>
            </div>
          </div>
          <div className="core-attribute-grid">
            {attributes.map(([key, label]) => (
              <AttributeField
                key={key}
                label={label}
                value={sheet.attributes[key]}
                onChange={(value) =>
                  change((next) => {
                    next.attributes[key] = value;
                    recalculateStatusLimits(next);
                    recalculateDerived(next);
                  })
                }
              />
            ))}
            <NumberField
              label="幸运"
              value={sheet.status.luck}
              onChange={(value) =>
                change((next) => {
                  next.status.luck = value;
                })
              }
            />
          </div>
          <div className="derived-line">
            <span>移动力 <strong>{sheet.derived.move}</strong></span>
            <span>体格 <strong>{sheet.derived.build}</strong></span>
            <span>伤害加值 <strong>{sheet.derived.damageBonus}</strong></span>
          </div>
        </section>
      </div>
      {occupationPickerOpen && (
        <div
          className="modal-backdrop"
          role="presentation"
          onMouseDown={(event) => {
            if (event.target === event.currentTarget) setOccupationPickerOpen(false);
          }}
        >
          <section className="occupation-picker" role="dialog" aria-modal="true" aria-label="选择职业">
            <header className="section-heading">
              <div>
                <h2>选择职业</h2>
                <p className="muted">选择后仍需点击页面底部的总保存按钮。</p>
              </div>
              <button className="secondary" type="button" onClick={() => setOccupationPickerOpen(false)}>
                关闭
              </button>
            </header>
            <input
              type="search"
              autoFocus
              placeholder="搜索职业名称或说明"
              value={occupationSearch}
              onChange={(event) => setOccupationSearch(event.target.value)}
            />
            <div className="occupation-picker-body">
              <div className="occupation-picker-list">
                {filteredOccupations.map((item) => (
                  <button
                    className={item.id === previewOccupation?.id ? "occupation-option occupation-option--active" : "occupation-option"}
                    type="button"
                    key={item.id}
                    onClick={() => setPreviewOccupationID(item.id)}
                  >
                    <strong>{item.name}</strong>
                    <small>信用评级 {item.creditRating.min}～{item.creditRating.max}</small>
                  </button>
                ))}
                {filteredOccupations.length === 0 && <p className="muted">没有匹配的职业</p>}
              </div>
              <div className="occupation-detail">
                {previewOccupation ? (
                  <>
                    <div>
                      <p className="eyebrow">职业详情</p>
                      <h2>{previewOccupation.name}</h2>
                    </div>
                    <dl>
                      <div><dt>适用年代</dt><dd>{previewOccupation.eras.join("、")}</dd></div>
                      <div><dt>信用评级</dt><dd>{previewOccupation.creditRating.min}～{previewOccupation.creditRating.max}</dd></div>
                      <div><dt>职业点公式</dt><dd>{previewOccupation.skillPointFormulas.map((item) => item.label).join(" / ")}</dd></div>
                      <div><dt>固定技能</dt><dd>{previewOccupation.fixedSkills.join("、") || "无"}</dd></div>
                      <div><dt>任意技能</dt><dd>{previewOccupation.freeChoiceCount} 项</dd></div>
                    </dl>
                    {previewOccupation.choiceGroups.map((group, index) => (
                      <div className="occupation-choice-detail" key={index}>
                        <strong>{group.category || `可选技能组 ${index + 1}`}：选择 {group.count} 项</strong>
                        {group.skills?.length ? <span>{group.skills.join("、")}</span> : null}
                      </div>
                    ))}
                    {previewOccupation.description && <p className="occupation-description">{previewOccupation.description}</p>}
                    <button
                      type="button"
                      onClick={() => {
                        selectOccupation(previewOccupation.id);
                        setOccupationPickerOpen(false);
                      }}
                    >
                      选择这个职业
                    </button>
                  </>
                ) : (
                  <p className="muted">请从左侧选择一个职业查看详情。</p>
                )}
              </div>
            </div>
          </section>
        </div>
      )}
      {character.canEdit &&
        !sheet.creation.ageAdjustment &&
        !sheet.creation.occupationSnapshot && (
          <section className="panel age-adjustment">
            <div className="section-heading">
              <div>
                <h2>年龄修正</h2>
                <p className="muted">
                  应在选择职业前执行一次。需要自行分配的属性扣减合计必须为{" "}
                  {ageRequirements(sheet.profile.age).physical} 点。
                </p>
              </div>
              <button
                type="button"
                disabled={saveState !== "saved"}
                onClick={() => void applyAgeAdjustment()}
              >
                应用年龄修正
              </button>
            </div>
            <div className="attribute-grid">
              {ageRequirements(sheet.profile.age).keys.map(([key, label]) => (
                <NumberField
                  key={key}
                  label={`${label}扣减`}
                  value={ageReductions[key] ?? 0}
                  onChange={(value) =>
                    setAgeReductions((current) => ({
                      ...current,
                      [key]: Math.max(0, value),
                    }))
                  }
                />
              ))}
            </div>
            <p className="muted">
              当前已分配{" "}
              {ageRequirements(sheet.profile.age).keys.reduce(
                (sum, [key]) => sum + (ageReductions[key] ?? 0),
                0,
              )}{" "}
              / {ageRequirements(sheet.profile.age).physical}{" "}
              点；外貌、教育、幸运和教育成长检定由系统按年龄段处理。
            </p>
          </section>
        )}
      {(ageResult ?? sheet.creation.ageAdjustment) && (
        <section className="panel age-result">
          <h2>年龄修正结果</h2>
          <AgeResult result={(ageResult ?? sheet.creation.ageAdjustment)!} />
        </section>
      )}
      {attributeCheckOpen && (
        <div
          className="modal-backdrop"
          role="presentation"
          onMouseDown={(event) => {
            if (event.target === event.currentTarget) setAttributeCheckOpen(false);
          }}
        >
          <section className="attribute-check-dialog" role="dialog" aria-modal="true" aria-label="属性检定">
            <header className="section-heading">
              <h2>属性检定</h2>
              <button className="secondary" type="button" onClick={() => setAttributeCheckOpen(false)}>关闭</button>
            </header>
            <div className="attribute-check-options">
          {[...attributes, ["luck", "幸运"], ["san", "理智"]].map(
            ([key, label]) => (
              <button
                type="button"
                key={key}
                onClick={() => void rollAttribute(key)}
              >
                {label}
              </button>
            ),
          )}
            </div>
        {quickCheckRoll && (
          <div
            className={`roll-result outcome-${quickCheckRoll.result.outcome}`}
          >
            <span>{quickCheckRoll.label}检定</span>
            <strong>{quickCheckRoll.result.value}</strong>
            <span>
              / {quickCheckRoll.result.target} ·{" "}
              {outcomeName(quickCheckRoll.result.outcome)}
            </span>
            {quickCheckRoll.result.candidates.length > 1 && (
              <small>候选：{quickCheckRoll.result.candidates.join("、")}</small>
            )}
            {quickCheckRoll.result.outcome === "failure" &&
              !quickCheckRoll.rerollOfId &&
              quickCheckRoll.label !== "幸运" &&
              quickCheckRoll.label !== "理智" && (
                <button
                  type="button"
                  onClick={() => void pushCheck(quickCheckRoll, "quick")}
                >
                  孤注一掷
                </button>
              )}
          </div>
        )}
        {quickCheckRoll?.label === "理智" && character.canEdit && (
          <div className="san-loss-tools">
            <label>
              成功损失
              <input
                value={sanSuccessLoss}
                onChange={(event) => setSanSuccessLoss(event.target.value)}
                placeholder="例如 0"
              />
            </label>
            <label>
              失败损失
              <input
                value={sanFailureLoss}
                onChange={(event) => setSanFailureLoss(event.target.value)}
                placeholder="例如 1d6"
              />
            </label>
            <button type="button" onClick={() => void rollSanityLoss()}>
              投掷并扣除理智
            </button>
            {sanLossRoll && (
              <strong>本次损失 {sanLossRoll.result.total} 点</strong>
            )}
          </div>
        )}
          </section>
        </div>
      )}
      <section className="panel">
        <h2>当前状态</h2>
        <div className="status-resource-grid">
          {(["hp", "mp", "san"] as const).map((key) => (
            <div className="status-resource" key={key}>
              <strong>{statusResourceNames[key]}</strong>
              <label>
                当前值
                <input
                  type="number"
                  value={sheet.status[key].current}
                  onChange={(event) =>
                    change((next) => {
                      next.status[key].current = Number(event.target.value);
                    })
                  }
                />
              </label>
              <label>
                最大值
                <input
                  type="number"
                  value={sheet.status[key].max}
                  readOnly
                  aria-label={`${statusResourceNames[key]}最大值（自动计算）`}
                />
              </label>
            </div>
          ))}
        </div>
        <div className="status-conditions">
          {statusConditionFields.map(([key, label]) => (
            <label key={key}>
              <input
                type="checkbox"
                checked={sheet.status[key]}
                onChange={(event) =>
                  change((next) => {
                    next.status[key] = event.target.checked;
                  })
                }
              />
              {label}
            </label>
          ))}
        </div>
      </section>
      <section className="panel">
        <div className="section-heading">
          <h2>技能</h2>
          <div className="skill-tools">
            <select
              aria-label="奖励骰或惩罚骰"
              value={bonusPenalty}
              onChange={(event) => setBonusPenalty(Number(event.target.value))}
            >
              <option value={2}>2 个奖励骰</option>
              <option value={1}>1 个奖励骰</option>
              <option value={0}>普通检定</option>
              <option value={-1}>1 个惩罚骰</option>
              <option value={-2}>2 个惩罚骰</option>
            </select>
          </div>
        </div>
        {latestRoll && (
          <div className={`roll-result outcome-${latestRoll.result.outcome}`}>
            <span>{latestRoll.label}检定</span>
            <strong>{latestRoll.result.value}</strong>
            <span>
              / {latestRoll.result.target} ·{" "}
              {outcomeName(latestRoll.result.outcome)}
            </span>
            {latestRoll.result.candidates.length > 1 && (
              <small>候选：{latestRoll.result.candidates.join("、")}</small>
            )}
            {latestRoll.result.outcome === "failure" &&
              !latestRoll.rerollOfId && (
                <button
                  type="button"
                  onClick={() => void pushCheck(latestRoll, "skill")}
                >
                  孤注一掷
                </button>
              )}
          </div>
        )}
        {selectedOccupation && (
          <div className="occupation-skill-guide">
            <strong>{selectedOccupation.name}</strong>
            <span>信用评级 {selectedOccupation.creditRating.min}～{selectedOccupation.creditRating.max}</span>
            <div className="occupation-point-totals">
              <span>
                职业点数 <strong>{occupationSkillPoints(selectedOccupation, sheet) - allocatedSkillPoints(sheet.creation.occupationAllocations)}</strong>
              </span>
              <span>
                兴趣点数 <strong>{sheet.attributes.int * 2 - allocatedSkillPoints(sheet.creation.interestAllocations)}</strong>
              </span>
            </div>
            <span>{selectedOccupation.description || "暂无职业介绍"}</span>
          </div>
        )}
        <nav className="skill-category-tabs" aria-label="技能分类">
          {skillCategories.map((category) => (
            <button
              className={skillCategory === category ? "skill-category-tab skill-category-tab--active" : "skill-category-tab"}
              type="button"
              key={category}
              onClick={() => setSkillCategory(category)}
            >
              {category}
            </button>
          ))}
        </nav>
        <div className="skill-table-wrap">
          <div className="skill-table-head">
            <span>技能</span><span>基础</span><span>职业</span><span>兴趣</span><span>成长</span><span>成功</span><span>困难</span><span>极难</span><span>检定</span>
          </div>
          {visibleSkills.map(([name, displayName, value]) => {
            const base = sheet.creation.baseSkills?.[name] ?? 0;
            return (
              <div className="skill-table-row" key={name}>
                <span className="skill-table-name">
                  <button
                    className="skill-description-button"
                    type="button"
                    title={`查看${displayName}技能说明`}
                    onClick={() => showSkillDescription(name)}
                  >
                    {displayName}
                  </button>
                  {skillSpecializationOptions[name] && (
                    <button
                      className="skill-specialization-button"
                      type="button"
                      onClick={() => setSpecializationSkill(name)}
                      title={`选择${name}分支`}
                    >
                      {sheet.skillSpecializations?.[name] || "选择类型"}
                    </button>
                  )}
                </span>
                <output>{base}</output>
                {(["occupationAllocations", "interestAllocations", "growthAllocations"] as const).map((kind) => (
                  <input
                    key={kind}
                    type="number"
                    min="0"
                    max="100"
                    aria-label={`${name}${skillAllocationNames[kind]}`}
                    value={sheet.creation[kind]?.[name] ?? 0}
                    disabled={!character.canEdit}
                    onFocus={(event) => event.currentTarget.select()}
                    onChange={(event) => setSkillAllocation(name, kind, Number(event.target.value))}
                  />
                ))}
                <output>{value}</output>
                <output>{Math.floor(value / 2)}</output>
                <output>{Math.floor(value / 5)}</output>
                <button
                  className="skill-roll-button"
                  type="button"
                  aria-label={`投掷${name}技能检定`}
                  title={`投掷${name}技能检定`}
                  disabled={rollingSkill !== ""}
                  onClick={() => void rollSkill(name)}
                >
                  {rollingSkill === name ? "…" : <span aria-hidden="true">◈</span>}
                </button>
              </div>
            );
          })}
        </div>
      </section>
      <CharacterSupplement
        character={character}
        sheet={sheet}
        change={change}
        campaignID={campaignID}
        section="combat"
      />
      <CharacterSupplement
        character={character}
        sheet={sheet}
        change={change}
        campaignID={campaignID}
        section="assets"
      />
      {specializationSkill && (
        <div
          className="modal-backdrop"
          role="presentation"
          onMouseDown={(event) => {
            if (event.target === event.currentTarget) setSpecializationSkill("");
          }}
        >
          <section className="specialization-dialog" role="dialog" aria-modal="true" aria-label={`选择${specializationSkill}分支`}>
            <header className="section-heading">
              <div>
                <p className="eyebrow">技能分支</p>
                <h2>{specializationSkill}</h2>
              </div>
              <button className="secondary" type="button" onClick={() => setSpecializationSkill("")}>关闭</button>
            </header>
            <div className="specialization-options">
              {skillSpecializationOptions[specializationSkill].map((option) => (
                <button
                  className={sheet.skillSpecializations?.[specializationSkill] === option ? "secondary specialization-option--active" : "secondary"}
                  type="button"
                  key={option}
                  onClick={() => setSkillSpecialization(specializationSkill, option)}
                >
                  {option}
                </button>
              ))}
            </div>
            {sheet.skillSpecializations?.[specializationSkill] && (
              <button className="secondary" type="button" onClick={() => setSkillSpecialization(specializationSkill, "")}>清除分支</button>
            )}
          </section>
        </div>
      )}
      {skillDescription && (
        <div className="modal-backdrop" role="presentation" onMouseDown={(event) => {
          if (event.target === event.currentTarget) setSkillDescription(null);
        }}>
          <section className="skill-description-dialog" role="dialog" aria-modal="true" aria-label={`${skillDescription.name}技能说明`}>
            <header className="section-heading">
              <div><p className="eyebrow">技能说明</p><h2>{skillDescription.name}</h2></div>
              <button className="secondary" type="button" onClick={() => setSkillDescription(null)}>关闭</button>
            </header>
            <dl className="skill-description-meta">
              <div><dt>基础成功率</dt><dd>{skillDescription.base || "—"}</dd></div>
              <div><dt>特殊适用</dt><dd>{skillDescription.applicability || "—"}</dd></div>
            </dl>
            {skillDescriptionSections.map(([key, title]) => {
              const content = skillDescription[key];
              return content && content !== "——" ? (
                <section className="skill-description-section" key={key}>
                  <h3>{title}</h3><p>{content}</p>
                </section>
              ) : null;
            })}
          </section>
        </div>
      )}
      <CharacterSupplement
        character={character}
        sheet={sheet}
        change={change}
        campaignID={campaignID}
        section="details"
      />
      <section className="panel">
        <h2>备注</h2>
        <textarea
          rows={8}
          value={sheet.notes}
          disabled={!character.canEdit}
          onChange={(event) => change((next) => {
            next.notes = event.target.value;
          })}
        />
      </section>
      {showLegacyPanels && account?.id === character.ownerAccountId && (
        <section className="panel character-management">
          <div>
            <h2>人物卡管理</h2>
            <p className="muted">这些操作属于网站管理，不影响人物卡规则数据。</p>
          </div>
          <label>
            档案状态
            <select
              aria-label="人物状态"
              value={character.status}
              disabled={saveState !== "saved"}
              onChange={(event) =>
                void setLifecycleStatus(
                  event.target.value as Character["status"],
                )
              }
            >
              {Object.entries(characterStatusNames).map(([value, label]) => (
                <option value={value} key={value}>
                  {label}
                </option>
              ))}
            </select>
          </label>
          <button
            type="button"
            disabled={saveState !== "saved"}
            onClick={() => void copyCharacter()}
          >
            复制人物卡
          </button>
        </section>
      )}
      {showLegacyPanels && <section className="panel history-panel">
        <div className="section-heading">
          <h2>修改历史</h2>
          <div className="version-compare">
            <select
              aria-label="比较起始版本"
              value={compareFrom}
              onChange={(event) => setCompareFrom(event.target.value)}
            >
              <option value="">起始版本</option>
              {versions.map((item) => (
                <option value={item.version} key={item.version}>
                  版本 {item.version}
                </option>
              ))}
            </select>
            <span>→</span>
            <select
              aria-label="比较目标版本"
              value={compareTo}
              onChange={(event) => setCompareTo(event.target.value)}
            >
              <option value="">目标版本</option>
              {versions.map((item) => (
                <option value={item.version} key={item.version}>
                  版本 {item.version}
                </option>
              ))}
            </select>
            <button
              type="button"
              disabled={!compareFrom || !compareTo || compareFrom === compareTo}
              onClick={() => void compareVersions()}
            >
              比较
            </button>
          </div>
        </div>
        <div className="history-filters">
          <select
            aria-label="按修改人筛选历史"
            value={historyActor}
            onChange={(event) => setHistoryActor(event.target.value)}
          >
            <option value="">全部修改人</option>
            {[...new Set(versions.map((item) => item.actorName))].map(
              (actor) => (
                <option value={actor} key={actor}>
                  {actor}
                </option>
              ),
            )}
          </select>
          <select
            aria-label="按变更类型筛选历史"
            value={historyKind}
            onChange={(event) => setHistoryKind(event.target.value)}
          >
            <option value="">全部类型</option>
            <option value="edit">编辑</option>
            <option value="generation">生成与创建</option>
            <option value="restore">历史恢复</option>
            <option value="import">导入</option>
            <option value="system">系统修改</option>
          </select>
          <input
            type="search"
            aria-label="搜索历史说明或字段"
            value={historySearch}
            placeholder="搜索说明或修改字段"
            onChange={(event) => setHistorySearch(event.target.value)}
          />
          <span className="muted">
            {visibleVersions.length} / {versions.length} 条
          </span>
        </div>
        {historyDetail && (
          <div className="version-detail">
            <div className="section-heading">
              <h3>
                {historyDetail.fromVersion
                  ? `版本 ${historyDetail.fromVersion} → ${historyDetail.toVersion}`
                  : `版本 ${historyDetail.toVersion} 创建快照`}
              </h3>
              <button
                className="text-button"
                type="button"
                onClick={() => setHistoryDetail(null)}
              >
                关闭
              </button>
            </div>
            {historyDetail.changes.length === 0 ? (
              <p className="muted">两个版本没有差异。</p>
            ) : (
              <div className="change-table">
                {historyDetail.changes.map((change, index) => (
                  <div className="change-row" key={`${change.path}-${index}`}>
                    <strong>{changePathName(change.path)}</strong>
                    <span className="before-value">
                      {formatChangeValue(change.before)}
                    </span>
                    <span>→</span>
                    <span className="after-value">
                      {formatChangeValue(change.after)}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}
        <div className="simple-list">
          {visibleVersions.map((item) => (
            <div className="simple-row" key={item.version}>
              <span>
                <strong>
                  版本 {item.version} · {item.actorName}
                </strong>
                <small>
                  {item.message || item.changeKind} ·{" "}
                  {new Date(item.createdAt).toLocaleString()}
                </small>
              </span>
              <button
                className="secondary"
                type="button"
                onClick={() => void viewVersion(item.version)}
              >
                详情
              </button>
              {character.canEdit &&
                item.version !== character.currentVersion && (
                  <button
                    className="secondary"
                    type="button"
                    disabled={saveState !== "saved"}
                    onClick={() => void restore(item.version)}
                  >
                    恢复
                  </button>
                )}
            </div>
          ))}
          {visibleVersions.length === 0 && (
            <p className="muted">没有符合筛选条件的历史记录。</p>
          )}
        </div>
      </section>}
    </section>
  );
}

const outcomeNames: Record<CheckResult["outcome"], string> = {
  critical: "大成功",
  extreme: "极难成功",
  hard: "困难成功",
  regular: "成功",
  failure: "失败",
  fumble: "大失败",
};

function outcomeName(outcome: CheckResult["outcome"]) {
  return outcomeNames[outcome];
}

function formatTerm(term: ExpressionResult["terms"][number]) {
  const sign = term.sign < 0 ? "−" : "+";
  const detail = term.rolls ? `[${term.rolls.join(", ")}]` : String(term.value);
  return `${sign} ${term.expression} ${detail}`;
}

const historyPathNames: Record<string, string> = {
  "/profile/name": "姓名",
  "/profile/occupation": "职业",
  "/profile/age": "年龄",
  "/profile/residence": "居住地",
  "/profile/birthplace": "出生地",
  "/status/hp/current": "当前生命",
  "/status/hp/max": "生命上限",
  "/status/mp/current": "当前魔法",
  "/status/mp/max": "魔法上限",
  "/status/san/current": "当前理智",
  "/status/san/max": "理智上限",
  "/status/luck": "幸运",
  "/status/majorWound": "重伤",
  "/status/dying": "濒死",
  "/status/unconscious": "昏迷",
  "/status/temporaryInsanity": "临时疯狂",
  "/status/indefiniteInsanity": "不定性疯狂",
  "/status/permanentInsanity": "永久疯狂",
  "/notes": "备注",
};
const historyAttributeNames: Record<string, string> = {
  str: "力量",
  con: "体质",
  siz: "体型",
  dex: "敏捷",
  app: "外貌",
  int: "智力",
  pow: "意志",
  edu: "教育",
};
const historyBackstoryNames: Record<string, string> = {
  description: "外貌描述",
  ideology: "思想与信念",
  significantPeople: "重要之人",
  meaningfulLocations: "意义非凡之地",
  treasuredPossessions: "宝贵之物",
  traits: "特质",
  injuriesScars: "伤口与疤痕",
  phobiasManias: "恐惧与躁狂症",
  tomesSpellsArtifacts: "典籍、法术与神话物品",
  strangeEncounters: "怪异生物与遭遇",
};

function changePathName(path: string) {
  if (historyPathNames[path]) return historyPathNames[path];
  const parts = path.split("/").slice(1).map(decodePointer);
  if (parts[0] === "attributes")
    return `属性 · ${historyAttributeNames[parts[1]] ?? parts[1]}`;
  if (parts[0] === "skills") return `技能 · ${parts[1]}`;
  if (parts[0] === "skillSpecializations") return `技能分支 · ${parts[1]}`;
  if (parts[0] === "backstory")
    return `背景 · ${historyBackstoryNames[parts[1]] ?? parts[1]}`;
  if (parts[0] === "weapons")
    return `武器 ${Number(parts[1]) + 1}${parts[2] ? ` · ${parts[2]}` : ""}`;
  if (parts[0] === "possessions")
    return `物品 ${Number(parts[1]) + 1}${parts[2] ? ` · ${parts[2]}` : ""}`;
  if (parts[0] === "finances") return `财产 · ${parts[1]}`;
  return path || "人物卡";
}
function decodePointer(value: string) {
  return value.replaceAll("~1", "/").replaceAll("~0", "~");
}
function formatChangeValue(value: unknown) {
  if (value === null || value === undefined || value === "")
    return "（未设置）";
  if (typeof value === "boolean") return value ? "是" : "否";
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}

function NumberField({
  label,
  value,
  onChange,
  disabled = false,
}: {
  label: string;
  value: number;
  onChange: (value: number) => void;
  disabled?: boolean;
}) {
  return (
    <label>
      {label}
      <input
        type="number"
        value={value}
        disabled={disabled}
        onChange={(event) => onChange(Number(event.target.value))}
      />
    </label>
  );
}

function AttributeField({
  label,
  value,
  onChange,
}: {
  label: string;
  value: number;
  onChange: (value: number) => void;
}) {
  return (
    <label className="attribute-field">
      <span>{label}</span>
      <span className="attribute-values">
        <input
          type="number"
          aria-label={`${label}本值`}
          title="本值"
          value={value}
          onChange={(event) => onChange(Number(event.target.value))}
        />
        <input
          type="number"
          aria-label={`${label}困难值`}
          title="困难值（二分之一）"
          value={Math.floor(value / 2)}
          readOnly
        />
        <input
          type="number"
          aria-label={`${label}极难值`}
          title="极难值（五分之一）"
          value={Math.floor(value / 5)}
          readOnly
        />
      </span>
    </label>
  );
}

function recalculateStatusLimits(sheet: CharacterSheet) {
  sheet.status.hp.max = Math.floor(
    (sheet.attributes.con + sheet.attributes.siz) / 10,
  );
  sheet.status.mp.max = Math.floor(sheet.attributes.pow / 5);
  sheet.status.san.max = Math.max(0, 99 - (sheet.skills["克苏鲁神话"] ?? 0));
}

function occupationSkillPoints(
  occupation: Occupation,
  sheet: CharacterSheet,
) {
  const formula = occupation.skillPointFormulas[0];
  if (!formula) return 0;
  return formula.terms.reduce(
    (total, term) =>
      total + (sheet.attributes[term.attribute] ?? 0) * term.multiplier,
    0,
  );
}

function allocatedSkillPoints(values: Record<string, number> | undefined) {
  return Object.values(values ?? {}).reduce(
    (total, value) => total + (Number.isFinite(value) ? value : 0),
    0,
  );
}

function recalculateDerived(sheet: CharacterSheet) {
  const { str, siz, dex } = sheet.attributes;
  let move = str < siz && dex < siz ? 7 : str > siz && dex > siz ? 9 : 8;
  if (sheet.profile.age >= 40)
    move -= Math.min(5, Math.floor((sheet.profile.age - 30) / 10));
  sheet.derived.move = Math.max(1, move);
  const total = str + siz;
  if (total <= 64) [sheet.derived.build, sheet.derived.damageBonus] = [-2, "-2"];
  else if (total <= 84) [sheet.derived.build, sheet.derived.damageBonus] = [-1, "-1"];
  else if (total <= 124) [sheet.derived.build, sheet.derived.damageBonus] = [0, "0"];
  else if (total <= 164) [sheet.derived.build, sheet.derived.damageBonus] = [1, "+1d4"];
  else if (total <= 204) [sheet.derived.build, sheet.derived.damageBonus] = [2, "+1d6"];
  else {
    const dice = 2 + Math.floor((total - 205) / 80);
    sheet.derived.build = dice + 1;
    sheet.derived.damageBonus = `+${dice}d6`;
  }
}

function rollLocal(count: number, sides: number) {
  const values = new Uint32Array(count);
  crypto.getRandomValues(values);
  return Array.from(values).reduce((sum, value) => sum + (value % sides) + 1, 0);
}

function ageRequirements(age: number) {
  if (age < 20)
    return {
      physical: 5,
      keys: [
        ["str", "力量"],
        ["siz", "体型"],
      ] as [string, string][],
    };
  if (age < 40) return { physical: 0, keys: [] as [string, string][] };
  const physical =
    age < 50 ? 5 : age < 60 ? 10 : age < 70 ? 20 : age < 80 ? 40 : 80;
  return {
    physical,
    keys: [
      ["str", "力量"],
      ["con", "体质"],
      ["dex", "敏捷"],
    ] as [string, string][],
  };
}

function AgeResult({ result }: { result: AgeAdjustmentResult }) {
  const reductions = Object.entries(result.physicalReductions)
    .filter(([, value]) => value > 0)
    .map(
      ([key, value]) =>
        `${attributes.find(([item]) => item === key)?.[1] ?? key} −${value}`,
    );
  return (
    <div className="age-result-list">
      <span>{reductions.join("、") || "无体能属性扣减"}</span>
      {result.appearanceReduction > 0 && (
        <span>外貌 −{result.appearanceReduction}</span>
      )}
      {result.educationReduction > 0 && (
        <span>教育 −{result.educationReduction}</span>
      )}
      {result.luckReroll > 0 && <span>额外幸运结果：{result.luckReroll}</span>}
      {result.educationChecks.map((item, index) => (
        <span key={index}>
          教育成长 {index + 1}：D100={item.roll}
          {item.increase > 0 ? `，增加 ${item.increase}` : "，未增加"}
        </span>
      ))}
    </div>
  );
}

const saveLabel = {
  saved: "已保存",
  dirty: "等待保存",
  saving: "保存中",
  error: "保存失败",
  conflict: "版本冲突，请刷新",
};
const characterStatusNames: Record<Character["status"], string> = {
  draft: "草稿",
  active: "启用",
  retired: "退役",
  deceased: "死亡",
  archived: "归档",
};
