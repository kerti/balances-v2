// A small pill showing a Tag's colour dot + name. Used in the breakdown
// report and anywhere a position's tag is surfaced. Colour-only fill would
// fail contrast across themes, so the dot carries the hue and the text stays
// in the foreground colour.
type Props = {
  name: string;
  color: string;
  className?: string;
};

export function TagBadge({ name, color, className }: Props) {
  return (
    <span
      data-testid="tag-badge"
      className={`inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-xs ${className ?? ""}`}
    >
      <span
        className="size-2 shrink-0 rounded-full"
        style={{ backgroundColor: color }}
        aria-hidden
      />
      {name}
    </span>
  );
}
