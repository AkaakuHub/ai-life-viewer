import { Alert, Button, Drawer, MultiSelect, Stack, TextInput } from "@mantine/core";
import type { FilterOptions } from "../../shared/timeline.ts";

export type Selection = FilterOptions & { q: string };

export function Filters({
  opened,
  close,
  options,
  failed,
  value,
  change,
}: {
  opened: boolean;
  close: () => void;
  options?: FilterOptions;
  failed?: boolean;
  value: Selection;
  change: (value: Selection) => void;
}) {
  return (
    <Drawer
      opened={opened}
      onClose={close}
      position="right"
      title="絞り込み"
      size="sm"
      closeButtonProps={{ "aria-label": "閉じる", size: 42 }}
    >
      <Stack>
        {failed && (
          <Alert color="red" role="alert">
            絞り込みの項目を取得できませんでした。
          </Alert>
        )}
        <TextInput
          label="検索"
          type="search"
          value={value.q}
          data-mantine-stop-propagation="true"
          onChange={(event) => change({ ...value, q: event.currentTarget.value })}
        />
        {(["agents", "projects"] as const).map((key) => (
          <MultiSelect
            key={key}
            label={key === "agents" ? "エージェント" : "プロジェクト"}
            data={options?.[key] ?? value[key]}
            value={value[key]}
            searchable
            clearable
            comboboxProps={{ withinPortal: false }}
            onChange={(items) => change({ ...value, [key]: items })}
          />
        ))}
        <Button
          variant="default"
          mih={42}
          style={{ alignSelf: "start" }}
          onClick={() => change({ agents: [], projects: [], q: "" })}
        >
          解除
        </Button>
      </Stack>
    </Drawer>
  );
}
