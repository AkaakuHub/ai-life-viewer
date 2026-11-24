import {
  AppShell,
  Badge,
  Button,
  Grid,
  Group,
  Text,
  ThemeIcon,
  UnstyledButton,
} from "@mantine/core";
import { CalendarClock, ChartColumn, SlidersHorizontal } from "lucide-react";
import type { MouseEvent } from "react";
import { screenParams, type AppView, type Screen } from "../calendar/screen.ts";
import type { View } from "../calendar/period.ts";

export function AppHeader({
  screen,
  calendarView,
  changeView,
  openFilters,
  openSummary,
}: {
  screen: Screen;
  calendarView: View;
  changeView: (view: AppView) => void;
  openFilters: () => void;
  openSummary: () => void;
}) {
  const calendar = screen.view !== "top";
  const { agents, projects, pcs, q } = screen.selection;
  const filterCount = agents.length + projects.length + pcs.length + Number(Boolean(q.trim()));

  function link(view: AppView) {
    return {
      href: `?${screenParams({ ...screen, view })}`,
      onClick: (event: MouseEvent<HTMLAnchorElement>) => {
        if (event.button !== 0 || event.metaKey || event.ctrlKey || event.altKey || event.shiftKey)
          return;
        event.preventDefault();
        changeView(view);
      },
    };
  }

  return (
    <AppShell.Header pos="static" h="auto" bg="dark.7">
      <Grid px={{ base: "xs", sm: "lg" }} py="xs" gap="xs" align="center">
        <Grid.Col span="content">
          <UnstyledButton component="a" {...link("top")} mih={42}>
            <Group gap="xs" wrap="nowrap">
              <ThemeIcon size={36} variant="light" radius="md" aria-hidden="true">
                <CalendarClock size={20} />
              </ThemeIcon>
              <Text component="span" fw={750} fz={{ base: "sm", sm: "md" }}>
                AI Life Viewer
              </Text>
            </Group>
          </UnstyledButton>
        </Grid.Col>
        <Grid.Col span={{ base: 12, sm: "content" }} order={{ base: 1, sm: 0 }}>
          <Group component="nav" aria-label="メインナビゲーション" gap={4} wrap="nowrap">
            <Button
              component="a"
              {...link("top")}
              aria-current={!calendar ? "page" : undefined}
              variant={!calendar ? "light" : "subtle"}
              color={!calendar ? "blue" : "gray"}
              mih={42}
            >
              ダッシュボード
            </Button>
            <Button
              component="a"
              {...link(calendarView)}
              aria-current={calendar ? "page" : undefined}
              variant={calendar ? "light" : "subtle"}
              color={calendar ? "blue" : "gray"}
              mih={42}
            >
              カレンダー
            </Button>
          </Group>
        </Grid.Col>
        <Grid.Col span="auto">
          <Group gap={6} justify="flex-end" wrap="nowrap">
            <Button
              variant={filterCount ? "light" : "default"}
              mih={42}
              px={{ base: "xs", sm: "sm" }}
              leftSection={<SlidersHorizontal size={15} />}
              rightSection={
                filterCount ? (
                  <Badge
                    size="sm"
                    circle={filterCount < 10}
                    variant="filled"
                    aria-label={`${filterCount}件`}
                  >
                    {filterCount}
                  </Badge>
                ) : undefined
              }
              onClick={openFilters}
            >
              絞り込み
            </Button>
            {calendar && (
              <Button
                variant="default"
                mih={42}
                px={{ base: "xs", sm: "sm" }}
                leftSection={<ChartColumn size={15} />}
                onClick={openSummary}
              >
                集計
              </Button>
            )}
          </Group>
        </Grid.Col>
      </Grid>
    </AppShell.Header>
  );
}
