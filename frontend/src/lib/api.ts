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

export const authApi = {
  getSetupStatus: async (): Promise<SetupStatus> => {
    return USE_MOCK ? mockService.getSetupStatus() : http.get<SetupStatus>("/auth/setup-status");
  },
  initAdmin: async (payload: AdminSetupInput): Promise<{ created: boolean }> => {
    return USE_MOCK ? mockService.initAdmin(payload) : http.post<{ created: boolean }>("/auth/setup", payload);
  },
  login: async (payload: LoginInput): Promise<AuthSession> => {
    return USE_MOCK ? mockService.login(payload) : http.post<AuthSession>("/auth/login", payload);
  },
  me: async (): Promise<CurrentUserPayload> => {
    return USE_MOCK ? mockService.me(token()) : http.get<CurrentUserPayload>("/auth/me");
  },
  logout: async (): Promise<{ ok: boolean }> => {
    return USE_MOCK ? mockService.logout(token()) : http.post<{ ok: boolean }>("/auth/logout");
  },
};

export const dashboardApi = {
  summary: async (): Promise<DashboardSummary> => {
    return USE_MOCK ? mockService.dashboardSummary(token()) : http.get<DashboardSummary>("/dashboard/summary");
  },
};

export const usersApi = {
  list: async (keyword = ""): Promise<User[]> => {
    return USE_MOCK
      ? mockService.listUsers(token(), keyword)
      : http.get<User[]>(`/users?keyword=${encodeURIComponent(keyword)}`);
  },
  save: async (payload: UserInput): Promise<User> => {
    if (USE_MOCK) {
      return mockService.saveUser(token(), payload);
    }
    return payload.id ? http.patch<User>(`/users/${payload.id}`, payload) : http.post<User>("/users", payload);
  },
};

export const rolesApi = {
  list: async (): Promise<Role[]> => {
    return USE_MOCK ? mockService.listRoles(token()) : http.get<Role[]>("/roles");
  },
  save: async (payload: RoleInput): Promise<Role> => {
    if (USE_MOCK) {
      return mockService.saveRole(token(), payload);
    }
    return payload.id ? http.patch<Role>(`/roles/${payload.id}`, payload) : http.post<Role>("/roles", payload);
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
