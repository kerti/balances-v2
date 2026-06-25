// i18n entry point. Initialises i18next with bundled catalogs imported
// statically — see ADR-0026. Language detection order: the users.locale value
// set via Settings (synced into localStorage at sign-in), then
// navigator.language as the first-login fallback, then 'en-GB'.
//
// Catalogs are bundled into the JS chunk rather than fetched at runtime: the
// ~30 KB cost is small, and the alternative (i18next-http-backend) had a v4
// ESM-interop quirk that left the resource store empty even after init
// resolved. Bundling also removes a first-paint network race — t() returns
// real copy synchronously on first render.
import i18n from 'i18next'
import LanguageDetector from 'i18next-browser-languagedetector'
import { initReactI18next } from 'react-i18next'

import enCommon from '../locales/en/common.json'
import enNav from '../locales/en/nav.json'
import enDashboard from '../locales/en/dashboard.json'
import enAssets from '../locales/en/assets.json'
import enLiabilities from '../locales/en/liabilities.json'
import enReceivables from '../locales/en/receivables.json'
import enInvestments from '../locales/en/investments.json'
import enIncome from '../locales/en/income.json'
import enSettings from '../locales/en/settings.json'
import enTags from '../locales/en/tags.json'
import enErrors from '../locales/en/errors.json'
import enOnboarding from '../locales/en/onboarding.json'

import idCommon from '../locales/id/common.json'
import idNav from '../locales/id/nav.json'
import idDashboard from '../locales/id/dashboard.json'
import idAssets from '../locales/id/assets.json'
import idLiabilities from '../locales/id/liabilities.json'
import idReceivables from '../locales/id/receivables.json'
import idInvestments from '../locales/id/investments.json'
import idIncome from '../locales/id/income.json'
import idSettings from '../locales/id/settings.json'
import idTags from '../locales/id/tags.json'
import idErrors from '../locales/id/errors.json'
import idOnboarding from '../locales/id/onboarding.json'

// Namespaces are split per feature so each extraction issue (issues #5–#11)
// ships an independently translatable unit. The 'errors' namespace covers
// validation + toast copy that crosses features.
export const NAMESPACES = [
  'common',
  'nav',
  'dashboard',
  'assets',
  'liabilities',
  'receivables',
  'investments',
  'income',
  'settings',
  'tags',
  'errors',
  'onboarding',
] as const

// Locales are stored and exchanged as BCP47 strings (matching users.locale and
// the Intl APIs). Catalog source directories under src/locales/ stay 2-letter
// ('en'/'id') for filesystem cleanliness; the resource bundles below re-key
// them to the full BCP47 form so the lookup matches supportedLngs without a
// region-strip step. To add a regional variant (e.g. 'en-US'), extend
// SUPPORTED_LOCALES and the matching CHECK in backend migration 00020; new
// catalog files are only needed if the translations actually diverge.
export const SUPPORTED_LOCALES = ['en-GB', 'id-ID'] as const
export type Locale = (typeof SUPPORTED_LOCALES)[number]
export const LOCALSTORAGE_KEY = 'balances.locale'

// Resource bundles are keyed by the full BCP47 string to match supportedLngs.
// Earlier attempt keyed by 2-letter ('en'/'id') with load: 'languageOnly';
// i18next then refused the lookup because the stripped 'en'/'id' was not in
// supportedLngs and nonExplicitSupportedLngs defaulted to false. Keying both
// resources and supportedLngs by full BCP47 lets the lookup resolve directly.
const resources = {
  'en-GB': {
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
  'id-ID': {
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
}

// init returns a promise but with bundled resources it resolves synchronously
// at the next microtask — no HTTP, no Suspense boundary needed. Exported for
// completeness in case a caller wants to await readiness; nothing in the app
// currently does.
export const i18nReady = i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources,
    fallbackLng: 'en-GB',
    supportedLngs: SUPPORTED_LOCALES as unknown as string[],
    ns: NAMESPACES as unknown as string[],
    defaultNS: 'common',
    interpolation: { escapeValue: false }, // React already escapes
    react: { useSuspense: false },
    detection: {
      // The Settings UI writes users.locale via PATCH /api/users/me and mirrors
      // the choice into localStorage so a returning user picks up their
      // language before the user-self query resolves. navigator.language is
      // the first-login fallback.
      order: ['localStorage', 'navigator'],
      lookupLocalStorage: LOCALSTORAGE_KEY,
      caches: ['localStorage'],
    },
  })

export default i18n
