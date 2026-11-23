export const MINUTE = 60_000;
export const HOUR = MINUTE * 60;
export const DAY = HOUR * 24;
const JST = HOUR * 9;

export function dayStart(time: number) {
  return Math.floor((time + JST) / DAY) * DAY - JST;
}

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
export type BlockResult = { blocks: Block[]; days: DayTotal[] };
export type Summary = Metrics &
  Cost & {
    totalMs: number;
    agents: GroupTotal[];
    projects: GroupTotal[];
    days: DayTotal[];
  };
export type FilterOptions = { agents: string[]; projects: string[] };
