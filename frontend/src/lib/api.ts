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
    return USE_MOCK ? mockService.dashboardSummary(token()) : http.get<DashboardSummary>("/dashboard/summary");
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
    return USE_MOCK
      ? mockService.listSecrets(token(), keyword)
      : http.get<Secret[]>(`/secrets?keyword=${encodeURIComponent(keyword)}`);
  },
  save: async (payload: SecretInputPayload): Promise<Secret> => {
    if (USE_MOCK) {
      return mockService.saveSecret(token(), payload);
    }
    return payload.id ? http.patch<Secret>(`/secrets/${payload.id}`, payload) : http.post<Secret>("/secrets", payload);
  },
};

export const hostsApi = {
  list: async (keyword = ""): Promise<Host[]> => {
    return USE_MOCK
      ? mockService.listHosts(token(), keyword)
      : http.get<Host[]>(`/hosts?keyword=${encodeURIComponent(keyword)}`);
  },
  save: async (payload: HostInput): Promise<Host> => {
    if (USE_MOCK) {
      return mockService.saveHost(token(), payload);
    }
    return payload.id ? http.patch<Host>(`/hosts/${payload.id}`, payload) : http.post<Host>("/hosts", payload);
  },
  testSsh: async (hostId: string): Promise<Task> => {
    return USE_MOCK ? mockService.testHostSsh(token(), hostId) : http.post<Task>(`/hosts/${hostId}/test-ssh`);
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
    return USE_MOCK ? mockService.listDockerNodes(token()) : http.get<DockerNode[]>("/docker/nodes");
  },
  saveNode: async (payload: DockerNodeInput): Promise<DockerNode> => {
    if (USE_MOCK) {
      return mockService.saveDockerNode(token(), payload);
    }
    return payload.id ? http.patch<DockerNode>(`/docker/nodes/${payload.id}`, payload) : http.post<DockerNode>("/docker/nodes", payload);
  },
  testNode: async (nodeId: string): Promise<Task> => {
    return USE_MOCK ? mockService.testDockerNode(token(), nodeId) : http.post<Task>(`/docker/nodes/${nodeId}/test`);
  },
  getNode: async (nodeId: string): Promise<DockerNode> => {
    return USE_MOCK ? mockService.getDockerNode(token(), nodeId) : http.get<DockerNode>(`/docker/nodes/${nodeId}`);
  },
  listContainers: async (nodeId: string): Promise<ContainerItem[]> => {
    return USE_MOCK
      ? mockService.listContainers(token(), nodeId)
      : http.get<ContainerItem[]>(`/docker/nodes/${nodeId}/containers`);
  },
  getContainerLogs: async (containerId: string): Promise<string[]> => {
    return USE_MOCK
      ? mockService.getContainerLogs(token(), containerId)
      : http.get<string[]>(`/docker/containers/${containerId}/logs`);
  },
  runContainerAction: async (nodeId: string, containerId: string, action: "start" | "stop" | "restart"): Promise<Task> => {
    return USE_MOCK
      ? mockService.performContainerAction(token(), nodeId, containerId, action)
      : http.post<Task>(`/docker/nodes/${nodeId}/containers/${containerId}/${action}`);
  },
};

export const tasksApi = {
  list: async (): Promise<Task[]> => {
    return USE_MOCK ? mockService.listTasks(token()) : http.get<Task[]>("/tasks");
  },
  detail: async (taskId: string): Promise<Task> => {
    return USE_MOCK ? mockService.getTask(token(), taskId) : http.get<Task>(`/tasks/${taskId}`);
  },
};

export const auditsApi = {
  list: async (): Promise<AuditLog[]> => {
    return USE_MOCK ? mockService.listAudits(token()) : http.get<AuditLog[]>("/audits");
  },
};
