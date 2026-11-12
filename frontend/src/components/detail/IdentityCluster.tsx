// The details card's identity line (#839): the position's identifiers on one
// muted, dot-separated line under the card title — "BCA · 1234 · Savings",
// "ASII · IDX", "Bar · 24K". Nullish pieces drop out (and take their dot with
// them), so a descriptor can pass optional pieces (an address, a plate) without
// guarding. The line wraps on a narrow card; the dots are decorative.
export function IdentityCluster({ lines }: { lines: (string | null | undefined)[] }) {
  const visible = lines.filter((line): line is string => Boolean(line));
  if (visible.length === 0) return null;
  return (
    <p className="text-sm text-muted-foreground" data-testid="identity-line">
      {visible.map((line, i) => (
        <span key={i}>
          {i > 0 && <span aria-hidden="true"> · </span>}
          <span>{line}</span>
        </span>
      ))}
    </p>
  );
}
