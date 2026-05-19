export type UserStatus = "ACTIVE" | "DISABLED";
export type HostStatus = "HEALTHY" | "UNREACHABLE" | "UNKNOWN" | "TESTING";
export type DockerNodeStatus = "ONLINE" | "OFFLINE" | "UNKNOWN" | "TESTING";
export type RegistryAuthType = "NONE" | "BASIC" | "TOKEN";
export type RegistryStatus = "ONLINE" | "OFFLINE" | "UNKNOWN";
export type NginxNodeStatus = "ONLINE" | "OFFLINE" | "UNKNOWN";
export type NginxConfigStatus = "DRAFT" | "ACTIVE";
export type ServiceStatus = "DRAFT" | "ACTIVE" | "ARCHIVED";
export type ServiceInstanceStatus = "PENDING" | "RUNNING" | "STOPPED" | "FAILED" | "ROLLBACK";
export type ServiceReleaseAction = "RELEASE" | "UPGRADE" | "ROLLBACK";
export type ServiceHealthStatus = "PENDING" | "RUNNING" | "SUCCESS" | "FAILED";
export type NotificationChannelType = "TELEGRAM" | "WECOM" | "EMAIL";
export type NotificationLanguage = "zh-CN" | "en-US";
export type NotificationRecordStatus = "PENDING" | "SUCCESS" | "FAILED";
export type AlertSeverity = "INFO" | "WARN" | "WARNING" | "CRITICAL";
export type AlertEventStatus = "OPEN" | "ACKED" | "RESOLVED";
export type AlertEventType =
  | "service_release_failed"
  | "service_health_check_failed"
  | "nginx_reload_failed"
  | "nginx_publish_failed"
  | "host_offline"
  | "host_recovered";
export type HealthCheckStrategyType = "HTTP" | "TCP" | "COMMAND";
export type TaskStatus = "PENDING" | "RUNNING" | "SUCCESS" | "FAILED" | "CANCELED";
export type TaskLogLevel = "INFO" | "WARN" | "ERROR";
export type SecretType =
  | "SSH_PASSWORD"
  | "SSH_PRIVATE_KEY"
  | "DOCKER_TLS"
  | "DOCKER_TOKEN"
  | "WEBHOOK"
  | "API_TOKEN"
  | "SMTP";
export type SecretStatus = "ACTIVE" | "DISABLED";
export type AuditResult = "SUCCESS" | "FAILED";
export type TerminalSessionStatus = "CONNECTED" | "DISCONNECTED";
export type TaskDispatchSource = "MANUAL" | "SYSTEM" | "SCHEDULED";
export type TaskDispatchStatus =
  | "PENDING"
  | "DISPATCHED"
  | "RUNNING"
  | "SUCCESS"
  | "FAILED"
  | "CANCELED"
  | "TIMEOUT";

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
  purpose?: string;
  status?: SecretStatus;
  valueMasked: string;
  keyVersion?: number;
  lastRotatedAt?: string;
  expiresAt?: string;
  createdBy?: string;
  updatedBy?: string;
  createdAt?: string;
  usedBy: string[];
  updatedAt: string;
};

export type SecretReference = {
  id: string;
  secretId: string;
  resourceType: string;
  resourceId: string;
  fieldName: string;
  createdBy?: string;
  createdAt: string;
};

