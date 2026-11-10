import { afterEach, describe, expect, it } from "vitest";
import { renderHook, screen } from "@testing-library/react";
import { renderWithProviders } from "@/test/renderWithProviders";
import { MemoryRouter } from "react-router";
import { AppSidebar } from "@/components/shell/AppSidebar";
import { ThemeProvider } from "@/theme/ThemeProvider";
import { SidebarProvider } from "@/components/ui/sidebar";
import { useIsMobile, useIsShellCompact } from "@/hooks/use-mobile";

const originalWidth = window.innerWidth;

function setWidth(w: number) {
  Object.defineProperty(window, "innerWidth", { configurable: true, writable: true, value: w });
}

afterEach(() => setWidth(originalWidth));

function renderSidebar() {
  return renderWithProviders(
    <MemoryRouter>
      <ThemeProvider>
        <SidebarProvider>
          <AppSidebar />
        </SidebarProvider>
      </ThemeProvider>
    </MemoryRouter>,
  );
}

describe("shell boundary (ADR-0050 amendment, 1024px)", () => {
  it.each([767, 800, 1000, 1023])("hides the sidebar behind the trigger at %ipx", (w) => {
    setWidth(w);
    renderSidebar();
    expect(screen.queryByRole("link", { name: "Settings" })).not.toBeInTheDocument();
  });

  it.each([1024, 1280])("docks the sidebar at %ipx", (w) => {
    setWidth(w);
    renderSidebar();
    expect(screen.getByRole("link", { name: "Settings" })).toBeInTheDocument();
  });

  it("keeps the content renderer boundary at 768px", () => {
    setWidth(800);
    expect(renderHook(() => useIsMobile()).result.current).toBe(false);
    expect(renderHook(() => useIsShellCompact()).result.current).toBe(true);
    setWidth(767);
    expect(renderHook(() => useIsMobile()).result.current).toBe(true);
  });

  it("lists Settings as a single entry without children", () => {
    setWidth(1280);
    renderSidebar();
    expect(screen.queryByRole("link", { name: /exchange rates/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /inflation rates/i })).not.toBeInTheDocument();
  });
});
