// The wordmark swaps its <img> src on every Appearance change. Both variants
// share one aspect ratio, and the <img> must declare it: the docked sidebar's
// logo is `w-full h-auto`, so without width/height it has no height until the
// new SVG loads and the nav below it jumps ~30px (#823).
import { it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { ThemeProvider } from "@/theme/ThemeProvider";
import { AppLogo } from "@/components/shell/AppLogo";

// covers: INV-PRESENTATION-30
it("reserves the wordmark's aspect ratio so a theme swap cannot collapse it", () => {
  render(
    <ThemeProvider>
      <AppLogo className="w-full h-auto" />
    </ThemeProvider>,
  );
  const img = screen.getByRole("img");
  expect(Number(img.getAttribute("width"))).toBeGreaterThan(0);
  expect(Number(img.getAttribute("height"))).toBeGreaterThan(0);
  expect(Number(img.getAttribute("width")) / Number(img.getAttribute("height"))).toBeCloseTo(
    439.1 / 117.98,
    1,
  );
});
