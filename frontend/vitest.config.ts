import path from "node:path";
import { defineConfig } from "vitest/config";

// Two projects share one runner. The `unit` project is the original pure-helper
// tier (ADR-0021): `lib/*` logic, `environment: 'node'`, no DOM. The `dom`
// project is the component tier activated by the descriptor-driven list screens
// (ADR-0043, #69): jsdom + React Testing Library + MSW, driven by
// `src/test/setup.ts`. Split by extension — `.test.ts` is node, `.test.tsx` is
// jsdom — so a helper test never pays for a DOM and a component test always
// gets one. The E2E suite (Playwright) stays out of this runner and out of the
// coverage metric.
const alias = { "@": path.resolve(__dirname, "./src") };

export default defineConfig({
  resolve: { alias },
  test: {
    // The two named projects below do all the collecting; scope the implicit
    // root project to nothing so vitest's default glob doesn't sweep in the
    // Playwright specs under `e2e/`.
    include: [],
    coverage: {
      provider: "v8",
      reporter: ["text", "lcov"],
      // Scope to hand-written source under test; widen as more is covered.
      include: ["src/lib/**", "src/components/positionList/**"],
    },
    projects: [
      {
        resolve: { alias },
        test: {
          name: "unit",
          environment: "node",
          include: ["src/**/*.test.ts"],
        },
      },
      {
        resolve: { alias },
        test: {
          name: "dom",
          environment: "jsdom",
          include: ["src/**/*.test.tsx"],
          setupFiles: ["./src/test/setup.ts"],
        },
      },
    ],
  },
});
