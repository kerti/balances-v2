type NavLeaf = { to: string };
type NavSection = { to: string; children?: NavLeaf[] };

const under = (pathname: string, to: string) => pathname === to || pathname.startsWith(to + "/");

export function isLeafActive(pathname: string, to: string): boolean {
  return under(pathname, to);
}

// A section with children yields to the child that claims the path; it
// highlights itself on its own path and on any page beneath it that no child
// claims (the bulk-entry routes, e.g. /assets/enter).
export function isSectionActive(pathname: string, section: NavSection): boolean {
  if (!section.children) return under(pathname, section.to);
  if (pathname === section.to) return true;
  return under(pathname, section.to) && !section.children.some((c) => under(pathname, c.to));
}
