import { useState, type ChangeEvent } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { errorMessage } from "@/lib/errorMessage";
import { useSession } from "@/hooks/useSession";
import { useUpdateMe } from "@/hooks/useUpdateMe";
import { useUpdateHouseholdSettings } from "@/hooks/useHouseholdSettings";
import { SUPPORTED_LOCALES, type Locale } from "@/i18n";
import { useLocale } from "@/i18n/useLocale";
import { SUPPORTED_THEMES, type Theme } from "@/theme";
import { useTheme } from "@/theme/useTheme";
import {
  SUPPORTED_CARRYOVER_DATE_MODES,
  type CarryoverDateMode,
} from "@/lib/dateLimits";
import {
  useFxRates,
  useCreateFxRate,
  useDeleteFxRate,
} from "@/hooks/useFxRates";
import { formatYearMonth } from "@/lib/format";
import { InviteForm } from "@/components/InviteForm";
import { ReactivationCard } from "@/components/ReactivationCard";
import { TagsCard } from "@/components/TagsCard";
import { BackupCard } from "@/components/BackupCard";
import { RestoreCard } from "@/components/RestoreCard";

export function SettingsScreen() {
  const { t } = useTranslation(["settings", "common"]);
  const { data: me } = useSession();
  const updateSettings = useUpdateHouseholdSettings();
  const [currency, setCurrency] = useState<string | null>(null);

  if (!me) return null;

  const reportingCurrency = (currency ?? me.reporting_currency).toUpperCase();

  const saveCurrency = () =>
    updateSettings.mutate({
      reporting_currency: reportingCurrency,
      multi_currency_enabled: me.multi_currency_enabled,
    });

  const toggleMulti = (enabled: boolean) =>
    updateSettings.mutate({
      reporting_currency: me.reporting_currency,
      multi_currency_enabled: enabled,
    });

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">{t("title")}</h1>
        <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
      </div>

      <NicknameCard />

      <LanguageCard />

      <ThemeCard />

      <CarryoverDateCard />

      <TagsCard />

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("currency.title")}</CardTitle>
          <CardDescription>{t("currency.description")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-end gap-3">
            <div className="space-y-1">
              <Label htmlFor="reporting-currency">
                {t("currency.reportingLabel")}
              </Label>
              <Input
                id="reporting-currency"
                className="w-28 uppercase"
                maxLength={3}
                value={reportingCurrency}
                onChange={(e) => setCurrency(e.target.value)}
              />
            </div>
            <Button
              variant="outline"
              onClick={saveCurrency}
              disabled={
                updateSettings.isPending || reportingCurrency.length !== 3
              }
            >
              {t("common:save")}
            </Button>
          </div>

          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              className="h-4 w-4"
              checked={me.multi_currency_enabled}
              disabled={updateSettings.isPending}
              onChange={(e) => toggleMulti(e.target.checked)}
            />
            {t("currency.multiToggle")}
          </label>

          {updateSettings.isError && (
            <p className="text-sm text-destructive">
              {errorMessage(updateSettings.error)}
            </p>
          )}
        </CardContent>
      </Card>

      {me.multi_currency_enabled && <FxRatesCard />}

      <InviteForm />

      <ReactivationCard />

      <BackupCard />

      <RestoreCard />
    </div>
  );
}

function NicknameCard() {
  const { t } = useTranslation(["settings", "common"]);
  const { data: me } = useSession();
  const updateMe = useUpdateMe();
  const [draft, setDraft] = useState<string | null>(null);

  if (!me) return null;

  // `draft ?? me.nickname ?? ''` — null draft means "untouched, show current";
  // once the user types, draft is a string (even "") and wins.
  const value = draft ?? me.nickname ?? "";
  const trimmed = value.trim();
  const current = me.nickname ?? "";
  const dirty = trimmed !== current;

  const save = () =>
    updateMe.mutate(
      { nickname: trimmed === "" ? null : trimmed },
      { onSuccess: () => setDraft(null) },
    );

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("nickname.title")}</CardTitle>
        <CardDescription>
          {t("nickname.description", { displayName: me.display_name })}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex items-end gap-3">
          <div className="space-y-1">
            <Label htmlFor="nickname">{t("nickname.label")}</Label>
            <Input
              id="nickname"
              className="w-56"
              maxLength={32}
              placeholder={me.display_name}
              value={value}
              onChange={(e) => setDraft(e.target.value)}
            />
          </div>
          <Button
            variant="outline"
            onClick={save}
            disabled={updateMe.isPending || !dirty}
          >
            {t("common:save")}
          </Button>
        </div>

        {updateMe.isError && (
          <p className="text-sm text-destructive">
            {errorMessage(updateMe.error)}
          </p>
        )}
      </CardContent>
    </Card>
  );
}

