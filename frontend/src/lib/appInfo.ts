// App identity surfaced in the sidebar footer (issue #75). The values are
// injected by Vite at build time: the deploy pipeline passes the release tag
// and target environment as VITE_ build args (see Dockerfile + deploy.yml). A
// local build leaves them unset, so we fall back to dev-friendly defaults.

// The release tag (e.g. "v0.6.0-alpha.2"), or "dev" on a local build.
export const APP_VERSION = import.meta.env.VITE_APP_VERSION || 'dev'

// The deploy target. 'local' covers any build without the pipeline's build arg
// (dev server, ad-hoc local builds). The display label is resolved via i18n at
// render time (nav: footer.deploy.<env>).
export type DeployEnv = 'preview' | 'demo' | 'production' | 'local'

const DEPLOY_ENVS: readonly DeployEnv[] = [
  'preview',
  'demo',
  'production',
  'local',
]

export const DEPLOY_ENV: DeployEnv = ((): DeployEnv => {
  const raw = import.meta.env.VITE_DEPLOY_ENV ?? 'local'
  return (DEPLOY_ENVS as readonly string[]).includes(raw)
    ? (raw as DeployEnv)
    : 'local'
})()

// External links shown in the footer.
export const REPO_URL = 'https://github.com/kerti/balances-v2'
export const MAINTAINER_URL = 'https://radityakertiyasa.com'
