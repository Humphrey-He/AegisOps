import { http } from "./http";
import { USE_MOCK } from "./config";
import { mockService } from "../mocks/service";
import { useSessionStore } from "../store/sessionStore";
import type {
  AdminSetupInput,
  AuthSession,
  AuditLog,
  ContainerItem,
  CurrentUserPayload,
  DashboardSummary,
  DockerNode,
  DockerNodeInput,
  Host,
  HostInput,
  LoginInput,
  Role,
  RoleInput,
  Secret,
  SecretInputPayload,
  SetupStatus,
  Task,
  TerminalSession,
  User,
  UserInput,
} from "../types/models";

function token() {
  return useSessionStore.getState().token;
}

type BackendPermission = {
  id: number;
  name?: string;
  code: string;
  resource?: string;
  action?: string;
  description?: string;
};

type BackendRole = {
  id: number;
  name: string;
  code: string;
  description?: string;
  permissions?: BackendPermission[];
  createdAt: string;
};

type BackendPage<T> = {
  items: T[];
  total?: number;
  page?: number;
  pageSize?: number;
  limit?: number;
  offset?: number;
};

type BackendUser = {
  id: number;
  username: string;
  displayName?: string;
  email?: string;
  status: "active" | "disabled" | "ACTIVE" | "DISABLED";
  isAdmin?: boolean;
  roles?: BackendRole[];
  createdAt: string;
  lastLoginAt?: string;
};

type BackendTokenPair = {
  accessToken: string;
  refreshToken: string;
  tokenType: string;
};

type BackendLoginResult = {
  user: BackendUser;
  tokens: BackendTokenPair;
};

type BackendTaskStep = {
  id: string;
  name: string;
  status: Task["status"];
  result?: string;
  error?: string;
  startedAt?: string;
  finishedAt?: string;
};

type BackendTaskLog = {
  id: string;
  level: "INFO" | "WARN" | "ERROR";
  message: string;
  createdAt: string;
};

type BackendTask = {
  id: string;
  type: string;
  title: string;
  status: Task["status"];
  targetType?: string;
  targetId?: string;
  result?: string;
  error?: string;
  createdBy?: string;
  createdAt: string;
  startedAt?: string;
  finishedAt?: string;
  steps?: BackendTaskStep[];
  logs?: BackendTaskLog[];
};

type BackendSecret = {
  id: string;
  name: string;
  type: Secret["type"];
  description?: string;
  maskedValue?: string;
  updatedAt: string;
};

type BackendHost = {
  id: string;
  name: string;
  address: string;
  sshPort: number;
  sshUser?: string;
  sshSecretId: string;
  group?: string;
  tags?: string;
  status: "ONLINE" | "OFFLINE" | "UNKNOWN" | Host["status"];
  lastTestAt?: string;
};

type BackendDockerNode = {
  id: string;
  name: string;
  endpoint: string;
  authType?: "NONE" | "TLS" | "TOKEN";
  secretId?: string;
  description?: string;
  status: DockerNode["status"];
  lastTestAt?: string;
};

type BackendAuditLog = {
  id: number;
  username?: string;
  action: string;
  resourceType?: string;
  resourceId?: string;
  result: "success" | "failure" | AuditLog["result"];
  message?: string;
  traceId?: string;
  createdAt: string;
};

type BackendDashboardSummary = {
  userCount: number;
  hostCount: number;
  dockerNodeCount: number;
  containerCount: number;
  unhealthyResourceCount: number;
  recentTasks: BackendTask[];
  recentAudits: BackendAuditLog[];
};

function normalizeUserStatus(status: BackendUser["status"]): User["status"] {
  return status === "disabled" || status === "DISABLED" ? "DISABLED" : "ACTIVE";
}

function mapBackendUser(user: BackendUser): User {
  return {
    id: String(user.id),
    username: user.username,
    displayName: user.displayName || user.username,
    email: user.email || "",
    status: normalizeUserStatus(user.status),
    roleIds: (user.roles ?? []).map((role) => String(role.id)),
    createdAt: user.createdAt,
    lastLoginAt: user.lastLoginAt,
  };
}

function mapBackendRole(role: BackendRole): Role {
  return {
    id: String(role.id),
    name: role.name,
    description: role.description || "",
    permissions: (role.permissions ?? []).map((permission) => permission.code),
    builtIn: role.code === "admin",
    createdAt: role.createdAt,
  };
}

function permissionsFromUser(user: BackendUser): string[] {
  if (user.isAdmin) {
    return ["*"];
  }
  return Array.from(
    new Set((user.roles ?? []).flatMap((role) => role.permissions ?? []).map((permission) => permission.code)),
  );
}

