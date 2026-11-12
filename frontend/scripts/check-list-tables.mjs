// Permanent guard (#838): a table flush with its card edge (`<Table inset>`) is
// a list table, and every list table uses `table-layout: fixed` so its columns
// land at the same x and never reflow. Run by `npm run lint`.
import { readdirSync, readFileSync } from "node:fs";
import { join, relative } from "node:path";

const root = new URL("..", import.meta.url).pathname;
function* walk(dir) {
  for (const e of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, e.name);
    if (e.isDirectory()) yield* walk(p);
    else if (/\.tsx$/.test(e.name)) yield p;
  }
}

let bad = 0;
for (const file of walk(join(root, "src"))) {
  const rel = relative(root, file);
  if (rel.startsWith("src/components/ui/")) continue;
  const src = readFileSync(file, "utf8");
  for (const m of src.matchAll(/<Table\b([^>]*)>/g)) {
    const attrs = m[1];
    if (/(^|\s)inset(\s|=|$)/.test(attrs) && !/\btable-fixed\b/.test(attrs)) {
      const line = src.slice(0, m.index).split("\n").length;
      console.error(`${rel}:${line}: <Table inset> is a list table; add "table-fixed" (#838)`);
      bad++;
    }
  }
}
if (bad) process.exit(1);
