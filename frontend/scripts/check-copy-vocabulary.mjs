// Permanent guard (#832): UI copy keeps one vocabulary. Locale VALUES (never keys)
// must not say "position", "terminate", "lifecycle", "NAV", "disposition",
// "incidental", "routine" or "native currency" — nor Bahasa "posisi". Name the
// subtype, or fall back to "item" (CONTEXT.md, UI vocabulary). Run by `npm run lint`.
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";

const BANNED =
  /\b(positions?|terminat\w*|lifecycle|NAV|dispositions?|incidental|routine|native currency|posisi)\b/i;
// The one hint #832 itself asks for: "the NAV your fund manager publishes".
const ALLOW = new Set(["en/investments.json:mutualFund.snapshotsDescription"]);

const root = new URL("../src/locales/", import.meta.url).pathname;
let bad = 0;
for (const locale of readdirSync(root, { withFileTypes: true })
  .filter((e) => e.isDirectory())
  .map((e) => e.name)) {
  for (const file of readdirSync(join(root, locale)).filter((f) => f.endsWith(".json"))) {
    const where = `${locale}/${file}`;
    const walk = (node, key) => {
      if (typeof node === "string") {
        if (BANNED.test(node) && !ALLOW.has(`${where}:${key}`)) {
          console.error(`src/locales/${where}: ${key} uses retired vocabulary (#832)\n  ${node}`);
          bad++;
        }
        return;
      }
      for (const [k, v] of Object.entries(node)) walk(v, key ? `${key}.${k}` : k);
    };
    walk(JSON.parse(readFileSync(join(root, locale, file), "utf8")), "");
  }
}
if (bad) process.exit(1);
