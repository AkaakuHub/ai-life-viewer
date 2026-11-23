import { Text, Tooltip } from "@mantine/core";

const exact = new Intl.NumberFormat("ja-JP");
const compact = new Intl.NumberFormat("en-US", {
  notation: "compact",
  maximumFractionDigits: 2,
  maximumSignificantDigits: 4,
  roundingPriority: "lessPrecision",
});

export function DashboardCount({ value }: { value?: number }) {
  if (value === undefined) return "—";
  const label = compact.format(value);
  const full = exact.format(value);
  if (label === full) return label;
  return (
    <Tooltip label={full} events={{ hover: true, focus: true, touch: true }}>
      <Text span inherit tabIndex={0} aria-label={full}>
        {label}
      </Text>
    </Tooltip>
  );
}
