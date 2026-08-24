export type Account = {
  id: string;
  username: string;
  displayName: string;
  role: "admin" | "user";
  status: "active" | "disabled";
  mustChangePassword: boolean;
  createdAt: string;
  lastLoginAt: string | null;
};

export class APIError extends Error {
  constructor(
    public readonly code: string,
    message: string,
    public readonly status: number,
  ) {
    super(message);
  }
}

export type CharacterSummary = {
  id: string;
  kind: "investigator" | "npc";
  status: "draft" | "active" | "retired" | "deceased" | "archived";
  name: string;
  occupation: string;
  currentVersion: number;
  updatedAt: string;
};
export type Campaign = {
  id: string;
  keeperAccountId: string;
  keeperName: string;
  title: string;
  summary: string;
  status: "preparing" | "active" | "finished" | "archived";
  coverAssetId: string | null;
  createdAt: string;
  updatedAt: string;
  canManage: boolean;
};
export type CampaignBlock = {
  id: string;
  campaignId: string;
  type: "heading" | "text" | "clue" | "image";
  title: string;
  content: string;
  visibility: "public" | "keeper";
  position: number;
  assetId: string | null;
  publishedAt: string | null;
  createdAt: string;
  updatedAt: string;
};
export type CampaignAsset = {
  id: string;
  campaignId: string;
  originalName: string;
  mimeType: string;
  byteSize: number;
  width: number | null;
  height: number | null;
  createdAt: string;
};
export type CampaignCharacter = {
  characterId: string;
  name: string;
  kind: "investigator" | "npc";
  ownerAccountId: string;
  ownerName: string;
  role: "investigator" | "npc";
  visibility: "public" | "hidden";
  joinedAt: string;
};
export type CharacterSheet = {
  schemaVersion: number;
  profile: {
    name: string;
    occupation: string;
    occupationId: string;
    age: number;
    gender: string;
    residence: string;
    birthplace: string;
  };
  attributes: Record<string, number>;
  status: {
    hp: { current: number; max: number };
    mp: { current: number; max: number };
    san: { current: number; max: number };
    luck: number;
    majorWound: boolean;
    dying: boolean;
    unconscious: boolean;
    temporaryInsanity: boolean;
    indefiniteInsanity: boolean;
    permanentInsanity: boolean;
  };
  derived: { move: number; build: number; damageBonus: string };
  skills: Record<string, number>;
  skillSpecializations: Record<string, string>;
  customSkills: string[];
  backstory: Record<string, string>;
  possessions: { id: string; name: string; quantity: number; notes: string }[];
  weapons: {
    id: string;
    name: string;
    skill: string;
    damage: string;
    range: string;
    attacks: number;
    ammo: number;
    malfunction: number;
    attacksText?: string;
    ammoText?: string;
    malfunctionText?: string;
    penetration?: string;
    era?: string;
    price?: string;
    invention?: string;
    category?: string;
    notes: string;
  }[];
  finances: { spendingLevel: string; cash: string; assets: string };
  notes: string;
  creation: {
    occupationSnapshot?: Occupation;
    formulaIndex: number;
    occupationPoints: number;
    interestPoints: number;
    occupationAllocations: Record<string, number>;
    interestAllocations: Record<string, number>;
    growthAllocations: Record<string, number>;
    baseSkills: Record<string, number>;
    choiceSelections: string[][];
    freeSkills: string[];
    ageAdjustment?: AgeAdjustmentResult;
  };
};
export type AgeAdjustmentResult = {
  age: number;
  physicalReductions: Record<string, number>;
  appearanceReduction: number;
  educationReduction: number;
  luckReroll: number;
  educationChecks: { roll: number; increase: number }[];
};
export type Character = CharacterSummary & {
  ownerAccountId: string;
  ruleset: "coc7";
  sheet: CharacterSheet;
  createdAt: string;
  canEdit: boolean;
};
export type CharacterVersion = {
  version: number;
  parentVersion: number | null;
  actorName: string;
  changeKind: string;
  message: string | null;
  changedPaths: string[];
  createdAt: string;
};
export type CharacterVersionDetail = {
  fromVersion: number | null;
  toVersion: number;
  snapshot: CharacterSheet;
  changes: { path: string; before: unknown; after: unknown }[];
};
export type SkillGrowthResult = {
  items: {
    skill: string;
    roll: number;
    succeeded: boolean;
    increase: number;
    before: number;
    after: number;
  }[];
};
export type Occupation = {
  id: string;
  name: string;
  eras: string[];
  source: "official" | "custom";
  creditRating: { min: number; max: number };
  skillPointFormulas: {
    label: string;
    terms: { attribute: string; multiplier: number }[];
  }[];
  fixedSkills: string[];
  choiceGroups: { count: number; skills?: string[]; category?: string }[];
  freeChoiceCount: number;
  description?: string;
};
export type CheckResult = {
  target: number;
  bonusPenalty: number;
  units: number;
  tens: number[];
  candidates: number[];
  value: number;
  outcome: "critical" | "extreme" | "hard" | "regular" | "failure" | "fumble";
  hard: number;
  extreme: number;
};
export type ExpressionResult = {
  terms: {
    sign: number;
    expression: string;
    rolls?: number[];
    value: number;
  }[];
  total: number;
};
export type DiceRoll<T> = {
  id: string;
  requestId: string;
  actorAccountId: string;
  characterId: string | null;
  characterName?: string;
  campaignId: string | null;
  campaignTitle?: string;
  actorName?: string;
  visibility: "public" | "keeper" | "test";
  kind: "check" | "expression";
  label: string;
  expression: string;
  result: T;
  rerollOfId: string | null;
  rerollKind: "push" | "reroll" | null;
  createdAt: string;
};

export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`/api/v1${path}`, {
    credentials: "same-origin",
    ...init,
    headers: { "Content-Type": "application/json", ...init?.headers },
  });
  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as {
      code?: string;
      message?: string;
    } | null;
    throw new APIError(
      body?.code ?? "request_failed",
      body?.message ?? "请求失败",
      response.status,
    );
  }
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}
