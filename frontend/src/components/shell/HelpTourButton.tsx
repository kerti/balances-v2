import { useTranslation } from "react-i18next";
import { driver } from "driver.js";
import "driver.js/dist/driver.css";
import { CircleHelp } from "lucide-react";
import { Button } from "@/components/ui/button";

// One step of a screen tour. `element` is a CSS selector (we anchor to
// data-testid attributes, mirroring the E2E convention); omit it for a
// centered modal step. Callers pass already-translated copy so this stays
// i18n-agnostic and reusable across every detail screen (issue #23).
export type TourStep = {
  element?: string;
  title: string;
  description: string;
};

type Props = {
  steps: TourStep[];
};

export function HelpTourButton({ steps }: Props) {
  const { t } = useTranslation("common");

  function start() {
    // Skip steps whose target isn't rendered this visit (the chart needs ≥2
    // snapshots; the add-buttons hide on closed positions) or is on screen only
    // when another tab is open (#839: the inactive Snapshots / Transactions
    // panel is display:none). A pruned step would otherwise pop up as a stray
    // centered modal.
    const present = steps.filter((s) => {
      if (!s.element) return true;
      const el = document.querySelector(s.element);
      return !!el && el.getClientRects().length > 0;
    });
    if (present.length === 0) return;
    driver({
      showProgress: true,
      popoverClass: "app-tour",
      nextBtnText: t("tour.next"),
      prevBtnText: t("tour.back"),
      doneBtnText: t("tour.done"),
      // driver.js substitutes {{current}}/{{total}} itself; feed the tokens
      // back as i18next values so its own interpolation leaves them intact.
      progressText: t("tour.progress", {
        current: "{{current}}",
        total: "{{total}}",
      }),
      steps: present.map((s) => ({
        element: s.element,
        popover: { title: s.title, description: s.description },
      })),
    }).drive();
  }

  return (
    // Icon-only `?` (#839): the detail header is Help · Edit · ⋯ on one row, so
    // Help spends no label width. `size-11` is the phone tap floor.
    <Button
      variant="outline"
      size="icon"
      className="max-md:size-11"
      onClick={start}
      aria-label={t("tour.help")}
      data-testid="help-tour"
    >
      <CircleHelp />
    </Button>
  );
}
