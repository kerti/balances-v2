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
    // snapshots; the add-buttons hide on closed positions). A pruned step would
    // otherwise pop up as a stray centered modal.
    const present = steps.filter((s) => !s.element || document.querySelector(s.element));
    if (present.length === 0) return;
    driver({
      showProgress: true,
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
    <Button variant="outline" size="sm" onClick={start} data-testid="help-tour">
      <CircleHelp className="mr-1 size-4" />
      {t("tour.help")}
    </Button>
  );
}
