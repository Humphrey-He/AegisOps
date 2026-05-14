import {
  LogoutOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  SafetyCertificateOutlined,
  UserOutlined,
} from "@ant-design/icons";
import { Avatar, Breadcrumb, Button, Dropdown, Layout, Menu, Space, Tag, Typography, App as AntApp } from "antd";
import type { MenuProps } from "antd";
import { useMemo, useState } from "react";
import { Link, Outlet, useLocation, useMatches, useNavigate } from "react-router-dom";
import { authApi } from "../lib/api";
import { filterNavItems, findFirstPath, navItems, type NavItem } from "../app/navigation";
import { useSessionStore } from "../store/sessionStore";

const { Header, Sider, Content } = Layout;

function toMenuItems(items: NavItem[]): MenuProps["items"] {
  return items.map((item) => ({
    key: item.key,
    icon: item.icon,
    label: item.children ? item.label : <Link to={item.key}>{item.label}</Link>,
    children: item.children ? toMenuItems(item.children) : undefined,
  }));
}

function findSelectedKey(pathname: string, items: NavItem[]): string {
  const leaves = items.flatMap((item) => (item.children?.length ? item.children : [item]));
  return leaves.find((item) => pathname.startsWith(item.key))?.key ?? pathname;
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

  const visibleNavItems = useMemo(() => filterNavItems(navItems, permissions), [permissions]);
  const menuItems = useMemo(() => toMenuItems(visibleNavItems), [visibleNavItems]);
  const selectedKey = findSelectedKey(location.pathname, visibleNavItems);
  const defaultTarget = findFirstPath(visibleNavItems) ?? "/dashboard";

  const breadcrumbItems = matches
    .filter((match) => (match.handle as { title?: string } | undefined)?.title)
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

  return (
    <Layout style={{ minHeight: "100vh" }}>
      <Sider trigger={null} collapsible collapsed={collapsed} width={248} theme="light">
        <div style={{ padding: 20, display: "flex", alignItems: "center", gap: 12 }}>
          <Avatar icon={<SafetyCertificateOutlined />} style={{ background: "#0f766e" }} />
          {!collapsed ? (
            <Space direction="vertical" size={0}>
              <Typography.Text strong>AegisOps</Typography.Text>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                MVP Frontend
              </Typography.Text>
            </Space>
          ) : null}
        </div>
        <Menu mode="inline" selectedKeys={[selectedKey]} defaultOpenKeys={["assets", "docker", "system"]} items={menuItems} />
      </Sider>
      <Layout>
        <Header
          style={{
            padding: "0 20px",
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            background: "#ffffff",
            borderBottom: "1px solid #e5e7eb",
          }}
        >
          <Space>
            <Button
              type="text"
              icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
              onClick={() => setCollapsed((value) => !value)}
            />
            <Breadcrumb items={breadcrumbItems} />
          </Space>
          <Space size={16}>
            <Tag color="cyan">Mock API</Tag>
            <Tag>{defaultTarget}</Tag>
            <Dropdown menu={{ items: userMenu }} placement="bottomRight">
              <Space style={{ cursor: "pointer" }}>
                <Avatar size="small" icon={<UserOutlined />} />
                <Typography.Text>{user?.displayName ?? "未登录"}</Typography.Text>
              </Space>
            </Dropdown>
          </Space>
        </Header>
        <Content style={{ padding: 20 }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
}
