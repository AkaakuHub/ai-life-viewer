import { Text } from "@mantine/core";
import { useResource } from "./use-resource.ts";

const yen = new Intl.NumberFormat("ja-JP", { maximumFractionDigits: 0 });
const rateNumber = new Intl.NumberFormat("ja-JP", { maximumFractionDigits: 4 });

export function DashboardYen({ dollars }: { dollars: number }) {
  const { data } = useResource<{ rate: number; date: string } | null>("/api/exchange-rate");
  if (!data) return null;
  return (
    <div className="dashboard-stage__yen">
      <Text component="div" fw={750} className="dashboard-stage__yen-amount">
        約{yen.format(dollars * data.rate)}円
      </Text>
      <Text size="xs" className="dashboard-stage__note">
        1 USD = {rateNumber.format(data.rate)}円 · {data.date}
      </Text>
    </div>
  );
}
