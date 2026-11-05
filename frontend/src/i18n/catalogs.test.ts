// Smoke-tests the catalog shape: every namespace declared in NAMESPACES has a
// JSON file under src/locales/<lang>/<ns>.json for every supported locale,
// and the en/id key sets stay in lockstep. Catches namespace drift (e.g. a new
// namespace added to the array without its catalog files) before runtime.
// JSON imports work via tsc's resolveJsonModule + Vite's JSON loader.
import codesGo from "../../../backend/internal/httperr/codes.go?raw";
import { describe, expect, it } from "vitest";
import { NAMESPACES, SUPPORTED_LOCALES, type Locale } from "./index";

import enCommon from "../locales/en/common.json";
import enNav from "../locales/en/nav.json";
import enDashboard from "../locales/en/dashboard.json";
import enAssets from "../locales/en/assets.json";
import enLiabilities from "../locales/en/liabilities.json";
import enReceivables from "../locales/en/receivables.json";
import enInvestments from "../locales/en/investments.json";
import enIncome from "../locales/en/income.json";
import enSettings from "../locales/en/settings.json";
import enTags from "../locales/en/tags.json";
import enErrors from "../locales/en/errors.json";
import enOnboarding from "../locales/en/onboarding.json";

import idCommon from "../locales/id/common.json";
import idNav from "../locales/id/nav.json";
import idDashboard from "../locales/id/dashboard.json";
import idAssets from "../locales/id/assets.json";
import idLiabilities from "../locales/id/liabilities.json";
import idReceivables from "../locales/id/receivables.json";
import idInvestments from "../locales/id/investments.json";
import idIncome from "../locales/id/income.json";
import idSettings from "../locales/id/settings.json";
import idTags from "../locales/id/tags.json";
import idErrors from "../locales/id/errors.json";
import idOnboarding from "../locales/id/onboarding.json";

type Catalog = Record<string, unknown>;
// Catalog directories under public/locales/ are 2-letter (i18next's
// load: 'languageOnly' strips the region at request time), so the imports
// resolve 'en' and 'id' regardless of the BCP47 locale identifier the rest
// of the app uses.
const CATALOGS: Record<Locale, Record<(typeof NAMESPACES)[number], Catalog>> = {
  "en-GB": {
    common: enCommon,
    nav: enNav,
    dashboard: enDashboard,
    assets: enAssets,
    liabilities: enLiabilities,
    receivables: enReceivables,
    investments: enInvestments,
    income: enIncome,
    settings: enSettings,
    tags: enTags,
    errors: enErrors,
    onboarding: enOnboarding,
  },
  "id-ID": {
    common: idCommon,
    nav: idNav,
    dashboard: idDashboard,
    assets: idAssets,
    liabilities: idLiabilities,
    receivables: idReceivables,
    investments: idInvestments,
    income: idIncome,
    settings: idSettings,
    tags: idTags,
    errors: idErrors,
    onboarding: idOnboarding,
  },
};

describe("i18n catalogs", () => {
  for (const lng of SUPPORTED_LOCALES) {
    for (const ns of NAMESPACES) {
      it(`${lng}/${ns}.json loads as an object`, () => {
        expect(CATALOGS[lng][ns]).toBeTypeOf("object");
      });
    }
  }

  it("every namespace catalog has the same key set across locales", () => {
    for (const ns of NAMESPACES) {
      const enKeys = Object.keys(CATALOGS["en-GB"][ns]).sort();
      const idKeys = Object.keys(CATALOGS["id-ID"][ns]).sort();
      expect(idKeys, `id/${ns}.json keys diverge from en/${ns}.json`).toEqual(enKeys);
    }
  });
});

// Every wire error code in internal/httperr/codes.go must have copy in both
// error catalogs; otherwise errorMessage() falls back to the generic UNKNOWN
// line and the user sees "Something went wrong" for a failure we can name.
// gen-ts-types doesn't emit the codes, so this reads the Go source.
// covers: INV-CONTRACT-05
describe("backend error codes", () => {
  const codes = [...codesGo.matchAll(/^\s*Code\w+\s+Code\s*=\s*"([A-Z0-9_]+)"/gm)].map((m) => m[1]);

  it("finds the codes it is meant to guard", () => {
    expect(codes.length).toBeGreaterThan(40);
    expect(codes).toContain("ERASURE_DISABLED_DEMO");
  });

  for (const lng of SUPPORTED_LOCALES) {
    it(`${lng} errors catalog has an entry for every code`, () => {
      const entries = (CATALOGS[lng].errors as { code: Record<string, string> }).code;
      const missing = codes.filter((c) => !entries[c]);
      expect(missing, `codes without ${lng} copy`).toEqual([]);
    });
  }
});
