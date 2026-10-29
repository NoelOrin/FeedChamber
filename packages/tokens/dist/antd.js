/* @orinforum/tokens - AntD ThemeConfig — AUTO-GENERATED from w3c-tokens.json, do not edit manually.
   Run: pnpm --filter @orinforum/tokens tokens:build */

import { theme as antdTheme } from 'antd';

export const lightTheme = {
  token: {
    colorPrimary: '#8c2de4',
    colorPrimaryHover: '#ab56f0',
    colorPrimaryActive: '#6a1cbd',
    colorBgBase: '#ffffff',
    colorBgLayout: '#ffffff',
    colorBgContainer: '#ffffff',
    colorBgElevated: '#ffffff',
    colorBgSpotlight: '#ffffff',
    colorTextBase: '#141416',
    colorText: '#141416',
    colorTextSecondary: '#6b625c',
    colorTextTertiary: '#6f6b68',
    colorTextQuaternary: '#6f6b68',
    colorTextPlaceholder: '#94918d',
    colorBorder: '#d6d0ca',
    colorBorderSecondary: '#d6d0ca',
    colorFill: '#f5f5f5',
    colorFillSecondary: '#f5f5f5',
    colorFillTertiary: '#f5f5f5',
    colorFillQuaternary: '#ffffff',
    colorSplit: '#d6d0ca',
    colorSuccess: '#137337',
    colorWarning: '#a24b08',
    colorError: '#c62222',
    colorInfo: '#3b82f6',
    colorLink: '#8c2de4',
    colorLinkHover: '#ab56f0',
    colorLinkActive: '#6a1cbd',
    colorLinkDecoration: 'none',
    colorTextLightSolid: '#ffffff',
    borderRadius: 8,
    borderRadiusLG: 10,
    borderRadiusSM: 4,
    fontFamily: `ui-sans-serif, system-ui, -apple-system, "Segoe UI", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif`,
    fontSize: 14,
    /* 控件几何对齐主站 shadcn 按钮：h-9=36 / h-11=44 / h-8=32。
       antd 默认 controlHeight 是 32，后台表格与表单会比前台矮一截。 */
    controlHeight: 36,
    controlHeightLG: 40,
    controlHeightSM: 28,
    lineHeight: 1.5715,
  },
  components: {
    Layout: {
      headerBg: '#ffffff',
      headerColor: '#141416',
      headerColorSecondary: '#6b625c',
      siderBg: '#ffffff',
      bodyBg: '#ffffff',
      colorTextMenuSelected: '#8c2de4',
      colorBgMenuItemSelected: '#f1d6ff',
    },
    Menu: {
      itemBg: '#ffffff',
      itemColor: '#6b625c',
      itemHoverBg: '#f5f5f5',
      itemHoverColor: '#141416',
      itemSelectedBg: '#f1d6ff',
      itemSelectedColor: '#8c2de4',
      subMenuItemBg: '#ffffff',
      activeBarBorderWidth: 0,
    },
    Table: {
      headerBg: '#f5f5f5',
      headerColor: '#141416',
      headerSplitColor: '#d6d0ca',
      rowHoverBg: '#f5f5f5',
      borderColor: '#d6d0ca',
    },
    Card: {
      headerBg: 'transparent',
      colorBorderSecondary: '#d6d0ca',
    },
    Tabs: {
      itemColor: '#6b625c',
      itemSelectedColor: '#8c2de4',
      itemHoverColor: '#ab56f0',
      inkBarColor: '#8c2de4',
    },
    Button: {
      defaultBg: '#ffffff',
      defaultColor: '#141416',
      defaultBorderColor: '#d6d0ca',
      defaultHoverBg: '#f5f5f5',
      defaultHoverColor: '#8c2de4',
      defaultHoverBorderColor: '#ab56f0',
    },
    Input: {
      colorBgContainer: '#ffffff',
      activeBorderColor: '#8c2de4',
      hoverBorderColor: '#ab56f0',
      activeShadow: 'none',
    },
    Select: {
      optionSelectedBg: '#f5f5f5',
      optionActiveBg: '#ffffff',
    },
    Tag: {
      defaultBg: '#f5f5f5',
      defaultColor: '#6b625c',
    },
    Alert: {
      defaultBg: '#f5f5f5',
    },
    Modal: {
      contentBg: '#ffffff',
      headerBg: '#ffffff',
    },
    Divider: {
      colorSplit: '#d6d0ca',
    },
    Typography: {
      colorTextHeading: '#141416',
      colorTextDescription: '#6f6b68',
    },
    Tooltip: {
      colorBgSpotlight: '#ffffff',
      colorTextLightSolid: '#141416',
    },
    Statistic: {
      contentFontSize: 24,
      titleFontSize: 14,
    },
  },
};

