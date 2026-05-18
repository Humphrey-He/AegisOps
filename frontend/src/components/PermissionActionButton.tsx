import { Button, Tooltip } from "antd";
import type { ComponentProps, ReactNode } from "react";
import { useSessionStore } from "../store/sessionStore";

type PermissionActionButtonProps = ComponentProps<typeof Button> & {
  permission?: string;
  permissionReason?: ReactNode;
  disabledReason?: ReactNode;
  hideWhenUnauthorized?: boolean;
};

export function PermissionActionButton({
  permission,
  permissionReason,
  disabledReason,
  hideWhenUnauthorized = false,
  disabled,
  children,
  ...buttonProps
}: PermissionActionButtonProps) {
  const permissions = useSessionStore((state) => state.permissions);
  const allowed = !permission || permissions.includes("*") || permissions.includes(permission);

  if (!allowed && hideWhenUnauthorized) {
    return null;
  }

  const finalDisabled = Boolean(disabled) || !allowed;
  const tooltipTitle = !allowed
    ? permissionReason ?? (permission ? `当前账号缺少 ${permission} 权限` : undefined)
    : disabled
      ? disabledReason
      : undefined;

  const button = (
    <Button {...buttonProps} disabled={finalDisabled}>
      {children}
    </Button>
  );

  if (!tooltipTitle) {
    return button;
  }

  return (
    <Tooltip title={tooltipTitle}>
      <span>{button}</span>
    </Tooltip>
  );
}
