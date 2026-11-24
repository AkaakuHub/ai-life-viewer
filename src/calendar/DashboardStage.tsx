import { Box, Skeleton, Text, Title } from "@mantine/core";
import { Crown } from "lucide-react";
import type { ReactNode } from "react";
import { usd } from "./format.ts";
import "./dashboard-stage.css";

export function DashboardStage({
  pcs,
  cost,
  note,
  children,
  loading = false,
}: {
  pcs?: string[];
  cost?: number;
  note?: ReactNode;
  children: ReactNode;
  loading?: boolean;
}) {
  const amount = usd(cost);
  const [whole, cents] = amount.split(".");
  return (
    <Box className="dashboard-stage">
      <div className="dashboard-stage__center">
        <Crown size={28} strokeWidth={1.6} aria-hidden="true" />
        <Title order={1} className="dashboard-stage__title">
          概算API料金
        </Title>
        <Text size="xs" c="dimmed" className="dashboard-stage__scope">
          {pcs?.length ? pcs.join("、") : "全期間"}
        </Text>
        {loading ? (
          <Skeleton className="dashboard-stage__placeholder" aria-hidden="true" />
        ) : (
          <div className="dashboard-stage__amount">
            <span className="dashboard-stage__depth" aria-hidden="true">
              {whole}
              {cents && <small>.{cents}</small>}
            </span>
            <span className="dashboard-stage__face">
              {whole}
              {cents && <small>.{cents}</small>}
            </span>
          </div>
        )}
        <div className="dashboard-stage__podium" aria-hidden="true" />
        {note}
      </div>
      <div className="dashboard-stage__stats">{children}</div>
    </Box>
  );
}
