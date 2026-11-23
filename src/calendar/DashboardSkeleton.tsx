import {
  Box,
  Card,
  Paper,
  SimpleGrid,
  Skeleton,
  Stack,
  Title,
  VisuallyHidden,
} from "@mantine/core";

export function DashboardSkeleton() {
  return (
    <Box
      component="section"
      aria-label="全期間ダッシュボード"
      flex={1}
      mih={0}
      style={{ overflow: "auto" }}
    >
      <VisuallyHidden role="status">ダッシュボードを読み込み中</VisuallyHidden>
      <Stack px={{ base: "xs", sm: "lg" }} py="md" gap="lg" maw={1280} mx="auto">
        <Stack gap="xs">
          <Title order={1} size="h4">
            全期間
          </Title>
          <SimpleGrid cols={{ base: 2, sm: 4 }} spacing={{ base: "xs", sm: "md" }}>
            {Array.from({ length: 7 }, (_, index) => (
              <Card key={index} withBorder padding="sm" aria-hidden="true">
                <Stack gap={6}>
                  <Skeleton h={12} w="66%" />
                  <Skeleton h={22} w="48%" />
                </Stack>
              </Card>
            ))}
          </SimpleGrid>
        </Stack>
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
