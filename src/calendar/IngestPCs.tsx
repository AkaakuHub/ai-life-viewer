import { Alert, Card, List, Loader, Stack, Text, Title } from "@mantine/core";
import { useResource } from "./use-resource.ts";

type IngestStatus = {
  serverPcId: string;
  ingestPcs: { id: string; lastIngest: number }[];
};

const date = new Intl.DateTimeFormat("ja-JP", { dateStyle: "short", timeStyle: "short" });

export function IngestPCs() {
  const { data, error } = useResource<IngestStatus>("/api/status");
  return (
    <Card withBorder component="section">
      <Stack gap="xs">
        <Title order={2} size="h5">
          取り込み元のPC
        </Title>
        {data ? (
          <>
            <Text size="sm">サーバー本体：{data.serverPcId}</Text>
            {data.ingestPcs.length > 0 && (
              <List size="sm" aria-label="履歴送信元と最終送信日時">
                {data.ingestPcs.map(({ id, lastIngest }) => (
                  <List.Item key={id}>
                    {id}（最終送信：{date.format(lastIngest)}）
                  </List.Item>
                ))}
              </List>
            )}
          </>
        ) : error ? (
          <Alert color="red">取り込み元のPCを取得できませんでした。</Alert>
        ) : (
          <Loader size="sm" role="status" aria-label="取り込み元のPCを読み込み中" />
        )}
      </Stack>
    </Card>
  );
}
