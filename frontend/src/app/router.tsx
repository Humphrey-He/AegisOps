import { Navigate, createBrowserRouter, useLocation } from "react-router-dom";
import { Suspense, lazy } from "react";

import { AppShell } from "../layouts/AppShell";
import { useSessionStore } from "../store/sessionStore";
import { PermissionGuard } from "../components/PermissionGuard";

const LoginPage = lazy(() => import("../pages/auth/LoginPage").then((module) => ({ default: module.LoginPage })));
const SetupAdminPage = lazy(() => import("../pages/auth/SetupAdminPage").then((module) => ({ default: module.SetupAdminPage })));
const DashboardPage = lazy(() => import("../pages/dashboard/DashboardPage").then((module) => ({ default: module.DashboardPage })));
const HostsPage = lazy(() => import("../pages/assets/HostsPage").then((module) => ({ default: module.HostsPage })));
const SecretsPage = lazy(() => import("../pages/secrets/SecretsPage").then((module) => ({ default: module.SecretsPage })));
const RegistriesPage = lazy(() => import("../pages/registries/RegistriesPage").then((module) => ({ default: module.RegistriesPage })));
const ServicesPage = lazy(() => import("../pages/services/ServicesPage").then((module) => ({ default: module.ServicesPage })));
const DockerNodesPage = lazy(() => import("../pages/docker/DockerNodesPage").then((module) => ({ default: module.DockerNodesPage })));
const DockerNodeDetailPage = lazy(() => import("../pages/docker/DockerNodeDetailPage").then((module) => ({ default: module.DockerNodeDetailPage })));
const NginxNodesPage = lazy(() => import("../pages/nginx/NginxNodesPage").then((module) => ({ default: module.NginxNodesPage })));
const NotificationsPage = lazy(() => import("../pages/settings/NotificationsPage").then((module) => ({ default: module.NotificationsPage })));
const AlertRulesPage = lazy(() => import("../pages/settings/AlertRulesPage").then((module) => ({ default: module.AlertRulesPage })));
const AlertEventsPage = lazy(() => import("../pages/alerts/AlertEventsPage").then((module) => ({ default: module.AlertEventsPage })));
const TasksPage = lazy(() => import("../pages/tasks/TasksPage").then((module) => ({ default: module.TasksPage })));
const TaskDetailPage = lazy(() => import("../pages/tasks/TaskDetailPage").then((module) => ({ default: module.TaskDetailPage })));
const AuditsPage = lazy(() => import("../pages/audits/AuditsPage").then((module) => ({ default: module.AuditsPage })));
const UsersPage = lazy(() => import("../pages/system/UsersPage").then((module) => ({ default: module.UsersPage })));
const RolesPage = lazy(() => import("../pages/system/RolesPage").then((module) => ({ default: module.RolesPage })));
const ScheduledJobsPage = lazy(() =>
  import("../pages/system/ScheduledJobsPage").then((module) => ({ default: module.ScheduledJobsPage })),
);
const TerminalPage = lazy(() => import("../pages/terminal/TerminalPage").then((module) => ({ default: module.TerminalPage })));
const ForbiddenPage = lazy(() => import("../pages/errors/ForbiddenPage").then((module) => ({ default: module.ForbiddenPage })));
const NotFoundPage = lazy(() => import("../pages/errors/NotFoundPage").then((module) => ({ default: module.NotFoundPage })));

