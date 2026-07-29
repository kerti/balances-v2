import { useState, type ChangeEvent, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Link } from "react-router";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { cn } from "@/lib/utils";
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
import { BackupPanel } from "@/components/common/BackupPanel";
import { RestorePanel } from "@/components/common/RestorePanel";
import { ErasePanel } from "@/components/common/ErasePanel";
import {
  SettingsControlRow,
  SettingsPanelGroup,
  SettingsRow,
  SettingsTable,
} from "@/components/common/SettingsSurface";
import { Select } from "@/components/ui/select";

// SectionHeading names each block of the Settings home page. Purely visual —
// no route split, unlike the Assets/Liabilities/Investments sidebar groups —
// because none of Profile/Household/Membership/Data hold a browsable list of
// records the way those groups' children do (just a handful of scalar fields
// or single actions each).
function SectionHeading({ children }: { children: ReactNode }) {
  return <h2 className="text-lg font-medium tracking-tight">{children}</h2>;
}

// Every settings control takes one of these two semantic widths — never a
// per-callsite pixel value (#562). The `w-28` / `w-56` / `w-72` this replaces
// were each picked against a desktop card and none was container-aware: at
// 390px a settings card is 326px wide (shell `p-4` + card `px-4`), which left
// up to 138px dead on a row and truncated the longest carry-over option.
//
// `full` is the default and is what almost everything wants: the control tracks
// its container at both widths. `narrow` is for controls whose content is
// inherently short — a three-letter currency code, a percentage — where a 326px
// box would communicate the wrong expected input. Width follows content
// semantics; when neither token fits, add a third token rather than a number.
const CONTROL_WIDTH = {
  full: "w-full",
  narrow: "w-24",
} as const;

// RowError renders a mutation failure inside the row that caused it. Household
// name, reporting currency and assumed inflation all ride the same
// `useUpdateHouseholdSettings()` PATCH; each row holds its own hook instance
// (and so its own mutation state), which is what keeps one failed save from
// printing under all three now that they share a card.
function RowError({ error }: { error: unknown }) {
  return <p className="text-sm text-destructive">{errorMessage(error)}</p>;
}

export function SettingsScreen() {
  const { t } = useTranslation("settings");

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">{t("title")}</h1>
        <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
      </div>

      <SectionHeading>{t("sections.profile")}</SectionHeading>
      <SettingsTable>
        <NicknameRow />
        <LanguageRow />
        <ThemeRow />
        <CarryoverDateRow />
      </SettingsTable>

      <SectionHeading>{t("sections.household")}</SectionHeading>
      <SettingsTable>
        <HouseholdNameRow />
        <ReportingCurrencyRow />
        <MultiCurrencyRow />
        <AssumedInflationRow />
      </SettingsTable>

      <SectionHeading>{t("sections.membership")}</SectionHeading>
      <InviteForm />
      <ReactivationCard />

      <SectionHeading>{t("sections.data")}</SectionHeading>
      <SettingsPanelGroup>
        <BackupPanel />
        <RestorePanel />
        <ErasePanel />
      </SettingsPanelGroup>
    </div>
  );
}

function NicknameRow() {
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
    <SettingsRow
      name={t("nickname.title")}
      description={t("nickname.description", { displayName: me.display_name })}
      htmlFor="nickname"
    >
      <SettingsControlRow
        action={
          <Button variant="outline" onClick={save} disabled={updateMe.isPending || !dirty}>
            {t("common:save")}
          </Button>
        }
      >
        <Input
          id="nickname"
          className={CONTROL_WIDTH.full}
          maxLength={32}
          placeholder={me.display_name}
          value={value}
          onChange={(e) => setDraft(e.target.value)}
        />
      </SettingsControlRow>

      {updateMe.isError && <RowError error={updateMe.error} />}
    </SettingsRow>
  );
}

// LANGUAGE_LABELS maps each supported BCP47 locale to its in-language display
// name. The label is shown in the dropdown regardless of the current UI
// language so a user reading the wrong language can still find their option.
const LANGUAGE_LABELS: Record<Locale, string> = {
  "en-GB": "English",
  "id-ID": "Bahasa Indonesia",
};

