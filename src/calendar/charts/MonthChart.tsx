import { BarChart } from "@mantine/charts";
import { Group, Paper, SegmentedControl, Stack, Text, Title } from "@mantine/core";
import { useState } from "react";
import type { BarProps } from "recharts";
import type { DashboardSummary } from "../../../shared/timeline.ts";
import { monthName } from "../format.ts";
import { chartMetrics, chartValue, monthSeries, type ChartMetric } from "./metrics.ts";

const monthTick = new Intl.DateTimeFormat("ja-JP", {
  timeZone: "Asia/Tokyo",
  year: "2-digit",
  month: "2-digit",
});

export function MonthChart({
  months,
  openMonth,
}: {
  months: DashboardSummary["history"]["months"];
  openMonth: (month: number) => void;
}) {
  const [metric, setMetric] = useState<ChartMetric>("hours");
  const data = monthSeries(months);
  const color = { hours: "#eabb73", cost: "#d98981", tokens: "#82c7cf" }[metric];
  const label = chartMetrics.find(({ value }) => value === metric)?.label ?? metric;
  const barProps: Partial<BarProps> = {
    cursor: "pointer",
    radius: [3, 3, 0, 0],
    onClick: (bar) => {
      const month = data[bar.originalDataIndex];
      if (month) openMonth(month.start);
    },
  };
  return (
    <Paper withBorder p={{ base: "sm", md: "md" }} component="section" aria-label="月別推移グラフ">
      <Stack gap="md">
        <Group justify="space-between" gap="xs">
          <Title order={2} size="h4">
            月別推移
          </Title>
          <SegmentedControl<ChartMetric>
            aria-label="月別推移の指標"
            size="xs"
            value={metric}
            onChange={setMetric}
            data={chartMetrics}
            styles={{ label: { minHeight: 40, display: "grid", placeItems: "center" } }}
          />
        </Group>
        {data.some((month) => month[metric] !== null) ? (
          <BarChart
            h={{ base: 234, sm: 270 }}
            data={data}
            dataKey="start"
            series={[{ name: metric, label, color }]}
            maxBarWidth={38}
            valueFormatter={(value) => chartValue(metric, value)}
            xAxisProps={{
              tickFormatter: (timestamp: number) => monthTick.format(timestamp),
              minTickGap: 13,
            }}
            yAxisProps={{
              width: 48,
              tickFormatter: (value: number) => chartValue(metric, value, true),
            }}
            tooltipProps={{
              labelFormatter: (timestamp) =>
                timestamp === undefined ? undefined : monthName.format(Number(timestamp)),
            }}
            barProps={barProps}
          />
        ) : (
          <Text c="dimmed" role="status">
            データなし
          </Text>
        )}
      </Stack>
    </Paper>
  );
}
