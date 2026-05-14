import { http } from "./http";
import { USE_MOCK } from "./config";
import { mockService } from "../mocks/service";
import { useSessionStore } from "../store/sessionStore";
import type {
  AdminSetupInput,
  DockerNodeInput,
  HostInput,
  LoginInput,
  RoleInput,
  SecretInputPayload,
  UserInput,
} from "../types/models";

function token() {
  return useSessionStore.getState().token;
}

export const authApi = {
  getSetupStatus: async () => {
    return USE_MOCK ? mockService.getSetupStatus() : http.get<{ initialized: boolean }>("/auth/setup-status");
  },
  initAdmin: async (payload: AdminSetupInput) => {
    return USE_MOCK ? mockService.initAdmin(payload) : http.post("/auth/setup", payload);
  },
  login: async (payload: LoginInput) => {
    return USE_MOCK ? mockService.login(payload) : http.post("/auth/login", payload);
  },
  me: async () => {
    return USE_MOCK ? mockService.me(token()) : http.get("/auth/me");
  },
  logout: async () => {
    return USE_MOCK ? mockService.logout(token()) : http.post("/auth/logout");
  },
};

export const dashboardApi = {
  summary: async () => {
    return USE_MOCK ? mockService.dashboardSummary(token()) : http.get("/dashboard/summary");
  },
};

export const usersApi = {
  list: async (keyword = "") => {
    return USE_MOCK ? mockService.listUsers(token(), keyword) : http.get(`/users?keyword=${encodeURIComponent(keyword)}`);
  },
  save: async (payload: UserInput) => {
    if (USE_MOCK) {
      return mockService.saveUser(token(), payload);
    }
    return payload.id ? http.patch(`/users/${payload.id}`, payload) : http.post("/users", payload);
  },
};

export const rolesApi = {
  list: async () => {
    return USE_MOCK ? mockService.listRoles(token()) : http.get("/roles");
  },
  save: async (payload: RoleInput) => {
    if (USE_MOCK) {
      return mockService.saveRole(token(), payload);
    }
    return payload.id ? http.patch(`/roles/${payload.id}`, payload) : http.post("/roles", payload);
  },
};

export const secretsApi = {
  list: async (keyword = "") => {
    return USE_MOCK ? mockService.listSecrets(token(), keyword) : http.get(`/secrets?keyword=${encodeURIComponent(keyword)}`);
  },
  save: async (payload: SecretInputPayload) => {
    if (USE_MOCK) {
      return mockService.saveSecret(token(), payload);
    }
    return payload.id ? http.patch(`/secrets/${payload.id}`, payload) : http.post("/secrets", payload);
  },
};

export const hostsApi = {
  list: async (keyword = "") => {
    return USE_MOCK ? mockService.listHosts(token(), keyword) : http.get(`/hosts?keyword=${encodeURIComponent(keyword)}`);
  },
  save: async (payload: HostInput) => {
    if (USE_MOCK) {
      return mockService.saveHost(token(), payload);
    }
    return payload.id ? http.patch(`/hosts/${payload.id}`, payload) : http.post("/hosts", payload);
  },
  testSsh: async (hostId: string) => {
    return USE_MOCK ? mockService.testHostSsh(token(), hostId) : http.post(`/hosts/${hostId}/test-ssh`);
  },
};

export const terminalApi = {
  create: async (hostId: string) => {
    return USE_MOCK
      ? mockService.createTerminalSession(token(), hostId)
      : http.post(`/hosts/${hostId}/terminal/sessions`);
  },
  detail: async (sessionId: string) => {
    return USE_MOCK
      ? mockService.getTerminalSession(token(), sessionId)
      : http.get(`/terminal/sessions/${sessionId}`);
  },
};

export const dockerApi = {
  listNodes: async () => {
    return USE_MOCK ? mockService.listDockerNodes(token()) : http.get("/docker/nodes");
  },
  saveNode: async (payload: DockerNodeInput) => {
    if (USE_MOCK) {
      return mockService.saveDockerNode(token(), payload);
    }
    return payload.id ? http.patch(`/docker/nodes/${payload.id}`, payload) : http.post("/docker/nodes", payload);
  },
  testNode: async (nodeId: string) => {
    return USE_MOCK ? mockService.testDockerNode(token(), nodeId) : http.post(`/docker/nodes/${nodeId}/test`);
  },
  getNode: async (nodeId: string) => {
    return USE_MOCK ? mockService.getDockerNode(token(), nodeId) : http.get(`/docker/nodes/${nodeId}`);
  },
  listContainers: async (nodeId: string) => {
    return USE_MOCK
      ? mockService.listContainers(token(), nodeId)
      : http.get(`/docker/nodes/${nodeId}/containers`);
  },
  getContainerLogs: async (containerId: string) => {
    return USE_MOCK
      ? mockService.getContainerLogs(token(), containerId)
      : http.get(`/docker/containers/${containerId}/logs`);
  },
  runContainerAction: async (nodeId: string, containerId: string, action: "start" | "stop" | "restart") => {
    return USE_MOCK
      ? mockService.performContainerAction(token(), nodeId, containerId, action)
      : http.post(`/docker/nodes/${nodeId}/containers/${containerId}/${action}`);
  },
};

export const tasksApi = {
  list: async () => {
    return USE_MOCK ? mockService.listTasks(token()) : http.get("/tasks");
  },
  detail: async (taskId: string) => {
    return USE_MOCK ? mockService.getTask(token(), taskId) : http.get(`/tasks/${taskId}`);
  },
};

export const auditsApi = {
  list: async () => {
    return USE_MOCK ? mockService.listAudits(token()) : http.get("/audits");
  },
};
