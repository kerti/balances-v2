/// <reference types="vite/client" />

// Build-time release tag baked into the SPA bundle (issue #75). Injected by
// the deploy pipeline as a VITE_ build arg (see Dockerfile + deploy.yml);
// unset on a local `npm run dev`/`npm run build`, hence optional. The deploy
// target (DEPLOY_ENV) is resolved at runtime instead (#354) — see
// @/hooks/useDeployEnv — since it varies per environment even though the same
// built image is deployed everywhere.
interface ImportMetaEnv {
  readonly VITE_APP_VERSION?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
