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
          borderRadius: 8,
          colorBgLayout: "#f4f7f6",
          colorTextHeading: "#0f172a",
        },
      }}
    >
      <AntApp>
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      </AntApp>
    </ConfigProvider>
  );
}