function pageItems<T>(page: BackendPage<T> | T[]): T[] {
  return Array.isArray(page) ? page : page.items ?? [];
}

function mapLoginResult(result: BackendLoginResult): AuthSession {
  return {
    token: result.tokens.accessToken,
    user: mapBackendUser(result.user),
    permissions: permissionsFromUser(result.user),
  };
}

function mapCurrentUser(user: BackendUser): CurrentUserPayload {
  return {
    user: mapBackendUser(user),
    permissions: permissionsFromUser(user),
  };
}

function progressFromStatus(status: Task["status"]): number {
  switch (status) {
    case "SUCCESS":
      return 100;
    case "FAILED":
    case "CANCELED":
      return 100;
    case "RUNNING":
      return 50;
    default:
      return 0;
  }
}

function mapBackendTask(task: BackendTask): Task {
  return {
    id: task.id,
    type: task.type,
    status: task.status,
    target: [task.targetType, task.targetId].filter(Boolean).join(":") || task.title,
    resourceType: task.targetType,
    resourceId: task.targetId,
    initiatedBy: task.createdBy || "-",
    progress: progressFromStatus(task.status),
    summary: task.error || task.result || task.title,
    createdAt: task.createdAt,
    startedAt: task.startedAt,
    finishedAt: task.finishedAt,
    steps: (task.steps ?? []).map((step) => ({
      id: step.id,
      title: step.name,
      status: step.status,
      detail: step.error || step.result,
      startedAt: step.startedAt,
      finishedAt: step.finishedAt,
    })),
    logs: (task.logs ?? []).map((log) => ({
      id: log.id,
      timestamp: log.createdAt,
      level: log.level,
      message: log.message,
    })),
  };
}

function mapBackendSecret(secret: BackendSecret): Secret {
  return {
    id: secret.id,
    name: secret.name,
    type: secret.type,
    description: secret.description,
    valueMasked: secret.maskedValue || "******",
    usedBy: [],
    updatedAt: secret.updatedAt,
  };
}

function mapHostStatus(status: BackendHost["status"]): Host["status"] {
  if (status === "ONLINE") {
    return "HEALTHY";
  }
  if (status === "OFFLINE") {
    return "UNREACHABLE";
  }
  return status === "TESTING" ? "TESTING" : "UNKNOWN";
}

function parseTags(tags?: string): string[] {
  if (!tags) {
    return [];
  }
  try {
    const parsed = JSON.parse(tags);
    if (Array.isArray(parsed)) {
      return parsed.map(String);
    }
  } catch {
    // Fall through to comma parsing for legacy/plain text tag storage.
  }
  return tags
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
}

function serializeTags(tags: string[]): string {
  return JSON.stringify(tags ?? []);
}

function mapBackendHost(host: BackendHost): Host {
  return {
    id: host.id,
    name: host.name,
    address: host.address,
    port: host.sshPort,
    secretId: host.sshSecretId,
    status: mapHostStatus(host.status),
    tags: parseTags(host.tags),
    description: host.group,
    lastCheckedAt: host.lastTestAt,
  };
}

function mapBackendDockerNode(node: BackendDockerNode): DockerNode {
  return {
    id: node.id,
    name: node.name,
    endpoint: node.endpoint,
    tlsEnabled: node.authType === "TLS",
    status: node.status,
    description: node.description,
    lastCheckedAt: node.lastTestAt,
    containerCount: 0,
  };
}

function mapBackendAudit(audit: BackendAuditLog): AuditLog {
  return {
    id: String(audit.id),
    actor: audit.username || "-",
    action: audit.action,
    resourceType: audit.resourceType || "-",
    resourceName: audit.resourceId || "-",
    result: audit.result === "success" ? "SUCCESS" : "FAILED",
    traceId: audit.traceId || "",
    createdAt: audit.createdAt,
    summary: audit.message || audit.action,
  };
}

function mapBackendDashboardSummary(summary: BackendDashboardSummary): DashboardSummary {
  return {
    userCount: summary.userCount ?? 0,
    hostCount: summary.hostCount ?? 0,
    dockerNodeCount: summary.dockerNodeCount ?? 0,
    containerCount: summary.containerCount ?? 0,
    unhealthyResourceCount: summary.unhealthyResourceCount ?? 0,
    recentTasks: (summary.recentTasks ?? []).map(mapBackendTask),
    recentAudits: (summary.recentAudits ?? []).map(mapBackendAudit),
  };
}

