import { useState, type ChangeEvent, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Link } from "react-router";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { errorMessage } from "@/lib/errorMessage";
import { useSession } from "@/hooks/useSession";
import { useUpdateMe } from "@/hooks/useUpdateMe";
import { useUpdateHouseholdSettings } from "@/hooks/useHouseholdSettings";
import { SUPPORTED_LOCALES, type Locale } from "@/i18n";
import { useLocale } from "@/i18n/useLocale";
import { SUPPORTED_THEMES, type Theme } from "@/theme";
import { useTheme } from "@/theme/useTheme";
import { SUPPORTED_CARRYOVER_DATE_MODES, type CarryoverDateMode } from "@/lib/dateLimits";
import { routes } from "@/lib/routes";
import { InviteForm } from "@/components/common/InviteForm";
import { ReactivationCard } from "@/components/common/ReactivationCard";
import { BackupCard } from "@/components/common/BackupCard";
import { RestoreCard } from "@/components/common/RestoreCard";
import { EraseCard } from "@/components/common/EraseCard";

// SectionHeading groups related cards on the Settings home page. Purely
// visual — no route split, unlike the Assets/Liabilities/Investments sidebar
// groups — because none of Profile/Household/Membership/Data hold a
// browsable list of records the way those groups' children do (just a
// handful of scalar fields or single actions each).
function SectionHeading({ children }: { children: ReactNode }) {
  return <h2 className="text-lg font-medium tracking-tight">{children}</h2>;
}

export function SettingsScreen() {
  const { t } = useTranslation(["settings", "common"]);
  const { data: me } = useSession();
  const updateSettings = useUpdateHouseholdSettings();
  const [currency, setCurrency] = useState<string | null>(null);

  if (!me) return null;

  const reportingCurrency = (currency ?? me.reporting_currency).toUpperCase();

  const saveCurrency = () =>
    updateSettings.mutate({
      display_name: me.household_display_name,
      reporting_currency: reportingCurrency,
      multi_currency_enabled: me.multi_currency_enabled,
      assumed_annual_inflation: me.assumed_annual_inflation,
    });

  const toggleMulti = (enabled: boolean) =>
    updateSettings.mutate({
      display_name: me.household_display_name,
      reporting_currency: me.reporting_currency,
      multi_currency_enabled: enabled,
      assumed_annual_inflation: me.assumed_annual_inflation,
    });

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">{t("title")}</h1>
        <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
      </div>

      <SectionHeading>{t("sections.profile")}</SectionHeading>
      <NicknameCard />
      <LanguageCard />
      <ThemeCard />
      <CarryoverDateCard />

      <SectionHeading>{t("sections.household")}</SectionHeading>
      <HouseholdNameCard />

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("currency.title")}</CardTitle>
          <CardDescription>{t("currency.description")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-end gap-3">
            <div className="space-y-1">
              <Label htmlFor="reporting-currency">{t("currency.reportingLabel")}</Label>
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
              className="min-h-11 md:min-h-0"
              onClick={saveCurrency}
              disabled={updateSettings.isPending || reportingCurrency.length !== 3}
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
            <p className="text-sm text-destructive">{errorMessage(updateSettings.error)}</p>
          )}

          {me.multi_currency_enabled && (
            <Link to={routes.settingsFxRates} className="block text-sm underline">
              {t("currency.manageFx")}
            </Link>
          )}
        </CardContent>
      </Card>

      <AssumedInflationCard />

      <SectionHeading>{t("sections.membership")}</SectionHeading>
      <InviteForm />
      <ReactivationCard />

      <SectionHeading>{t("sections.data")}</SectionHeading>
      <BackupCard />
      <RestoreCard />
      <EraseCard />
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
            className="min-h-11 md:min-h-0"
            onClick={save}
            disabled={updateMe.isPending || !dirty}
          >
            {t("common:save")}
          </Button>
        </div>

        {updateMe.isError && (
          <p className="text-sm text-destructive">{errorMessage(updateMe.error)}</p>
        )}
      </CardContent>
    </Card>
  );
}

