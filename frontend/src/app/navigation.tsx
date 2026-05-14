import {
  AuditOutlined,
  DatabaseOutlined,
  DeploymentUnitOutlined,
  DesktopOutlined,
  InboxOutlined,
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
      {
        key: "/assets/hosts",
        label: "主机",
        permission: "hosts.view",
      },
      {
        key: "/assets/secrets",
        label: "凭证",
        permission: "secrets.view",
      },
    ],
  },
  {
    key: "docker",
    label: "Docker",
    icon: <DatabaseOutlined />,
    children: [
      {
        key: "/docker/nodes",
        label: "节点",
        permission: "docker.view",
      },
    ],
  },
  {
    key: "delivery",
    label: "交付发布",
    icon: <InboxOutlined />,
    children: [
      {
        key: "/delivery/registries",
        label: "Registry",
        permission: "registries.view",
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
    label: "审计日志",
    icon: <AuditOutlined />,
    permission: "audits.view",
  },
  {
    key: "system",
    label: "系统管理",
    icon: <TeamOutlined />,
    children: [
      {
        key: "/system/users",
        label: "用户",
        permission: "users.view",
      },
      {
        key: "/system/roles",
        label: "角色",
        permission: "roles.view",
      },
    ],
  },
];

export const hiddenNavTitles: Record<string, string> = {
  "/setup/admin": "初始化管理员",
  "/login": "登录",
  "/terminal": "终端",
  "/delivery/registries": "Registry",
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
