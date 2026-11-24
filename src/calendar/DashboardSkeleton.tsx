import { DashboardStage } from "./DashboardStage.tsx";
import { Box, Card, Paper, SimpleGrid, Skeleton, Stack, VisuallyHidden } from "@mantine/core";

export function DashboardSkeleton() {
  return (
    <Box
      component="section"
      aria-label="全期間ダッシュボード"
      className="dashboard"
      flex={1}
      mih={0}
      style={{ overflow: "auto" }}
    >
      <VisuallyHidden role="status">ダッシュボードを読み込み中</VisuallyHidden>
      <Stack px={{ base: "xs", sm: "lg" }} py="md" gap="lg" maw={1280} mx="auto">
        <DashboardStage loading>
          {Array.from({ length: 6 }, (_, index) => (
            <Card key={index} withBorder padding="sm" aria-hidden="true">
              <Stack gap={6}>
                <Skeleton h={12} w="66%" />
                <Skeleton h={22} w="48%" />
              </Stack>
            </Card>
          ))}
        </DashboardStage>
        <Paper withBorder p={{ base: "sm", md: "md" }} aria-hidden="true">
          <Stack gap="md">
            <Skeleton h={40} w={124} />
            <Skeleton h={{ base: 234, sm: 270 }} />
          </Stack>
        </Paper>
        <SimpleGrid cols={{ base: 1, md: 2 }} spacing={{ base: "xs", sm: "md" }} aria-hidden="true">
          <Skeleton h={300} />
          <Skeleton h={300} />
        </SimpleGrid>
      </Stack>
    </Box>
  );
}
