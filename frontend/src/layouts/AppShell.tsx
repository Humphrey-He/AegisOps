import {
  BellOutlined,
  LogoutOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  SafetyCertificateOutlined,
  SearchOutlined,
  UserOutlined,
} from "@ant-design/icons";
import {
  App as AntApp,
  Avatar,
  Breadcrumb,
  Button,
  Dropdown,
  Layout,
  Menu,
  Select,
  Space,
  Tag,
  Tooltip,
  Typography,
} from "antd";
import type { MenuProps } from "antd";
import { useEffect, useMemo, useState } from "react";
import { Link, Outlet, useLocation, useMatches, useNavigate } from "react-router-dom";
import { filterNavItems, navItems, type NavItem } from "../app/navigation";
import { authApi } from "../lib/api";
import { USE_MOCK } from "../lib/config";
import { useSessionStore } from "../store/sessionStore";

const { Header, Sider, Content } = Layout;

function toMenuItems(items: NavItem[]): MenuProps["items"] {
  return items.map((item) => ({
    key: item.key,
    icon: item.icon,
    title: item.label,
    label: item.children ? item.label : <Link to={item.key}>{item.label}</Link>,
    children: item.children ? toMenuItems(item.children) : undefined,
  }));
}

function flattenNavItems(items: NavItem[]): Array<{ key: string; label: string }> {
  return items.flatMap((item) =>
    item.children?.length ? flattenNavItems(item.children) : [{ key: item.key, label: item.label }],
  );
}

function collectOpenKeys(items: NavItem[]): string[] {
  return items.flatMap((item) => (item.children?.length ? [item.key, ...collectOpenKeys(item.children)] : []));
}

function findSelectedKey(pathname: string, items: NavItem[]): string {
  const leaves = items.flatMap((item) => (item.children?.length ? flattenNavItems(item.children) : [item]));
  return leaves.find((item) => pathname.startsWith(item.key))?.key ?? pathname;
}

function findSelectedParentKeys(pathname: string, items: NavItem[], parents: string[] = []): string[] {
  for (const item of items) {
    if (item.children?.length) {
      const matchedParents = findSelectedParentKeys(pathname, item.children, [...parents, item.key]);
      if (matchedParents.length) {
        return matchedParents;
      }
      continue;
    }
    if (pathname.startsWith(item.key)) {
      return parents;
    }
  }
  return [];
}

