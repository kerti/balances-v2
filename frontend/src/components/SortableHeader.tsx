import { ChevronDown, ChevronUp, ChevronsUpDown } from "lucide-react";
import { TableHead } from "@/components/ui/table";
import { cn } from "@/lib/utils";

export type SortDir = "asc" | "desc";

type Props = {
  label: string;
  // Whether this column is the active sort key.
  active: boolean;
  dir: SortDir;
  onSort: () => void;
  align?: "left" | "right";
  className?: string;
  testId?: string;
};

// A clickable table header that drives single-column sorting. The neutral
// (inactive) state shows an up/down chevron pair; the active column shows the
// current direction. aria-sort exposes the state to assistive tech, and the
// label sits in a real <button> so it's keyboard-operable.
export function SortableHeader({
  label,
  active,
  dir,
  onSort,
  align = "left",
  className,
  testId,
}: Props) {
  const Icon = !active ? ChevronsUpDown : dir === "asc" ? ChevronUp : ChevronDown;
  return (
    <TableHead
      aria-sort={active ? (dir === "asc" ? "ascending" : "descending") : "none"}
      className={cn(align === "right" && "text-right", className)}
    >
      <button
        type="button"
        onClick={onSort}
        data-testid={testId}
        className={cn(
          "inline-flex items-center gap-1 font-medium hover:text-foreground/70",
          align === "right" && "w-full flex-row-reverse",
        )}
      >
        {label}
        <Icon className={cn("size-3.5", active ? "text-foreground" : "text-muted-foreground")} />
      </button>
    </TableHead>
  );
}
