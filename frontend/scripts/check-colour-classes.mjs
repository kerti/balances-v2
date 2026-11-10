// Permanent guard (#835, ADR-0054 amendment): raw red/green/amber/emerald/rose
// Tailwind colour classes are banned outside the allowlist; use the semantic
// tokens (--positive, --negative, --warning, --primary) instead. Run by `npm run lint`.
import { readdirSync, readFileSync } from "node:fs";
import { join, relative } from "node:path";

// components/ui = vendored shadcn; the rest are category/risk palettes (#836 reworks them).
const ALLOW = [
  "src/components/ui/",
  "src/components/common/RiskProfileBadge.tsx",
  "src/components/common/StatusBadge.tsx",
];
const BANNED =
  /\b(?:text|bg|border|ring|fill|stroke|from|to|via|outline|divide|decoration|accent|shadow|caret|placeholder)-(?:amber|emerald|green|rose|red)-\d/;

const root = new URL("..", import.meta.url).pathname;
function* walk(dir) {
  for (const e of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, e.name);
    if (e.isDirectory()) yield* walk(p);
    else if (/\.(ts|tsx)$/.test(e.name)) yield p;
  }
}

let bad = 0;
for (const file of walk(join(root, "src"))) {
  const rel = relative(root, file);
  if (ALLOW.some((a) => rel.startsWith(a))) continue;
  readFileSync(file, "utf8")
    .split("\n")
    .forEach((line, i) => {
      const code = line.replace(/(^|\s)\/\/.*$/, "");
      if (/^\s*(\*|\/\*)/.test(code)) return;
      if (BANNED.test(code)) {
        console.error(`${rel}:${i + 1}: raw colour class; use a semantic token\n  ${line.trim()}`);
        bad++;
      }
    });
}
if (bad) process.exit(1);