export function AppShell() {
  const { message } = AntApp.useApp();
  const user = useSessionStore((state) => state.user);
  const permissions = useSessionStore((state) => state.permissions);
  const clearSession = useSessionStore((state) => state.clearSession);
  const location = useLocation();
  const matches = useMatches();
  const navigate = useNavigate();
  const [collapsed, setCollapsed] = useState(false);
  const [quickJumpKeyword, setQuickJumpKeyword] = useState("");
  const [openKeys, setOpenKeys] = useState<string[]>([]);
  const isLocalEnvironment =
    typeof window !== "undefined" &&
    ["localhost", "127.0.0.1", "::1"].includes(window.location.hostname.toLowerCase());

  const visibleNavItems = useMemo(() => filterNavItems(navItems, permissions), [permissions]);
  const menuItems = useMemo(() => toMenuItems(visibleNavItems), [visibleNavItems]);
  const defaultOpenKeys = useMemo(() => collectOpenKeys(visibleNavItems), [visibleNavItems]);
  const quickJumpOptions = useMemo(
    () => flattenNavItems(visibleNavItems).map((item) => ({ value: item.key, label: item.label })),
    [visibleNavItems],
  );
  const selectedKey = findSelectedKey(location.pathname, visibleNavItems);
  const selectedParentKeys = useMemo(
    () => findSelectedParentKeys(location.pathname, visibleNavItems),
    [location.pathname, visibleNavItems],
  );

  useEffect(() => {
    setOpenKeys((current) => Array.from(new Set([...current, ...selectedParentKeys])));
  }, [selectedParentKeys]);

  useEffect(() => {
    setOpenKeys((current) => current.filter((key) => defaultOpenKeys.includes(key)));
  }, [defaultOpenKeys]);

  const breadcrumbItems = matches
    .filter((match) => (match.handle as { title?: string } | undefined)?.title)
    .slice(-3)
    .map((match) => ({
      title: <span>{(match.handle as { title: string }).title}</span>,
    }));

  const userMenu: MenuProps["items"] = [
    {
      key: "profile",
      icon: <UserOutlined />,
      label: <span>{user?.displayName ?? user?.username}</span>,
      disabled: true,
    },
    {
      key: "logout",
      icon: <LogoutOutlined />,
      label: "退出登录",
      onClick: async () => {
        await authApi.logout();
        clearSession();
        void message.success("已退出登录");
        navigate("/login", { replace: true });
      },
    },
  ];

  const environmentLabel = USE_MOCK ? "Demo" : isLocalEnvironment ? "Local" : "Production";
  const environmentTone = USE_MOCK ? "gold" : isLocalEnvironment ? "blue" : "green";
  const connectionLabel = USE_MOCK ? "Mock 数据源" : isLocalEnvironment ? "Local API 已连接" : "API 已连接";

  return (
    <Layout className="app-shell-root" style={{ minHeight: "100vh" }}>
      <Sider
        trigger={null}
        collapsible
        collapsed={collapsed}
        width={248}
        theme="light"
        className="app-shell-sider"
      >
        <Link to="/dashboard" className="app-shell-brand">
          <Avatar icon={<SafetyCertificateOutlined />} style={{ background: "#0f766e" }} />
          {!collapsed ? (
            <div className="app-shell-brand-meta">
              <Typography.Text strong>AegisOps</Typography.Text>
              <Typography.Text className="app-shell-brand-text">企业级运维控制台</Typography.Text>
              <Tag color={environmentTone} className="app-shell-env-tag">
                {environmentLabel}
              </Tag>
            </div>
          ) : null}
        </Link>

        <Menu
          className="app-shell-menu"
          mode="inline"
          inlineCollapsed={collapsed}
          selectedKeys={[selectedKey]}
          openKeys={collapsed ? undefined : openKeys}
          defaultOpenKeys={defaultOpenKeys}
          items={menuItems}
          onOpenChange={(keys) => setOpenKeys(keys)}
          style={{ borderInlineEnd: 0 }}
        />
      </Sider>

      <Layout>
        <Header className="app-shell-header">
          <Space size={16} style={{ minWidth: 0, flex: 1 }}>
            <Tooltip title={collapsed ? "展开导航" : "收起导航"}>
              <Button
                type="text"
                icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
                onClick={() => setCollapsed((value) => !value)}
              />
            </Tooltip>
            <div style={{ minWidth: 0, overflow: "hidden" }}>
              <Breadcrumb items={breadcrumbItems} />
            </div>
          </Space>

          <Space size={12} wrap>
            <Select
              showSearch
              allowClear
              value={undefined}
              searchValue={quickJumpKeyword}
              placeholder="快速跳转"
              style={{ width: 240 }}
              suffixIcon={<SearchOutlined />}
              options={quickJumpOptions}
              optionFilterProp="label"
              onSearch={setQuickJumpKeyword}
              onBlur={() => setQuickJumpKeyword("")}
              onClear={() => setQuickJumpKeyword("")}
              onSelect={(value) => {
                setQuickJumpKeyword("");
                if (typeof value === "string" && value) {
                  navigate(value);
                }
              }}
              filterOption={(input, option) =>
                String(option?.label ?? "")
                  .toLowerCase()
                  .includes(input.toLowerCase())
              }
            />

            <div className="shell-utility">
              <span className="shell-utility-dot" aria-hidden />
              <span>{connectionLabel}</span>
            </div>

            <Tooltip title="告警中心">
              <Button type="text" icon={<BellOutlined />} onClick={() => navigate("/alerts/events")} />
            </Tooltip>

            <Dropdown menu={{ items: userMenu }} placement="bottomRight">
              <Space style={{ cursor: "pointer" }}>
                <Avatar size="small" icon={<UserOutlined />} />
                <Typography.Text>{user?.displayName ?? "未登录"}</Typography.Text>
              </Space>
            </Dropdown>
          </Space>
        </Header>

        <Content className="app-shell-content">
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
}
