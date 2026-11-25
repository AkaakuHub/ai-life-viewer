import { Paper, Stack, Table, Title, UnstyledButton } from "@mantine/core";
import type { DashboardSummary } from "../../shared/timeline.ts";
import { DashboardCount } from "./DashboardCount.tsx";
import { hours, monthName, usd } from "./format.ts";

const monthLabel = new Intl.DateTimeFormat("ja-JP", {
  timeZone: "Asia/Tokyo",
  year: "numeric",
  month: "2-digit",
});

export function DashboardMonths({
  months,
  openMonth,
}: {
  months: DashboardSummary["history"]["months"];
  openMonth: (day: number) => void;
}) {
  return (
    <Stack component="section" gap="xs">
      <Title order={2} size="h4">
        月別明細
      </Title>
      <Paper withBorder>
        <Table.ScrollContainer minWidth={330} type="native">
          <Table
            highlightOnHover
            horizontalSpacing={6}
            verticalSpacing={0}
            fz={{ base: "xs", sm: "sm" }}
            style={{ whiteSpace: "nowrap" }}
          >
            <Table.Thead>
              <Table.Tr>
                <Table.Th scope="col" py="xs">
                  月
                </Table.Th>
                <Table.Th scope="col" ta="right">
                  稼働
                </Table.Th>
                <Table.Th scope="col" ta="right">
                  概算USD
                </Table.Th>
                <Table.Th scope="col" ta="right">
                  トークン
                </Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {[...months].reverse().map((month) => (
                <Table.Tr
                  key={month.start}
                  onClick={() => openMonth(month.start)}
                  style={{ cursor: "pointer" }}
                >
                  <Table.Th scope="row">
                    <UnstyledButton
                      mih={42}
                      fw={650}
                      fz="inherit"
                      c="blue.3"
                      aria-label={`${monthName.format(month.start)}のカレンダーを開く`}
                    >
                      {monthLabel.format(month.start)}
                    </UnstyledButton>
                  </Table.Th>
                  <Table.Td ta="right">{hours(month.ms)}</Table.Td>
                  <Table.Td ta="right">{usd(month.estimatedCostUsd)}</Table.Td>
                  <Table.Td ta="right">
                    <DashboardCount value={month.tokens} />
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </Table.ScrollContainer>
      </Paper>
    </Stack>
  );
}
