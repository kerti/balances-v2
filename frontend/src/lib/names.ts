// preferredName resolves the compact label for a person — their self-set
// nickname when present, otherwise the (Google-sourced) display_name. The
// backend stores NULL rather than an empty nickname, but we guard against
// blank/whitespace anyway so a stray "" never renders as an empty label.
export function preferredName(person: { nickname?: string | null; display_name: string }): string {
  const nick = person.nickname?.trim();
  return nick ? nick : person.display_name;
}

// initials derives a 1–2 character avatar fallback from a name: first + last
// word initial for multi-word names, the single initial for one word, and "?"
// for an empty/blank name. Always uppercased.
export function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return "?";
  if (parts.length === 1) return parts[0]!.charAt(0).toUpperCase();
  return (parts[0]!.charAt(0) + parts[parts.length - 1]!.charAt(0)).toUpperCase();
}
