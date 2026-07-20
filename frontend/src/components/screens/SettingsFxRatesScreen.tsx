import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { useSession } from "@/hooks/useSession";
import { routes } from "@/lib/routes";
import { FxRatesCard } from "@/components/common/FxRatesCard";

// Settings ▸ Exchange Rates (routes.settingsFxRates). h1 + subtitle header,
// same convention as every other routed screen (TagsScreen, SettingsScreen,
// PositionListScreen, …) — FxRatesCard itself carries no title/description of
// its own. FX rates are meaningless without multi-currency on, so a disabled
// household gets a pointer back to the Currency toggle instead of a dead-end
// CRUD form (the header still renders either way).
export function SettingsFxRatesScreen() {
  const { t } = useTranslation("settings");
  const { data: me } = useSession();

  if (!me) return null;

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">{t("fx.title")}</h1>
        <p className="text-sm text-muted-foreground">{t("fx.description")}</p>
      </div>

      {me.multi_currency_enabled ? (
        <FxRatesCard />
      ) : (
        <p className="text-sm text-muted-foreground">
          {t("fx.disabledHint")}{" "}
          <Link to={routes.settings} className="underline">
            {t("fx.disabledLink")}
          </Link>
        </p>
      )}
    </div>
  );
}
