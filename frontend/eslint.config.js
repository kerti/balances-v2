import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import tseslint from 'typescript-eslint'
import { defineConfig, globalIgnores } from 'eslint/config'
import prettier from 'eslint-config-prettier'

export default defineConfig([
  globalIgnores(['dist', 'coverage']),
  {
    files: ['**/*.{ts,tsx}'],
    extends: [
      js.configs.recommended,
      tseslint.configs.recommended,
      reactHooks.configs.flat.recommended,
      reactRefresh.configs.vite,
      prettier,
    ],
    languageOptions: {
      globals: globals.browser,
    },
    rules: {
      // i18n guard (ADR-0026). JSXText nodes with any non-whitespace content
      // and string-literal JSX attributes for user-facing props must go
      // through `t(...)` / <Trans>. Set to 'warn' during the extraction
      // milestone so existing untranslated copy doesn't fail CI; tighten to
      // 'error' once the catalog sweep completes (issue #12 close).
      // eslint-plugin-react isn't compatible with ESLint 10 yet, so the rule
      // is expressed as AST selectors via the built-in no-restricted-syntax.
      'no-restricted-syntax': [
        'warn',
        {
          selector: 'JSXText[value=/\\S/]',
          message:
            "Bare JSX text not allowed — use t('namespace.key') or <Trans>. See ADR-0026.",
        },
        {
          selector:
            "JSXAttribute[name.name=/^(placeholder|title|aria-label|alt)$/] > Literal[value=/\\S/]",
          message:
            "Bare string attribute not allowed — use t('namespace.key'). See ADR-0026.",
        },
      ],
    },
  },
  {
    files: ['src/components/ui/**/*.{ts,tsx}'],
    rules: {
      'react-refresh/only-export-components': 'off',
      // shadcn-generated UI primitives are vendored verbatim; don't fight them.
      'no-restricted-syntax': 'off',
    },
  },
])
