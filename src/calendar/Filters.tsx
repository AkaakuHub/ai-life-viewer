import { Alert, Button, Drawer, Group, Loader, MultiSelect, Stack, TextInput } from "@mantine/core";
import { useRef } from "react";
import type { FilterOptions } from "../../shared/timeline.ts";

import type { Selection } from "./screen.ts";

export function Filters({
  opened,
  close,
  options,
  failed,
  loading,
  value,
  change,
}: {
  opened: boolean;
  close: () => void;
  options?: FilterOptions;
  failed?: boolean;
  loading: boolean;
  value: Selection;
  change: (value: Selection, mode?: "push" | "replace") => void;
}) {
  const editingSearch = useRef(false);
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
          onBlur={() => {
            editingSearch.current = false;
          }}
          onChange={(event) => {
            change(
              { ...value, q: event.currentTarget.value },
              editingSearch.current ? "replace" : "push",
            );
            editingSearch.current = true;
          }}
        />
        {loading && (
          <Group>
            <Loader size="xs" role="status" aria-label="絞り込みの候補を読み込み中" />
          </Group>
        )}
        {(["pcs", "agents", "projects"] as const).map((key) => (
          <MultiSelect
            key={key}
            label={{ pcs: "記録元PC", agents: "エージェント", projects: "プロジェクト" }[key]}
            data={options?.[key] ?? value[key]}
            value={value[key]}
            disabled={loading}
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
          onClick={() => change({ agents: [], projects: [], pcs: [], q: "" })}
        >
          解除
        </Button>
      </Stack>
    </Drawer>
  );
}