function LanguageRow() {
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
    <SettingsRow
      name={t("language.title")}
      description={t("language.description")}
      htmlFor="language"
    >
      <Select
        id="language"
        data-testid="settings-language-select"
        className={CONTROL_WIDTH.full}
        value={locale}
        onChange={onChange}
        disabled={updateMe.isPending}
      >
        {SUPPORTED_LOCALES.map((l) => (
          <option key={l} value={l}>
            {LANGUAGE_LABELS[l]}
          </option>
        ))}
      </Select>
    </SettingsRow>
  );
}

// ThemeRow mirrors LanguageRow: a two-option select bound to the active theme.
// Selecting optimistically applies the theme (useTheme persists to localStorage
// + toggles the `dark` class on <html>); the PATCH persists the choice
// server-side so it follows the user across devices. Labels come from the
// catalog so they render in the current UI language.
function ThemeRow() {
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
    <SettingsRow name={t("theme.title")} description={t("theme.description")} htmlFor="theme">
      <Select
        id="theme"
        data-testid="settings-theme-select"
        className={CONTROL_WIDTH.full}
        value={theme}
        onChange={onChange}
        disabled={updateMe.isPending}
      >
        {SUPPORTED_THEMES.map((th) => (
          <option key={th} value={th}>
            {t(`theme.${th}`)}
          </option>
        ))}
      </Select>
    </SettingsRow>
  );
}

// CarryoverDateRow mirrors LanguageRow: a select bound to the user's carryover
// date-mode preference (issue #105), governing the as-of date the snapshot
// carryover dialogs pre-fill. Unlike theme/locale there is no local UI effect to
// apply optimistically — the value only feeds those dialogs — so the select
// reads straight from the session and the PATCH (autosave, toast confirmation,
// ADR-0032) refreshes it. Labels come from the catalog so they render in the
// current UI language.
function CarryoverDateRow() {
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
    <SettingsRow
      name={t("carryoverDate.title")}
      description={t("carryoverDate.description")}
      htmlFor="carryover-date-mode"
    >
      <Select
        id="carryover-date-mode"
        data-testid="settings-carryover-date-select"
        className={CONTROL_WIDTH.full}
        value={me.carryover_date_mode}
        onChange={onChange}
        disabled={updateMe.isPending}
      >
        {SUPPORTED_CARRYOVER_DATE_MODES.map((m) => (
          <option key={m} value={m}>
            {t(`carryoverDate.modes.${m}`)}
          </option>
        ))}
      </Select>
    </SettingsRow>
  );
}

// HouseholdNameRow mirrors NicknameRow: a button-driven rename, editable by
// any member (#265 — Founder is creation-lineage only, per CONTEXT, not a
// permission tier). It rides the same full-replace PATCH as the currency rows,
// so the mutation carries the household's current currency settings through
// unchanged.
function HouseholdNameRow() {
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
    <SettingsRow
      name={t("householdName.title")}
      description={t("householdName.description")}
      htmlFor="household-name"
    >
      <SettingsControlRow
        action={
          <Button variant="outline" onClick={save} disabled={updateSettings.isPending || !dirty}>
            {t("common:save")}
          </Button>
        }
      >
        <Input
          id="household-name"
          className={CONTROL_WIDTH.full}
          maxLength={60}
          value={value}
          onChange={(e) => setDraft(e.target.value)}
        />
      </SettingsControlRow>

      {updateSettings.isError && <RowError error={updateSettings.error} />}
    </SettingsRow>
  );
}

