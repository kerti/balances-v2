import { useTranslation } from "react-i18next";
import { PageHeader } from "@/components/common/PageHeader";
import { routes } from "@/lib/routes";
import { InflationRatesCard } from "@/components/common/InflationRatesCard";

// Settings ▸ Inflation Rates (routes.settingsInflationRates). h1 + subtitle
// header, same convention as every other routed screen — InflationRatesCard
// itself carries no title/description of its own. Distinct copy from the
// "inflation" namespace (used by the Settings-home AssumedInflationCard),
// since that card's description already points here — reusing it verbatim
// would read as self-referential on this page.
export function SettingsInflationRatesScreen() {
  const { t } = useTranslation("settings");

  return (
    <div className="space-y-6">
      <PageHeader
        variant="sub-page"
        back={{ label: t("backToSettings"), to: routes.settings }}
        title={t("inflationRatesPage.title")}
        subtitle={t("inflationRatesPage.subtitle")}
      />

      <InflationRatesCard />
    </div>
  );
}