export const authApi = {
  getSetupStatus: async (): Promise<SetupStatus> => {
    if (USE_MOCK) {
      return mockService.getSetupStatus();
    }
    return { initialized: true };
  },
  initAdmin: async (payload: AdminSetupInput): Promise<{ created: boolean }> => {
    return USE_MOCK ? mockService.initAdmin(payload) : Promise.resolve({ created: false });
  },
  login: async (payload: LoginInput): Promise<AuthSession> => {
    if (USE_MOCK) {
      return mockService.login(payload);
    }
    const result = await http.post<BackendLoginResult>("/auth/login", payload);
    return mapLoginResult(result);
  },
  me: async (): Promise<CurrentUserPayload> => {
    if (USE_MOCK) {
      return mockService.me(token());
    }
    const user = await http.get<BackendUser>("/auth/me");
    return mapCurrentUser(user);
  },
  logout: async (): Promise<{ ok: boolean }> => {
    if (USE_MOCK) {
      return mockService.logout(token());
    }
    await http.post<void>("/auth/logout");
    return { ok: true };
  },
};

export const dashboardApi = {
  summary: async (): Promise<DashboardSummary> => {
    if (USE_MOCK) {
      return mockService.dashboardSummary(token());
    }
    const summary = await http.get<BackendDashboardSummary>("/dashboard/summary");
    return mapBackendDashboardSummary(summary);
  },
};

export const usersApi = {
  list: async (keyword = ""): Promise<User[]> => {
    if (USE_MOCK) {
      return mockService.listUsers(token(), keyword);
    }
    const page = await http.get<BackendPage<BackendUser>>(`/users?keyword=${encodeURIComponent(keyword)}`);
    return pageItems(page).map(mapBackendUser);
  },
  save: async (payload: UserInput): Promise<User> => {
    if (USE_MOCK) {
      return mockService.saveUser(token(), payload);
    }
    const backendPayload = {
      username: payload.username,
      password: payload.password,
      displayName: payload.displayName,
      email: payload.email,
      status: payload.status === "DISABLED" ? "disabled" : "active",
      roleIds: payload.roleIds.map((id) => Number(id)),
    };
    const user = payload.id
      ? await http.patch<BackendUser>(`/users/${payload.id}`, backendPayload)
      : await http.post<BackendUser>("/users", backendPayload);
    return mapBackendUser(user);
  },
};

export const rolesApi = {
  list: async (): Promise<Role[]> => {
    if (USE_MOCK) {
      return mockService.listRoles(token());
    }
    const page = await http.get<BackendPage<BackendRole>>("/roles");
    return pageItems(page).map(mapBackendRole);
  },
  save: async (payload: RoleInput): Promise<Role> => {
    if (USE_MOCK) {
      return mockService.saveRole(token(), payload);
    }
    const backendPayload = {
      name: payload.name,
      code: payload.name.toLowerCase().replace(/\s+/g, "_"),
      description: payload.description,
      permissionIds: [],
    };
    const role = payload.id
      ? await http.patch<BackendRole>(`/roles/${payload.id}`, backendPayload)
      : await http.post<BackendRole>("/roles", backendPayload);
    return mapBackendRole(role);
  },
};

export const secretsApi = {
  list: async (keyword = ""): Promise<Secret[]> => {
    if (USE_MOCK) {
      return mockService.listSecrets(token(), keyword);
    }
    const page = await http.get<BackendPage<BackendSecret>>(`/secrets?keyword=${encodeURIComponent(keyword)}`);
    return pageItems(page).map(mapBackendSecret);
  },
  save: async (payload: SecretInputPayload): Promise<Secret> => {
    if (USE_MOCK) {
      return mockService.saveSecret(token(), payload);
    }
    const backendPayload = {
      name: payload.name,
      type: payload.type,
      description: payload.description,
      value: payload.secretValue,
    };
    const secret = payload.id
      ? await http.patch<BackendSecret>(`/secrets/${payload.id}`, backendPayload)
      : await http.post<BackendSecret>("/secrets", backendPayload);
    return mapBackendSecret(secret);
  },
};

export const hostsApi = {
  list: async (keyword = ""): Promise<Host[]> => {
    if (USE_MOCK) {
      return mockService.listHosts(token(), keyword);
    }
    const page = await http.get<BackendPage<BackendHost>>(`/hosts?keyword=${encodeURIComponent(keyword)}`);
    return pageItems(page).map(mapBackendHost);
  },
  detail: async (hostId: string): Promise<Host> => {
    if (USE_MOCK) {
      const hosts = await mockService.listHosts(token(), "");
      const host = hosts.find((item) => item.id === hostId);
      if (!host) {
        throw new Error("主机不存在。");
      }
      return host;
    }
    const host = await http.get<BackendHost>(`/hosts/${hostId}`);
    return mapBackendHost(host);
  },
  save: async (payload: HostInput): Promise<Host> => {
    if (USE_MOCK) {
      return mockService.saveHost(token(), payload);
    }
    const backendPayload = {
      name: payload.name,
      address: payload.address,
      sshPort: payload.port,
      sshUser: "root",
      sshSecretId: payload.secretId,
      group: payload.description,
      tags: serializeTags(payload.tags),
    };
    const host = payload.id
      ? await http.patch<BackendHost>(`/hosts/${payload.id}`, backendPayload)
      : await http.post<BackendHost>("/hosts", backendPayload);
    return mapBackendHost(host);
  },
  testSsh: async (hostId: string): Promise<{ connected: boolean }> => {
    return USE_MOCK ? mockService.testHostSsh(token(), hostId).then(() => ({ connected: true })) : http.post<{ connected: boolean }>(`/hosts/${hostId}/test-ssh`);
  },
};

