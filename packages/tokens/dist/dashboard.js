/* @feedchamber/tokens - Dashboard tokens — AUTO-GENERATED from w3c-tokens.json, do not edit manually.
   Run: pnpm --filter @feedchamber/tokens tokens:build */

export const lightDashboardTokens = {
  headingColor: '#141416',
  secondaryColor: '#6b625c',
  mutedColor: '#6f6b68',
  placeholderColor: '#94918d',
  successColor: '#137337',
  warningColor: '#a24b08',
  errorColor: '#c62222',
  infoColor: '#3b82f6',
  purpleColor: '#8c2de4',
  bgGrad: 'linear-gradient(180deg, #ffffff 0%, #ffffff 55%, #f5f5f5 100%)',
  avatarBg: '#f1d6ff',
};

export const darkDashboardTokens = {
  headingColor: '#f0f0ea',
  secondaryColor: '#a1a1aa',
  mutedColor: '#909097',
  placeholderColor: '#68686a',
  successColor: '#4ade80',
  warningColor: '#fbbf24',
  errorColor: '#ef4444',
  infoColor: '#60a5fa',
  purpleColor: '#9b4fd9',
  bgGrad: 'linear-gradient(180deg, #0f0f11 0%, #1a1a1d 60%, #0f0f11 100%)',
  avatarBg: '#24103a',
};

/** Select dashboard tokens based on dark mode */
export function getDashboardTokens(isDark) {
  return isDark ? darkDashboardTokens : lightDashboardTokens;
}
