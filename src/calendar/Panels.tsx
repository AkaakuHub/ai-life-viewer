import {
  Alert,
  Group,
  Modal,
  Paper,
  Progress,
  SimpleGrid,
  Stack,
  Table,
  Text,
  Title,
} from "@mantine/core";
import {
  MINUTE,
  type Block,
  type GroupTotal,
  type Metrics,
  type Summary,
} from "../../shared/timeline.ts";
import { clock, date, hours, monthNumber, usd, weekday } from "./format.ts";
import { Markdown } from "./Markdown.tsx";
import { supplementaryPreview } from "./markdown.ts";
import { monthTotals } from "./period.ts";

const closeButton = { "aria-label": "閉じる", size: 42 };
const wrap = { overflowWrap: "anywhere" } as const;

export function SummaryPanel({
  opened,
  close,
  summary,
  failed,
  yearStart,
}: {
  opened: boolean;
  close: () => void;
  summary?: Summary;
  failed?: boolean;
  yearStart?: number;
}) {
  return (
    <Modal
      opened={opened}
      onClose={close}
      title="集計"
      size="lg"
      centered
      closeButtonProps={closeButton}
    >
      {summary ? (
        <SummaryContent summary={summary} yearStart={yearStart} />
      ) : failed ? (
        <Alert color="red" role="alert">
          集計を取得できませんでした。
        </Alert>
      ) : (
        <Text role="status">読み込み中</Text>
      )}
    </Modal>
  );
}

function Breakdown({ title, totals }: { title: string; totals: GroupTotal[] }) {
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

function SummaryContent({ summary, yearStart }: { summary: Summary; yearStart?: number }) {
  const monthly = yearStart !== undefined;
  const totals = monthly ? monthTotals(yearStart, summary.days) : summary.days;
  const max = Math.max(1, ...totals.map(({ ms }) => ms));
  return (
    <Stack gap="md">
      <Paper withBorder p="sm">
        <Group justify="space-between" align="start">
          <div>
            <Text size="sm" c="dimmed">
              合計稼働時間
            </Text>
            <Text fw={700} size="xl">
              {hours(summary.totalMs)}
            </Text>
          </div>
          <div>
            <Text size="sm" c="dimmed">
              概算API料金
            </Text>
            <Text fw={700} size="xl">
              {usd(summary.estimatedCostUsd)}
            </Text>
            {Boolean(summary.unpricedSessions) && (
              <Text size="xs" c="dimmed">
                未算出 {summary.unpricedSessions}件
              </Text>
            )}
          </div>
        </Group>
      </Paper>
      <WorkMetrics metrics={summary} title="対象セッションの合計" />
      <Stack component="section" gap="xs">
        <Title order={3} size="sm">
          {monthly ? "月別" : "日別"}
        </Title>
        {totals.map(({ day, ms }) => (
          <Group key={day} gap="xs" wrap="nowrap">
            <Text size="sm" w={82} style={{ flexShrink: 0 }}>
              {monthly ? monthNumber.format(day) : `${date.format(day)} (${weekday.format(day)})`}
            </Text>
            <Progress value={(ms / max) * 100} flex={1} aria-label={hours(ms)} />
            <Text size="sm" miw={46} ta="right">
              {hours(ms)}
            </Text>
          </Group>
        ))}
      </Stack>
      <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="md">
        <Breakdown title="エージェント別" totals={summary.agents} />
        <Breakdown title="プロジェクト別" totals={summary.projects} />
      </SimpleGrid>
    </Stack>
  );
}

const count = new Intl.NumberFormat("ja-JP");
const metricFields: Array<[keyof Metrics, string]> = [
  ["inputTokens", "入力トークン"],
  ["cachedInputTokens", "キャッシュ済み入力"],
  ["outputTokens", "出力トークン"],
  ["reasoningTokens", "推論トークン"],
  ["turnCount", "ターン数"],
  ["toolCallCount", "ツール呼び出し"],
];

function Details({ fields }: { fields: Array<[string, string | undefined]> }) {
  return (
    <Table layout="fixed">
      <Table.Tbody>
        {fields
          .filter(([, value]) => value)
          .map(([label, value]) => (
            <Table.Tr key={label}>
              <Table.Th scope="row" w="39%" fw={500} c="dimmed" style={wrap}>
                {label}
              </Table.Th>
              <Table.Td style={wrap}>{value}</Table.Td>
            </Table.Tr>
          ))}
      </Table.Tbody>
    </Table>
  );
}

function WorkMetrics({ metrics, title }: { metrics: Metrics; title: string }) {
  const fields: Array<[string, string]> = [];
  for (const [key, label] of metricFields) {
    const value = metrics[key];
    if (value !== undefined) fields.push([label, count.format(value)]);
  }
  return (
    fields.length > 0 && (
      <Stack component="section" gap="xs">
        <Title order={3} size="sm">
          {title}
        </Title>
        <Details fields={fields} />
      </Stack>
    )
  );
}

export function BlockPanel({ block, close }: { block?: Block; close: () => void }) {
  const preview = block && supplementaryPreview(block.title, block.preview);
  return (
    <Modal
      opened={Boolean(block)}
      onClose={close}
      size="lg"
      centered
      closeButtonProps={closeButton}
      title={block?.title && <Markdown>{block.title}</Markdown>}
    >
      {block && (
        <Stack gap="md">
          <Details
            fields={[
              ["エージェント", block.agent],
              ["プロジェクト", block.project],
              ["作業ディレクトリ", block.cwd],
              ["開始", `${date.format(block.start)} ${clock.format(block.start)}`],
              ["終了", `${date.format(block.end)} ${clock.format(block.end)}`],
              ["稼働時間", `${Math.round((block.end - block.start) / MINUTE)}分`],
              ["モデル", block.model],
              ["概算API料金（セッション）", usd(block.estimatedCostUsd)],
              ["推論レベル", block.reasoningEffort],
              ["Gitブランチ", block.branch],
              ["Gitコミット", block.gitSha],
              ["Gitリモート", block.gitOriginUrl],
              ["合計トークン", block.tokens?.toLocaleString("ja-JP")],
            ]}
          />
          <WorkMetrics metrics={block} title="セッションの集計" />
          {preview && (
            <Stack component="section" gap="xs">
              <Title order={3} size="sm">
                概要
              </Title>
              <Markdown>{preview}</Markdown>
            </Stack>
          )}
        </Stack>
      )}
    </Modal>
  );
}
