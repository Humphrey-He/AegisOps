import { App as AntApp, ConfigProvider } from "antd";
import type { PropsWithChildren } from "react";
import { QueryClientProvider } from "@tanstack/react-query";
import zhCN from "antd/locale/zh_CN";
import { queryClient } from "./queryClient";

export function Providers({ children }: PropsWithChildren) {
  return (
    <ConfigProvider
      locale={zhCN}
      theme={{
        token: {
          colorPrimary: "#0f766e",
          colorPrimaryHover: "#115e59",
          colorInfo: "#2563eb",
          colorSuccess: "#15803d",
          colorWarning: "#b45309",
          colorError: "#b91c1c",
          borderRadius: 6,
          borderRadiusLG: 8,
          fontSize: 14,
          controlHeight: 36,
          colorBgLayout: "#f2f5f7",
          colorBgContainer: "#ffffff",
          colorBgElevated: "#ffffff",
          colorBorder: "#d9e2ec",
          colorBorderSecondary: "#e7edf3",
          colorText: "#111827",
          colorTextSecondary: "#5b6b7f",
          colorTextHeading: "#0f172a",
        },
        components: {
          Layout: {
            headerBg: "#ffffff",
            siderBg: "#ffffff",
            bodyBg: "#f3f6f8",
            headerHeight: 60,
            triggerBg: "#ffffff",
          },
          Menu: {
            itemBorderRadius: 8,
            itemMarginInline: 10,
            itemMarginBlock: 4,
            itemSelectedBg: "#e8f3f1",
            itemSelectedColor: "#0f766e",
            itemHoverColor: "#0f766e",
            itemHoverBg: "#f5faf9",
            subMenuItemBg: "#ffffff",
          },
          Card: {
            borderRadiusLG: 8,
          },
          Table: {
            headerBg: "#f8fafc",
            headerColor: "#475569",
            rowHoverBg: "#f6fbfa",
            rowSelectedBg: "#edf8f6",
            borderColor: "#e7edf3",
          },
          Drawer: {
            footerPaddingBlock: 16,
            footerPaddingInline: 24,
          },
          Button: {
            controlHeight: 36,
            borderRadius: 8,
            primaryShadow: "none",
          },
          Input: {
            controlHeight: 36,
          },
          Select: {
            controlHeight: 36,
          },
        },
      }}
    >
      <AntApp>
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      </AntApp>
    </ConfigProvider>
  );
}
