import { useTranslation } from "react-i18next";
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
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">{t("inflationRatesPage.title")}</h1>
        <p className="text-sm text-muted-foreground">{t("inflationRatesPage.subtitle")}</p>
      </div>

      <InflationRatesCard />
    </div>
  );
}
