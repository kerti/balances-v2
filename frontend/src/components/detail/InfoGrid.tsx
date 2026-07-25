import { cn } from "@/lib/utils";
import type { InfoField } from "@/components/detail/types";

// The shared info-card body (ADR-0051). Lays out label/value pairs and owns the
// responsive reflow. It **never inspects field identity or domain meaning**:
// alignment/formatting ride on the value node itself, and each value cell is a
// flex container so a per-node `ml-auto` / `text-right` lines up a numeric
// column without the grid learning what "numeric" means. Renders nothing for an
// empty field list.
//
// `mobileLayout` (Phase B) picks how each pair reads below 768px:
//   - `"stacked"` (default): label on its own line, value beneath — each piece
//     on its own line, so a long value never crowds the label.
//   - `"inline"`: label left, value right on one line — for short shared-surface
//     meta (ownership / currency / status) where the compact row reads better.
// At/above 768px both dissolve their per-pair wrapper via `md:contents` so
// `dt`/`dd` rejoin the shared two-column grid, identical on desktop.
export function InfoGrid({
  fields,
  mobileLayout = "stacked",
}: {
  fields: InfoField[];
  mobileLayout?: "stacked" | "inline";
}) {
  if (fields.length === 0) return null;
  const inline = mobileLayout === "inline";
  return (
    <dl
      className={cn(
        "flex flex-col text-sm md:grid md:grid-cols-[auto_1fr] md:gap-x-6 md:gap-y-1",
        inline ? "gap-2" : "gap-3",
      )}
    >
      {fields.map((field, i) => (
        <div
          key={i}
          className={cn(
            "md:contents",
            inline ? "flex items-baseline justify-between gap-3" : "flex flex-col gap-0.5",
          )}
        >
          <dt className="text-muted-foreground">{field.label}</dt>
          <dd
            className={cn("flex", inline && "justify-end text-right md:justify-start md:text-left")}
          >
            {field.value}
          </dd>
        </div>
      ))}
    </dl>
  );
}