export const terminalApi = {
  create: async (hostId: string): Promise<TerminalSession> => {
    return USE_MOCK
      ? mockService.createTerminalSession(token(), hostId)
      : http.post<TerminalSession>(`/hosts/${hostId}/terminal/sessions`);
  },
  detail: async (sessionId: string): Promise<TerminalSession> => {
    return USE_MOCK
      ? mockService.getTerminalSession(token(), sessionId)
      : http.get<TerminalSession>(`/terminal/sessions/${sessionId}`);
  },
};

export const dockerApi = {
  listNodes: async (): Promise<DockerNode[]> => {
    if (USE_MOCK) {
      return mockService.listDockerNodes(token());
    }
    const page = await http.get<BackendPage<BackendDockerNode>>("/docker/nodes");
    return pageItems(page).map(mapBackendDockerNode);
  },
  saveNode: async (payload: DockerNodeInput): Promise<DockerNode> => {
    if (USE_MOCK) {
      return mockService.saveDockerNode(token(), payload);
    }
    const backendPayload = {
      name: payload.name,
      endpoint: payload.endpoint,
      authType: payload.tlsEnabled ? "TLS" : "NONE",
      description: payload.description,
    };
    const node = payload.id
      ? await http.patch<BackendDockerNode>(`/docker/nodes/${payload.id}`, backendPayload)
      : await http.post<BackendDockerNode>("/docker/nodes", backendPayload);
    return mapBackendDockerNode(node);
  },
  testNode: async (nodeId: string): Promise<{ connected: boolean }> => {
    return USE_MOCK ? mockService.testDockerNode(token(), nodeId).then(() => ({ connected: true })) : http.post<{ connected: boolean }>(`/docker/nodes/${nodeId}/test`);
  },
  getNode: async (nodeId: string): Promise<DockerNode> => {
    if (USE_MOCK) {
      return mockService.getDockerNode(token(), nodeId);
    }
    const node = await http.get<BackendDockerNode>(`/docker/nodes/${nodeId}`);
    return mapBackendDockerNode(node);
  },
  listContainers: async (nodeId: string): Promise<ContainerItem[]> => {
    return USE_MOCK
      ? mockService.listContainers(token(), nodeId)
      : http.get<ContainerItem[]>(`/docker/nodes/${nodeId}/containers`);
  },
  getContainerLogs: async (nodeId: string, containerId: string): Promise<string[]> => {
    if (USE_MOCK) {
      return mockService.getContainerLogs(token(), containerId);
    }
    const result = await http.get<{ logs: string }>(`/docker/nodes/${nodeId}/containers/${containerId}/logs`);
    return result.logs ? result.logs.split(/\r?\n/).filter(Boolean) : [];
  },
  runContainerAction: async (
    nodeId: string,
    containerId: string,
    action: "start" | "stop" | "restart",
  ): Promise<{ ok: boolean }> => {
    return USE_MOCK
      ? mockService.performContainerAction(token(), nodeId, containerId, action).then(() => ({ ok: true }))
      : http.post<{ started?: boolean; stopped?: boolean; restarted?: boolean }>(
          `/docker/nodes/${nodeId}/containers/${containerId}/${action}`,
        ).then(() => ({ ok: true }));
  },
};

export const tasksApi = {
  list: async (): Promise<Task[]> => {
    if (USE_MOCK) {
      return mockService.listTasks(token());
    }
    const page = await http.get<BackendPage<BackendTask>>("/tasks");
    return pageItems(page).map(mapBackendTask);
  },
  detail: async (taskId: string): Promise<Task> => {
    if (USE_MOCK) {
      return mockService.getTask(token(), taskId);
    }
    const task = await http.get<BackendTask>(`/tasks/${taskId}`);
    return mapBackendTask(task);
  },
};

export const auditsApi = {
  list: async (): Promise<AuditLog[]> => {
    if (USE_MOCK) {
      return mockService.listAudits(token());
    }
    const page = await http.get<BackendPage<BackendAuditLog>>("/audits");
    return pageItems(page).map(mapBackendAudit);
  },
};
