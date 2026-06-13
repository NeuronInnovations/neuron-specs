import tseslint from '@typescript-eslint/eslint-plugin';
import tsparser from '@typescript-eslint/parser';

export default [
  {
    files: ['src/**/*.ts', 'tests/**/*.ts', 'scripts/**/*.ts', 'examples/**/*.ts'],
    languageOptions: {
      parser: tsparser,
      parserOptions: {
        project: './tsconfig.json',
      },
    },
    plugins: {
      '@typescript-eslint': tseslint,
    },
    rules: {
      ...tseslint.configs['strict'].rules,
      '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_', varsIgnorePattern: '^_' }],
      '@typescript-eslint/explicit-function-return-type': 'error',
      'no-console': 'error',
    },
  },
  // Spec 012 demo surfaces: intentional console logging is demo-forensic
  // (`[neuron-012]` / `[seller-flow]` / `[file-send]` / `[orchestrator]` /
  // `[vite]` tags visible in browser DevTools + seller terminal during the
  // manual acceptance walkthrough in quickstart.md). Phase 2 H3 swaps to a
  // structured logger when Playwright automation lands.
  {
    files: [
      'src/browser-client/**/*.ts',
      'src/browser-client-wt/**/*.ts',
      'src/server-demo/**/*.ts',
      'scripts/**/*.ts',
      'examples/**/*.ts',
      'tests/smoke/**/*.ts',
    ],
    rules: {
      'no-console': 'off',
    },
  },
  // Test files: non-null assertions are idiomatic on values that are
  // constructed-and-then-asserted-about within the same test, and the
  // project's existing 002/004/005 suites already follow this style.
  {
    files: ['tests/**/*.ts'],
    rules: {
      '@typescript-eslint/no-non-null-assertion': 'off',
      '@typescript-eslint/explicit-function-return-type': 'off',
    },
  },
];