export type SecretReadAudit = {
  id: string;
  secretId: string;
  resourceType?: string;
  resourceId?: string;
  action: string;
  operatorId?: string;
  taskId?: string;
  result: AuditResult;
  errorMessage?: string;
  createdAt: string;
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
  lastOfflineAt?: string;
  lastRecoveredAt?: string;
  latestAlertStatus?: AlertEventStatus;
  consecutiveFailureCount?: number;
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

export type NginxNode = {
  id: string;
  name: string;
  hostId: string;
  hostName?: string;
  configPath: string;
  testCommand: string;
  reloadCommand: string;
  description?: string;
  status: NginxNodeStatus;
  lastTestAt?: string;
  lastReloadStatus?: TaskStatus;
  lastReloadAt?: string;
  lastFailureReason?: string;
  latestNotificationStatus?: NotificationRecordStatus;
  createdAt: string;
  updatedAt: string;
};

export type NginxConfigVersion = {
  id: string;
  nodeId: string;
  version: string;
  content: string;
  checksum: string;
  status: NginxConfigStatus;
  message?: string;
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
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
  healthCheckStatus?: ServiceHealthStatus;
  notificationStatus?: NotificationRecordStatus;
  rollbackSuggested?: boolean;
  suggestedRollbackVersionId?: string;
  suggestedRollbackVersion?: string;
  failureSummary?: string;
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
};

export type ServiceReleaseResult = {
  taskId: string;
  releaseId: string;
};

export type ServiceHealthCheck = {
  id: string;
  serviceId: string;
  releaseId: string;
  taskId?: string;
  strategyType: HealthCheckStrategyType;
  target: string;
  status: ServiceHealthStatus;
  httpStatus?: number;
  latencyMs?: number;
  output?: string;
  errorMessage?: string;
  startedAt: string;
  finishedAt?: string;
};

export type RollbackSuggestion = {
  serviceId: string;
  suggestedVersionId: string;
  suggestedVersion: string;
  suggestedImageTag?: string;
  reason: string;
  sourceReleaseId?: string;
  available: boolean;
};

export type NotificationChannel = {
  id: string;
  name: string;
  type: NotificationChannelType;
  enabled: boolean;
  language: NotificationLanguage;
  target: string;
  config?: string;
  publicConfig?: string;
  configSecretId?: string;
  lastTestStatus?: NotificationRecordStatus;
  lastTestAt?: string;
  lastFailureReason?: string;
  updatedAt: string;
};

export type NotificationChannelInput = {
  id?: string;
  name: string;
  type: NotificationChannelType;
  enabled: boolean;
  language: NotificationLanguage;
  target: string;
  config?: string;
  publicConfig?: string;
  configSecretId?: string;
};

export type NotificationTestResult = {
  ok: boolean;
  recordId?: string;
};

export type NotificationRecord = {
  id: string;
  eventId?: string;
  channelId: string;
  channelName: string;
  channelType: NotificationChannelType;
  status: NotificationRecordStatus;
  providerMessageId?: string;
  responseExcerpt?: string;
  errorMessage?: string;
  createdAt: string;
  finishedAt?: string;
};

export type AlertRule = {
  id: string;
  name: string;
  eventType: AlertEventType;
  resourceType?: string;
  resourceScope?: string;
  language?: NotificationLanguage;
  channelIds: string[];
  enabled: boolean;
  dedupeWindowSeconds: number;
  requireAck: boolean;
  suppressDuplicates: boolean;
  createdAt: string;
  updatedAt: string;
};

export type AlertRuleInput = {
  id?: string;
  name: string;
  eventType: AlertEventType;
  resourceType?: string;
  resourceScope?: string;
  language?: NotificationLanguage;
  channelIds: string[];
  enabled: boolean;
  dedupeWindowSeconds: number;
  requireAck: boolean;
  suppressDuplicates: boolean;
};

export type AlertEvent = {
  id: string;
  eventType: AlertEventType;
  resourceType: string;
  resourceId?: string;
  resourceName: string;
  taskId?: string;
  releaseId?: string;
  severity: AlertSeverity;
  status: AlertEventStatus;
  summary: string;
  detail?: string;
  dedupeKey?: string;
  firstTriggeredAt: string;
  lastTriggeredAt: string;
  resolvedAt?: string;
  suggestedRollbackVersionId?: string;
  suggestedRollbackVersion?: string;
  notificationStatus?: NotificationRecordStatus;
};

export type HostAvailabilityCheck = {
  id: string;
  hostId: string;
  taskId?: string;
  status: HostStatus;
  failureReason?: string;
  startedAt: string;
  finishedAt?: string;
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
  dispatchSource?: TaskDispatchSource;
  dispatchStatus?: TaskDispatchStatus;
  retryCount?: number;
  maxRetry?: number;
  timeoutSeconds?: number;
  concurrencyKey?: string;
  queuedAt?: string;
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
  resourceId?: string;
  resourceName: string;
  result: AuditResult;
  traceId: string;
  createdAt: string;
  summary: string;
};

export type ResourceSummary = {
  resourceType: string;
  resourceId: string;
  name: string;
  status: string;
  endpoint?: string;
  updatedAt: string;
  resource?: unknown;
};

export type ResourceNavigation = {
  detailPath: string;
  tasksPath: string;
  auditsPath: string;
  alertsPath: string;
};

export type ResourceRisk = {
  level: "critical" | "warning" | "normal" | string;
  summary: string;
  openAlertCount: number;
  failedTaskCount: number;
  highRiskAuditCount: number;
  lastFailureReason?: string;
};

export type ResourceActionHint = {
  key: string;
  label: string;
  kind: "primary" | "secondary" | string;
  permission?: string;
  path?: string;
  reason?: string;
};

export type TaskContext = {
  task?: Task;
  resource?: ResourceSummary;
  navigation: ResourceNavigation;
  risk: ResourceRisk;
  relatedTasks: Task[];
  relatedAudits: AuditLog[];
  relatedAlerts: AlertEvent[];
  notifications: NotificationRecord[];
  failureSummary?: string;
  nextActions: ResourceActionHint[];
};

export type DashboardSummary = {
  userCount: number;
  hostCount: number;
  dockerNodeCount: number;
  containerCount: number;
  unhealthyResourceCount: number;
  openAlertCount?: number;
  recentAlertEvents?: AlertEvent[];
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
  refreshToken?: string;
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
  purpose?: string;
  status?: SecretStatus;
  expiresAt?: string;
  secretValue: string;
};

export type ScheduledJob = {
  id: string;
  name: string;
  type: string;
  enabled: boolean;
  cronExpr: string;
  targetType: string;
  targetId: string;
  payloadJson: string;
  retryPolicyJson: string;
  timeoutSeconds: number;
  concurrencyKey: string;
  lastRunAt?: string;
  nextRunAt?: string;
  createdBy?: string;
  updatedBy?: string;
  createdAt: string;
  updatedAt: string;
};

export type ScheduledJobDispatch = {
  id: string;
  taskId: string;
  jobId: string;
  source?: TaskDispatchSource;
  status?: TaskDispatchStatus;
  retryCount?: number;
  maxRetry?: number;
  timeoutSeconds?: number;
  concurrencyKey?: string;
  queuedAt?: string;
  startedAt?: string;
  finishedAt?: string;
};

export type ScheduledJobInput = {
  id?: string;
  name: string;
  type: string;
  enabled: boolean;
  cronExpr: string;
  targetType?: string;
  targetId?: string;
  payloadJson?: string;
  retryPolicyJson?: string;
  timeoutSeconds: number;
  concurrencyKey?: string;
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

export type NginxNodeInput = {
  id?: string;
  name: string;
  hostId: string;
  configPath?: string;
  testCommand?: string;
  reloadCommand?: string;
  description?: string;
};

export type NginxConfigInput = {
  version: string;
  content: string;
  message?: string;
  activate?: boolean;
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
