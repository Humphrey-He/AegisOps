import { http } from "./http";
import { API_BASE_URL, USE_MOCK } from "./config";
import { mockService } from "../mocks/service";
import { useSessionStore } from "../store/sessionStore";
import type {
  AdminSetupInput,
  AlertEvent,
  AlertRule,
  AlertRuleInput,
  AuthSession,
  AuditLog,
  ContainerItem,
  CurrentUserPayload,
  DashboardSummary,
  DockerNode,
  DockerNodeInput,
  Host,
  HostAvailabilityCheck,
  HostInput,
  LoginInput,
  NotificationChannel,
  NotificationChannelInput,
  NotificationLanguage,
  NotificationRecord,
  NotificationTestResult,
  NginxConfigInput,
  NginxConfigVersion,
  NginxNode,
  NginxNodeInput,
  ResourceActionHint,
  ResourceNavigation,
  ResourceRisk,
  ResourceSummary,
  RollbackSuggestion,
  Role,
  RoleInput,
  Registry,
  RegistryInput,
  RegistryManifestResult,
  RegistryRepositoriesResult,
  RegistryTagsResult,
  ServiceDefinition,
  ServiceDefinitionInput,
  ServiceHealthCheck,
  ServiceInstance,
  ServiceReleaseInput,
  ServiceReleaseRecord,
  ServiceReleaseResult,
  ServiceRollbackInput,
  ServiceVersion,
  Secret,
  SecretReadAudit,
  SecretReference,
  SecretInputPayload,
  ScheduledJob,
  ScheduledJobDispatch,
  ScheduledJobInput,
  SetupStatus,
  Task,
  TaskContext,
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

let permissionCache: BackendPermission[] | null = null;

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

type BackendTaskDispatch = {
  id: string;
  taskId: string;
  source?: Task["dispatchSource"];
  status?: Task["dispatchStatus"];
  retryCount?: number;
  maxRetry?: number;
  timeoutSeconds?: number;
  concurrencyKey?: string;
  queuedAt?: string;
  startedAt?: string;
  finishedAt?: string;
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
  payload?: string;
  createdAt: string;
  updatedAt?: string;
  startedAt?: string;
  finishedAt?: string;
  dispatches?: BackendTaskDispatch[];
  steps?: BackendTaskStep[];
  logs?: BackendTaskLog[];
};

type BackendSecret = {
  id: string;
  name: string;
  type: Secret["type"];
  description?: string;
  purpose?: string;
  status?: Secret["status"];
  maskedValue?: string;
  keyVersion?: number;
  lastRotatedAt?: string;
  expiresAt?: string;
  createdBy?: string;
  updatedBy?: string;
  createdAt?: string;
  updatedAt: string;
};

type BackendSecretReference = {
  id: string;
  secretId: string;
  resourceType: string;
  resourceId: string;
  fieldName: string;
  createdBy?: string;
  createdAt: string;
};

type BackendSecretReadAudit = {
  id: string;
  secretId: string;
  resourceType?: string;
  resourceId?: string;
  action: string;
  operatorId?: string;
  taskId?: string;
  result: SecretReadAudit["result"] | "success" | "failure";
  errorMessage?: string;
  createdAt: string;
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

type SetupStatusResult = {
  initialized: boolean;
};

type BackendRegistry = {
  id: string;
  name: string;
  url: string;
  authType: Registry["authType"];
  secretId: string;
  description?: string;
  status: Registry["status"];
  lastTestAt?: string;
  createdBy?: string;
  updatedBy?: string;
  createdAt: string;
  updatedAt: string;
};

type BackendRegistryRepositoriesResult = {
  repositories?: string[];
};

type BackendRegistryTagsResult = {
  name: string;
  tags?: string[];
};

type BackendRegistryManifestResult = {
  repository: string;
  reference: string;
  digest?: string;
  contentType?: string;
  manifest?: unknown;
};

type BackendNginxNode = {
  id: string;
  name: string;
  hostId: string;
  host?: BackendHost;
  configPath: string;
  testCommand: string;
  reloadCommand: string;
  description?: string;
  status: NginxNode["status"];
  lastTestAt?: string;
  createdAt: string;
  updatedAt: string;
};

type BackendNginxConfigVersion = {
  id: string;
  nodeId: string;
  version: string;
  content: string;
  checksum: string;
  status: NginxConfigVersion["status"];
  message?: string;
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
};

type BackendServiceDefinition = {
  id: string;
  name: string;
  code: string;
  group?: string;
  tags?: string;
  description?: string;
  registryId?: string;
  image: string;
  defaultTag?: string;
  ports?: string;
  envs?: string;
  mounts?: string;
  resourceLimits?: string;
  targetType?: string;
  targetId?: string;
  status: ServiceDefinition["status"];
  currentVersion?: string;
  createdBy?: string;
  updatedBy?: string;
  createdAt: string;
  updatedAt: string;
};

type BackendServiceVersion = {
  id: string;
  serviceId: string;
  version: string;
  image: string;
  imageTag: string;
  imageDigest?: string;
  config?: string;
  createdBy?: string;
  createdAt: string;
};

type BackendServiceInstance = {
  id: string;
  serviceId: string;
  versionId?: string;
  version?: string;
  image: string;
  imageTag: string;
  dockerNodeId?: string;
  containerId?: string;
  name: string;
  status: ServiceInstance["status"];
  lastError?: string;
  startedAt?: string;
  stoppedAt?: string;
  createdAt: string;
  updatedAt: string;
};

type BackendServiceReleaseRecord = {
  id: string;
  serviceId: string;
  taskId: string;
  action: ServiceReleaseRecord["action"];
  fromVersionId?: string;
  fromVersion?: string;
  targetVersionId?: string;
  targetVersion?: string;
  status: ServiceReleaseRecord["status"];
  message?: string;
  healthCheckStatus?: ServiceReleaseRecord["healthCheckStatus"];
  notificationStatus?: ServiceReleaseRecord["notificationStatus"];
  rollbackSuggested?: boolean;
  suggestedRollbackVersionId?: string;
  suggestedRollbackVersion?: string;
  failureSummary?: string;
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
};

type BackendServiceReleaseResult = {
  taskId: string;
  releaseId: string;
};

type BackendDashboardSummary = {
  userCount: number;
  hostCount: number;
  dockerNodeCount: number;
  containerCount: number;
  unhealthyResourceCount: number;
  openAlertCount?: number;
  failedTaskCount?: number;
  highRiskAuditCount?: number;
  openAlerts?: BackendAlertEvent[];
  failedTasks?: BackendTask[];
  highRiskAudits?: BackendAuditLog[];
  recentTasks: BackendTask[];
  recentAudits: BackendAuditLog[];
};

type BackendResourceContext = {
  resourceType: string;
  resourceId: string;
  summary?: unknown;
  recentTasks?: BackendTask[];
  recentAudits?: BackendAuditLog[];
  recentAlerts?: BackendAlertEvent[];
};

type BackendResourceSummary = {
  resourceType: string;
  resourceId: string;
  name: string;
  status: string;
  endpoint?: string;
  updatedAt: string;
  resource?: unknown;
};

type BackendResourceNavigation = {
  detailPath: string;
  tasksPath: string;
  auditsPath: string;
  alertsPath: string;
};

type BackendResourceRisk = {
  level: string;
  summary: string;
  openAlertCount: number;
  failedTaskCount: number;
  highRiskAuditCount: number;
  lastFailureReason?: string;
};

type BackendResourceActionHint = {
  key: string;
  label: string;
  kind: string;
  permission?: string;
  path?: string;
  reason?: string;
};

type BackendTaskContext = {
  task?: BackendTask;
  resource?: BackendResourceSummary;
  navigation?: BackendResourceNavigation;
  risk?: BackendResourceRisk;
  relatedTasks?: BackendTask[];
  relatedAudits?: BackendAuditLog[];
  relatedAlerts?: BackendAlertEvent[];
  notifications?: BackendNotificationRecord[];
  failureSummary?: string;
  nextActions?: BackendResourceActionHint[];
};

type BackendServiceHealthCheck = {
  id: string;
  serviceId: string;
  releaseId: string;
  taskId?: string;
  strategyType: ServiceHealthCheck["strategyType"];
  target: string;
  status: ServiceHealthCheck["status"];
  httpStatus?: number;
  latencyMs?: number;
  output?: string;
  errorMessage?: string;
  startedAt: string;
  finishedAt?: string;
};

type BackendRollbackSuggestion = {
  serviceId: string;
  suggestedVersionId?: string;
  suggestedVersion?: string;
  imageTag?: string;
  reason?: string;
  sourceReleaseId?: string;
  available?: boolean;
};

type BackendNotificationChannel = {
  id: string;
  name: string;
  type: NotificationChannel["type"] | string;
  enabled: boolean;
  language?: NotificationLanguage | string;
  config?: string;
  publicConfig?: string;
  configSecretId?: string;
  defaultTarget?: string;
  lastStatus?: NotificationChannel["lastTestStatus"];
  lastError?: string;
  lastSentAt?: string;
  updatedAt: string;
};

type BackendNotificationRecord = {
  id: string;
  eventId?: string;
  channelId: string;
  channelName?: string;
  channelType?: NotificationRecord["channelType"] | string;
  status: NotificationRecord["status"];
  providerMessageId?: string;
  responseExcerpt?: string;
  errorMessage?: string;
  createdAt: string;
  finishedAt?: string;
};

type BackendAlertRule = {
  id: string;
  name: string;
  eventType: AlertRule["eventType"] | string;
  resourceType?: string;
  resourceScope?: string;
  language?: NotificationLanguage | string;
  channelIds?: string | Array<string | number>;
  enabled: boolean;
  dedupeWindowSeconds?: number;
  requireAck?: boolean;
  suppressDuplicates?: boolean;
  createdAt: string;
  updatedAt: string;
};

type BackendAlertEvent = {
  id: string;
  eventType: AlertEvent["eventType"] | string;
  resourceType?: string;
  resourceId?: string;
  resourceName?: string;
  taskId?: string;
  releaseId?: string;
  severity?: AlertEvent["severity"];
  status?: AlertEvent["status"];
  summary?: string;
  detail?: string;
  dedupeKey?: string;
  suggestion?: string;
  firstTriggeredAt?: string;
  lastTriggeredAt?: string;
  resolvedAt?: string;
  suggestedRollbackVersionId?: string;
  suggestedRollbackVersion?: string;
  notificationStatus?: AlertEvent["notificationStatus"];
};

type BackendHostAvailabilityCheck = {
  id: string;
  hostId: string;
  taskId?: string;
  status: "ONLINE" | "UNREACHABLE" | HostAvailabilityCheck["status"];
  failureReason?: string;
  startedAt: string;
  finishedAt?: string;
};

type BackendScheduledJob = {
  id: string;
  name: string;
  type: string;
  enabled: boolean;
  cronExpr: string;
  targetType?: string;
  targetId?: string;
  payloadJson?: string;
  retryPolicyJson?: string;
  timeoutSeconds?: number;
  concurrencyKey?: string;
  lastRunAt?: string;
  nextRunAt?: string;
  createdBy?: string;
  updatedBy?: string;
  createdAt: string;
  updatedAt: string;
};

type BackendScheduledJobDispatch = {
  id: string;
  taskId: string;
  jobId?: string;
  scheduledJobId?: string;
  source?: ScheduledJobDispatch["source"];
  status?: ScheduledJobDispatch["status"];
  retryCount?: number;
  maxRetry?: number;
  timeoutSeconds?: number;
  concurrencyKey?: string;
  queuedAt?: string;
  startedAt?: string;
  finishedAt?: string;
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

function pageItems<T>(page?: BackendPage<T> | T[] | null): T[] {
  if (!page) {
    return [];
  }
  return Array.isArray(page) ? page : page.items ?? [];
}

function mapLoginResult(result: BackendLoginResult): AuthSession {
  return {
    token: result.tokens.accessToken,
    refreshToken: result.tokens.refreshToken,
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
  const dispatch = task.dispatches?.[0];
  return {
    id: task.id,
    type: task.type,
    status: task.status,
    target: [task.targetType, task.targetId].filter(Boolean).join(":") || task.title,
    resourceType: task.targetType,
    resourceId: task.targetId,
    initiatedBy: task.createdBy || "-",
    dispatchSource: dispatch?.source,
    dispatchStatus: dispatch?.status,
    retryCount: dispatch?.retryCount,
    maxRetry: dispatch?.maxRetry,
    timeoutSeconds: dispatch?.timeoutSeconds,
    concurrencyKey: dispatch?.concurrencyKey,
    queuedAt: dispatch?.queuedAt,
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
    username: undefined,
    description: secret.description,
    purpose: secret.purpose,
    status: secret.status,
    valueMasked: secret.maskedValue || "******",
    keyVersion: secret.keyVersion,
    lastRotatedAt: secret.lastRotatedAt,
    expiresAt: secret.expiresAt,
    createdBy: secret.createdBy,
    updatedBy: secret.updatedBy,
    createdAt: secret.createdAt,
    usedBy: [],
    updatedAt: secret.updatedAt,
  };
}

function mapSecretReference(item: BackendSecretReference): SecretReference {
  return {
    id: item.id,
    secretId: item.secretId,
    resourceType: item.resourceType,
    resourceId: item.resourceId,
    fieldName: item.fieldName,
    createdBy: item.createdBy,
    createdAt: item.createdAt,
  };
}

function mapSecretReadAudit(item: BackendSecretReadAudit): SecretReadAudit {
  return {
    id: item.id,
    secretId: item.secretId,
    resourceType: item.resourceType,
    resourceId: item.resourceId,
    action: item.action,
    operatorId: item.operatorId,
    taskId: item.taskId,
    result: item.result === "success" ? "SUCCESS" : item.result === "failure" ? "FAILED" : item.result,
    errorMessage: item.errorMessage,
    createdAt: item.createdAt,
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
    secretId: node.secretId,
    status: node.status,
    description: node.description,
    lastCheckedAt: node.lastTestAt,
    containerCount: 0,
  };
}

function mapBackendRegistry(registry: BackendRegistry): Registry {
  return {
    id: registry.id,
    name: registry.name,
    url: registry.url,
    authType: registry.authType,
    secretId: registry.secretId,
    description: registry.description,
    status: registry.status,
    lastTestAt: registry.lastTestAt,
    createdBy: registry.createdBy,
    updatedBy: registry.updatedBy,
    createdAt: registry.createdAt,
    updatedAt: registry.updatedAt,
  };
}

function mapBackendNginxNode(node: BackendNginxNode): NginxNode {
  return {
    id: node.id,
    name: node.name,
    hostId: node.hostId,
    hostName: node.host?.name,
    configPath: node.configPath,
    testCommand: node.testCommand,
    reloadCommand: node.reloadCommand,
    description: node.description,
    status: node.status,
    lastTestAt: node.lastTestAt,
    createdAt: node.createdAt,
    updatedAt: node.updatedAt,
  };
}

function mapBackendNginxConfig(config: BackendNginxConfigVersion): NginxConfigVersion {
  return {
    id: config.id,
    nodeId: config.nodeId,
    version: config.version,
    content: config.content,
    checksum: config.checksum,
    status: config.status,
    message: config.message,
    createdBy: config.createdBy,
    createdAt: config.createdAt,
    updatedAt: config.updatedAt,
  };
}

function parseJsonField<T>(value: string | undefined, fallback: T): T {
  if (!value) {
    return fallback;
  }
  try {
    return JSON.parse(value) as T;
  } catch {
    return fallback;
  }
}

function mapBackendServiceDefinition(service: BackendServiceDefinition): ServiceDefinition {
  return {
    id: service.id,
    name: service.name,
    code: service.code,
    group: service.group ?? "",
    tags: parseTags(service.tags),
    description: service.description,
    registryId: service.registryId ?? "",
    image: service.image,
    defaultTag: service.defaultTag ?? "latest",
    ports: parseJsonField(service.ports, []),
    envs: parseJsonField(service.envs, []),
    mounts: parseJsonField(service.mounts, []),
    resourceLimits: parseJsonField(service.resourceLimits, {}),
    targetType: (service.targetType as ServiceDefinition["targetType"]) ?? "DOCKER_NODE",
    targetId: service.targetId ?? "",
    status: service.status,
    currentVersion: service.currentVersion ?? "",
    createdBy: service.createdBy,
    updatedBy: service.updatedBy,
    createdAt: service.createdAt,
    updatedAt: service.updatedAt,
  };
}

function mapBackendServiceVersion(version: BackendServiceVersion): ServiceVersion {
  return {
    id: version.id,
    serviceId: version.serviceId,
    version: version.version,
    image: version.image,
    imageTag: version.imageTag,
    imageDigest: version.imageDigest ?? "",
    config: parseJsonField(version.config, {}),
    createdBy: version.createdBy,
    createdAt: version.createdAt,
  };
}

function mapBackendServiceInstance(instance: BackendServiceInstance): ServiceInstance {
  return {
    id: instance.id,
    serviceId: instance.serviceId,
    versionId: instance.versionId ?? "",
    version: instance.version ?? "",
    image: instance.image,
    imageTag: instance.imageTag,
    dockerNodeId: instance.dockerNodeId ?? "",
    containerId: instance.containerId ?? "",
    name: instance.name,
    status: instance.status,
    lastError: instance.lastError ?? "",
    startedAt: instance.startedAt,
    stoppedAt: instance.stoppedAt,
    createdAt: instance.createdAt,
    updatedAt: instance.updatedAt,
  };
}

function mapBackendServiceReleaseRecord(record: BackendServiceReleaseRecord): ServiceReleaseRecord {
  return {
    id: record.id,
    serviceId: record.serviceId,
    taskId: record.taskId,
    action: record.action,
    fromVersionId: record.fromVersionId ?? "",
    fromVersion: record.fromVersion ?? "",
    targetVersionId: record.targetVersionId ?? "",
    targetVersion: record.targetVersion ?? "",
    status: record.status,
    message: record.message ?? "",
    healthCheckStatus: record.healthCheckStatus,
    notificationStatus: record.notificationStatus,
    rollbackSuggested: record.rollbackSuggested,
    suggestedRollbackVersionId: record.suggestedRollbackVersionId,
    suggestedRollbackVersion: record.suggestedRollbackVersion,
    failureSummary: record.failureSummary,
    createdBy: record.createdBy,
    createdAt: record.createdAt,
    updatedAt: record.updatedAt,
  };
}

function mapBackendServiceHealthCheck(check: BackendServiceHealthCheck): ServiceHealthCheck {
  return {
    id: check.id,
    serviceId: check.serviceId,
    releaseId: check.releaseId,
    taskId: check.taskId,
    strategyType: check.strategyType,
    target: check.target,
    status: check.status,
    httpStatus: check.httpStatus,
    latencyMs: check.latencyMs,
    output: check.output,
    errorMessage: check.errorMessage,
    startedAt: check.startedAt,
    finishedAt: check.finishedAt,
  };
}

function mapRollbackSuggestion(result: BackendRollbackSuggestion): RollbackSuggestion {
  return {
    serviceId: result.serviceId,
    suggestedVersionId: result.suggestedVersionId ?? "",
    suggestedVersion: result.suggestedVersion ?? "",
    suggestedImageTag: result.imageTag ?? "",
    reason: result.reason ?? "",
    sourceReleaseId: result.sourceReleaseId,
    available: Boolean(result.available),
  };
}

function normalizeNotificationLanguage(value?: string): NotificationLanguage {
  return value === "en-US" ? "en-US" : "zh-CN";
}

function normalizeNotificationChannelType(value?: string): NotificationChannel["type"] {
  const normalized = (value ?? "").trim().toUpperCase();
  switch (normalized) {
    case "TELEGRAM":
      return "TELEGRAM";
    case "WECOM":
      return "WECOM";
    case "EMAIL":
      return "EMAIL";
    default:
      return "TELEGRAM";
  }
}

function normalizeAlertEventType(value?: string): AlertEvent["eventType"] {
  const normalized = (value ?? "").trim();
  switch (normalized) {
    case "service_release_failed":
    case "service_health_check_failed":
    case "nginx_reload_failed":
    case "nginx_publish_failed":
    case "host_offline":
    case "host_recovered":
      return normalized;
    default:
      return "service_release_failed";
  }
}

function normalizeAlertSeverity(value?: string): AlertEvent["severity"] {
  const normalized = (value ?? "").trim().toUpperCase();
  switch (normalized) {
    case "INFO":
      return "INFO";
    case "WARN":
      return "WARN";
    case "WARNING":
      return "WARNING";
    case "CRITICAL":
      return "CRITICAL";
    default:
      return "WARNING";
  }
}

function parseAlertRuleChannelIds(value?: string | Array<string | number>): string[] {
  if (!value) {
    return [];
  }
  if (Array.isArray(value)) {
    return value.map(String).filter(Boolean);
  }
  const trimmed = value.trim();
  if (!trimmed) {
    return [];
  }
  if (trimmed.startsWith("[")) {
    try {
      const parsed = JSON.parse(trimmed) as Array<string | number>;
      return Array.isArray(parsed) ? parsed.map(String).filter(Boolean) : [];
    } catch {
      return [];
    }
  }
  return trimmed
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
}

function mapNotificationChannel(channel: BackendNotificationChannel): NotificationChannel {
  return {
    id: channel.id,
    name: channel.name,
    type: normalizeNotificationChannelType(channel.type),
    enabled: channel.enabled,
    language: normalizeNotificationLanguage(channel.language),
    target: channel.defaultTarget ?? "",
    config: channel.config ?? "",
    publicConfig: channel.publicConfig ?? "",
    configSecretId: channel.configSecretId ?? "",
    lastTestStatus: channel.lastStatus,
    lastTestAt: channel.lastSentAt,
    lastFailureReason: channel.lastError,
    updatedAt: channel.updatedAt,
  };
}

function mapNotificationRecord(record: BackendNotificationRecord): NotificationRecord {
  return {
    id: record.id,
    eventId: record.eventId,
    channelId: record.channelId,
    channelName: record.channelName ?? "",
    channelType: normalizeNotificationChannelType(record.channelType),
    status: record.status,
    providerMessageId: record.providerMessageId,
    responseExcerpt: record.responseExcerpt,
    errorMessage: record.errorMessage,
    createdAt: record.createdAt,
    finishedAt: record.finishedAt,
  };
}

function mapAlertRule(rule: BackendAlertRule): AlertRule {
  return {
    id: rule.id,
    name: rule.name,
    eventType: normalizeAlertEventType(rule.eventType),
    resourceType: rule.resourceType,
    resourceScope: rule.resourceScope,
    language: rule.language ? normalizeNotificationLanguage(rule.language) : undefined,
    channelIds: parseAlertRuleChannelIds(rule.channelIds),
    enabled: rule.enabled,
    dedupeWindowSeconds: rule.dedupeWindowSeconds ?? 300,
    requireAck: Boolean(rule.requireAck),
    suppressDuplicates: Boolean(rule.suppressDuplicates),
    createdAt: rule.createdAt,
    updatedAt: rule.updatedAt,
  };
}

function parseRollbackSuggestion(detail?: string) {
  if (!detail) {
    return { suggestedRollbackVersionId: undefined, suggestedRollbackVersion: undefined };
  }
  try {
    const parsed = JSON.parse(detail) as { versionId?: string; version?: string };
    return {
      suggestedRollbackVersionId: parsed.versionId,
      suggestedRollbackVersion: parsed.version,
    };
  } catch {
    return { suggestedRollbackVersionId: undefined, suggestedRollbackVersion: undefined };
  }
}

function mapAlertEvent(event: BackendAlertEvent): AlertEvent {
  const suggestion = parseRollbackSuggestion(event.suggestion);
  return {
    id: event.id,
    eventType: normalizeAlertEventType(event.eventType),
    resourceType: event.resourceType ?? "",
    resourceId: event.resourceId,
    resourceName: event.resourceName ?? event.resourceId ?? "",
    taskId: event.taskId,
    releaseId: event.releaseId,
    severity: normalizeAlertSeverity(event.severity),
    status: event.status ?? "OPEN",
    summary: event.summary ?? "",
    detail: event.detail,
    dedupeKey: event.dedupeKey,
    firstTriggeredAt: event.firstTriggeredAt ?? "",
    lastTriggeredAt: event.lastTriggeredAt ?? "",
    resolvedAt: event.resolvedAt,
    suggestedRollbackVersionId: event.suggestedRollbackVersionId ?? suggestion.suggestedRollbackVersionId,
    suggestedRollbackVersion: event.suggestedRollbackVersion ?? suggestion.suggestedRollbackVersion,
    notificationStatus: event.notificationStatus,
  };
}

function mapHostAvailabilityCheck(check: BackendHostAvailabilityCheck): HostAvailabilityCheck {
  const status = check.status === "ONLINE" ? "HEALTHY" : check.status === "UNREACHABLE" ? "UNREACHABLE" : check.status;
  return {
    id: check.id,
    hostId: check.hostId,
    taskId: check.taskId,
    status: status as HostAvailabilityCheck["status"],
    failureReason: check.failureReason,
    startedAt: check.startedAt,
    finishedAt: check.finishedAt,
  };
}

function mapScheduledJob(item: BackendScheduledJob): ScheduledJob {
  return {
    id: item.id,
    name: item.name,
    type: item.type,
    enabled: item.enabled,
    cronExpr: item.cronExpr,
    targetType: item.targetType ?? "",
    targetId: item.targetId ?? "",
    payloadJson: item.payloadJson ?? "",
    retryPolicyJson: item.retryPolicyJson ?? "",
    timeoutSeconds: item.timeoutSeconds ?? 300,
    concurrencyKey: item.concurrencyKey ?? "",
    lastRunAt: item.lastRunAt,
    nextRunAt: item.nextRunAt,
    createdBy: item.createdBy,
    updatedBy: item.updatedBy,
    createdAt: item.createdAt,
    updatedAt: item.updatedAt,
  };
}

function mapScheduledJobDispatch(item: BackendScheduledJobDispatch, jobId: string): ScheduledJobDispatch {
  return {
    id: item.id,
    taskId: item.taskId,
    jobId: item.jobId ?? item.scheduledJobId ?? jobId,
    source: item.source,
    status: item.status,
    retryCount: item.retryCount,
    maxRetry: item.maxRetry,
    timeoutSeconds: item.timeoutSeconds,
    concurrencyKey: item.concurrencyKey,
    queuedAt: item.queuedAt,
    startedAt: item.startedAt,
    finishedAt: item.finishedAt,
  };
}

function mapBackendAudit(audit: BackendAuditLog): AuditLog {
  return {
    id: String(audit.id),
    actor: audit.username || "-",
    action: audit.action,
    resourceType: audit.resourceType || "-",
    resourceId: audit.resourceId || "",
    resourceName: audit.resourceId || "",
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
    openAlertCount: summary.openAlertCount ?? 0,
    recentAlertEvents: (summary.openAlerts ?? []).map(mapAlertEvent),
    recentTasks: (summary.recentTasks ?? []).map(mapBackendTask),
    recentAudits: (summary.recentAudits ?? []).map(mapBackendAudit),
  };
}

function mapResourceSummary(summary: BackendResourceSummary): ResourceSummary {
  return {
    resourceType: summary.resourceType,
    resourceId: summary.resourceId,
    name: summary.name,
    status: summary.status,
    endpoint: summary.endpoint,
    updatedAt: summary.updatedAt,
    resource: summary.resource,
  };
}

function mapResourceNavigation(navigation?: BackendResourceNavigation): ResourceNavigation {
  return {
    detailPath: navigation?.detailPath ?? "/dashboard",
    tasksPath: navigation?.tasksPath ?? "/tasks",
    auditsPath: navigation?.auditsPath ?? "/audits",
    alertsPath: navigation?.alertsPath ?? "/alerts/events",
  };
}

function mapResourceRisk(risk?: BackendResourceRisk): ResourceRisk {
  return {
    level: risk?.level ?? "normal",
    summary: risk?.summary ?? "当前资源暂无待处理风险。",
    openAlertCount: risk?.openAlertCount ?? 0,
    failedTaskCount: risk?.failedTaskCount ?? 0,
    highRiskAuditCount: risk?.highRiskAuditCount ?? 0,
    lastFailureReason: risk?.lastFailureReason,
  };
}

function mapResourceActionHint(hint: BackendResourceActionHint): ResourceActionHint {
  return {
    key: hint.key,
    label: hint.label,
    kind: hint.kind,
    permission: hint.permission,
    path: hint.path,
    reason: hint.reason,
  };
}

function mapTaskContext(context: BackendTaskContext): TaskContext {
  return {
    task: context.task ? mapBackendTask(context.task) : undefined,
    resource: context.resource ? mapResourceSummary(context.resource) : undefined,
    navigation: mapResourceNavigation(context.navigation),
    risk: mapResourceRisk(context.risk),
    relatedTasks: (context.relatedTasks ?? []).map(mapBackendTask),
    relatedAudits: (context.relatedAudits ?? []).map(mapBackendAudit),
    relatedAlerts: (context.relatedAlerts ?? []).map(mapAlertEvent),
    notifications: (context.notifications ?? []).map(mapNotificationRecord),
    failureSummary: context.failureSummary,
    nextActions: (context.nextActions ?? []).map(mapResourceActionHint),
  };
}

export const authApi = {
  getSetupStatus: async (): Promise<SetupStatus> => {
    if (USE_MOCK) {
      return mockService.getSetupStatus();
    }
    try {
      const result = await http.get<SetupStatusResult>("/auth/setup-status");
      return { initialized: result.initialized };
    } catch {
      return { initialized: true };
    }
  },
  initAdmin: async (payload: AdminSetupInput): Promise<{ created: boolean }> => {
    return USE_MOCK ? mockService.initAdmin(payload) : Promise.resolve({ created: true });
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

export const resourcesApi = {
  context: async (
    resourceType: string,
    resourceId: string,
  ): Promise<{ recentTasks: Task[]; recentAudits: AuditLog[]; recentAlerts: AlertEvent[] }> => {
    if (USE_MOCK) {
      const [recentTasks, recentAudits, recentAlerts] = await Promise.all([
        mockService.listTasks(token(), { resourceType, resourceId }),
        mockService.listAudits(token(), { resourceType, resourceId }),
        mockService.listAlertEvents(token()),
      ]);
      return {
        recentTasks,
        recentAudits,
        recentAlerts: recentAlerts.filter((item) => item.resourceType === resourceType && item.resourceId === resourceId),
      };
    }
    const params = new URLSearchParams({ resourceType, resourceId });
    const context = await http.get<BackendResourceContext>(`/resources/context?${params.toString()}`);
    return {
      recentTasks: (context.recentTasks ?? []).map(mapBackendTask),
      recentAudits: (context.recentAudits ?? []).map(mapBackendAudit),
      recentAlerts: (context.recentAlerts ?? []).map(mapAlertEvent),
    };
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
    let permissions = await loadBackendPermissions();
    let permissionIds = mapPermissionIds(payload.permissions, permissions);
    if (payload.permissions.length > 0 && permissionIds.length !== payload.permissions.length) {
      permissionCache = null;
      permissions = await loadBackendPermissions();
      permissionIds = mapPermissionIds(payload.permissions, permissions);
    }
    if (payload.permissions.length > 0 && permissionIds.length !== payload.permissions.length) {
      const missing = payload.permissions.filter((code) => !permissions.some((permission) => permission.code === code));
      throw new Error(`后端未返回完整的权限字典，缺失：${missing.join(", ")}`);
    }
    const backendPayload = {
      name: payload.name,
      code: payload.name.toLowerCase().replace(/\s+/g, "_"),
      description: payload.description,
      permissionIds,
    };
    const role = payload.id
      ? await http.patch<BackendRole>(`/roles/${payload.id}`, backendPayload)
      : await http.post<BackendRole>("/roles", backendPayload);
    return mapBackendRole(role);
  },
};

function mapPermissionIds(codes: string[], permissions: BackendPermission[]): number[] {
  return codes
    .map((code) => permissions.find((permission) => permission.code === code)?.id)
    .filter((id): id is number => typeof id === "number");
}

async function loadBackendPermissions(): Promise<BackendPermission[]> {
  if (permissionCache) {
    return permissionCache;
  }
  const page = await http.get<BackendPage<BackendPermission> | BackendPermission[]>("/permissions");
  permissionCache = pageItems(page);
  return permissionCache;
}

export const secretsApi = {
  list: async (keyword = ""): Promise<Secret[]> => {
    if (USE_MOCK) {
      return mockService.listSecrets(token(), keyword);
    }
    const page = await http.get<BackendPage<BackendSecret>>(`/secrets?keyword=${encodeURIComponent(keyword)}`);
    return pageItems(page).map(mapBackendSecret);
  },
  detail: async (secretId: string): Promise<Secret> => {
    if (USE_MOCK) {
      const items = await mockService.listSecrets(token(), "");
      const item = items.find((secret) => secret.id === secretId);
      if (!item) {
        throw new Error("凭证不存在");
      }
      return item;
    }
    const item = await http.get<BackendSecret>(`/secrets/${secretId}`);
    return mapBackendSecret(item);
  },
  references: async (secretId: string): Promise<SecretReference[]> => {
    if (USE_MOCK) {
      return [];
    }
    const result = await http.get<{ items?: BackendSecretReference[] }>(`/secrets/${secretId}/references`);
    return pageItems(result.items ?? []).map(mapSecretReference);
  },
  readAudits: async (secretId: string): Promise<SecretReadAudit[]> => {
    if (USE_MOCK) {
      return [];
    }
    const result = await http.get<BackendPage<BackendSecretReadAudit>>(`/secrets/${secretId}/read-audits`);
    return pageItems(result).map(mapSecretReadAudit);
  },
  save: async (payload: SecretInputPayload): Promise<Secret> => {
    if (USE_MOCK) {
      return mockService.saveSecret(token(), payload);
    }
    const backendPayload = {
      name: payload.name,
      type: payload.type,
      description: payload.description,
      purpose: payload.purpose,
      status: payload.status,
      expiresAt: payload.expiresAt,
      value: payload.secretValue,
    };
    const secret = payload.id
      ? await http.patch<BackendSecret>(`/secrets/${payload.id}`, backendPayload)
      : await http.post<BackendSecret>("/secrets", backendPayload);
    return mapBackendSecret(secret);
  },
  remove: async (secretId: string): Promise<{ deleted: boolean }> => {
    if (USE_MOCK) {
      throw new Error("Mock 模式暂不支持删除凭证");
    }
    return http.delete<{ deleted: boolean }>(`/secrets/${secretId}`);
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
        throw new Error("主机不存在");
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
  testSsh: async (hostId: string): Promise<{ connected: boolean; taskId?: string }> => {
    if (USE_MOCK) {
      const task = await mockService.testHostSsh(token(), hostId);
      return { connected: true, taskId: task.id };
    }
    return http.post<{ connected: boolean; taskId?: string }>(`/hosts/${hostId}/test-ssh`);
  },
};

export const registriesApi = {
  list: async (keyword = ""): Promise<Registry[]> => {
    if (USE_MOCK) {
      return mockService.listRegistries(token(), keyword);
    }
    const page = await http.get<BackendPage<BackendRegistry>>(`/registries?keyword=${encodeURIComponent(keyword)}`);
    return pageItems(page).map(mapBackendRegistry);
  },
  detail: async (registryId: string): Promise<Registry> => {
    if (USE_MOCK) {
      return mockService.getRegistry(token(), registryId);
    }
    const registry = await http.get<BackendRegistry>(`/registries/${registryId}`);
    return mapBackendRegistry(registry);
  },
  save: async (payload: RegistryInput): Promise<Registry> => {
    if (USE_MOCK) {
      return mockService.saveRegistry(token(), payload);
    }
    const backendPayload = {
      name: payload.name,
      url: payload.url,
      authType: payload.authType,
      secretId: payload.secretId ?? "",
      description: payload.description ?? "",
    };
    const registry = payload.id
      ? await http.patch<BackendRegistry>(`/registries/${payload.id}`, backendPayload)
      : await http.post<BackendRegistry>("/registries", backendPayload);
    return mapBackendRegistry(registry);
  },
  remove: async (registryId: string): Promise<{ deleted: boolean }> => {
    if (USE_MOCK) {
      return mockService.deleteRegistry(token(), registryId);
    }
    return http.delete<{ deleted: boolean }>(`/registries/${registryId}`);
  },
  test: async (registryId: string): Promise<{ connected: boolean; taskId?: string }> => {
    if (USE_MOCK) {
      return mockService.testRegistry(token(), registryId);
    }
    return http.post<{ connected: boolean; taskId?: string }>(`/registries/${registryId}/test`);
  },
  repositories: async (registryId: string): Promise<RegistryRepositoriesResult> => {
    if (USE_MOCK) {
      return mockService.listRegistryRepositories(token(), registryId);
    }
    const result = await http.get<BackendRegistryRepositoriesResult>(`/registries/${registryId}/repositories`);
    return { repositories: result.repositories ?? [] };
  },
  tags: async (registryId: string, repository: string): Promise<RegistryTagsResult> => {
    if (USE_MOCK) {
      return mockService.listRegistryTags(token(), registryId, repository);
    }
    const result = await http.get<BackendRegistryTagsResult>(
      `/registries/${registryId}/repositories/${encodeURIComponent(repository)}/tags`,
    );
    return {
      name: result.name,
      tags: result.tags ?? [],
    };
  },
  manifest: async (registryId: string, repository: string, reference: string): Promise<RegistryManifestResult> => {
    if (USE_MOCK) {
      return mockService.getRegistryManifest(token(), registryId, repository, reference);
    }
    const result = await http.get<BackendRegistryManifestResult>(
      `/registries/${registryId}/repositories/${encodeURIComponent(repository)}/manifests/${encodeURIComponent(reference)}`,
    );
    return {
      repository: result.repository,
      reference: result.reference,
      digest: result.digest ?? "",
      contentType: result.contentType ?? "",
      manifest: result.manifest ?? {},
    };
  },
};

export const nginxApi = {
  listNodes: async (keyword = ""): Promise<NginxNode[]> => {
    if (USE_MOCK) {
      return mockService.listNginxNodes(token(), keyword);
    }
    const page = await http.get<BackendPage<BackendNginxNode>>(`/nginx/nodes?keyword=${encodeURIComponent(keyword)}`);
    return pageItems(page).map(mapBackendNginxNode);
  },
  detailNode: async (nodeId: string): Promise<NginxNode> => {
    if (USE_MOCK) {
      return mockService.getNginxNode(token(), nodeId);
    }
    const node = await http.get<BackendNginxNode>(`/nginx/nodes/${nodeId}`);
    return mapBackendNginxNode(node);
  },
  saveNode: async (payload: NginxNodeInput): Promise<NginxNode> => {
    if (USE_MOCK) {
      return mockService.saveNginxNode(token(), payload);
    }
    const backendPayload = {
      name: payload.name,
      hostId: payload.hostId,
      configPath: payload.configPath,
      testCommand: payload.testCommand,
      reloadCommand: payload.reloadCommand,
      description: payload.description,
    };
    const node = payload.id
      ? await http.patch<BackendNginxNode>(`/nginx/nodes/${payload.id}`, backendPayload)
      : await http.post<BackendNginxNode>("/nginx/nodes", backendPayload);
    return mapBackendNginxNode(node);
  },
  removeNode: async (nodeId: string): Promise<{ deleted: boolean }> => {
    if (USE_MOCK) {
      return mockService.deleteNginxNode(token(), nodeId);
    }
    return http.delete<{ deleted: boolean }>(`/nginx/nodes/${nodeId}`);
  },
  testNode: async (nodeId: string): Promise<{ ok: boolean; taskId?: string }> => {
    if (USE_MOCK) {
      return mockService.testNginxNode(token(), nodeId);
    }
    return http.post<{ ok: boolean; taskId?: string }>(`/nginx/nodes/${nodeId}/test`);
  },
  reloadNode: async (nodeId: string): Promise<{ ok: boolean; taskId?: string }> => {
    if (USE_MOCK) {
      return mockService.reloadNginxNode(token(), nodeId);
    }
    return http.post<{ ok: boolean; taskId?: string }>(`/nginx/nodes/${nodeId}/reload`);
  },
  publishConfig: async (nodeId: string, configId: string): Promise<{ ok: boolean; taskId?: string }> => {
    if (USE_MOCK) {
      return mockService.publishNginxConfig(token(), nodeId, configId);
    }
    return http.post<{ ok: boolean; taskId?: string }>(`/nginx/nodes/${nodeId}/publish`, { configId });
  },
  listConfigs: async (nodeId: string): Promise<NginxConfigVersion[]> => {
    if (USE_MOCK) {
      return mockService.listNginxConfigs(token(), nodeId);
    }
    const page = await http.get<BackendPage<BackendNginxConfigVersion>>(`/nginx/nodes/${nodeId}/configs`);
    return pageItems(page).map(mapBackendNginxConfig);
  },
  saveConfig: async (nodeId: string, payload: NginxConfigInput): Promise<NginxConfigVersion> => {
    if (USE_MOCK) {
      return mockService.saveNginxConfig(token(), nodeId, payload);
    }
    const config = await http.post<BackendNginxConfigVersion>(`/nginx/nodes/${nodeId}/configs`, payload);
    return mapBackendNginxConfig(config);
  },
  rollback: async (nodeId: string, configId: string): Promise<{ ok: boolean; taskId?: string }> => {
    if (USE_MOCK) {
      return mockService.rollbackNginxConfig(token(), nodeId, configId);
    }
    return http.post<{ ok: boolean; taskId?: string }>(`/nginx/nodes/${nodeId}/rollback`, { configId });
  },
};

function serializeJsonField(value: unknown): string {
  return JSON.stringify(value ?? []);
}

export const servicesApi = {
  list: async (keyword = "", status = ""): Promise<ServiceDefinition[]> => {
    if (USE_MOCK) {
      return mockService.listServices(token(), keyword, status);
    }
    const page = await http.get<BackendPage<BackendServiceDefinition>>(
      `/services?keyword=${encodeURIComponent(keyword)}&status=${encodeURIComponent(status)}`,
    );
    return pageItems(page).map(mapBackendServiceDefinition);
  },
  detail: async (serviceId: string): Promise<ServiceDefinition> => {
    if (USE_MOCK) {
      return mockService.getService(token(), serviceId);
    }
    const service = await http.get<BackendServiceDefinition>(`/services/${serviceId}`);
    return mapBackendServiceDefinition(service);
  },
  save: async (payload: ServiceDefinitionInput): Promise<ServiceDefinition> => {
    if (USE_MOCK) {
      return mockService.saveService(token(), payload);
    }
    const backendPayload = {
      name: payload.name,
      code: payload.code,
      group: payload.group,
      tags: serializeTags(payload.tags),
      description: payload.description ?? "",
      registryId: payload.registryId,
      image: payload.image,
      defaultTag: payload.defaultTag,
      ports: payload.ports,
      envs: payload.envs,
      mounts: payload.mounts,
      resourceLimits: payload.resourceLimits,
      targetType: payload.targetType,
      targetId: payload.targetId,
      status: payload.status,
    };
    const service = payload.id
      ? await http.patch<BackendServiceDefinition>(`/services/${payload.id}`, backendPayload)
      : await http.post<BackendServiceDefinition>("/services", backendPayload);
    return mapBackendServiceDefinition(service);
  },
  remove: async (serviceId: string): Promise<{ deleted: boolean }> => {
    if (USE_MOCK) {
      return mockService.deleteService(token(), serviceId);
    }
    return http.delete<{ deleted: boolean }>(`/services/${serviceId}`);
  },
  instances: async (serviceId: string): Promise<ServiceInstance[]> => {
    if (USE_MOCK) {
      return mockService.listServiceInstances(token(), serviceId);
    }
    const page = await http.get<BackendPage<BackendServiceInstance>>(`/services/${serviceId}/instances`);
    return pageItems(page).map(mapBackendServiceInstance);
  },
  releases: async (serviceId: string): Promise<ServiceReleaseRecord[]> => {
    if (USE_MOCK) {
      return mockService.listServiceReleases(token(), serviceId);
    }
    const page = await http.get<BackendPage<BackendServiceReleaseRecord>>(`/services/${serviceId}/releases`);
    return pageItems(page).map(mapBackendServiceReleaseRecord);
  },
  history: async (serviceId: string): Promise<ServiceReleaseRecord[]> => {
    if (USE_MOCK) {
      return mockService.listServiceHistory(token(), serviceId);
    }
    const page = await http.get<BackendPage<BackendServiceReleaseRecord>>(`/services/${serviceId}/history`);
    return pageItems(page).map(mapBackendServiceReleaseRecord);
  },
  versions: async (serviceId: string): Promise<ServiceVersion[]> => {
    if (USE_MOCK) {
      return mockService.listServiceVersions(token(), serviceId);
    }
    const page = await http.get<BackendPage<BackendServiceVersion>>(`/services/${serviceId}/versions`);
    return pageItems(page).map(mapBackendServiceVersion);
  },
  healthChecks: async (serviceId: string): Promise<ServiceHealthCheck[]> => {
    if (USE_MOCK) {
      return mockService.listServiceHealthChecks(token(), serviceId);
    }
    const page = await http.get<BackendPage<BackendServiceHealthCheck>>(`/services/${serviceId}/health-checks`);
    return pageItems(page).map(mapBackendServiceHealthCheck);
  },
  rollbackSuggestion: async (serviceId: string): Promise<RollbackSuggestion> => {
    if (USE_MOCK) {
      return mockService.getServiceRollbackSuggestion(token(), serviceId);
    }
    const result = await http.get<BackendRollbackSuggestion & { imageTag?: string }>(
      `/services/${serviceId}/rollback-suggestion`,
    );
    return mapRollbackSuggestion(result);
  },
  release: async (serviceId: string, payload: ServiceReleaseInput): Promise<ServiceReleaseResult> => {
    if (USE_MOCK) {
      return mockService.releaseService(token(), serviceId, payload);
    }
    const result = await http.post<BackendServiceReleaseResult>(`/services/${serviceId}/releases`, payload);
    return { taskId: result.taskId, releaseId: result.releaseId };
  },
  upgrade: async (serviceId: string, payload: ServiceReleaseInput): Promise<ServiceReleaseResult> => {
    if (USE_MOCK) {
      return mockService.upgradeService(token(), serviceId, payload);
    }
    const result = await http.post<BackendServiceReleaseResult>(`/services/${serviceId}/upgrades`, payload);
    return { taskId: result.taskId, releaseId: result.releaseId };
  },
  rollback: async (serviceId: string, payload: ServiceRollbackInput): Promise<ServiceReleaseResult> => {
    if (USE_MOCK) {
      return mockService.rollbackService(token(), serviceId, payload);
    }
    const result = await http.post<BackendServiceReleaseResult>(`/services/${serviceId}/rollbacks`, payload);
    return { taskId: result.taskId, releaseId: result.releaseId };
  },
};

export const notificationsApi = {
  listChannels: async (): Promise<NotificationChannel[]> => {
    if (USE_MOCK) {
      return mockService.listNotificationChannels(token());
    }
    const page = await http.get<BackendPage<BackendNotificationChannel>>("/notifications/channels");
    return pageItems(page).map(mapNotificationChannel);
  },
  saveChannel: async (payload: NotificationChannelInput): Promise<NotificationChannel> => {
    if (USE_MOCK) {
      return mockService.saveNotificationChannel(token(), payload);
    }
    const backendPayload = {
      name: payload.name,
      type: payload.type.toLowerCase(),
      enabled: payload.enabled,
      language: payload.language,
      config: payload.config ?? "",
      publicConfig: payload.publicConfig ?? "",
      configSecretId: payload.configSecretId ?? "",
      defaultTarget: payload.target,
    };
    const result = payload.id
      ? await http.patch<BackendNotificationChannel>(`/notifications/channels/${payload.id}`, backendPayload)
      : await http.post<BackendNotificationChannel>("/notifications/channels", backendPayload);
    return mapNotificationChannel(result);
  },
  removeChannel: async (channelId: string): Promise<{ deleted: boolean }> => {
    if (USE_MOCK) {
      return mockService.deleteNotificationChannel(token(), channelId);
    }
    return http.delete<{ deleted: boolean }>(`/notifications/channels/${channelId}`);
  },
  testChannel: async (channelId: string): Promise<NotificationTestResult> => {
    if (USE_MOCK) {
      return mockService.testNotificationChannel(token(), channelId);
    }
    const result = await http.post<BackendNotificationRecord>(`/notifications/channels/${channelId}/test`);
    return { ok: result.status === "SUCCESS", recordId: result.id };
  },
  listRecords: async (): Promise<NotificationRecord[]> => {
    if (USE_MOCK) {
      return mockService.listNotificationRecords(token());
    }
    const page = await http.get<BackendPage<BackendNotificationRecord>>("/notifications/records");
    return pageItems(page).map(mapNotificationRecord);
  },
};

export const alertRulesApi = {
  list: async (): Promise<AlertRule[]> => {
    if (USE_MOCK) {
      return mockService.listAlertRules(token());
    }
    const page = await http.get<BackendPage<BackendAlertRule>>("/alert-rules");
    return pageItems(page).map(mapAlertRule);
  },
  save: async (payload: AlertRuleInput): Promise<AlertRule> => {
    if (USE_MOCK) {
      return mockService.saveAlertRule(token(), payload);
    }
    const backendPayload = {
      name: payload.name,
      eventType: payload.eventType,
      resourceType: payload.resourceType ?? "",
      resourceScope: payload.resourceScope ?? "",
      language: payload.language ?? "",
      channelIds: JSON.stringify(payload.channelIds),
      enabled: payload.enabled,
      dedupeWindowSeconds: payload.dedupeWindowSeconds,
      requireAck: payload.requireAck,
    };
    const result = payload.id
      ? await http.patch<BackendAlertRule>(`/alert-rules/${payload.id}`, backendPayload)
      : await http.post<BackendAlertRule>("/alert-rules", backendPayload);
    return mapAlertRule(result);
  },
  remove: async (ruleId: string): Promise<{ deleted: boolean }> => {
    if (USE_MOCK) {
      return mockService.deleteAlertRule(token(), ruleId);
    }
    return http.delete<{ deleted: boolean }>(`/alert-rules/${ruleId}`);
  },
};

export const alertsApi = {
  listEvents: async (filters?: {
    status?: string;
    eventType?: string;
    resourceType?: string;
    resourceId?: string;
  }): Promise<AlertEvent[]> => {
    if (USE_MOCK) {
      return mockService.listAlertEvents(token(), filters);
    }
    const params = new URLSearchParams();
    if (filters?.status) {
      params.set("status", filters.status);
    }
    if (filters?.eventType) {
      params.set("eventType", filters.eventType);
    }
    if (filters?.resourceType) {
      params.set("resourceType", filters.resourceType);
    }
    if (filters?.resourceId) {
      params.set("resourceId", filters.resourceId);
    }
    const query = params.toString();
    const page = await http.get<BackendPage<BackendAlertEvent>>(query ? `/alerts/events?${query}` : "/alerts/events");
    return pageItems(page).map(mapAlertEvent);
  },
  ackEvent: async (eventId: string): Promise<AlertEvent> => {
    if (USE_MOCK) {
      return mockService.ackAlertEvent(token(), eventId);
    }
    const result = await http.post<BackendAlertEvent>(`/alerts/events/${eventId}/ack`);
    return mapAlertEvent(result);
  },
  resolveEvent: async (eventId: string): Promise<AlertEvent> => {
    if (USE_MOCK) {
      return mockService.resolveAlertEvent(token(), eventId);
    }
    const result = await http.post<BackendAlertEvent>(`/alerts/events/${eventId}/resolve`);
    return mapAlertEvent(result);
  },
  listRecords: async (): Promise<NotificationRecord[]> => {
    if (USE_MOCK) {
      return mockService.listNotificationRecords(token());
    }
    const page = await http.get<BackendPage<BackendNotificationRecord>>("/alerts/records");
    return pageItems(page).map(mapNotificationRecord);
  },
};

export const hostAvailabilityApi = {
  list: async (hostId: string): Promise<HostAvailabilityCheck[]> => {
    if (USE_MOCK) {
      return mockService.listHostAvailability(token(), hostId);
    }
    const page = await http.get<BackendPage<BackendHostAvailabilityCheck>>(`/hosts/${hostId}/availability`);
    return pageItems(page).map(mapHostAvailabilityCheck);
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
  wsUrl: (sessionId: string): string => {
    const sessionToken = useSessionStore.getState().token;
    const base = API_BASE_URL.startsWith("http") ? API_BASE_URL : `${window.location.origin}${API_BASE_URL}`;
    const url = new URL(`${base}/terminal/sessions/${sessionId}/ws`);
    if (sessionToken) {
      url.searchParams.set("token", sessionToken);
    }
    url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
    return url.toString();
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
      secretId: payload.tlsEnabled ? payload.secretId : "",
      description: payload.description,
    };
    const node = payload.id
      ? await http.patch<BackendDockerNode>(`/docker/nodes/${payload.id}`, backendPayload)
      : await http.post<BackendDockerNode>("/docker/nodes", backendPayload);
    return mapBackendDockerNode(node);
  },
  testNode: async (nodeId: string): Promise<{ connected: boolean; taskId?: string }> => {
    if (USE_MOCK) {
      const task = await mockService.testDockerNode(token(), nodeId);
      return { connected: true, taskId: task.id };
    }
    return http.post<{ connected: boolean; taskId?: string }>(`/docker/nodes/${nodeId}/test`);
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
  ): Promise<{ ok: boolean; taskId?: string }> => {
    if (USE_MOCK) {
      const task = await mockService.performContainerAction(token(), nodeId, containerId, action);
      return { ok: true, taskId: task.id };
    }
    return http
      .post<{ started?: boolean; stopped?: boolean; restarted?: boolean; taskId?: string }>(
        `/docker/nodes/${nodeId}/containers/${containerId}/${action}`,
      )
      .then((result) => ({ ok: true, taskId: result.taskId }));
  },
};

export const tasksApi = {
  list: async (filters?: {
    status?: string;
    keyword?: string;
    resourceType?: string;
    resourceId?: string;
  }): Promise<Task[]> => {
    if (USE_MOCK) {
      return mockService.listTasks(token(), filters);
    }
    const params = new URLSearchParams();
    if (filters?.status) {
      params.set("status", filters.status);
    }
    if (filters?.resourceType) {
      params.set("resourceType", filters.resourceType);
    }
    if (filters?.resourceId) {
      params.set("resourceId", filters.resourceId);
    }
    const query = params.toString();
    const page = await http.get<BackendPage<BackendTask>>(query ? `/tasks?${query}` : "/tasks");
    return pageItems(page).map(mapBackendTask);
  },
    detail: async (taskId: string): Promise<Task> => {
      if (USE_MOCK) {
        return mockService.getTask(token(), taskId);
      }
      const task = await http.get<BackendTask>(`/tasks/${taskId}`);
      return mapBackendTask(task);
    },
    context: async (taskId: string): Promise<TaskContext> => {
      if (USE_MOCK) {
        return mockService.getTaskContext(token(), taskId);
      }
      const context = await http.get<BackendTaskContext>(`/tasks/${taskId}/context`);
      return mapTaskContext(context);
    },
    cancel: async (taskId: string): Promise<{ canceled: boolean }> => {
      if (USE_MOCK) {
        throw new Error("Mock 模式暂不支持取消任务");
      }
    return http.post<{ canceled: boolean }>(`/tasks/${taskId}/cancel`);
  },
  retry: async (taskId: string): Promise<Task> => {
    if (USE_MOCK) {
      throw new Error("Mock 模式暂不支持重试任务");
    }
    const task = await http.post<BackendTask>(`/tasks/${taskId}/retry`);
    return mapBackendTask(task);
  },
};

export const scheduledJobsApi = {
  list: async (): Promise<ScheduledJob[]> => {
    if (USE_MOCK) {
      return [];
    }
    const page = await http.get<BackendPage<BackendScheduledJob>>("/scheduled-jobs");
    return pageItems(page).map(mapScheduledJob);
  },
  detail: async (jobId: string): Promise<ScheduledJob> => {
    if (USE_MOCK) {
      throw new Error("Mock 模式暂不支持读取调度任务");
    }
    const item = await http.get<BackendScheduledJob>(`/scheduled-jobs/${jobId}`);
    return mapScheduledJob(item);
  },
  dispatches: async (jobId: string): Promise<ScheduledJobDispatch[]> => {
    if (USE_MOCK) {
      return [];
    }
    const page = await http.get<BackendPage<BackendScheduledJobDispatch>>(`/scheduled-jobs/${jobId}/dispatches`);
    return pageItems(page).map((item) => mapScheduledJobDispatch(item, jobId));
  },
  save: async (payload: ScheduledJobInput): Promise<ScheduledJob> => {
    if (USE_MOCK) {
      throw new Error("Mock 模式暂不支持维护调度任务");
    }
    const backendPayload = {
      name: payload.name,
      type: payload.type,
      enabled: payload.enabled,
      cronExpr: payload.cronExpr,
      targetType: payload.targetType ?? "",
      targetId: payload.targetId ?? "",
      payloadJson: payload.payloadJson ?? "",
      retryPolicyJson: payload.retryPolicyJson ?? "",
      timeoutSeconds: payload.timeoutSeconds,
      concurrencyKey: payload.concurrencyKey ?? "",
    };
    const item = payload.id
      ? await http.patch<BackendScheduledJob>(`/scheduled-jobs/${payload.id}`, backendPayload)
      : await http.post<BackendScheduledJob>("/scheduled-jobs", backendPayload);
    return mapScheduledJob(item);
  },
  remove: async (jobId: string): Promise<{ deleted: boolean }> => {
    if (USE_MOCK) {
      throw new Error("Mock 模式暂不支持删除调度任务");
    }
    return http.delete<{ deleted: boolean }>(`/scheduled-jobs/${jobId}`);
  },
};

export const auditsApi = {
  list: async (filters?: {
    username?: string;
    action?: string;
    resourceType?: string;
    resourceId?: string;
    result?: string;
    keyword?: string;
  }): Promise<AuditLog[]> => {
    if (USE_MOCK) {
      return mockService.listAudits(token(), filters);
    }
    const params = new URLSearchParams();
    if (filters?.username) {
      params.set("username", filters.username);
    }
    if (filters?.action) {
      params.set("action", filters.action);
    }
    if (filters?.resourceType) {
      params.set("resourceType", filters.resourceType);
    }
    if (filters?.resourceId) {
      params.set("resourceId", filters.resourceId);
    }
    if (filters?.result) {
      params.set("result", filters.result.toLowerCase());
    }
    const query = params.toString();
    const page = await http.get<BackendPage<BackendAuditLog>>(query ? `/audits?${query}` : "/audits");
    return pageItems(page).map(mapBackendAudit);
  },
};