// LANGUAGE_LABELS maps each supported BCP47 locale to its in-language display
// name. The label is shown in the dropdown regardless of the current UI
// language so a user reading the wrong language can still find their option.
const LANGUAGE_LABELS: Record<Locale, string> = {
  "en-GB": "English",
  "id-ID": "Bahasa Indonesia",
};

function LanguageCard() {
  const { t } = useTranslation("settings");
  const { data: me } = useSession();
  const { locale, setLocale } = useLocale();
  const updateMe = useUpdateMe();

  if (!me) return null;

  // Buttonless autosave (issue #54, ADR-0032): the select mutates on change
  // with no Save button, so a toast confirms the write. Optimistically switch
  // the UI; on PATCH failure roll the language back and surface the error.
  const onChange = (e: ChangeEvent<HTMLSelectElement>) => {
    const next = e.target.value as Locale;
    const previous = locale as Locale;
    void setLocale(next);
    updateMe.mutate(
      { locale: next },
      {
        // Resolve the confirmation in the newly-chosen language (`lng: next`),
        // not whatever i18next has finished switching to when the PATCH lands.
        onSuccess: () => toast.success(t("language.saved", { lng: next })),
        onError: (err) => {
          void setLocale(previous);
          toast.error(errorMessage(err));
        },
      },
    );
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("language.title")}</CardTitle>
        <CardDescription>{t("language.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex items-end gap-3">
          <div className="space-y-1">
            <Label htmlFor="language">{t("language.label")}</Label>
            <select
              id="language"
              data-testid="settings-language-select"
              className="flex h-9 w-56 rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus-visible:ring-1 focus-visible:ring-ring focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
              value={locale}
              onChange={onChange}
              disabled={updateMe.isPending}
            >
              {SUPPORTED_LOCALES.map((l) => (
                <option key={l} value={l}>
                  {LANGUAGE_LABELS[l]}
                </option>
              ))}
            </select>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

// ThemeCard mirrors LanguageCard: a two-option select bound to the active
// theme. Selecting optimistically applies the theme (useTheme persists to
// localStorage + toggles the `dark` class on <html>); the PATCH persists the
// choice server-side so it follows the user across devices. Labels come from
// the catalog so they render in the current UI language.
function ThemeCard() {
  const { t } = useTranslation("settings");
  const { data: me } = useSession();
  const { theme, setTheme } = useTheme();
  const updateMe = useUpdateMe();

  if (!me) return null;

  // Buttonless autosave (issue #54, ADR-0032): toast confirms the write; on
  // PATCH failure roll the theme back to its previous value.
  const onChange = (e: ChangeEvent<HTMLSelectElement>) => {
    const next = e.target.value as Theme;
    const previous = theme;
    setTheme(next);
    updateMe.mutate(
      { theme: next },
      {
        onSuccess: () => toast.success(t("theme.saved")),
        onError: (err) => {
          setTheme(previous);
          toast.error(errorMessage(err));
        },
      },
    );
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("theme.title")}</CardTitle>
        <CardDescription>{t("theme.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex items-end gap-3">
          <div className="space-y-1">
            <Label htmlFor="theme">{t("theme.label")}</Label>
            <select
              id="theme"
              data-testid="settings-theme-select"
              className="flex h-9 w-56 rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus-visible:ring-1 focus-visible:ring-ring focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
              value={theme}
              onChange={onChange}
              disabled={updateMe.isPending}
            >
              {SUPPORTED_THEMES.map((th) => (
                <option key={th} value={th}>
                  {t(`theme.${th}`)}
                </option>
              ))}
            </select>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

// CarryoverDateCard mirrors LanguageCard: a select bound to the user's
// carryover date-mode preference (issue #105), governing the as-of date the
// snapshot carryover dialogs pre-fill. Unlike theme/locale there is no local UI
// effect to apply optimistically — the value only feeds those dialogs — so the
// select reads straight from the session and the PATCH (autosave, toast
// confirmation, ADR-0032) refreshes it. Labels come from the catalog so they
// render in the current UI language.
function CarryoverDateCard() {
  const { t } = useTranslation("settings");
  const { data: me } = useSession();
  const updateMe = useUpdateMe();

  if (!me) return null;

  const onChange = (e: ChangeEvent<HTMLSelectElement>) => {
    const next = e.target.value as CarryoverDateMode;
    updateMe.mutate(
      { carryover_date_mode: next },
      {
        onSuccess: () => toast.success(t("carryoverDate.saved")),
        onError: (err) => toast.error(errorMessage(err)),
      },
    );
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("carryoverDate.title")}</CardTitle>
        <CardDescription>{t("carryoverDate.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex items-end gap-3">
          <div className="space-y-1">
            <Label htmlFor="carryover-date-mode">
              {t("carryoverDate.label")}
            </Label>
            <select
              id="carryover-date-mode"
              data-testid="settings-carryover-date-select"
              className="flex h-9 w-72 rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus-visible:ring-1 focus-visible:ring-ring focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
              value={me.carryover_date_mode}
              onChange={onChange}
              disabled={updateMe.isPending}
            >
              {SUPPORTED_CARRYOVER_DATE_MODES.map((m) => (
                <option key={m} value={m}>
                  {t(`carryoverDate.modes.${m}`)}
                </option>
              ))}
            </select>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

function FxRatesCard() {
  const { t } = useTranslation(["settings", "common"]);
  const { data: rates, isPending } = useFxRates();
  const createRate = useCreateFxRate();
  const deleteRate = useDeleteFxRate();

  const [month, setMonth] = useState("");
  const [currency, setCurrency] = useState("");
  const [rate, setRate] = useState("");

  const add = () => {
    createRate.mutate(
      { year_month: month, currency: currency.toUpperCase(), rate },
      {
        onSuccess: () => {
          setMonth("");
          setCurrency("");
          setRate("");
        },
      },
    );
  };

  const canAdd =
    month !== "" && currency.length === 3 && rate !== "" && Number(rate) > 0;

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("fx.title")}</CardTitle>
        <CardDescription>{t("fx.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex flex-wrap items-end gap-3">
          <div className="space-y-1">
            <Label htmlFor="fx-month">{t("fx.month")}</Label>
            <Input
              id="fx-month"
              type="month"
              className="w-40"
              value={month}
              onChange={(e) => setMonth(e.target.value)}
            />
          </div>
          <div className="space-y-1">
            <Label htmlFor="fx-currency">{t("fx.currency")}</Label>
            <Input
              id="fx-currency"
              className="w-24 uppercase"
              maxLength={3}
              // ISO currency code — a data token, not translatable copy.
              placeholder={"USD"}
              value={currency}
              onChange={(e) => setCurrency(e.target.value)}
            />
          </div>
          <div className="space-y-1">
            <Label htmlFor="fx-rate">{t("fx.rate")}</Label>
            <Input
              id="fx-rate"
              inputMode="decimal"
              className="w-36"
              // Example numeric rate; not translatable copy.
              placeholder={"16000"}
              value={rate}
              onChange={(e) => setRate(e.target.value)}
            />
          </div>
          <Button onClick={add} disabled={!canAdd || createRate.isPending}>
            {t("fx.addRate")}
          </Button>
        </div>

        {createRate.isError && (
          <p className="text-sm text-destructive">
            {errorMessage(createRate.error)}
          </p>
        )}

        {isPending && (
          <p className="text-sm text-muted-foreground">{t("common:loading")}</p>
        )}

        {rates && rates.length === 0 && (
          <p className="text-sm text-muted-foreground">{t("fx.empty")}</p>
        )}

        {rates && rates.length > 0 && (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("fx.month")}</TableHead>
                <TableHead>{t("fx.currency")}</TableHead>
                <TableHead>{t("fx.rate")}</TableHead>
                <TableHead className="w-16"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rates.map((r) => (
                <TableRow key={r.id}>
                  <TableCell>{formatYearMonth(r.year_month)}</TableCell>
                  <TableCell>{r.currency}</TableCell>
                  <TableCell className="tabular-nums">{r.rate}</TableCell>
                  <TableCell>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => deleteRate.mutate(r.id)}
                    >
                      {t("common:delete")}
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}
