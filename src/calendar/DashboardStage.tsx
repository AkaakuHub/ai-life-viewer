import { Box, Skeleton, Text, Title, VisuallyHidden } from "@mantine/core";
import { Crown } from "lucide-react";
import type { ReactNode } from "react";
import { DashboardYen } from "./DashboardYen.tsx";
import { usd } from "./format.ts";
import "./dashboard-stage.css";

function amountDigits(value: string, laterDigits = 0) {
  return Array.from(value, (digit, index) => (
    <span
      key={index}
      style={{ animationDelay: `${(laterDigits + value.length - 1 - index) * 55}ms` }}
    >
      {digit}
    </span>
  ));
}

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
  const fraction = cents ? `.${cents}` : "";
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
            <VisuallyHidden>{amount}</VisuallyHidden>
            <span className="dashboard-stage__depth" aria-hidden="true">
              {amountDigits(whole, fraction.length)}
              {fraction && <small>{amountDigits(fraction)}</small>}
            </span>
            <span className="dashboard-stage__face" aria-hidden="true">
              {amountDigits(whole, fraction.length)}
              {fraction && <small>{amountDigits(fraction)}</small>}
            </span>
          </div>
        )}
        <div className="dashboard-stage__podium" aria-hidden="true" />
        {cost !== undefined && <DashboardYen dollars={cost} />}
        {note}
      </div>
      <div className="dashboard-stage__stats">{children}</div>
    </Box>
  );
}
