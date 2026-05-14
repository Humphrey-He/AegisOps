import { Result } from "antd";
import type { PropsWithChildren, ReactNode } from "react";
import { useSessionStore } from "../store/sessionStore";

type PermissionGuardProps = PropsWithChildren<{
  permission?: string;
  fallback?: ReactNode;
  forbiddenPage?: boolean;
}>;

export function PermissionGuard({
  permission,
  fallback = null,
  forbiddenPage = false,
  children,
}: PermissionGuardProps) {
  const permissions = useSessionStore((state) => state.permissions);
  if (!permission || permissions.includes("*") || permissions.includes(permission)) {
    return <>{children}</>;
  }
  if (forbiddenPage) {
    return <Result status="403" title="403" subTitle="你没有访问这个区域的权限。" />;
  }
  return <>{fallback}</>;
}
