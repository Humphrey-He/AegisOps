import type { PermissionDefinition } from "../types/models";

export const permissionCatalog: PermissionDefinition[] = [
  { key: "dashboard.view", label: "查看工作台", description: "访问工作台总览", group: "工作台" },
  { key: "hosts.view", label: "查看主机", description: "查看主机资产", group: "资产管理" },
  { key: "hosts.manage", label: "管理主机", description: "创建与编辑主机", group: "资产管理" },
  { key: "secrets.view", label: "查看凭证", description: "查看凭证列表", group: "资产管理" },
  { key: "secrets.manage", label: "管理凭证", description: "新增与编辑凭证", group: "资产管理" },
  { key: "registries.view", label: "查看镜像仓库", description: "查看 Registry 列表与仓库内容", group: "交付发布" },
  { key: "registries.manage", label: "管理镜像仓库", description: "创建、编辑、测试与删除 Registry", group: "交付发布" },
  { key: "services.view", label: "查看服务定义", description: "查看服务列表、版本和实例", group: "交付发布" },
  { key: "services.manage", label: "管理服务定义", description: "创建、编辑与删除服务定义", group: "交付发布" },
  { key: "services.release", label: "执行服务发布", description: "发布、升级和回滚服务版本", group: "交付发布" },
  { key: "terminal.open", label: "打开终端", description: "创建 WebSSH 会话", group: "运维执行" },
  { key: "docker.view", label: "查看 Docker", description: "查看节点与容器", group: "运维执行" },
  { key: "docker.manage", label: "管理 Docker", description: "执行容器启停操作", group: "运维执行" },
  { key: "tasks.view", label: "查看任务", description: "查看任务中心", group: "运维执行" },
  { key: "audits.view", label: "查看审计", description: "查看审计日志", group: "安全审计" },
  { key: "users.view", label: "查看用户", description: "查看用户列表", group: "系统管理" },
  { key: "users.manage", label: "管理用户", description: "新增与编辑用户", group: "系统管理" },
  { key: "roles.view", label: "查看角色", description: "查看角色列表", group: "系统管理" },
  { key: "roles.manage", label: "管理角色", description: "创建角色并配置权限", group: "系统管理" },
];

export const permissionGroups = permissionCatalog.reduce<Record<string, PermissionDefinition[]>>((acc, item) => {
  acc[item.group] = acc[item.group] ?? [];
  acc[item.group].push(item);
  return acc;
}, {});
