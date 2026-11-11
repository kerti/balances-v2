// Shows a position's identifier (a bank account number) without spelling it out
// in full: the last four characters behind a bullet run, so the person can tell
// two accounts apart in an import preview without the whole number sitting on
// screen. A value of four characters or fewer would be fully revealed by "last
// four", so it is masked entirely.
const KEEP = 4;
const BULLET = "•";

export function maskIdentifier(raw: string): string {
  const s = raw.trim();
  if (s === "") return "";
  if (s.length <= KEEP) return BULLET.repeat(s.length);
  return `${BULLET.repeat(KEEP)} ${s.slice(-KEEP)}`;
}
