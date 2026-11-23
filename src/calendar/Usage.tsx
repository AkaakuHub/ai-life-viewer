import { Modal, Stack, Table, Text, Title, Tooltip, UnstyledButton } from "@mantine/core";
import type { Summary, UsageDay } from "../../shared/timeline.ts";

const number = new Intl.NumberFormat("ja-JP");
const compactNumber = new Intl.NumberFormat("en", {
  notation: "compact",
  maximumFractionDigits: 1,
});
const source = "Codex /usage（日別トークン履歴・時刻詳細なし）";

export function UsageBadge({
  usage,
  compact = false,
  onClick,
}: {
  usage: UsageDay;
  compact?: boolean;
  onClick?: () => void;
}) {
  const exact = `${source} ${number.format(usage.tokens)}トークン`;
  const label = compact
    ? compactNumber.format(usage.tokens)
    : `cx ${compactNumber.format(usage.tokens)} tok`;
  return (
    <Tooltip
      label={exact}
      multiline
      maw={280}
      events={{ hover: true, focus: true, touch: false }}
      withinPortal
    >
      {onClick ? (
        <UnstyledButton onClick={onClick} aria-label={exact} fz="xs" c="teal.3" mih={36} px={4}>
          {label}
        </UnstyledButton>
      ) : (
        <Text
          component="span"
          display="block"
          aria-label={exact}
          fz={10}
          lh={1.25}
          c="teal.3"
          truncate
        >
          {label}
        </Text>
      )}
    </Tooltip>
  );
}

export function UsageDayPanel({ usage, close }: { usage?: UsageDay; close: () => void }) {
  return (
    <Modal
      opened={Boolean(usage)}
      onClose={close}
      title="Codex /usage"
      centered
      closeButtonProps={{ "aria-label": "閉じる", size: 42 }}
    >
      {usage && (
        <Stack gap="xs">
          <Text>{usage.startDate}</Text>
          <Text size="xl" fw={700}>
            {number.format(usage.tokens)}トークン
          </Text>
          <Text size="sm" c="dimmed">
            アカウントの日別トークン履歴・時刻詳細なし
          </Text>
        </Stack>
      )}
    </Modal>
  );
}

export function AccountUsageFields({ usage }: { usage: Summary["accountUsage"] }) {
  const { summary, days } = usage;
  if (!summary && days.length === 0) return null;
  const total = days.reduce((sum, day) => sum + day.tokens, 0);
  const fields: Array<[string, string]> = [];
  if (days.length > 0) fields.push(["表示期間のトークン", number.format(total)]);
  if (summary)
    fields.push(
      ["累計トークン", number.format(summary.lifetimeTokens)],
      ["1日の最大トークン", number.format(summary.peakDailyTokens)],
      ["連続利用", `${number.format(summary.currentStreakDays)}日`],
      ["最長連続利用", `${number.format(summary.longestStreakDays)}日`],
    );
  return (
    <Stack component="section" gap="xs">
      <Title order={3} size="sm">
        Codexアカウント全体（/usage）
      </Title>
      <Table layout="fixed">
        <Table.Tbody>
          {fields.map(([label, value]) => (
            <Table.Tr key={label}>
              <Table.Th scope="row" fw={500} w="55%">
                {label}
              </Table.Th>
              <Table.Td ta="right" style={{ overflowWrap: "anywhere" }}>
                {value}
              </Table.Td>
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
    </Stack>
  );
}
