// A tight, label-less identity block for the details card's left column
// (ADR-0051 Phase B). The first non-empty line reads as the primary identifier
// (default weight), the rest muted beneath it — a compact stack that saves the
// vertical space labelled rows would spend. Nullish lines drop out, so a
// descriptor can pass optional pieces (an address, a plate) without guarding.
export function IdentityCluster({ lines }: { lines: (string | null | undefined)[] }) {
  const visible = lines.filter((line): line is string => Boolean(line));
  if (visible.length === 0) return null;
  return (
    <div className="text-sm leading-snug">
      {visible.map((line, i) => (
        <div key={i} className={i === 0 ? "font-medium" : "text-muted-foreground"}>
          {line}
        </div>
      ))}
    </div>
  );
}
