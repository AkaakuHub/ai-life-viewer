import { Alert, Box, Card, SimpleGrid, Stack, Text, Title } from "@mantine/core";
import type { ReactNode } from "react";
import type { DashboardSummary } from "../../shared/timeline.ts";
import { Breakdown } from "./Breakdown.tsx";
import { DashboardCount } from "./DashboardCount.tsx";
import { DashboardMonths } from "./DashboardMonths.tsx";
import { hours, usd } from "./format.ts";

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
  const cards: Array<{ label: string; value: ReactNode; note?: ReactNode }> = [
    { label: "累計稼働時間", value: hours(summary.totalMs) },
    {
      label: "概算API料金",
      value: usd(summary.estimatedCostUsd),
      note: summary.unpricedSessions ? (
        <>
          未算出 <DashboardCount value={summary.unpricedSessions} />件
        </>
      ) : undefined,
    },
    { label: "総セッション", value: <DashboardCount value={summary.sessionCount} /> },
    {
      label: "ターン / ツール",
      value: (
        <>
          <DashboardCount value={summary.turnCount} /> /{" "}
          <DashboardCount value={summary.toolCallCount} />
        </>
      ),
    },
    { label: "Codex累計トークン", value: <DashboardCount value={account?.lifetimeTokens} /> },
    { label: "最長連続利用", value: account ? `${account.longestStreakDays}日` : "—" },
    { label: "1日最大トークン", value: <DashboardCount value={account?.peakDailyTokens} /> },
  ];
  return (
    <Box
      component="section"
      aria-label="全期間ダッシュボード"
      flex={1}
      mih={0}
      style={{ overflow: "auto" }}
    >
      <Stack px={{ base: "xs", sm: "lg" }} py="md" gap="lg" maw={1280} mx="auto">
        <Stack gap="xs">
          <Title order={1} size="h4">
            全期間
          </Title>
          <SimpleGrid cols={{ base: 2, sm: 4 }} spacing={{ base: "xs", sm: "md" }}>
            {cards.map(({ label, value, note }) => (
              <Card key={label} withBorder padding="sm">
                <Text size="xs" c="dimmed">
                  {label}
                </Text>
                <Text fw={700} fz={{ base: 17, sm: 22 }} component="div">
                  {value}
                </Text>
                {note && (
                  <Text size="xs" c="dimmed">
                    {note}
                  </Text>
                )}
              </Card>
            ))}
          </SimpleGrid>
        </Stack>
        <DashboardMonths months={summary.history.months} openMonth={openMonth} />
        <SimpleGrid cols={{ base: 1, md: 2 }}>
          <Breakdown title="エージェント別" totals={summary.agents} />
          <Breakdown title="プロジェクト別" totals={summary.projects} />
        </SimpleGrid>
      </Stack>
    </Box>
  );
}