export const darkTheme = {
  algorithm: antdTheme.darkAlgorithm,
  token: {
    colorPrimary: '#9b4fd9',
    colorPrimaryHover: '#c17df0',
    colorPrimaryActive: '#7928c4',
    colorBgBase: '#0f0f11',
    colorBgLayout: '#0f0f11',
    colorBgContainer: '#1a1a1d',
    colorBgElevated: '#1a1a1d',
    colorBgSpotlight: '#1a1a1d',
    colorTextBase: '#f0f0ea',
    colorText: '#f0f0ea',
    colorTextSecondary: '#a1a1aa',
    colorTextTertiary: '#909097',
    colorTextQuaternary: '#909097',
    colorTextPlaceholder: '#68686a',
    colorBorder: '#3a3a3d',
    colorBorderSecondary: '#3a3a3d',
    colorFill: '#26262a',
    colorFillSecondary: '#26262a',
    colorFillTertiary: '#26262a',
    colorFillQuaternary: '#1a1a1d',
    colorSplit: '#3a3a3d',
    colorSuccess: '#4ade80',
    colorWarning: '#fbbf24',
    colorError: '#ef4444',
    colorInfo: '#60a5fa',
    colorLink: '#9b4fd9',
    colorLinkHover: '#c17df0',
    colorLinkActive: '#7928c4',
    colorLinkDecoration: 'none',
    colorTextLightSolid: '#ffffff',
    borderRadius: 8,
    borderRadiusLG: 10,
    borderRadiusSM: 4,
    fontFamily: `ui-sans-serif, system-ui, -apple-system, "Segoe UI", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif`,
    fontSize: 14,
    /* 控件几何对齐主站 shadcn 按钮：h-9=36 / h-11=44 / h-8=32。
       antd 默认 controlHeight 是 32，后台表格与表单会比前台矮一截。 */
    controlHeight: 36,
    controlHeightLG: 40,
    controlHeightSM: 28,
    lineHeight: 1.5715,
  },
  components: {
    Layout: {
      headerBg: '#0f0f11',
      headerColor: '#f0f0ea',
      headerColorSecondary: '#a1a1aa',
      siderBg: '#0f0f11',
      bodyBg: '#0f0f11',
      colorTextMenuSelected: '#9b4fd9',
      colorBgMenuItemSelected: '#24103a',
    },
    Menu: {
      itemBg: '#0f0f11',
      itemColor: '#a1a1aa',
      itemHoverBg: '#26262a',
      itemHoverColor: '#f0f0ea',
      itemSelectedBg: '#24103a',
      itemSelectedColor: '#9b4fd9',
      subMenuItemBg: '#0f0f11',
      activeBarBorderWidth: 0,
    },
    Table: {
      headerBg: '#26262a',
      headerColor: '#f0f0ea',
      headerSplitColor: '#3a3a3d',
      rowHoverBg: '#26262a',
      borderColor: '#3a3a3d',
    },
    Card: {
      headerBg: 'transparent',
      colorBorderSecondary: '#3a3a3d',
    },
    Tabs: {
      itemColor: '#a1a1aa',
      itemSelectedColor: '#9b4fd9',
      itemHoverColor: '#c17df0',
      inkBarColor: '#9b4fd9',
    },
    Button: {
      defaultBg: '#1a1a1d',
      defaultColor: '#f0f0ea',
      defaultBorderColor: '#3a3a3d',
      defaultHoverBg: '#26262a',
      defaultHoverColor: '#9b4fd9',
      defaultHoverBorderColor: '#c17df0',
    },
    Input: {
      colorBgContainer: '#1a1a1d',
      activeBorderColor: '#9b4fd9',
      hoverBorderColor: '#c17df0',
      activeShadow: 'none',
    },
    Select: {
      optionSelectedBg: '#26262a',
      optionActiveBg: '#1a1a1d',
    },
    Tag: {
      defaultBg: '#26262a',
      defaultColor: '#a1a1aa',
    },
    Alert: {
      defaultBg: '#26262a',
    },
    Modal: {
      contentBg: '#1a1a1d',
      headerBg: '#1a1a1d',
    },
    Divider: {
      colorSplit: '#3a3a3d',
    },
    Typography: {
      colorTextHeading: '#f0f0ea',
      colorTextDescription: '#909097',
    },
    Tooltip: {
      colorBgSpotlight: '#1a1a1d',
      colorTextLightSolid: '#f0f0ea',
    },
    Statistic: {
      contentFontSize: 24,
      titleFontSize: 14,
    },
  },
};

/** Select theme based on dark mode — zero runtime derivation, just object selection */
export function getAntdTheme(isDark) {
  return isDark ? darkTheme : lightTheme;
}
