export const MINUTE = 60_000;
export const HOUR = MINUTE * 60;
export const DAY = HOUR * 24;
const JST = HOUR * 9;

export function dayStart(time: number) {
  return Math.floor((time + JST) / DAY) * DAY - JST;
}

export function daysIn(start: number, end: number) {
  const days: number[] = [];
  for (let day = dayStart(start); day < end; day += DAY) days.push(day);
  return days;
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
export type GroupTotal = { name: string; ms: number };
export type BlockResult = { blocks: Block[]; days: DayTotal[] };
export type Summary = Metrics & {
  totalMs: number;
  agents: GroupTotal[];
  projects: GroupTotal[];
  days: DayTotal[];
};
export type FilterOptions = { agents: string[]; projects: string[] };
