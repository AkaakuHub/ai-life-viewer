import { Alert, Box, Card, Paper, Progress, SimpleGrid, Stack, Text, Title } from "@mantine/core";
import type { DashboardSummary } from "../../shared/timeline.ts";
import { Breakdown } from "./Breakdown.tsx";
import { hours, monthName, usd } from "./format.ts";

const number = new Intl.NumberFormat("ja-JP");
const count = (value?: number) => (value === undefined ? "—" : number.format(value));

export function Dashboard({
  summary,
  failed,
  openMonth,
}: {
  summary?: DashboardSummary;
  failed?: boolean;
  openMonth: (day: number) => void;
}) {
  if (!summary)
    return failed ? (
      <Alert color="red" role="alert">
        全期間の集計を取得できませんでした。
      </Alert>
    ) : (
      <Text p="md" role="status">
        読み込み中
      </Text>
    );

  const account = summary.accountUsage.summary;
  const cards: Array<[string, string]> = [
    ["累計稼働時間", hours(summary.totalMs)],
    ["累計概算API料金", usd(summary.estimatedCostUsd)],
    ["総セッション数", count(summary.sessionCount)],
    ["ターン数", count(summary.turnCount)],
    ["ツール実行回数", count(summary.toolCallCount)],
    ["Codex /usageの累計トークン", count(account?.lifetimeTokens)],
    ["Codex /usageの最長連続利用", account ? `${count(account.longestStreakDays)}日` : "—"],
    ["Codex /usageの1日最大トークン", count(account?.peakDailyTokens)],
  ];
  const max = Math.max(1, ...summary.history.months.map(({ ms }) => ms));
  return (
    <Box
      component="section"
      aria-label="全期間ダッシュボード"
      flex={1}
      mih={0}
      style={{ overflow: "auto" }}
    >
      <Stack p="md" gap="xl" maw={1320} mx="auto">
        <Stack gap="sm">
          <Title order={2} size="h4">
            全期間
          </Title>
          <SimpleGrid cols={{ base: 2, sm: 4 }}>
            {cards.map(([label, value], index) => (
              <Card key={label} withBorder padding="sm">
                <Text size="xs" c="dimmed">
                  {label}
                </Text>
                <Text
                  fw={700}
                  fz={{ base: value.length > 12 ? 14 : 18, sm: 18 }}
                  style={{ overflowWrap: "anywhere" }}
                >
                  {value}
                </Text>
                {index === 1 && Boolean(summary.unpricedSessions) && (
                  <Text size="xs" c="dimmed">
                    未算出 {count(summary.unpricedSessions)}件
                  </Text>
                )}
              </Card>
            ))}
          </SimpleGrid>
        </Stack>
        <Stack component="section" gap="sm">
          <Title order={2} size="h4">
            月別推移
          </Title>
          {[...summary.history.months].reverse().map((month) => (
            <Paper
              key={month.start}
              withBorder
              p="sm"
              component="button"
              type="button"
              w="100%"
              ta="start"
              style={{ cursor: "pointer" }}
              onClick={() => openMonth(month.start)}
            >
              <Stack gap="xs">
                <Text fw={650}>{monthName.format(month.start)}</Text>
                <Progress value={(month.ms / max) * 100} aria-label="稼働時間" />
                <SimpleGrid cols={{ base: 2, sm: 4 }} spacing="xs">
                  {(
                    [
                      ["稼働時間", hours(month.ms)],
                      ["概算API料金", usd(month.estimatedCostUsd)],
                      ["ローカルトークン", count(month.tokens)],
                      ["Codex /usage", count(month.accountTokens)],
                    ] as const
                  ).map(([label, value]) => (
                    <div key={label}>
                      <Text size="xs" c="dimmed">
                        {label}
                      </Text>
                      <Text size="sm" style={{ overflowWrap: "anywhere" }}>
                        {value}
                      </Text>
                    </div>
                  ))}
                </SimpleGrid>
              </Stack>
            </Paper>
          ))}
        </Stack>
        <SimpleGrid cols={{ base: 1, md: 2 }}>
          <Breakdown title="エージェント別" totals={summary.agents} />
          <Breakdown title="プロジェクト別" totals={summary.projects} />
        </SimpleGrid>
      </Stack>
    </Box>
  );
}
