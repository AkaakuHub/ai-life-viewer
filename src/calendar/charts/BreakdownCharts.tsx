import { BarChart, DonutChart } from "@mantine/charts";
import {
  ColorSwatch,
  Group,
  Paper,
  SegmentedControl,
  Stack,
  Table,
  Text,
  Title,
} from "@mantine/core";
import { useMediaQuery } from "@mantine/hooks";
import { useState } from "react";
import { HOUR, type GroupTotal } from "../../../shared/timeline.ts";
import { chartValue, rankProjects, workMetrics, type WorkMetric } from "./metrics.ts";

const palette = [
  "#eabb73",
  "#82c7cf",
  "#d98981",
  "#c0a3e2",
  "#a3cd99",
  "#de975b",
  "#8da9ed",
  "#d0b8aa",
];

function share(value: number, total: number) {
  if (total === 0) return "—";
  const percentage = (value / total) * 100;
  return percentage > 0 && percentage < 0.1 ? "<0.1%" : `${percentage.toFixed(1)}%`;
}

export function AgentChart({ agents }: { agents: GroupTotal[] }) {
  const rows = agents.map(({ name, ms, estimatedCostUsd }, index) => ({
    name,
    hours: ms / HOUR,
    cost: estimatedCostUsd,
    color: palette[index % palette.length],
  }));
  const totalHours = rows.reduce((sum, row) => sum + row.hours, 0);
  const totalCost = rows.reduce((sum, row) => sum + (row.cost ?? 0), 0);
  return (
    <Paper
      withBorder
      p={{ base: "sm", md: "md" }}
      component="section"
      aria-label="エージェント別の割合"
      miw={0}
    >
      <Stack gap="sm">
        <Title order={2} size="h4">
          エージェント別
        </Title>
        <Group justify="space-evenly" gap={4} wrap="wrap">
          {workMetrics.map(({ value, label }) => {
            const data = rows.flatMap((row) => {
              const amount = row[value];
              return amount !== undefined && amount > 0
                ? [{ name: row.name, value: amount, color: row.color }]
                : [];
            });
            return data.length ? (
              <DonutChart
                key={value}
                aria-label={`エージェント別の${label}`}
                data={data}
                chartLabel={label}
                size={126}
                thickness={20}
                strokeWidth={0}
                valueFormatter={(amount) => chartValue(value, amount)}
                tooltipProps={{
                  position: { x: 0, y: 0 },
                  content: ({ active, payload }) => {
                    const item = payload?.[0];
                    if (!active || typeof item?.value !== "number") return null;
                    return (
                      <Paper withBorder shadow="md" p="xs" maw={126}>
                        <Text size="xs" fw={650}>
                          {item.name}
                        </Text>
                        <Text size="xs">{chartValue(value, item.value)}</Text>
                      </Paper>
                    );
                  },
                }}
              />
            ) : (
              <Text key={value} c="dimmed" size="sm">
                {label} —
              </Text>
            );
          })}
        </Group>
        {rows.length > 0 && (
          <Table horizontalSpacing={6} verticalSpacing={5} fz="xs">
            <Table.Thead>
              <Table.Tr>
                <Table.Th scope="col">エージェント</Table.Th>
                <Table.Th scope="col" ta="right">
                  稼働時間
                </Table.Th>
                <Table.Th scope="col" ta="right">
                  概算料金
                </Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {rows.map((row) => (
                <Table.Tr key={row.name}>
                  <Table.Th scope="row" fw={500}>
                    <Group gap={6} wrap="nowrap">
                      <ColorSwatch
                        size={10}
                        color={row.color}
                        withShadow={false}
                        aria-hidden="true"
                      />
                      {row.name}
                    </Group>
                  </Table.Th>
                  <Table.Td ta="right">{share(row.hours, totalHours)}</Table.Td>
                  <Table.Td ta="right">
                    {row.cost === undefined ? "—" : share(row.cost, totalCost)}
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        )}
      </Stack>
    </Paper>
  );
}

export function ProjectChart({ projects }: { projects: GroupTotal[] }) {
  const [metric, setMetric] = useState<WorkMetric>("hours");
  const wide = useMediaQuery("(min-width: 48em)");
  const data = rankProjects(projects, metric);
  return (
    <Paper
      withBorder
      p={{ base: "sm", md: "md" }}
      component="section"
      aria-label="プロジェクト別ランキング"
      miw={0}
    >
      <Stack gap="sm">
        <Group justify="space-between" gap="xs">
          <Title order={2} size="h4">
            プロジェクト上位
          </Title>
          <SegmentedControl<WorkMetric>
            aria-label="プロジェクト上位の指標"
            size="xs"
            value={metric}
            onChange={setMetric}
            data={workMetrics}
            styles={{ label: { minHeight: 40, display: "grid", placeItems: "center" } }}
          />
        </Group>
        {data.length ? (
          <BarChart
            h={Math.max(160, data.length * 35 + 26)}
            data={data}
            dataKey="name"
            orientation="vertical"
            gridAxis="y"
            tickLine="none"
            series={[
              {
                name: "value",
                label: metric === "hours" ? "稼働時間" : "概算料金",
                color: metric === "hours" ? "#eabb73" : "#d98981",
              },
            ]}
            maxBarWidth={20}
            valueFormatter={(value) => chartValue(metric, value)}
            barProps={{ radius: [0, 3, 3, 0] }}
            xAxisProps={{
              tickFormatter: (value: number) => chartValue(metric, value, true),
              minTickGap: 10,
            }}
            yAxisProps={{
              width: wide ? 150 : 111,
              interval: 0,
              tick: { fontSize: 11 },
              tickFormatter: (name: string) => {
                const characters = Array.from(name);
                const max = wide ? 13 : 9;
                return characters.length > max ? `${characters.slice(0, max).join("")}…` : name;
              },
            }}
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
