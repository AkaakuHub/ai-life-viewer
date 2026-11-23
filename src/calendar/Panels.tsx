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
import { type Block, type Metrics, type Summary } from "../../shared/timeline.ts";
import { blockTime, date, duration, hours, monthNumber, usd, weekday } from "./format.ts";
import { Breakdown } from "./Breakdown.tsx";
import { Markdown } from "./Markdown.tsx";
import { supplementaryPreview } from "./markdown.ts";
import { AccountUsageFields } from "./Usage.tsx";
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
      <AccountUsageFields usage={summary.accountUsage} />
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

function modelLabel(model: string, tier?: string) {
  const name = model || "モデル不明";
  return tier && tier !== "default" && tier !== "standard" ? `${name} · ${tier}` : name;
}

function ModelUsageDetails({ block }: { block: Block }) {
  return (
    <Stack component="section" gap="xs">
      <Title order={3} size="sm">
        モデル別の利用内訳
      </Title>
      {block.usageByModel?.map((row) => (
        <Stack key={`${row.model}:${row.serviceTier ?? "default"}`} gap={0}>
          <Text fw={650} size="sm" style={wrap}>
            {modelLabel(row.model, row.serviceTier)}
          </Text>
          <Details
            fields={[
              ...metricFields.map(([key, label]): [string, string | undefined] => {
                const value =
                  key in row && key !== "turnCount" && key !== "toolCallCount"
                    ? row[key]
                    : undefined;
                return [label, value === undefined ? undefined : count.format(value)];
              }),
              ["概算API料金", usd(row.estimatedCostUsd)],
            ]}
          />
        </Stack>
      ))}
    </Stack>
  );
}

export function BlockPanel({ block, close }: { block?: Block; close: () => void }) {
  const preview = block && supplementaryPreview(block.title, block.preview);
  const firstModel = block?.usageByModel?.[0];
  const showUsage =
    firstModel &&
    ((block?.usageByModel?.length ?? 0) > 1 ||
      (firstModel.serviceTier && !["default", "standard"].includes(firstModel.serviceTier)));
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
              ["時間帯", blockTime(block)],
              ["実稼働", duration(block.activeMs)],
              [
                "モデル",
                showUsage
                  ? undefined
                  : modelLabel(
                      firstModel?.model ?? block.model,
                      firstModel?.serviceTier ?? block.serviceTier,
                    ),
              ],
              [
                "記録元",
                block.provenance === "metadata"
                  ? "Codexスレッドメタデータ"
                  : block.provenance === "turns"
                    ? "Codexターン履歴"
                    : undefined,
              ],
              [
                "概算API料金（セッション）",
                `${usd(block.estimatedCostUsd)}${block.incompleteCost && block.estimatedCostUsd !== undefined ? "（一部未算出）" : ""}`,
              ],
              ["推論レベル", block.reasoningEffort],
              ["Gitブランチ", block.branch],
              ["Gitコミット", block.gitSha],
              ["Gitリモート", block.gitOriginUrl],
              ["合計トークン", block.tokens?.toLocaleString("ja-JP")],
            ]}
          />
          {showUsage && <ModelUsageDetails block={block} />}
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
