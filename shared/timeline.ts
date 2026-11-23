export const MINUTE = 60_000;
export const HOUR = MINUTE * 60;
export const DAY = HOUR * 24;
const JST = HOUR * 9;

export function dayStart(time: number) {
  return Math.floor((time + JST) / DAY) * DAY - JST;
}

export function dayKey(time: number) {
  return new Date(time + JST).toISOString().slice(0, 10);
}

export type UsageDay = {
  startDate: string;
  tokens: number;
};

type AccountUsage = {
  days: UsageDay[];
  summary?: {
    currentStreakDays: number;
    lifetimeTokens: number;
    longestRunningTurnSec: number;
    longestStreakDays: number;
    peakDailyTokens: number;
  };
};

export type Metrics = {
  inputTokens?: number;
  cachedInputTokens?: number;
  outputTokens?: number;
  reasoningTokens?: number;
  turnCount?: number;
  toolCallCount?: number;
};

export type Block = Metrics & {
  estimatedCostUsd?: number;
  reasoningEffort?: string;
  gitSha?: string;
  gitOriginUrl?: string;
  preview?: string;
  provenance?: "rollout" | "turns" | "metadata" | "t3";
  id: number;
  agent: string;
  title: string;
  project: string;
  cwd: string;
  model: string;
  branch: string;
  tokens: number | null;
  start: number;
  end: number;
};

export type DayTotal = { day: number; ms: number };
type Cost = {
  estimatedCostUsd?: number;
  unpricedSessions?: number;
};

export type GroupTotal = Cost & { name: string; ms: number };
export type BlockResult = { blocks: Block[]; days: DayTotal[]; accountUsage: AccountUsage };
export type Summary = Metrics &
  Cost & {
    accountUsage: AccountUsage;
    totalMs: number;
    sessionCount: number;
    agents: GroupTotal[];
    projects: GroupTotal[];
    days: DayTotal[];
  };
export type FilterOptions = { agents: string[]; projects: string[] };

type PeriodTotal = Cost & {
  start: number;
  ms: number;
  tokens?: number;
  accountTokens?: number;
};
export type DashboardSummary = Summary & {
  history: { start: number; months: PeriodTotal[]; years: PeriodTotal[] };
};
