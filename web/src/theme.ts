import { theme, ThemeConfig } from "antd";

type Theme = "dark" | "light";

const cachedCssVariables: Record<Theme, Record<string, string>> = { light: {}, dark: {} };

const getCssVariable = (name: string, currentTheme: Theme) => {
  if (cachedCssVariables[currentTheme][name]) {
    return cachedCssVariables[currentTheme][name];
  }

  if (typeof window !== "undefined") {
    const element = document.documentElement;

    if (element.getAttribute("data-theme") !== currentTheme) {
      element.setAttribute("data-theme", currentTheme);
    }

    const value = getComputedStyle(element).getPropertyValue(name).trim();
    cachedCssVariables[currentTheme][name] = value;
    return value;
  }
  return "";
};

export const getThemeConfig = (currentTheme: Theme): ThemeConfig => ({
  token: {
    fontSize: 13,
    fontFamily: getCssVariable("--pcl-font-family-sans", currentTheme),
    colorPrimary: getCssVariable("--pcl-color-bg-fill-brand", currentTheme),
    colorInfo: getCssVariable("--pcl-color-fg-info", currentTheme),
    colorBgBase: getCssVariable("--pcl-color-bg", currentTheme),
    colorLink: getCssVariable("--pcl-color-fg-link", currentTheme),
    colorLinkHover: getCssVariable("--pcl-color-fg-link-hover", currentTheme),
    colorTextBase: getCssVariable("--pcl-color-fg", currentTheme),
    borderRadius: 4,
    colorSuccess: getCssVariable("--pcl-color-fg-success", currentTheme),
    colorWarning: getCssVariable("--pcl-color-fg-warning", currentTheme),
    colorError: getCssVariable("--pcl-color-fg-critical", currentTheme),
    colorTextSecondary: getCssVariable("--pcl-color-fg-secondary", currentTheme),
    colorTextTertiary: getCssVariable("--pcl-color-fg-tertiary", currentTheme),
    colorTextQuaternary: getCssVariable("--pcl-color-fg-tertiary", currentTheme),
    colorFillQuaternary: getCssVariable("--pcl-color-bg-fill-disabled", currentTheme),
    colorFillTertiary: getCssVariable("--pcl-color-bg-fill-tertiary", currentTheme),
    colorFillSecondary: getCssVariable("--pcl-color-bg-fill-secondary", currentTheme),
    colorFill: getCssVariable("--pcl-color-bg", currentTheme),
    colorBorder: getCssVariable("--pcl-color-input-border", currentTheme),
    colorBorderSecondary: getCssVariable("--pcl-color-input-border-secondary", currentTheme),
    colorBgContainer: getCssVariable("--transparent", currentTheme),
    colorBgElevated: getCssVariable("--pcl-color-bg-surface", currentTheme),
    colorBgLayout: getCssVariable("--pcl-color-bg-surface", currentTheme),
    colorBgSpotlight: getCssVariable("--pcl-color-bg-surface", currentTheme),
    boxShadowSecondary: getCssVariable("--box-shadow", currentTheme),
    colorHighlight: getCssVariable("--pcl-color-fg-brand", currentTheme),
  },
  components: {
    Button: {
      defaultShadow: "none",
      primaryShadow: "none",
      paddingInlineLG: "0.75rem",
      paddingInline: "0.5rem",
      paddingInlineSM: "0.375rem",
      sizeLG: 32,
      sizeMD: 24,
      sizeSM: 16,
      fontSizeLG: 13,
      fontSizeSM: 12,
      contentFontSizeLG: 13,
      contentFontSize: 13,
      contentFontSizeSM: 12,
      controlHeightLG: 32,
      controlHeight: 26,
      controlHeightSM: 20,
      borderRadiusLG: 6,
      borderRadius: 6,
      borderRadiusSM: 6,
      colorError: getCssVariable("--pcl-color-bg-fill-critical", currentTheme),
      colorErrorHover: getCssVariable("--pcl-color-bg-fill-critical-hover", currentTheme),
      colorErrorActive: getCssVariable("--pcl-color-bg-fill-critical-hover", currentTheme),
      colorErrorBorder: getCssVariable("--pcl-color-bg-fill-critical-hover", currentTheme),
      colorErrorBorderHover: getCssVariable("--pcl-color-bg-fill-critical-hover", currentTheme),
      colorErrorTextHover: getCssVariable("--pcl-color-bg-fill-danger-hover", currentTheme),
      colorErrorTextActive: getCssVariable("--pcl-color-bg-fill-danger-hover", currentTheme),
      colorTextLightSolid: getCssVariable("--pcl-color-fg-inverse", currentTheme),
      colorLinkHover: getCssVariable("--pcl-color-fg", currentTheme),
      colorLinkActive: getCssVariable("--pcl-color-fg", currentTheme),
    },
    Menu: {
      itemSelectedBg: getCssVariable("--pcl-color-bg-surface-active", currentTheme),
      itemSelectedColor: getCssVariable("--pcl-color-fg", currentTheme),
      itemHoverBg: getCssVariable("--pcl-color-bg-fill-transparent-hover", currentTheme),
    },
    Tooltip: {
      colorBgSpotlight: getCssVariable("--pcl-color-bg-fill-inverse", currentTheme),
      colorTextLightSolid: getCssVariable("--pcl-color-fg-inverse", currentTheme),
    },
    Tag: {
      colorBgContainer: getCssVariable("--pcl-color-bg-surface", currentTheme),
    },
    Table: {
      borderColor: getCssVariable("--pcl-color-border", currentTheme),
      headerBg: getCssVariable("--pcl-color-bg-fill-secondary", currentTheme),
      colorBgContainer: getCssVariable("--pcl-color-bg-fill", currentTheme),
      rowHoverBg: getCssVariable("--pcl-color-bg-fill-active", currentTheme),
      headerBorderRadius: 12,
      fontWeightStrong: 400,
      headerColor: getCssVariable("--pcl-color-fg", currentTheme),
      fontSize: 14,
    },
    Divider: {
      colorSplit: getCssVariable("--pcl-color-border", currentTheme),
    },
    Radio: {
      fontSizeLG: 14,
      fontSize: 13,
      fontSizeSM: 13,
      radioSize: 20,
      dotSize: 8,
      controlHeightLG: 32,
      controlHeight: 24,
      controlHeightSM: 20,
      borderRadiusLG: 6,
      borderRadius: 6,
      borderRadiusSM: 6,
      colorPrimary: getCssVariable("--pcl-color-bg-fill-inverse-active", currentTheme),
      buttonCheckedBg: getCssVariable("--pcl-color-bg-fill-active", currentTheme),
      buttonColor: getCssVariable("--pcl-color-fg-tertiary", currentTheme),
      colorPrimaryHover: getCssVariable("--pcl-color-fg", currentTheme),
      colorPrimaryActive: getCssVariable("--pcl-color-fg", currentTheme),
      colorPrimaryBorder: getCssVariable("--pcl-color-border", currentTheme),
      colorBorder: getCssVariable("--pcl-color-input-border", currentTheme),
    },
    Input: {
      colorBgContainer: getCssVariable("--pcl-color-bg-fill", currentTheme),
      hoverBorderColor: getCssVariable("--pcl-color-border-hover", currentTheme),
      activeBorderColor: getCssVariable("--pcl-color-border-active", currentTheme),
      colorBorder: getCssVariable("--pcl-color-input-border", currentTheme),
      boxShadow: "none",
    },
    Skeleton: {
      gradientFromColor: getCssVariable("--pcl-color-bg-surface-active", currentTheme),
      gradientToColor: getCssVariable("--pcl-color-bg-surface-disabled", currentTheme),
      controlHeight: 40,
      blockRadius: 8,
    },
    Dropdown: {
      colorBgElevated: getCssVariable("--pcl-color-bg-fill-secondary", currentTheme),
    },
    Select: {
      colorBgElevated: getCssVariable("--pcl-color-bg-fill-secondary", currentTheme),
    },
    Segmented: {
      itemSelectedBg: getCssVariable("--pcl-color-bg-surface-active", currentTheme),
      itemSelectedColor: getCssVariable("--pcl-color-fg", currentTheme),
      itemHoverBg: getCssVariable("--pcl-color-bg-fill-transparent-hover", currentTheme),
    },
    Checkbox: {
      colorBorder: getCssVariable("--pcl-color-input-border", currentTheme),
      colorPrimary: getCssVariable("--pcl-color-bg-fill-inverse-active", currentTheme),
    },
    Tabs: {
      colorPrimary: getCssVariable("--pcl-color-highlight", currentTheme),
    },
  },
  algorithm: currentTheme === "dark" ? [theme.darkAlgorithm] : [theme.defaultAlgorithm],
});
