// App identity surfaced in the sidebar footer (issue #75). The release tag is
// injected by Vite at build time (VITE_APP_VERSION build arg — see Dockerfile
// + deploy.yml); it's identical across every environment a single build gets
// deployed to, so baking it in is safe. A local build leaves it unset, so we
// fall back to a dev-friendly default.

// Resolve the release tag, falling back to "dev" when the build arg is unset or
// empty (a local build leaves VITE_APP_VERSION blank).
export function resolveAppVersion(raw: string | undefined): string {
  return raw || "dev";
}

// The release tag (e.g. "v0.6.0-alpha.2"), or "dev" on a local build.
export const APP_VERSION = resolveAppVersion(import.meta.env.VITE_APP_VERSION);

// The deploy target, unlike APP_VERSION above, varies per environment even
// though the same built image is deployed everywhere (#354) — so it can't be a
// Vite build-time constant. It's resolved at runtime from /healthz; see
// useDeployEnv in @/hooks/useDeployEnv.
export type DeployEnv = "preview" | "demo" | "production" | "self-hosted" | "local";

const DEPLOY_ENVS: readonly DeployEnv[] = ["preview", "demo", "production", "self-hosted", "local"];

// Validate a raw value against the known targets, falling back to 'local' for
// anything unset or unrecognised.
export function resolveDeployEnv(raw: string | undefined): DeployEnv {
  return (DEPLOY_ENVS as readonly string[]).includes(raw ?? "") ? (raw as DeployEnv) : "local";
}

// External links shown in the footer.
export const REPO_URL = "https://github.com/kerti/balances-v2";
export const MAINTAINER_URL = "https://radityakertiyasa.com";