function LazyPage({ children }: { children: React.ReactNode }) {
  return <Suspense fallback={null}>{children}</Suspense>;
}

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
    element: (
      <LazyPage>
        <SetupAdminPage />
      </LazyPage>
    ),
    handle: { title: "初始化管理员" },
  },
  {
    path: "/login",
    element: (
      <LazyPage>
        <LoginPage />
      </LazyPage>
    ),
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
            <LazyPage>
              <DashboardPage />
            </LazyPage>
          </RequirePermission>
        ),
        handle: { title: "工作台" },
      },
      {
        path: "assets/hosts",
        element: (
          <RequirePermission permission="hosts.view">
            <LazyPage>
              <HostsPage />
            </LazyPage>
          </RequirePermission>
        ),
        handle: { title: "主机" },
      },
      {
        path: "assets/secrets",
        element: (
          <RequirePermission permission="secrets.view">
            <LazyPage>
              <SecretsPage />
            </LazyPage>
          </RequirePermission>
        ),
        handle: { title: "凭证" },
      },
      {
        path: "delivery/registries",
        element: (
          <RequirePermission permission="registries.view">
            <LazyPage>
              <RegistriesPage />
            </LazyPage>
          </RequirePermission>
        ),
        handle: { title: "Registry" },
      },
      {
        path: "delivery/services",
        element: (
          <RequirePermission permission="services.view">
            <LazyPage>
              <ServicesPage />
            </LazyPage>
          </RequirePermission>
        ),
        handle: { title: "服务定义" },
      },
      {
        path: "docker/nodes",
        element: (
          <RequirePermission permission="docker.view">
            <LazyPage>
              <DockerNodesPage />
            </LazyPage>
          </RequirePermission>
        ),
        handle: { title: "Docker 节点" },
      },
      {
        path: "docker/nodes/:nodeId",
        element: (
          <RequirePermission permission="docker.view">
            <LazyPage>
              <DockerNodeDetailPage />
            </LazyPage>
          </RequirePermission>
        ),
        handle: { title: "Docker 节点详情" },
      },
      {
        path: "nginx/nodes",
        element: (
          <RequirePermission permission="nginx.view">
            <LazyPage>
              <NginxNodesPage />
            </LazyPage>
          </RequirePermission>
        ),
        handle: { title: "Nginx" },
      },
      {
        path: "settings/notifications",
        element: (
          <RequirePermission permission="notifications.view">
            <LazyPage>
              <NotificationsPage />
            </LazyPage>
          </RequirePermission>
        ),
        handle: { title: "通知通道" },
      },
      {
        path: "settings/alert-rules",
        element: (
          <RequirePermission permission="alerts.view">
            <LazyPage>
              <AlertRulesPage />
            </LazyPage>
          </RequirePermission>
        ),
        handle: { title: "告警规则" },
      },
      {
        path: "alerts/events",
        element: (
          <RequirePermission permission="alerts.view">
            <LazyPage>
              <AlertEventsPage />
            </LazyPage>
          </RequirePermission>
        ),
        handle: { title: "告警事件" },
      },
      {
        path: "tasks",
        element: (
          <RequirePermission permission="tasks.view">
            <LazyPage>
              <TasksPage />
            </LazyPage>
          </RequirePermission>
        ),
        handle: { title: "任务中心" },
      },
      {
        path: "tasks/:taskId",
        element: (
          <RequirePermission permission="tasks.view">
            <LazyPage>
              <TaskDetailPage />
            </LazyPage>
          </RequirePermission>
        ),
        handle: { title: "任务详情" },
      },
      {
        path: "audits",
        element: (
          <RequirePermission permission="audits.view">
            <LazyPage>
              <AuditsPage />
            </LazyPage>
          </RequirePermission>
        ),
        handle: { title: "审计日志" },
      },
      {
        path: "system/users",
        element: (
          <RequirePermission permission="users.view">
            <LazyPage>
              <UsersPage />
            </LazyPage>
          </RequirePermission>
        ),
        handle: { title: "用户" },
      },
      {
        path: "system/roles",
        element: (
          <RequirePermission permission="roles.view">
            <LazyPage>
              <RolesPage />
            </LazyPage>
          </RequirePermission>
        ),
        handle: { title: "角色" },
      },
      {
        path: "system/scheduled-jobs",
        element: (
          <RequirePermission permission="scheduler.view">
            <LazyPage>
              <ScheduledJobsPage />
            </LazyPage>
          </RequirePermission>
        ),
        handle: { title: "调度任务" },
      },
      {
        path: "terminal/:sessionId",
        element: (
          <RequirePermission permission="terminal.open">
            <LazyPage>
              <TerminalPage />
            </LazyPage>
          </RequirePermission>
        ),
        handle: { title: "终端" },
      },
      {
        path: "403",
        element: (
          <LazyPage>
            <ForbiddenPage />
          </LazyPage>
        ),
        handle: { title: "无权限" },
      },
      {
        path: "*",
        element: (
          <LazyPage>
            <NotFoundPage />
          </LazyPage>
        ),
        handle: { title: "页面不存在" },
      },
    ],
  },
]);
