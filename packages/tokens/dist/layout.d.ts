export declare const lightLayoutTokens: {
  headerBg: string;
  headerColor: string;
  headerColorSecondary: string;
  siderBg: string;
  bodyBg: string;
  colorTextMenuSelected: string;
  colorBgMenuItemSelected: string;
};

export declare const darkLayoutTokens: {
  headerBg: string;
  headerColor: string;
  headerColorSecondary: string;
  siderBg: string;
  bodyBg: string;
  colorTextMenuSelected: string;
  colorBgMenuItemSelected: string;
};

export declare function getLayoutTokens(isDark: boolean): typeof lightLayoutTokens;
