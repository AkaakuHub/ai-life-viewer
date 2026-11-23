import { Stack, Table, Title } from "@mantine/core";
import type { GroupTotal } from "../../shared/timeline.ts";
import { hours, usd } from "./format.ts";

const wrap = { overflowWrap: "anywhere" } as const;

export function Breakdown({ title, totals }: { title: string; totals: GroupTotal[] }) {
  return (
    <Stack component="section" gap="xs" miw={0}>
      <Title order={3} size="sm">
        {title}
      </Title>
      <Table layout="fixed">
        <Table.Thead>
          <Table.Tr>
            <Table.Th scope="col">名前</Table.Th>
            <Table.Th scope="col" ta="right">
              稼働
            </Table.Th>
            <Table.Th scope="col" ta="right">
              概算USD
            </Table.Th>
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {totals.map(({ name, ms, estimatedCostUsd }) => (
            <Table.Tr key={name}>
              <Table.Th scope="row" fw={400} style={wrap}>
                {name}
              </Table.Th>
              <Table.Td ta="right">{hours(ms)}</Table.Td>
              <Table.Td ta="right" style={wrap}>
                {usd(estimatedCostUsd)}
              </Table.Td>
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
    </Stack>
  );
}