// HouseholdNameCard mirrors NicknameCard: a button-driven rename, editable by
// any member (#265 — Founder is creation-lineage only, per CONTEXT, not a
// permission tier). It rides the same full-replace PATCH as the currency card,
// so the mutation carries the household's current currency settings through
// unchanged.
function HouseholdNameCard() {
  const { t } = useTranslation(["settings", "common"]);
  const { data: me } = useSession();
  const updateSettings = useUpdateHouseholdSettings();
  const [draft, setDraft] = useState<string | null>(null);

  if (!me) return null;

  const value = draft ?? me.household_display_name;
  const trimmed = value.trim();
  const dirty = trimmed !== me.household_display_name && trimmed !== "";

  const save = () =>
    updateSettings.mutate(
      {
        display_name: trimmed,
        reporting_currency: me.reporting_currency,
        multi_currency_enabled: me.multi_currency_enabled,
        assumed_annual_inflation: me.assumed_annual_inflation,
      },
      { onSuccess: () => setDraft(null) },
    );

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("householdName.title")}</CardTitle>
        <CardDescription>{t("householdName.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex items-end gap-3">
          <div className="space-y-1">
            <Label htmlFor="household-name">{t("householdName.label")}</Label>
            <Input
              id="household-name"
              className="w-56"
              maxLength={60}
              value={value}
              onChange={(e) => setDraft(e.target.value)}
            />
          </div>
          <Button
            variant="outline"
            className="min-h-11 md:min-h-0"
            onClick={save}
            disabled={updateSettings.isPending || !dirty}
          >
            {t("common:save")}
          </Button>
        </div>

        {updateSettings.isError && (
          <p className="text-sm text-destructive">{errorMessage(updateSettings.error)}</p>
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
            <Label htmlFor="carryover-date-mode">{t("carryoverDate.label")}</Label>
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

// AssumedInflationCard holds only the assumed-annual-inflation setting (the
// Fund Resilience fallback, ADR-0048) — a single household preference, so it
// stays on the Settings home page. The monthly lookup table lives on its own
// subpage (routes.settingsInflationRates), linked from here.
function AssumedInflationCard() {
  const { t } = useTranslation(["settings", "common"]);
  const { data: me } = useSession();
  const updateSettings = useUpdateHouseholdSettings();
  const [assumed, setAssumed] = useState<string | null>(null);

  if (!me) return null;

  const assumedValue = assumed ?? me.assumed_annual_inflation;
  const assumedDirty =
    assumedValue.trim() !== me.assumed_annual_inflation && assumedValue.trim() !== "";

  const saveAssumed = () =>
    updateSettings.mutate(
      {
        display_name: me.household_display_name,
        reporting_currency: me.reporting_currency,
        multi_currency_enabled: me.multi_currency_enabled,
        assumed_annual_inflation: assumedValue.trim(),
      },
      { onSuccess: () => setAssumed(null) },
    );

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("inflation.title")}</CardTitle>
        <CardDescription>{t("inflation.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex items-end gap-3">
          <div className="space-y-1">
            <Label htmlFor="assumed-inflation">{t("inflation.assumedLabel")}</Label>
            <Input
              id="assumed-inflation"
              inputMode="decimal"
              className="w-28"
              value={assumedValue}
              onChange={(e) => setAssumed(e.target.value)}
            />
          </div>
          <Button
            variant="outline"
            className="min-h-11 md:min-h-0"
            onClick={saveAssumed}
            disabled={updateSettings.isPending || !assumedDirty}
          >
            {t("common:save")}
          </Button>
        </div>

        {updateSettings.isError && (
          <p className="text-sm text-destructive">{errorMessage(updateSettings.error)}</p>
        )}

        <Link to={routes.settingsInflationRates} className="block text-sm underline">
          {t("inflation.manageRates")}
        </Link>
      </CardContent>
    </Card>
  );
}
