export type UserStatus = "ACTIVE" | "DISABLED";
export type HostStatus = "HEALTHY" | "UNREACHABLE" | "UNKNOWN" | "TESTING";
export type DockerNodeStatus = "ONLINE" | "OFFLINE" | "UNKNOWN" | "TESTING";
export type RegistryAuthType = "NONE" | "BASIC" | "TOKEN";
export type RegistryStatus = "ONLINE" | "OFFLINE" | "UNKNOWN";
export type ServiceStatus = "DRAFT" | "ACTIVE" | "ARCHIVED";
export type ServiceInstanceStatus = "PENDING" | "RUNNING" | "STOPPED" | "FAILED" | "ROLLBACK";
export type ServiceReleaseAction = "RELEASE" | "UPGRADE" | "ROLLBACK";
export type TaskStatus = "PENDING" | "RUNNING" | "SUCCESS" | "FAILED" | "CANCELED";
export type TaskLogLevel = "INFO" | "WARN" | "ERROR";
export type SecretType = "SSH_PASSWORD" | "SSH_PRIVATE_KEY" | "DOCKER_TLS" | "DOCKER_TOKEN";
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
  secretId?: string;
  status: DockerNodeStatus;
  description?: string;
  lastCheckedAt?: string;
  containerCount: number;
};

export type Registry = {
  id: string;
  name: string;
  url: string;
  authType: RegistryAuthType;
  secretId: string;
  description?: string;
  status: RegistryStatus;
  lastTestAt?: string;
  createdBy?: string;
  updatedBy?: string;
  createdAt: string;
  updatedAt: string;
};

export type RegistryRepositoriesResult = {
  repositories: string[];
};

export type RegistryTagsResult = {
  name: string;
  tags: string[];
};

export type RegistryManifestResult = {
  repository: string;
  reference: string;
  digest: string;
  contentType: string;
  manifest: unknown;
};

export type ServicePort = {
  name: string;
  containerPort: number;
  hostPort?: number;
  protocol?: "TCP" | "UDP";
};

export type ServiceEnvVar = {
  key: string;
  value: string;
};

export type ServiceMount = {
  source: string;
  target: string;
  readOnly?: boolean;
};

export type ServiceResourceLimits = {
  cpu?: string;
  memory?: string;
};

export type ServiceTargetType = "DOCKER_NODE";

export type ServiceDefinition = {
  id: string;
  name: string;
  code: string;
  group: string;
  tags: string[];
  description?: string;
  registryId: string;
  image: string;
  defaultTag: string;
  ports: ServicePort[];
  envs: ServiceEnvVar[];
  mounts: ServiceMount[];
  resourceLimits: ServiceResourceLimits;
  targetType: ServiceTargetType;
  targetId: string;
  status: ServiceStatus;
  currentVersion: string;
  createdBy?: string;
  updatedBy?: string;
  createdAt: string;
  updatedAt: string;
};

export type ServiceVersion = {
  id: string;
  serviceId: string;
  version: string;
  image: string;
  imageTag: string;
  imageDigest: string;
  config?: unknown;
  createdBy?: string;
  createdAt: string;
};

export type ServiceInstance = {
  id: string;
  serviceId: string;
  versionId: string;
  version: string;
  image: string;
  imageTag: string;
  dockerNodeId: string;
  containerId: string;
  name: string;
  status: ServiceInstanceStatus;
  lastError: string;
  startedAt?: string;
  stoppedAt?: string;
  createdAt: string;
  updatedAt: string;
};

export type ServiceReleaseRecord = {
  id: string;
  serviceId: string;
  taskId: string;
  action: ServiceReleaseAction;
  fromVersionId: string;
  fromVersion: string;
  targetVersionId: string;
  targetVersion: string;
  status: TaskStatus;
  message: string;
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
};

export type ServiceReleaseResult = {
  taskId: string;
  releaseId: string;
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
  resourceType?: string;
  resourceId?: string;
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

export type SetupStatus = {
  initialized: boolean;
};

export type AuthSession = {
  token: string;
  user: User;
  permissions: string[];
};

export type CurrentUserPayload = {
  user: User;
  permissions: string[];
};

export type HealthStatus = {
  status: "ok";
  mode: "mock" | "api";
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
  secretId?: string;
  description?: string;
};

export type RegistryInput = {
  id?: string;
  name: string;
  url: string;
  authType: RegistryAuthType;
  secretId?: string;
  description?: string;
};

export type ServiceDefinitionInput = {
  id?: string;
  name: string;
  code: string;
  group: string;
  tags: string[];
  description?: string;
  registryId: string;
  image: string;
  defaultTag: string;
  ports: ServicePort[];
  envs: ServiceEnvVar[];
  mounts: ServiceMount[];
  resourceLimits: ServiceResourceLimits;
  targetType: ServiceTargetType;
  targetId: string;
  status: ServiceStatus;
};

export type ServiceReleaseInput = {
  version: string;
  imageTag: string;
  imageDigest?: string;
  targetId?: string;
};

export type ServiceRollbackInput = {
  versionId: string;
};
