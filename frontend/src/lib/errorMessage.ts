// Maps an unknown caught value (mutation/query error) to a translated
// user-facing string for a dialog or toast. Replaces the ~50 local
// `formatError(err, unknownMsg)` clones that used to render the raw English
// server body. The wire contract is the ADR-0027 `{code, args}` envelope on
// `ApiError.body`; everything else falls through to a sensible default.
//
// Resolution order:
//   1. ApiError + envelope body  -> `errors:code.<CODE>` with args interpolated.
//      Unknown codes fall to `errors:code.UNKNOWN` (logged in dev so a missing
//      catalog entry doesn't go silent).
//   2. ApiError without envelope -> generic UNKNOWN copy. We deliberately stop
//      surfacing raw English bodies — the OAuth callback redirects don't reach
//      this helper, and the snapshot-importer's per-row 422 is consumed as a
//      success-shaped ImportResult, not an error.
//   3. Native Error                -> err.message (network failures, JSON
//      parse errors). Localising these is out of scope; they're already noise.
//   4. Anything else               -> the optional `fallback` arg, else the
//      generic UNKNOWN copy.
//
// Mirrors the i18n-call pattern in lib/lifecycle.ts: callers re-render via
// useTranslation on locale change; this stays a pure function that pulls the
// live catalog from the shared i18n instance.
import i18n from "@/i18n";
import { ApiError, isEnvelope } from "@/api/client";

export function errorMessage(err: unknown, fallback?: string): string {
  if (err instanceof ApiError) {
    if (isEnvelope(err.body)) {
      const env = err.body;
      // VALIDATION is the one code whose `rule` arg is itself a translatable
      // token (`required`, `oneof`, `gt`, ...). We resolve the rule sub-key
      // first, then feed the human form into the outer template. JSON keys
      // can't be both string and object at the same level, so the rule subkeys
      // live under a sibling `VALIDATION_RULE.<rule>` rather than nested
      // under `VALIDATION` — a small structural deviation from the ADR sketch.
      if (env.code === "VALIDATION" && env.args) {
        const ruleArg = String(env.args.rule ?? "");
        const rule = i18n.t(`errors:code.VALIDATION_RULE.${ruleArg}`, {
          defaultValue: ruleArg,
        });
        return i18n.t("errors:code.VALIDATION", {
          field: String(env.args.field ?? ""),
          rule,
        });
      }
      const key = `errors:code.${env.code}`;
      const translated = i18n.t(key, {
        ...(env.args ?? {}),
        defaultValue: "",
      });
      if (translated) return translated;
      if (import.meta.env.DEV) {
        // Surface missing catalog entries during development without changing
        // the user-facing copy. Production stays silent.
        console.warn("[errorMessage] missing catalog entry for", key, env.args);
      }
      return i18n.t("errors:code.UNKNOWN");
    }
    return i18n.t("errors:code.UNKNOWN");
  }
  if (err instanceof Error) return err.message;
  return fallback ?? i18n.t("errors:code.UNKNOWN");
}
