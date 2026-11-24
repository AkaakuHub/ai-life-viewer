import "@mantine/charts/styles.css";
import { Alert, Box, Card, SimpleGrid, Stack, Text, Title } from "@mantine/core";
import type { ReactNode } from "react";
import type { DashboardSummary } from "../../shared/timeline.ts";
import { Breakdown } from "./Breakdown.tsx";
import { DashboardCount } from "./DashboardCount.tsx";
import { DashboardSkeleton } from "./DashboardSkeleton.tsx";
import { DashboardMonths } from "./DashboardMonths.tsx";
import { IngestPCs } from "./IngestPCs.tsx";
import { AgentChart, ProjectChart } from "./charts/BreakdownCharts.tsx";
import { MonthChart } from "./charts/MonthChart.tsx";
import { hours, usd } from "./format.ts";

const number = new Intl.NumberFormat("ja-JP");

export default function Dashboard({
  summary,
  failed,
  pcs,
  openMonth,
}: {
  summary?: DashboardSummary;
  failed?: boolean;
  pcs: string[];
  openMonth: (day: number) => void;
}) {
  if (!summary)
    return failed ? (
      <Alert color="red" role="alert">
        全期間の集計を取得できませんでした。
      </Alert>
    ) : (
      <DashboardSkeleton />
    );

  const account = summary.accountUsage.summary;
  const showAccount = pcs.length === 0;
  const missingBreakdown = summary.missingTokenBreakdownSessions ?? 0;
  const missingRates = (summary.unpricedSessions ?? 0) - missingBreakdown;
  const cards: Array<{ label: string; value: ReactNode; note?: ReactNode }> = [
    { label: "累計稼働時間", value: hours(summary.totalMs) },
    {
      label: "概算API料金",
      value: usd(summary.estimatedCostUsd),
      note: summary.unpricedSessions ? (
        <Stack gap={2}>
          {missingBreakdown > 0 && (
            <span>トークン内訳不足 {number.format(missingBreakdown)}件</span>
          )}
          {missingRates > 0 && <span>料金情報不足 {number.format(missingRates)}件</span>}
        </Stack>
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
  ];
  if (showAccount) {
    cards.push(
      { label: "Codex累計トークン", value: <DashboardCount value={account?.lifetimeTokens} /> },
      { label: "最長連続利用", value: account ? `${account.longestStreakDays}日` : "—" },
      { label: "1日最大トークン", value: <DashboardCount value={account?.peakDailyTokens} /> },
    );
  }
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
            {showAccount ? "全期間" : pcs.join("、")}
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
                  <Text component="div" size="xs" c="dimmed">
                    {note}
                  </Text>
                )}
              </Card>
            ))}
          </SimpleGrid>
        </Stack>
        <MonthChart
          months={summary.history.months}
          openMonth={openMonth}
          showAccount={showAccount}
        />
        <SimpleGrid cols={{ base: 1, md: 2 }} spacing={{ base: "xs", sm: "md" }}>
          <AgentChart agents={summary.agents} />
          <ProjectChart projects={summary.projects} />
        </SimpleGrid>
        <DashboardMonths
          months={summary.history.months}
          openMonth={openMonth}
          showAccount={showAccount}
        />
        <IngestPCs />
        <SimpleGrid cols={{ base: 1, md: 2 }}>
          <Breakdown title="エージェント別" totals={summary.agents} />
          <Breakdown title="プロジェクト別" totals={summary.projects} showPCs />
        </SimpleGrid>
      </Stack>
    </Box>
  );
}
