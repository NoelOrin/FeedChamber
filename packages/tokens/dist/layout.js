/* @orinforum/tokens - Layout tokens — AUTO-GENERATED from w3c-tokens.json, do not edit manually.
   Run: pnpm --filter @orinforum/tokens tokens:build */

export const lightLayoutTokens = {
  headerBg: '#ffffff',
  headerColor: '#141416',
  headerColorSecondary: '#6b625c',
  siderBg: '#ffffff',
  bodyBg: '#ffffff',
  colorTextMenuSelected: '#8c2de4',
  colorBgMenuItemSelected: '#f1d6ff',
};

export const darkLayoutTokens = {
  headerBg: '#0f0f11',
  headerColor: '#f0f0ea',
  headerColorSecondary: '#a1a1aa',
  siderBg: '#0f0f11',
  bodyBg: '#0f0f11',
  colorTextMenuSelected: '#9b4fd9',
  colorBgMenuItemSelected: '#24103a',
};

/** Select layout tokens based on dark mode */
export function getLayoutTokens(isDark) {
  return isDark ? darkLayoutTokens : lightLayoutTokens;
}
