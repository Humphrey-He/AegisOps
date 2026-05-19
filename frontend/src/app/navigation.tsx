import {
  AppstoreOutlined,
  AuditOutlined,
  BellOutlined,
  DatabaseOutlined,
  DeploymentUnitOutlined,
  DesktopOutlined,
  InboxOutlined,
  SettingOutlined,
  TeamOutlined,
  UnorderedListOutlined,
} from "@ant-design/icons";
import type { ReactNode } from "react";

export type NavItem = {
  key: string;
  label: string;
  icon?: ReactNode;
  permission?: string;
  children?: NavItem[];
};

export const navItems: NavItem[] = [
  {
    key: "/dashboard",
    label: "工作台",
    icon: <DeploymentUnitOutlined />,
    permission: "dashboard.view",
  },
  {
    key: "assets",
    label: "资产管理",
    icon: <DesktopOutlined />,
    children: [
      { key: "/assets/hosts", label: "主机", permission: "hosts.view" },
      { key: "/assets/secrets", label: "凭证", permission: "secrets.view" },
    ],
  },
  {
      key: "docker",
      label: "运行资源",
      icon: <DatabaseOutlined />,
      children: [
        { key: "/docker/nodes", label: "Docker 节点", icon: <DatabaseOutlined />, permission: "docker.view" },
        { key: "/nginx/nodes", label: "Nginx 节点", icon: <DeploymentUnitOutlined />, permission: "nginx.view" },
      ],
    },
    {
      key: "delivery",
      label: "应用交付",
      icon: <InboxOutlined />,
      children: [
        { key: "/delivery/registries", label: "镜像仓库", icon: <InboxOutlined />, permission: "registries.view" },
        {
          key: "/delivery/services",
          label: "服务定义",
        icon: <AppstoreOutlined />,
        permission: "services.view",
      },
    ],
  },
  {
    key: "/tasks",
    label: "任务中心",
    icon: <UnorderedListOutlined />,
    permission: "tasks.view",
  },
  {
    key: "/audits",
    label: "操作审计",
    icon: <AuditOutlined />,
    permission: "audits.view",
  },
  {
    key: "alerts",
    label: "告警中心",
    icon: <BellOutlined />,
    children: [{ key: "/alerts/events", label: "告警事件", permission: "alerts.view" }],
  },
  {
    key: "settings",
    label: "通知与规则",
    icon: <SettingOutlined />,
    children: [
      { key: "/settings/notifications", label: "通知通道", permission: "notifications.view" },
      { key: "/settings/alert-rules", label: "告警规则", permission: "alerts.view" },
      { key: "/settings/exports", label: "导出中心", permission: "exports.view" },
      { key: "/settings/backups", label: "备份中心", permission: "backups.view" },
    ],
  },
  {
    key: "system",
    label: "系统管理",
    icon: <TeamOutlined />,
    children: [
      { key: "/system/users", label: "用户", permission: "users.view" },
      { key: "/system/roles", label: "角色", permission: "roles.view" },
      { key: "/system/scheduled-jobs", label: "调度任务", permission: "scheduler.view" },
    ],
  },
];

export const hiddenNavTitles: Record<string, string> = {
  "/setup/admin": "初始化管理员",
  "/login": "登录",
  "/terminal": "终端",
  "/delivery/registries": "镜像仓库",
  "/delivery/services": "服务定义",
  "/nginx/nodes": "Nginx 节点",
  "/alerts/events": "告警事件",
  "/settings/notifications": "通知通道",
  "/settings/alert-rules": "告警规则",
  "/settings/exports": "导出中心",
  "/settings/backups": "备份中心",
  "/system/scheduled-jobs": "调度任务",
  "/403": "无权限",
  "/404": "页面不存在",
};

export function filterNavItems(items: NavItem[], permissions: string[]): NavItem[] {
  if (permissions.includes("*")) {
    return items;
  }
  return items
    .map((item) => {
      if (item.children) {
        const children = filterNavItems(item.children, permissions);
        return children.length ? { ...item, children } : null;
      }
      if (!item.permission || permissions.includes(item.permission)) {
        return item;
      }
      return null;
    })
    .filter(Boolean) as NavItem[];
}

export function findFirstPath(items: NavItem[]): string | null {
  for (const item of items) {
    if (item.children?.length) {
      const first = findFirstPath(item.children);
      if (first) {
        return first;
      }
      continue;
    }
    return item.key;
  }
  return null;
}