// ReportingCurrencyRow keeps a visible field label: "Reporting currency" says
// which of a household's currencies this one is, which the row name "Currency"
// on its own does not.
function ReportingCurrencyRow() {
  const { t } = useTranslation(["settings", "common"]);
  const { data: me } = useSession();
  const updateSettings = useUpdateHouseholdSettings();
  const [currency, setCurrency] = useState<string | null>(null);

  if (!me) return null;

  const reportingCurrency = (currency ?? me.reporting_currency).toUpperCase();
  const dirty = reportingCurrency !== me.reporting_currency.toUpperCase();

  const save = () =>
    updateSettings.mutate(
      {
        display_name: me.household_display_name,
        reporting_currency: reportingCurrency,
        multi_currency_enabled: me.multi_currency_enabled,
        assumed_annual_inflation: me.assumed_annual_inflation,
      },
      { onSuccess: () => setCurrency(null) },
    );

  return (
    <SettingsRow name={t("currency.title")} description={t("currency.description")}>
      <SettingsControlRow
        action={
          <Button
            variant="outline"
            onClick={save}
            disabled={updateSettings.isPending || !dirty || reportingCurrency.length !== 3}
          >
            {t("common:save")}
          </Button>
        }
      >
        <Label htmlFor="reporting-currency">{t("currency.reportingLabel")}</Label>
        <Input
          id="reporting-currency"
          className={cn(CONTROL_WIDTH.narrow, "uppercase")}
          maxLength={3}
          value={reportingCurrency}
          onChange={(e) => setCurrency(e.target.value)}
        />
      </SettingsControlRow>

      {updateSettings.isError && <RowError error={updateSettings.error} />}
    </SettingsRow>
  );
}

// MultiCurrencyRow promotes the multi-currency toggle out from under the
// reporting-currency input, where it read as a footnote to that field rather
// than the household-shaping switch it is (ADR-0002). It is buttonless — the
// checkbox PATCHes on change — so the row name carries the setting and the
// control keeps a short label of its own; the checkbox is the affordance but
// the whole label is the hit area (`max-md:min-h-11` lifts it to the tap floor
// on phones without padding the desktop form out, INV-PRESENTATION-08).
function MultiCurrencyRow() {
  const { t } = useTranslation("settings");
  const { data: me } = useSession();
  const updateSettings = useUpdateHouseholdSettings();

  if (!me) return null;

  const toggle = (enabled: boolean) =>
    updateSettings.mutate({
      display_name: me.household_display_name,
      reporting_currency: me.reporting_currency,
      multi_currency_enabled: enabled,
      assumed_annual_inflation: me.assumed_annual_inflation,
    });

  return (
    <SettingsRow name={t("currency.multiTitle")} description={t("currency.multiDescription")}>
      <label className="flex max-md:min-h-11 items-center gap-2 text-sm">
        <input
          type="checkbox"
          className="h-4 w-4"
          data-testid="multi-currency-toggle"
          checked={me.multi_currency_enabled}
          disabled={updateSettings.isPending}
          onChange={(e) => toggle(e.target.checked)}
        />
        {t("currency.multiLabel")}
      </label>

      {updateSettings.isError && <RowError error={updateSettings.error} />}

      {me.multi_currency_enabled && (
        <Link
          to={routes.settingsFxRates}
          className="inline-flex max-md:min-h-11 items-center text-sm underline"
        >
          {t("currency.manageFx")}
        </Link>
      )}
    </SettingsRow>
  );
}

// AssumedInflationRow holds only the assumed-annual-inflation setting (the Fund
// Resilience fallback, ADR-0048) — a single household preference, so it stays a
// row on the Settings home page. The monthly lookup table lives on its own
// subpage (routes.settingsInflationRates), linked from here. Like the currency
// row it keeps a visible field label: "Assumed annual %" carries both a unit and
// a qualifier that the row name "Inflation" does not.
function AssumedInflationRow() {
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
    <SettingsRow name={t("inflation.title")} description={t("inflation.description")}>
      <SettingsControlRow
        action={
          <Button
            variant="outline"
            onClick={saveAssumed}
            disabled={updateSettings.isPending || !assumedDirty}
          >
            {t("common:save")}
          </Button>
        }
      >
        <Label htmlFor="assumed-inflation">{t("inflation.assumedLabel")}</Label>
        <Input
          id="assumed-inflation"
          inputMode="decimal"
          className={CONTROL_WIDTH.narrow}
          value={assumedValue}
          onChange={(e) => setAssumed(e.target.value)}
        />
      </SettingsControlRow>

      {updateSettings.isError && <RowError error={updateSettings.error} />}

      <Link
        to={routes.settingsInflationRates}
        className="inline-flex max-md:min-h-11 items-center text-sm underline"
      >
        {t("inflation.manageRates")}
      </Link>
    </SettingsRow>
  );
}
