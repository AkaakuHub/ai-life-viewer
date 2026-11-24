import { Badge, Group, Stack, Table, Text, Title } from "@mantine/core";
import type { GroupTotal } from "../../shared/timeline.ts";
import { hours, usd } from "./format.ts";

const wrap = { overflowWrap: "anywhere" } as const;

export function Breakdown({
  title,
  totals,
  showPCs = false,
}: {
  title: string;
  totals: GroupTotal[];
  showPCs?: boolean;
}) {
  return (
    <Stack component="section" gap="xs" miw={0}>
      <Title order={3} size="sm">
        {title}
      </Title>
      <Table layout="fixed">
        <Table.Thead>
          <Table.Tr>
            <Table.Th scope="col" w={showPCs ? "50%" : undefined}>
              {showPCs ? "プロジェクト・記録元PC" : "名前"}
            </Table.Th>
            <Table.Th scope="col" ta="right">
              稼働
            </Table.Th>
            <Table.Th scope="col" ta="right">
              概算USD
            </Table.Th>
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {totals.map(({ name, ms, estimatedCostUsd, pcIds }) => (
            <Table.Tr key={name}>
              <Table.Th scope="row" fw={400} style={wrap}>
                {name}
                {showPCs && (
                  <Group gap={4} mt={4}>
                    {pcIds?.length ? (
                      pcIds.map((id) => (
                        <Badge key={id} variant="light" size="sm" tt="none" title={id} maw="100%">
                          {id}
                        </Badge>
                      ))
                    ) : (
                      <Text size="xs" c="dimmed">
                        PC不明
                      </Text>
                    )}
                  </Group>
                )}
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
