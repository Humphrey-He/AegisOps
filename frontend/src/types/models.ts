export type UserStatus = "ACTIVE" | "DISABLED";
export type HostStatus = "HEALTHY" | "UNREACHABLE" | "UNKNOWN" | "TESTING";
export type DockerNodeStatus = "ONLINE" | "OFFLINE" | "UNKNOWN" | "TESTING";
export type TaskStatus = "PENDING" | "RUNNING" | "SUCCESS" | "FAILED" | "CANCELED";
export type TaskLogLevel = "INFO" | "WARN" | "ERROR";
export type SecretType = "SSH_PASSWORD" | "SSH_PRIVATE_KEY" | "DOCKER_TOKEN";
export type AuditResult = "SUCCESS" | "FAILED";
export type TerminalSessionStatus = "CONNECTED" | "DISCONNECTED";

export type PermissionDefinition = {
  key: string;
  label: string;
  description: string;
  group: string;
};

export type Role = {
  id: string;
  name: string;
  description: string;
  permissions: string[];
  builtIn?: boolean;
  createdAt: string;
};

export type User = {
  id: string;
  username: string;
  displayName: string;
  email: string;
  status: UserStatus;
  roleIds: string[];
  createdAt: string;
  lastLoginAt?: string;
};

export type Secret = {
  id: string;
  name: string;
  type: SecretType;
  username?: string;
  description?: string;
  valueMasked: string;
  usedBy: string[];
  updatedAt: string;
};

export type Host = {
  id: string;
  name: string;
  address: string;
  port: number;
  secretId: string;
  status: HostStatus;
  tags: string[];
  description?: string;
  lastCheckedAt?: string;
};

export type DockerNode = {
  id: string;
  name: string;
  endpoint: string;
  tlsEnabled: boolean;
  status: DockerNodeStatus;
  description?: string;
  lastCheckedAt?: string;
  containerCount: number;
};

export type ContainerItem = {
  id: string;
  nodeId: string;
  name: string;
  image: string;
  status: "running" | "exited" | "paused";
  ports: string[];
  restartCount: number;
  createdAt: string;
};

export type TaskLog = {
  id: string;
  timestamp: string;
  level: TaskLogLevel;
  message: string;
};

export type TaskStep = {
  id: string;
  title: string;
  status: TaskStatus;
  detail?: string;
  startedAt?: string;
  finishedAt?: string;
};

export type Task = {
  id: string;
  type: string;
  status: TaskStatus;
  target: string;
  initiatedBy: string;
  progress: number;
  summary?: string;
  createdAt: string;
  startedAt?: string;
  finishedAt?: string;
  steps: TaskStep[];
  logs: TaskLog[];
};

export type AuditLog = {
  id: string;
  actor: string;
  action: string;
  resourceType: string;
  resourceName: string;
  result: AuditResult;
  traceId: string;
  createdAt: string;
  summary: string;
};

export type DashboardSummary = {
  userCount: number;
  hostCount: number;
  dockerNodeCount: number;
  containerCount: number;
  unhealthyResourceCount: number;
  recentTasks: Task[];
  recentAudits: AuditLog[];
};

export type TerminalSession = {
  id: string;
  hostId: string;
  hostName: string;
  status: TerminalSessionStatus;
  createdAt: string;
  welcomeLines: string[];
};

export type AdminSetupInput = {
  username: string;
  password: string;
  displayName: string;
  email: string;
};

export type LoginInput = {
  username: string;
  password: string;
};

export type UserInput = {
  id?: string;
  username: string;
  displayName: string;
  email: string;
  status: UserStatus;
  roleIds: string[];
  password?: string;
};

export type RoleInput = {
  id?: string;
  name: string;
  description: string;
  permissions: string[];
};

export type SecretInputPayload = {
  id?: string;
  name: string;
  type: SecretType;
  username?: string;
  description?: string;
  secretValue: string;
};

export type HostInput = {
  id?: string;
  name: string;
  address: string;
  port: number;
  secretId: string;
  tags: string[];
  description?: string;
};

export type DockerNodeInput = {
  id?: string;
  name: string;
  endpoint: string;
  tlsEnabled: boolean;
  description?: string;
};
