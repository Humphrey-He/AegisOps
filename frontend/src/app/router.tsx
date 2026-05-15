import { Navigate, createBrowserRouter, useLocation } from "react-router-dom";

import { AppShell } from "../layouts/AppShell";
import { LoginPage } from "../pages/auth/LoginPage";
import { SetupAdminPage } from "../pages/auth/SetupAdminPage";
import { DashboardPage } from "../pages/dashboard/DashboardPage";
import { HostsPage } from "../pages/assets/HostsPage";
import { SecretsPage } from "../pages/secrets/SecretsPage";
import { RegistriesPage } from "../pages/registries/RegistriesPage";
import { ServicesPage } from "../pages/services/ServicesPage";
import { DockerNodesPage } from "../pages/docker/DockerNodesPage";
import { DockerNodeDetailPage } from "../pages/docker/DockerNodeDetailPage";
import { TasksPage } from "../pages/tasks/TasksPage";
import { TaskDetailPage } from "../pages/tasks/TaskDetailPage";
import { AuditsPage } from "../pages/audits/AuditsPage";
import { UsersPage } from "../pages/system/UsersPage";
import { RolesPage } from "../pages/system/RolesPage";
import { TerminalPage } from "../pages/terminal/TerminalPage";
import { ForbiddenPage } from "../pages/errors/ForbiddenPage";
import { NotFoundPage } from "../pages/errors/NotFoundPage";
import { useSessionStore } from "../store/sessionStore";
import { PermissionGuard } from "../components/PermissionGuard";

function RequireAuth() {
  const token = useSessionStore((state) => state.token);
  const location = useLocation();

  if (!token) {
    return <Navigate to="/login" replace state={{ from: location.pathname }} />;
  }

  return <AppShell />;
}

function RequirePermission({ permission, children }: { permission: string; children: React.ReactNode }) {
  return (
    <PermissionGuard permission={permission} forbiddenPage>
      {children}
    </PermissionGuard>
  );
}

export const router = createBrowserRouter([
  {
    path: "/setup/admin",
    element: <SetupAdminPage />,
    handle: { title: "初始化管理员" },
  },
  {
    path: "/login",
    element: <LoginPage />,
    handle: { title: "登录" },
  },
  {
    path: "/",
    element: <RequireAuth />,
    children: [
      {
        index: true,
        element: <Navigate to="/dashboard" replace />,
      },
      {
        path: "dashboard",
        element: (
          <RequirePermission permission="dashboard.view">
            <DashboardPage />
          </RequirePermission>
        ),
        handle: { title: "工作台" },
      },
      {
        path: "assets/hosts",
        element: (
          <RequirePermission permission="hosts.view">
            <HostsPage />
          </RequirePermission>
        ),
        handle: { title: "主机" },
      },
      {
        path: "assets/secrets",
        element: (
          <RequirePermission permission="secrets.view">
            <SecretsPage />
          </RequirePermission>
        ),
        handle: { title: "凭证" },
      },
      {
        path: "delivery/registries",
        element: (
          <RequirePermission permission="registries.view">
            <RegistriesPage />
          </RequirePermission>
        ),
        handle: { title: "Registry" },
      },
      {
        path: "delivery/services",
        element: (
          <RequirePermission permission="services.view">
            <ServicesPage />
          </RequirePermission>
        ),
        handle: { title: "服务定义" },
      },
      {
        path: "docker/nodes",
        element: (
          <RequirePermission permission="docker.view">
            <DockerNodesPage />
          </RequirePermission>
        ),
        handle: { title: "Docker 节点" },
      },
      {
        path: "docker/nodes/:nodeId",
        element: (
          <RequirePermission permission="docker.view">
            <DockerNodeDetailPage />
          </RequirePermission>
        ),
        handle: { title: "Docker 节点详情" },
      },
      {
        path: "tasks",
        element: (
          <RequirePermission permission="tasks.view">
            <TasksPage />
          </RequirePermission>
        ),
        handle: { title: "任务中心" },
      },
      {
        path: "tasks/:taskId",
        element: (
          <RequirePermission permission="tasks.view">
            <TaskDetailPage />
          </RequirePermission>
        ),
        handle: { title: "任务详情" },
      },
      {
        path: "audits",
        element: (
          <RequirePermission permission="audits.view">
            <AuditsPage />
          </RequirePermission>
        ),
        handle: { title: "审计日志" },
      },
      {
        path: "system/users",
        element: (
          <RequirePermission permission="users.view">
            <UsersPage />
          </RequirePermission>
        ),
        handle: { title: "用户" },
      },
      {
        path: "system/roles",
        element: (
          <RequirePermission permission="roles.view">
            <RolesPage />
          </RequirePermission>
        ),
        handle: { title: "角色" },
      },
      {
        path: "terminal/:sessionId",
        element: (
          <RequirePermission permission="terminal.open">
            <TerminalPage />
          </RequirePermission>
        ),
        handle: { title: "终端" },
      },
      {
        path: "403",
        element: <ForbiddenPage />,
        handle: { title: "无权限" },
      },
      {
        path: "*",
        element: <NotFoundPage />,
        handle: { title: "页面不存在" },
      },
    ],
  },
]);
