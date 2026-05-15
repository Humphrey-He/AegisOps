import { permissionCatalog } from "../lib/permissions";
import { ApiError, type FieldErrors } from "../types/api";
import type {
  AdminSetupInput,
  AuditLog,
  ContainerItem,
  DashboardSummary,
  DockerNode,
  DockerNodeInput,
  Host,
  HostInput,
  LoginInput,
  Role,
  RoleInput,
  Registry,
  RegistryInput,
  RegistryManifestResult,
  RegistryRepositoriesResult,
  RegistryTagsResult,
  ServiceDefinition,
  ServiceDefinitionInput,
  ServiceInstance,
  ServiceReleaseInput,
  ServiceReleaseRecord,
  ServiceReleaseResult,
  ServiceRollbackInput,
  ServiceVersion,
  Secret,
  SecretInputPayload,
  Task,
  TaskLogLevel,
  TaskStep,
  TaskStatus,
  TerminalSession,
  User,
  UserInput,
} from "../types/models";

const STORAGE_KEY = "aegisops-mvp-db";
const RESPONSE_DELAY = 260;
const TASK_DELAY = 1600;
const DEMO_DATASET_VERSION = 3;
const MINUTE_MS = 60 * 1000;

type StoredUser = User & {
  password: string;
};

type StoredSecret = Secret & {
  secretValue: string;
};

type StoredRegistryManifest = {
  digest: string;
  contentType: string;
  manifest: unknown;
};

type StoredRegistryCatalog = {
  repositories: string[];
  tags: Record<string, string[]>;
  manifests: Record<string, Record<string, StoredRegistryManifest>>;
};

type MockDb = {
  demoVersion: number;
  users: StoredUser[];
  roles: Role[];
  secrets: StoredSecret[];
  hosts: Host[];
  registries: Registry[];
  registryCatalogs: Record<string, StoredRegistryCatalog>;
  services: ServiceDefinition[];
  serviceVersions: ServiceVersion[];
  serviceInstances: ServiceInstance[];
  serviceReleases: ServiceReleaseRecord[];
  dockerNodes: DockerNode[];
  containers: ContainerItem[];
  tasks: Task[];
  audits: AuditLog[];
  terminalSessions: TerminalSession[];
};

function now() {
  return new Date().toISOString();
}

function traceId() {
  return crypto.randomUUID();
}

function delay<T>(value: T, ms = RESPONSE_DELAY) {
  return new Promise<T>((resolve) => {
    window.setTimeout(() => resolve(value), ms);
  });
}

function timestampMinutesAgo(minutesAgo: number) {
  return new Date(Date.now() - minutesAgo * MINUTE_MS).toISOString();
}

function maskSecret(secretValue: string) {
  if (secretValue.length <= 8) {
    return "*".repeat(secretValue.length);
  }
  return `${secretValue.slice(0, 2)}${"*".repeat(Math.max(4, secretValue.length - 4))}${secretValue.slice(-2)}`;
}

function uniqueStrings(values: string[]) {
  return Array.from(new Set(values.map((item) => item.trim()).filter(Boolean)));
}

function upsertItem<T>(
  items: T[],
  matcher: (item: T) => boolean,
  create: () => T,
  merge?: (item: T) => void,
) {
  const existing = items.find(matcher);
  if (existing) {
    merge?.(existing);
    return existing;
  }
  const created = create();
  items.unshift(created);
  return created;
}

function mergeRegistryCatalog(target: StoredRegistryCatalog, source: StoredRegistryCatalog) {
  target.repositories = uniqueStrings([...target.repositories, ...source.repositories]);

  for (const [repository, tags] of Object.entries(source.tags)) {
    target.tags[repository] = uniqueStrings([...(target.tags[repository] ?? []), ...tags]);
  }

  for (const [repository, manifests] of Object.entries(source.manifests)) {
    target.manifests[repository] = {
      ...(target.manifests[repository] ?? {}),
      ...manifests,
    };
  }
}

function setTaskTimeline(
  task: Task,
  options: {
    status: TaskStatus;
    startedMinutesAgo: number;
    finishedMinutesAgo?: number;
    progress?: number;
    logs?: Array<{ level: TaskLogLevel; message: string }>;
    stepDetails?: Array<string | undefined>;
  },
) {
  const startedAt = new Date(Date.now() - options.startedMinutesAgo * MINUTE_MS);
  const finishedAt =
    options.finishedMinutesAgo !== undefined ? new Date(Date.now() - options.finishedMinutesAgo * MINUTE_MS) : undefined;
  const computedProgress =
    options.progress ??
    (options.status === "SUCCESS" || options.status === "FAILED"
      ? 100
      : options.status === "RUNNING"
        ? 62
        : 0);
  const endTime = finishedAt?.getTime() ?? startedAt.getTime() + Math.max(task.steps.length, 2) * MINUTE_MS;
  const entries: Array<{ level: TaskLogLevel; message: string }> =
    options.logs?.length
      ? options.logs
      : [
          {
            level: options.status === "FAILED" ? ("ERROR" as const) : ("INFO" as const),
            message: task.summary ?? task.type,
          },
        ];

  task.createdAt = startedAt.toISOString();
  task.startedAt = task.createdAt;
  task.finishedAt = finishedAt?.toISOString();
  task.status = options.status;
  task.progress = computedProgress;

  task.logs = entries.map((entry, index) => {
    const timestamp = new Date(
      startedAt.getTime() + Math.floor(((index + 1) * (endTime - startedAt.getTime())) / (entries.length + 1)),
    );
    return {
      id: task.logs[index]?.id ?? crypto.randomUUID(),
      timestamp: timestamp.toISOString(),
      level: entry.level,
      message: entry.message,
    };
  });

  task.steps = task.steps.map((step, index) => {
    let status: TaskStatus = "PENDING";
    if (options.status === "SUCCESS") {
      status = "SUCCESS";
    } else if (options.status === "FAILED") {
      status = index < task.steps.length - 1 ? "SUCCESS" : "FAILED";
    } else if (options.status === "RUNNING") {
      status = index === 0 ? "SUCCESS" : index === 1 ? "RUNNING" : "PENDING";
    }

    const stepStartedAt =
      status === "PENDING" ? undefined : new Date(startedAt.getTime() + index * MINUTE_MS).toISOString();
    const stepFinishedAt =
      status === "SUCCESS" || status === "FAILED"
        ? new Date(
            Math.min(
              startedAt.getTime() + (index + 1) * MINUTE_MS,
              finishedAt?.getTime() ?? startedAt.getTime() + (index + 1) * MINUTE_MS,
            ),
          ).toISOString()
        : undefined;

    return {
      ...step,
      detail: options.stepDetails?.[index] ?? step.detail,
      status,
      startedAt: stepStartedAt,
      finishedAt: stepFinishedAt,
    };
  });
}

function setAuditTimeline(audit: AuditLog, minutesAgo: number) {
  audit.createdAt = timestampMinutesAgo(minutesAgo);
}

function createStoredDb(): MockDb {
  return {
    demoVersion: 0,
    users: [],
    roles: createDefaultRoles(),
    secrets: [],
    hosts: [],
    registries: [],
    registryCatalogs: {},
    services: [],
    serviceVersions: [],
    serviceInstances: [],
    serviceReleases: [],
    dockerNodes: [],
    containers: [],
    tasks: [],
    audits: [],
    terminalSessions: [],
  };
}

function migrateDb(rawDb: Partial<MockDb>): MockDb {
  const db: MockDb = {
    demoVersion: rawDb.demoVersion ?? 0,
    users: rawDb.users ?? [],
    roles: rawDb.roles ?? createDefaultRoles(),
    secrets: (rawDb.secrets ?? []).map((secret) => ({
      ...secret,
      usedBy: secret.usedBy ?? [],
    })),
    hosts: rawDb.hosts ?? [],
    registries: rawDb.registries ?? [],
    registryCatalogs: rawDb.registryCatalogs ?? {},
    services: rawDb.services ?? [],
    serviceVersions: rawDb.serviceVersions ?? [],
    serviceInstances: rawDb.serviceInstances ?? [],
    serviceReleases: rawDb.serviceReleases ?? [],
    dockerNodes: rawDb.dockerNodes ?? [],
    containers: rawDb.containers ?? [],
    tasks: rawDb.tasks ?? [],
    audits: rawDb.audits ?? [],
    terminalSessions: rawDb.terminalSessions ?? [],
  };

  const allPermissions = permissionCatalog.map((item) => item.key);
  db.roles = db.roles.map((role) => {
    if (role.id === "role-admin") {
      return { ...role, permissions: allPermissions };
    }
    if (role.id === "role-ops") {
      return {
        ...role,
        permissions: Array.from(
          new Set([
            ...role.permissions,
            "registries.view",
            "registries.manage",
            "services.view",
            "services.manage",
            "services.release",
          ]),
        ),
      };
    }
    return role;
  });

  if (db.users.length > 0 && db.demoVersion < DEMO_DATASET_VERSION) {
    const admin = db.users.find((item) => item.roleIds.includes("role-admin")) ?? db.users[0];
    seedDemoResources(db, admin);
    upgradeDemoResources(db, admin);
  }

  return db;
}

function readDb(): MockDb {
  const raw = window.localStorage.getItem(STORAGE_KEY);
  if (!raw) {
    const initial = createStoredDb();
    writeDb(initial);
    return initial;
  }

  try {
    const migrated = migrateDb(JSON.parse(raw) as Partial<MockDb>);
    writeDb(migrated);
    return migrated;
  } catch {
    const reset = createStoredDb();
    writeDb(reset);
    return reset;
  }
}

function writeDb(db: MockDb) {
  window.localStorage.setItem(STORAGE_KEY, JSON.stringify(db));
}

function createDefaultRoles(): Role[] {
  const createdAt = now();
  return [
    {
      id: "role-admin",
      name: "platform-admin",
      description: "平台管理员，拥有一期 MVP 全量权限。",
      permissions: permissionCatalog.map((item) => item.key),
      builtIn: true,
      createdAt,
    },
    {
      id: "role-ops",
      name: "ops-engineer",
      description: "运维工程师，负责主机、Docker 和任务操作。",
      permissions: [
        "dashboard.view",
        "hosts.view",
        "hosts.manage",
        "secrets.view",
        "registries.view",
        "registries.manage",
        "terminal.open",
        "docker.view",
        "docker.manage",
        "tasks.view",
      ],
      builtIn: true,
      createdAt,
    },
    {
      id: "role-auditor",
      name: "security-auditor",
      description: "安全审计视角，只读工作台与审计。",
      permissions: ["dashboard.view", "audits.view", "tasks.view"],
      builtIn: true,
      createdAt,
    },
  ];
}

function getPermissions(db: MockDb, user: StoredUser) {
  const roleMap = new Map(db.roles.map((item) => [item.id, item]));
  return Array.from(
    new Set(
      user.roleIds.flatMap((roleId) => {
        return roleMap.get(roleId)?.permissions ?? [];
      }),
    ),
  );
}

function publicUser(user: StoredUser): User {
  const { password: _password, ...rest } = user;
  return rest;
}

function getUserFromToken(token: string | null | undefined, db: MockDb) {
  if (!token) {
    return null;
  }
  const userId = token.split(":")[1];
  return db.users.find((item) => item.id === userId) ?? null;
}

function requireAuth(token: string | null | undefined, db = readDb()) {
  const actor = getUserFromToken(token, db);
  if (!actor) {
    throw new ApiError({
      status: 401,
      code: "UNAUTHORIZED",
      message: "登录态已失效，请重新登录。",
      traceId: traceId(),
    });
  }
  if (actor.status !== "ACTIVE") {
    throw new ApiError({
      status: 403,
      code: "ACCOUNT_DISABLED",
      message: "当前账号已被禁用。",
      traceId: traceId(),
    });
  }
  return actor;
}

function requirePermission(token: string | null | undefined, permission: string, db = readDb()) {
  const actor = requireAuth(token, db);
  if (!getPermissions(db, actor).includes(permission)) {
    throw new ApiError({
      status: 403,
      code: "FORBIDDEN",
      message: "你没有执行该操作的权限。",
      traceId: traceId(),
    });
  }
  return actor;
}

function assertUniqueName(condition: boolean, message: string, fieldErrors?: FieldErrors) {
  if (!condition) {
    throw new ApiError({
      status: 409,
      code: "CONFLICT",
      message,
      traceId: traceId(),
      fieldErrors,
    });
  }
}

function appendAudit(
  db: MockDb,
  payload: Omit<AuditLog, "id" | "createdAt" | "traceId"> & { traceId?: string },
) {
  db.audits.unshift({
    id: crypto.randomUUID(),
    createdAt: now(),
    traceId: payload.traceId ?? traceId(),
    ...payload,
  });
}

function createTask(
  db: MockDb,
  payload: Pick<Task, "type" | "target" | "initiatedBy" | "summary" | "resourceType" | "resourceId"> & {
    steps: string[];
  },
) {
  const startedAt = now();
  const steps: TaskStep[] = payload.steps.map((title, index) => ({
    id: crypto.randomUUID(),
    title,
    status: index === 0 ? "RUNNING" : "PENDING",
    startedAt: index === 0 ? startedAt : undefined,
  }));
  const task: Task = {
    id: crypto.randomUUID(),
    type: payload.type,
    target: payload.target,
    resourceType: payload.resourceType,
    resourceId: payload.resourceId,
    initiatedBy: payload.initiatedBy,
    summary: payload.summary,
    status: "RUNNING",
    progress: 15,
    createdAt: startedAt,
    startedAt,
    steps,
    logs: [
      {
        id: crypto.randomUUID(),
        timestamp: startedAt,
        level: "INFO",
        message: `${payload.type} 已进入执行队列`,
      },
    ],
  };
  db.tasks.unshift(task);
  return task;
}

function finishTask(db: MockDb, taskId: string, summary: string, success = true) {
  const task = db.tasks.find((item) => item.id === taskId);
  if (!task) {
    return;
  }

  task.progress = 100;
  task.status = success ? "SUCCESS" : "FAILED";
  task.finishedAt = now();
  task.logs.push({
    id: crypto.randomUUID(),
    timestamp: task.finishedAt,
    level: success ? "INFO" : "ERROR",
    message: summary,
  });

  task.steps = task.steps.map((step, index) => {
    const isLast = index === task.steps.length - 1;
    return {
      ...step,
      status: isLast ? (success ? "SUCCESS" : "FAILED") : "SUCCESS",
      startedAt: step.startedAt ?? task.startedAt,
      finishedAt: task.finishedAt,
    };
  });
}

function seedDemoResources(db: MockDb, admin: StoredUser) {
  if (db.hosts.length || db.dockerNodes.length || db.secrets.length) {
    return;
  }

  const secretId = crypto.randomUUID();
  db.secrets.push({
    id: secretId,
    name: "prod-root-ssh",
    type: "SSH_PRIVATE_KEY",
    username: "root",
    description: "一期演示环境 SSH 凭证",
    secretValue: "-----BEGIN OPENSSH PRIVATE KEY-----demo-----END OPENSSH PRIVATE KEY-----",
    valueMasked: "-----BEGIN************KEY-----",
    usedBy: ["app-prod-01"],
    updatedAt: now(),
  });

  const hostId = crypto.randomUUID();
  db.hosts.push({
    id: hostId,
    name: "app-prod-01",
    address: "10.23.8.14",
    port: 22,
    secretId,
    status: "HEALTHY",
    tags: ["production", "web"],
    description: "一期演示主机",
    lastCheckedAt: now(),
  });

  const registrySecretId = crypto.randomUUID();
  db.secrets.push({
    id: registrySecretId,
    name: "harbor-basic-auth",
    type: "DOCKER_TOKEN",
    username: "release-bot",
    description: "二期 Registry 演示凭证，mock 环境下存储为 basic auth 示例。",
    secretValue: "release-bot:Harbor123!",
    valueMasked: maskSecret("release-bot:Harbor123!"),
    usedBy: ["harbor-prod"],
    updatedAt: now(),
  });

  const registryId = crypto.randomUUID();
  db.registries.push({
    id: registryId,
    name: "harbor-prod",
    url: "https://harbor.aegisops.local",
    authType: "BASIC",
    secretId: registrySecretId,
    description: "生产镜像仓库，用于二期发布链路演示。",
    status: "ONLINE",
    lastTestAt: now(),
    createdBy: admin.id,
    updatedBy: admin.id,
    createdAt: now(),
    updatedAt: now(),
  });
  db.registryCatalogs[registryId] = {
    repositories: ["aegisops/api", "aegisops/console", "aegisops/worker"],
    tags: {
      "aegisops/api": ["v0.1.0", "v0.2.0", "latest"],
      "aegisops/console": ["v0.2.0", "main-20260514", "latest"],
      "aegisops/worker": ["v0.1.3", "v0.2.0-rc1"],
    },
    manifests: {
      "aegisops/api": {
        "v0.1.0": {
          digest: "sha256:7fb6c95aa3c7456d4eac3af9122c7c8dd7e5c8e6a178efcae738aa01b8169d10",
          contentType: "application/vnd.oci.image.manifest.v1+json",
          manifest: {
            schemaVersion: 2,
            mediaType: "application/vnd.oci.image.manifest.v1+json",
            config: { digest: "sha256:api-v010", size: 1780 },
            layers: [{ digest: "sha256:api-layer-1", size: 1024 }],
          },
        },
        "v0.2.0": {
          digest: "sha256:9b1c9ef81ee8603cc502af42be1dd88d9637b6d96398b5aa07ed9e419f5bb2ab",
          contentType: "application/vnd.oci.image.manifest.v1+json",
          manifest: {
            schemaVersion: 2,
            mediaType: "application/vnd.oci.image.manifest.v1+json",
            config: { digest: "sha256:api-v020", size: 1824 },
            layers: [{ digest: "sha256:api-layer-2", size: 2048 }],
          },
        },
        latest: {
          digest: "sha256:9b1c9ef81ee8603cc502af42be1dd88d9637b6d96398b5aa07ed9e419f5bb2ab",
          contentType: "application/vnd.oci.image.manifest.v1+json",
          manifest: {
            schemaVersion: 2,
            mediaType: "application/vnd.oci.image.manifest.v1+json",
            config: { digest: "sha256:api-v020", size: 1824 },
            layers: [{ digest: "sha256:api-layer-2", size: 2048 }],
          },
        },
      },
      "aegisops/console": {
        "v0.2.0": {
          digest: "sha256:34c011cb7d08b2d997d13ac60c8edc3df6caf90bf9d5e0d17c6e0fbfd3db5108",
          contentType: "application/vnd.oci.image.manifest.v1+json",
          manifest: {
            schemaVersion: 2,
            mediaType: "application/vnd.oci.image.manifest.v1+json",
            config: { digest: "sha256:console-v020", size: 1544 },
            layers: [{ digest: "sha256:console-layer-1", size: 4096 }],
          },
        },
        "main-20260514": {
          digest: "sha256:8e9cbc69487b94914c5235f170d0f4dcb267d2df14d744fa0fa51f9c6211f947",
          contentType: "application/vnd.oci.image.manifest.v1+json",
          manifest: {
            schemaVersion: 2,
            mediaType: "application/vnd.oci.image.manifest.v1+json",
            config: { digest: "sha256:console-main-20260514", size: 1622 },
            layers: [{ digest: "sha256:console-layer-2", size: 6144 }],
          },
        },
        latest: {
          digest: "sha256:8e9cbc69487b94914c5235f170d0f4dcb267d2df14d744fa0fa51f9c6211f947",
          contentType: "application/vnd.oci.image.manifest.v1+json",
          manifest: {
            schemaVersion: 2,
            mediaType: "application/vnd.oci.image.manifest.v1+json",
            config: { digest: "sha256:console-main-20260514", size: 1622 },
            layers: [{ digest: "sha256:console-layer-2", size: 6144 }],
          },
        },
      },
      "aegisops/worker": {
        "v0.1.3": {
          digest: "sha256:0cb1cb17ddf7c93f24574ff544d1bce28f30c747831c7aee3c19d14e4b9e6f10",
          contentType: "application/vnd.oci.image.manifest.v1+json",
          manifest: {
            schemaVersion: 2,
            mediaType: "application/vnd.oci.image.manifest.v1+json",
            config: { digest: "sha256:worker-v013", size: 1337 },
            layers: [{ digest: "sha256:worker-layer-1", size: 512 }],
          },
        },
        "v0.2.0-rc1": {
          digest: "sha256:7e88c2d8348e2088095c733fca488e5ce537a0ff9dd4f829d8c0bc6363915ca2",
          contentType: "application/vnd.oci.image.manifest.v1+json",
          manifest: {
            schemaVersion: 2,
            mediaType: "application/vnd.oci.image.manifest.v1+json",
            config: { digest: "sha256:worker-v020rc1", size: 1488 },
            layers: [{ digest: "sha256:worker-layer-2", size: 768 }],
          },
        },
      },
    },
  };

  const nodeId = crypto.randomUUID();
  db.dockerNodes.push({
    id: nodeId,
    name: "docker-prod-01",
    endpoint: "tcp://10.23.8.14:2376",
    tlsEnabled: true,
    status: "ONLINE",
    description: "生产区 Docker 节点",
    lastCheckedAt: now(),
    containerCount: 2,
  });

  db.containers.push(
    {
      id: "container-nginx",
      nodeId,
      name: "gateway-nginx",
      image: "nginx:1.27-alpine",
      status: "running",
      ports: ["80:80", "443:443"],
      restartCount: 1,
      createdAt: now(),
    },
    {
      id: "container-api",
      nodeId,
      name: "aegisops-api",
      image: "aegisops/api:0.1.0",
      status: "running",
      ports: ["8080:8080"],
      restartCount: 0,
      createdAt: now(),
    },
  );

  const serviceId = crypto.randomUUID();
  const currentVersionId = crypto.randomUUID();
  const serviceReleaseTask = createTask(db, {
    type: "SERVICE_RELEASE",
    target: "service:aegisops-api",
    resourceType: "service",
    resourceId: serviceId,
    initiatedBy: admin.displayName,
    summary: "演示任务：发布 Aegis API 服务",
    steps: ["校验服务定义", "固化镜像版本", "记录发布状态"],
  });
  finishTask(db, serviceReleaseTask.id, "服务发布演示任务已完成。");

  db.services.push({
    id: serviceId,
    name: "Aegis API",
    code: "aegisops-api",
    group: "core",
    tags: ["production", "api"],
    description: "二期服务定义演示数据，复用 Registry 和 Docker 节点信息。",
    registryId,
    image: "aegisops/api",
    defaultTag: "latest",
    ports: [{ name: "http", containerPort: 8080, hostPort: 18080, protocol: "TCP" }],
    envs: [
      { key: "GIN_MODE", value: "release" },
      { key: "AEGIS_ENV", value: "prod" },
    ],
    mounts: [{ source: "/data/aegisops/api", target: "/app/data" }],
    resourceLimits: { cpu: "500m", memory: "512Mi" },
    targetType: "DOCKER_NODE",
    targetId: nodeId,
    status: "ACTIVE",
    currentVersion: "v0.2.0",
    createdBy: admin.id,
    updatedBy: admin.id,
    createdAt: now(),
    updatedAt: now(),
  });

  db.serviceVersions.push(
    {
      id: currentVersionId,
      serviceId,
      version: "v0.2.0",
      image: "aegisops/api",
      imageTag: "v0.2.0",
      imageDigest: "sha256:9b1c9ef81ee8603cc502af42be1dd88d9637b6d96398b5aa07ed9e419f5bb2ab",
      config: serializeServiceConfig({
        ports: [{ name: "http", containerPort: 8080, hostPort: 18080 }],
        envs: [{ key: "GIN_MODE", value: "release" }],
      }),
      createdBy: admin.id,
      createdAt: now(),
    },
    {
      id: crypto.randomUUID(),
      serviceId,
      version: "v0.1.0",
      image: "aegisops/api",
      imageTag: "v0.1.0",
      imageDigest: "sha256:7fb6c95aa3c7456d4eac3af9122c7c8dd7e5c8e6a178efcae738aa01b8169d10",
      config: serializeServiceConfig({
        ports: [{ name: "http", containerPort: 8080, hostPort: 18080 }],
      }),
      createdBy: admin.id,
      createdAt: now(),
    },
  );

  db.serviceInstances.push({
    id: crypto.randomUUID(),
    serviceId,
    versionId: currentVersionId,
    version: "v0.2.0",
    image: "aegisops/api",
    imageTag: "v0.2.0",
    dockerNodeId: nodeId,
    containerId: "container-api",
    name: "aegisops-api",
    status: "RUNNING",
    lastError: "",
    startedAt: now(),
    createdAt: now(),
    updatedAt: now(),
  });

  db.serviceReleases.push(
    {
      id: crypto.randomUUID(),
      serviceId,
      taskId: serviceReleaseTask.id,
      action: "RELEASE",
      fromVersionId: "",
      fromVersion: "",
      targetVersionId: currentVersionId,
      targetVersion: "v0.2.0",
      status: "SUCCESS",
      message: "首次发布到生产节点。",
      createdBy: admin.id,
      createdAt: now(),
      updatedAt: now(),
    },
    {
      id: crypto.randomUUID(),
      serviceId,
      taskId: serviceReleaseTask.id,
      action: "UPGRADE",
      fromVersionId: "",
      fromVersion: "v0.1.0",
      targetVersionId: currentVersionId,
      targetVersion: "v0.2.0",
      status: "SUCCESS",
      message: "升级到当前版本。",
      createdBy: admin.id,
      createdAt: now(),
      updatedAt: now(),
    },
  );

  const task = createTask(db, {
    type: "DOCKER_RESTART",
    target: "gateway-nginx",
    resourceType: "container",
    resourceId: "container-nginx",
    initiatedBy: admin.displayName,
    summary: "演示任务：重启网关容器",
    steps: ["检查目标容器", "发送重启命令", "校验容器状态"],
  });
  finishTask(db, task.id, "容器已成功重启。");

  appendAudit(db, {
    actor: admin.displayName,
    action: "admin.setup",
    resourceType: "system",
    resourceName: "bootstrap",
    result: "SUCCESS",
    summary: "初始化管理员并注入一期演示数据。",
  });
  appendAudit(db, {
    actor: admin.displayName,
    action: "registry.create",
    resourceType: "registry",
    resourceId: registryId,
    resourceName: "harbor-prod",
    result: "SUCCESS",
    summary: "注入二期 Registry 演示数据。",
  });
}

function upgradeDemoResources(db: MockDb, admin: StoredUser) {
  const ensureTask = (
    id: string,
    payload: Pick<Task, "type" | "target" | "initiatedBy" | "summary" | "resourceType" | "resourceId"> & {
      steps: string[];
    },
    timeline: {
      status: TaskStatus;
      startedMinutesAgo: number;
      finishedMinutesAgo?: number;
      progress?: number;
      logs?: Array<{ level: TaskLogLevel; message: string }>;
      stepDetails?: Array<string | undefined>;
    },
  ) => {
    let task = db.tasks.find((item) => item.id === id);
    if (!task) {
      task = createTask(db, payload);
      task.id = id;
    } else {
      task.type = payload.type;
      task.target = payload.target;
      task.resourceType = payload.resourceType;
      task.resourceId = payload.resourceId;
      task.initiatedBy = payload.initiatedBy;
      task.summary = payload.summary;
      task.steps = payload.steps.map((title, index) => ({
        id: task?.steps[index]?.id ?? crypto.randomUUID(),
        title,
        status: "PENDING",
      }));
      task.logs = [];
    }
    setTaskTimeline(task, timeline);
    return task;
  };

  const ensureAudit = (
    payload: Omit<AuditLog, "id" | "traceId" | "createdAt"> & { minutesAgo: number },
  ) => {
    const existing = db.audits.find(
      (item) => item.action === payload.action && item.resourceId === payload.resourceId && item.summary === payload.summary,
    );
    if (existing) {
      existing.actor = payload.actor;
      existing.resourceType = payload.resourceType;
      existing.resourceName = payload.resourceName;
      existing.result = payload.result;
      existing.summary = payload.summary;
      setAuditTimeline(existing, payload.minutesAgo);
      return existing;
    }
    appendAudit(db, payload);
    const created = db.audits[0];
    setAuditTimeline(created, payload.minutesAgo);
    return created;
  };

  const opsLead = upsertItem(
    db.users,
    (item) => item.username === "ops.lead",
    (): StoredUser => ({
      id: "user-ops-lead",
      username: "ops.lead",
      displayName: "Lin Ops",
      email: "ops.lead@aegisops.local",
      status: "ACTIVE",
      roleIds: ["role-ops"],
      password: "ChangeMe123!",
      createdAt: timestampMinutesAgo(60 * 24 * 28),
      lastLoginAt: timestampMinutesAgo(18),
    }),
  );

  const auditor = upsertItem(
    db.users,
    (item) => item.username === "sec.audit",
    (): StoredUser => ({
      id: "user-sec-audit",
      username: "sec.audit",
      displayName: "Mika Audit",
      email: "sec.audit@aegisops.local",
      status: "ACTIVE",
      roleIds: ["role-auditor"],
      password: "ChangeMe123!",
      createdAt: timestampMinutesAgo(60 * 24 * 20),
      lastLoginAt: timestampMinutesAgo(42),
    }),
  );

  upsertItem(
    db.users,
    (item) => item.username === "release.bot",
    (): StoredUser => ({
      id: "user-release-bot",
      username: "release.bot",
      displayName: "Release Bot",
      email: "release.bot@aegisops.local",
      status: "DISABLED",
      roleIds: ["role-ops"],
      password: "ChangeMe123!",
      createdAt: timestampMinutesAgo(60 * 24 * 16),
      lastLoginAt: timestampMinutesAgo(60 * 24 * 5),
    }),
  );

  const prodRootSecret = upsertItem(
    db.secrets,
    (item) => item.name === "prod-root-ssh",
    (): StoredSecret => ({
      id: "secret-prod-root-ssh",
      name: "prod-root-ssh",
      type: "SSH_PRIVATE_KEY",
      username: "root",
      description: "Production Linux host SSH private key.",
      secretValue: "-----BEGIN OPENSSH PRIVATE KEY-----demo-prod-----END OPENSSH PRIVATE KEY-----",
      valueMasked: "-----BEGIN************KEY-----",
      usedBy: [],
      updatedAt: timestampMinutesAgo(60 * 24 * 12),
    }),
  );

  const stagingSshSecret = upsertItem(
    db.secrets,
    (item) => item.name === "staging-deploy-password",
    (): StoredSecret => ({
      id: "secret-staging-password",
      name: "staging-deploy-password",
      type: "SSH_PASSWORD",
      username: "deploy",
      description: "Staging bastion password for smoke tests.",
      secretValue: "Stage@2026!",
      valueMasked: maskSecret("Stage@2026!"),
      usedBy: [],
      updatedAt: timestampMinutesAgo(60 * 24 * 6),
    }),
  );

  const dockerTlsSecret = upsertItem(
    db.secrets,
    (item) => item.name === "docker-prod-tls",
    (): StoredSecret => ({
      id: "secret-docker-prod-tls",
      name: "docker-prod-tls",
      type: "DOCKER_TLS",
      username: "docker-admin",
      description: "Docker remote TLS certificate bundle.",
      secretValue: "-----BEGIN CERTIFICATE-----mock-docker-prod-----END CERTIFICATE-----",
      valueMasked: "-----BEGIN************CERT-----",
      usedBy: [],
      updatedAt: timestampMinutesAgo(60 * 24 * 9),
    }),
  );

  const harborSecret = upsertItem(
    db.secrets,
    (item) => item.name === "harbor-basic-auth",
    (): StoredSecret => ({
      id: "secret-harbor-basic-auth",
      name: "harbor-basic-auth",
      type: "DOCKER_TOKEN",
      username: "release-bot",
      description: "Harbor basic auth used by delivery workflows.",
      secretValue: "release-bot:Harbor123!",
      valueMasked: maskSecret("release-bot:Harbor123!"),
      usedBy: [],
      updatedAt: timestampMinutesAgo(60 * 24 * 8),
    }),
  );

  const badRegistrySecret = upsertItem(
    db.secrets,
    (item) => item.name === "registry-bad-basic-auth",
    (): StoredSecret => ({
      id: "secret-registry-bad-basic-auth",
      name: "registry-bad-basic-auth",
      type: "DOCKER_TOKEN",
      username: "invalid",
      description: "Broken registry credential for failure drills.",
      secretValue: "invalid-basic-secret",
      valueMasked: maskSecret("invalid-basic-secret"),
      usedBy: [],
      updatedAt: timestampMinutesAgo(60 * 24 * 2),
    }),
  );

  const prodHost = upsertItem(
    db.hosts,
    (item) => item.name === "app-prod-01",
    (): Host => ({
      id: "host-app-prod-01",
      name: "app-prod-01",
      address: "10.23.8.14",
      port: 22,
      secretId: prodRootSecret.id,
      status: "HEALTHY",
      tags: ["production", "web", "core"],
      description: "Primary API host.",
      lastCheckedAt: timestampMinutesAgo(14),
    }),
  );

  const workerHost = upsertItem(
    db.hosts,
    (item) => item.name === "job-worker-01",
    (): Host => ({
      id: "host-job-worker-01",
      name: "job-worker-01",
      address: "10.23.8.26",
      port: 22,
      secretId: prodRootSecret.id,
      status: "HEALTHY",
      tags: ["production", "worker", "batch"],
      description: "Batch worker and image sync host.",
      lastCheckedAt: timestampMinutesAgo(26),
    }),
  );

  const stagingHost = upsertItem(
    db.hosts,
    (item) => item.name === "staging-console-01",
    (): Host => ({
      id: "host-staging-console-01",
      name: "staging-console-01",
      address: "10.10.12.33",
      port: 22,
      secretId: stagingSshSecret.id,
      status: "UNREACHABLE",
      tags: ["staging", "console", "canary"],
      description: "Staging console host with intermittent network failures.",
      lastCheckedAt: timestampMinutesAgo(47),
    }),
  );

  const harborProd = upsertItem(
    db.registries,
    (item) => item.name === "harbor-prod",
    (): Registry => ({
      id: "registry-harbor-prod",
      name: "harbor-prod",
      url: "https://harbor.aegisops.local",
      authType: "BASIC",
      secretId: harborSecret.id,
      description: "Primary production registry.",
      status: "ONLINE",
      lastTestAt: timestampMinutesAgo(22),
      createdBy: admin.id,
      updatedBy: admin.id,
      createdAt: timestampMinutesAgo(60 * 24 * 14),
      updatedAt: timestampMinutesAgo(22),
    }),
  );

  const harborStaging = upsertItem(
    db.registries,
    (item) => item.name === "harbor-staging",
    (): Registry => ({
      id: "registry-harbor-staging",
      name: "harbor-staging",
      url: "https://harbor-staging.aegisops.local",
      authType: "BASIC",
      secretId: harborSecret.id,
      description: "Staging registry for canary validation.",
      status: "ONLINE",
      lastTestAt: timestampMinutesAgo(31),
      createdBy: admin.id,
      updatedBy: opsLead.id,
      createdAt: timestampMinutesAgo(60 * 24 * 11),
      updatedAt: timestampMinutesAgo(31),
    }),
  );

  const legacyRegistry = upsertItem(
    db.registries,
    (item) => item.name === "legacy-registry-offline",
    (): Registry => ({
      id: "registry-legacy-offline",
      name: "legacy-registry-offline",
      url: "https://offline-registry.aegisops.local",
      authType: "BASIC",
      secretId: badRegistrySecret.id,
      description: "Legacy registry kept for network and auth failure drills.",
      status: "OFFLINE",
      lastTestAt: timestampMinutesAgo(58),
      createdBy: admin.id,
      updatedBy: opsLead.id,
      createdAt: timestampMinutesAgo(60 * 24 * 30),
      updatedAt: timestampMinutesAgo(58),
    }),
  );

  db.registryCatalogs[harborProd.id] = db.registryCatalogs[harborProd.id] ?? { repositories: [], tags: {}, manifests: {} };
  db.registryCatalogs[harborStaging.id] =
    db.registryCatalogs[harborStaging.id] ?? { repositories: [], tags: {}, manifests: {} };
  db.registryCatalogs[legacyRegistry.id] =
    db.registryCatalogs[legacyRegistry.id] ?? { repositories: [], tags: {}, manifests: {} };

  mergeRegistryCatalog(db.registryCatalogs[harborProd.id], {
    repositories: ["aegisops/api", "aegisops/console", "aegisops/worker", "aegisops/agent"],
    tags: {
      "aegisops/api": ["v0.1.0", "v0.2.0", "v0.3.0", "latest"],
      "aegisops/console": ["v0.2.0", "v0.3.0-rc1", "main-20260515", "latest"],
      "aegisops/worker": ["v0.1.3", "v0.2.0", "latest"],
      "aegisops/agent": ["v0.1.0", "v0.1.1"],
    },
    manifests: {
      "aegisops/api": {
        "v0.3.0": {
          digest: "sha256:70d98f5397b1f18138a77ef2d8f5e1a5ad1207a0b6f5b77e6d5dcaf5e29ce713",
          contentType: "application/vnd.oci.image.manifest.v1+json",
          manifest: {
            schemaVersion: 2,
            mediaType: "application/vnd.oci.image.manifest.v1+json",
            config: { digest: "sha256:api-v030", size: 1896 },
            layers: [{ digest: "sha256:api-layer-3", size: 3072 }],
          },
        },
        latest: {
          digest: "sha256:70d98f5397b1f18138a77ef2d8f5e1a5ad1207a0b6f5b77e6d5dcaf5e29ce713",
          contentType: "application/vnd.oci.image.manifest.v1+json",
          manifest: {
            schemaVersion: 2,
            mediaType: "application/vnd.oci.image.manifest.v1+json",
            config: { digest: "sha256:api-v030", size: 1896 },
            layers: [{ digest: "sha256:api-layer-3", size: 3072 }],
          },
        },
      },
    },
  });

  mergeRegistryCatalog(db.registryCatalogs[harborStaging.id], {
    repositories: ["aegisops/api", "aegisops/console"],
    tags: {
      "aegisops/api": ["v0.2.0", "v0.3.0-rc1", "canary-20260515"],
      "aegisops/console": ["main-20260515", "preview-login-fix"],
    },
    manifests: {},
  });

  mergeRegistryCatalog(db.registryCatalogs[legacyRegistry.id], {
    repositories: ["legacy/service-a", "legacy/service-b"],
    tags: {
      "legacy/service-a": ["2024.11", "2025.01"],
      "legacy/service-b": ["snapshot"],
    },
    manifests: {},
  });

  const prodNode = upsertItem(
    db.dockerNodes,
    (item) => item.name === "docker-prod-01",
    (): DockerNode => ({
      id: "docker-node-prod-01",
      name: "docker-prod-01",
      endpoint: "tcp://10.23.8.14:2376",
      tlsEnabled: true,
      secretId: dockerTlsSecret.id,
      status: "ONLINE",
      description: "Production node for API and gateway.",
      lastCheckedAt: timestampMinutesAgo(17),
      containerCount: 0,
    }),
  );

  const workerNode = upsertItem(
    db.dockerNodes,
    (item) => item.name === "docker-worker-01",
    (): DockerNode => ({
      id: "docker-node-worker-01",
      name: "docker-worker-01",
      endpoint: "tcp://10.23.8.26:2376",
      tlsEnabled: true,
      secretId: dockerTlsSecret.id,
      status: "ONLINE",
      description: "Worker node for background jobs.",
      lastCheckedAt: timestampMinutesAgo(25),
      containerCount: 0,
    }),
  );

  const stagingNode = upsertItem(
    db.dockerNodes,
    (item) => item.name === "docker-staging-01",
    (): DockerNode => ({
      id: "docker-node-staging-01",
      name: "docker-staging-01",
      endpoint: "tcp://10.10.12.33:2376",
      tlsEnabled: true,
      secretId: dockerTlsSecret.id,
      status: "OFFLINE",
      description: "Staging node currently unreachable.",
      lastCheckedAt: timestampMinutesAgo(48),
      containerCount: 0,
    }),
  );

  const containers: ContainerItem[] = [
    {
      id: "container-gateway-nginx",
      nodeId: prodNode.id,
      name: "gateway-nginx",
      image: "nginx:1.27-alpine",
      status: "running",
      ports: ["80:80", "443:443"],
      restartCount: 1,
      createdAt: timestampMinutesAgo(60 * 24 * 9),
    },
    {
      id: "container-aegis-api",
      nodeId: prodNode.id,
      name: "aegisops-api",
      image: "aegisops/api:v0.3.0",
      status: "running",
      ports: ["8080:8080"],
      restartCount: 0,
      createdAt: timestampMinutesAgo(34),
    },
    {
      id: "container-console-web",
      nodeId: stagingNode.id,
      name: "aegisops-console",
      image: "aegisops/console:preview-login-fix",
      status: "paused",
      ports: ["4173:4173"],
      restartCount: 3,
      createdAt: timestampMinutesAgo(60 * 24 * 2),
    },
    {
      id: "container-worker-sync",
      nodeId: workerNode.id,
      name: "aegisops-worker-sync",
      image: "aegisops/worker:v0.2.0",
      status: "running",
      ports: [],
      restartCount: 0,
      createdAt: timestampMinutesAgo(60 * 10),
    },
  ];

  for (const container of containers) {
    upsertItem(db.containers, (item) => item.id === container.id, () => container, (existing) => {
      Object.assign(existing, container);
    });
  }

  prodNode.containerCount = db.containers.filter((item) => item.nodeId === prodNode.id).length;
  workerNode.containerCount = db.containers.filter((item) => item.nodeId === workerNode.id).length;
  stagingNode.containerCount = db.containers.filter((item) => item.nodeId === stagingNode.id).length;

  bindSecretUsage(db, prodRootSecret.id, prodHost.name, true);
  bindSecretUsage(db, prodRootSecret.id, workerHost.name, true);
  bindSecretUsage(db, stagingSshSecret.id, stagingHost.name, true);
  bindSecretUsage(db, harborSecret.id, harborProd.name, true);
  bindSecretUsage(db, harborSecret.id, harborStaging.name, true);
  bindSecretUsage(db, badRegistrySecret.id, legacyRegistry.name, true);
  bindSecretUsage(db, dockerTlsSecret.id, prodNode.name, true);
  bindSecretUsage(db, dockerTlsSecret.id, workerNode.name, true);
  bindSecretUsage(db, dockerTlsSecret.id, stagingNode.name, true);

  const apiService = upsertItem(
    db.services,
    (item) => item.code === "aegisops-api",
    (): ServiceDefinition => ({
      id: "service-aegis-api",
      name: "Aegis API",
      code: "aegisops-api",
      group: "core",
      tags: ["production", "api", "critical"],
      description: "Core backend API service.",
      registryId: harborProd.id,
      image: "aegisops/api",
      defaultTag: "latest",
      ports: [{ name: "http", containerPort: 8080, hostPort: 18080, protocol: "TCP" }],
      envs: [
        { key: "GIN_MODE", value: "release" },
        { key: "AEGIS_ENV", value: "prod" },
      ],
      mounts: [{ source: "/data/aegisops/api", target: "/app/data" }],
      resourceLimits: { cpu: "500m", memory: "512Mi" },
      targetType: "DOCKER_NODE",
      targetId: prodNode.id,
      status: "ACTIVE",
      currentVersion: "v0.3.0",
      createdBy: admin.id,
      updatedBy: opsLead.id,
      createdAt: timestampMinutesAgo(60 * 24 * 10),
      updatedAt: timestampMinutesAgo(23),
    }),
  );
  apiService.currentVersion = "v0.3.0";
  apiService.updatedBy = opsLead.id;
  apiService.updatedAt = timestampMinutesAgo(23);

  const consoleService = upsertItem(
    db.services,
    (item) => item.code === "aegisops-console",
    (): ServiceDefinition => ({
      id: "service-aegis-console",
      name: "Aegis Console",
      code: "aegisops-console",
      group: "frontend",
      tags: ["staging", "console", "canary"],
      description: "Frontend console under staging verification.",
      registryId: harborStaging.id,
      image: "aegisops/console",
      defaultTag: "main-20260515",
      ports: [{ name: "web", containerPort: 4173, hostPort: 4173, protocol: "TCP" }],
      envs: [
        { key: "VITE_USE_MOCK", value: "true" },
        { key: "AEGIS_ENV", value: "staging" },
      ],
      mounts: [],
      resourceLimits: { cpu: "300m", memory: "384Mi" },
      targetType: "DOCKER_NODE",
      targetId: stagingNode.id,
      status: "ACTIVE",
      currentVersion: "main-20260515",
      createdBy: admin.id,
      updatedBy: opsLead.id,
      createdAt: timestampMinutesAgo(60 * 24 * 6),
      updatedAt: timestampMinutesAgo(35),
    }),
  );

  const workerService = upsertItem(
    db.services,
    (item) => item.code === "aegisops-worker",
    (): ServiceDefinition => ({
      id: "service-aegis-worker",
      name: "Aegis Worker",
      code: "aegisops-worker",
      group: "jobs",
      tags: ["production", "worker"],
      description: "Background job and registry sync worker.",
      registryId: harborProd.id,
      image: "aegisops/worker",
      defaultTag: "v0.2.0",
      ports: [],
      envs: [
        { key: "WORKER_CONCURRENCY", value: "6" },
        { key: "AEGIS_ENV", value: "prod" },
      ],
      mounts: [{ source: "/data/aegisops/worker", target: "/app/data" }],
      resourceLimits: { cpu: "300m", memory: "256Mi" },
      targetType: "DOCKER_NODE",
      targetId: workerNode.id,
      status: "ACTIVE",
      currentVersion: "v0.2.0",
      createdBy: admin.id,
      updatedBy: admin.id,
      createdAt: timestampMinutesAgo(60 * 24 * 8),
      updatedAt: timestampMinutesAgo(41),
    }),
  );

  const apiVersionV1 = upsertItem(
    db.serviceVersions,
    (item) => item.serviceId === apiService.id && item.version === "v0.1.0",
    (): ServiceVersion => ({
      id: "service-version-api-v010",
      serviceId: apiService.id,
      version: "v0.1.0",
      image: "aegisops/api",
      imageTag: "v0.1.0",
      imageDigest: "sha256:7fb6c95aa3c7456d4eac3af9122c7c8dd7e5c8e6a178efcae738aa01b8169d10",
      config: serializeServiceConfig({ ports: [{ name: "http", containerPort: 8080, hostPort: 18080 }] }),
      createdBy: admin.id,
      createdAt: timestampMinutesAgo(60 * 24 * 9),
    }),
  );

  const apiVersionV2 = upsertItem(
    db.serviceVersions,
    (item) => item.serviceId === apiService.id && item.version === "v0.2.0",
    (): ServiceVersion => ({
      id: "service-version-api-v020",
      serviceId: apiService.id,
      version: "v0.2.0",
      image: "aegisops/api",
      imageTag: "v0.2.0",
      imageDigest: "sha256:9b1c9ef81ee8603cc502af42be1dd88d9637b6d96398b5aa07ed9e419f5bb2ab",
      config: serializeServiceConfig({
        ports: [{ name: "http", containerPort: 8080, hostPort: 18080 }],
        envs: [{ key: "GIN_MODE", value: "release" }],
      }),
      createdBy: admin.id,
      createdAt: timestampMinutesAgo(60 * 24 * 4),
    }),
  );

  const apiVersionV3 = upsertItem(
    db.serviceVersions,
    (item) => item.serviceId === apiService.id && item.version === "v0.3.0",
    (): ServiceVersion => ({
      id: "service-version-api-v030",
      serviceId: apiService.id,
      version: "v0.3.0",
      image: "aegisops/api",
      imageTag: "v0.3.0",
      imageDigest: "sha256:70d98f5397b1f18138a77ef2d8f5e1a5ad1207a0b6f5b77e6d5dcaf5e29ce713",
      config: serializeServiceConfig({
        ports: [{ name: "http", containerPort: 8080, hostPort: 18080 }],
        envs: [
          { key: "GIN_MODE", value: "release" },
          { key: "AEGIS_ENV", value: "prod" },
        ],
      }),
      createdBy: opsLead.id,
      createdAt: timestampMinutesAgo(38),
    }),
  );

  const consoleVersionMain = upsertItem(
    db.serviceVersions,
    (item) => item.serviceId === consoleService.id && item.version === "main-20260515",
    (): ServiceVersion => ({
      id: "service-version-console-main",
      serviceId: consoleService.id,
      version: "main-20260515",
      image: "aegisops/console",
      imageTag: "main-20260515",
      imageDigest: "sha256:8e9cbc69487b94914c5235f170d0f4dcb267d2df14d744fa0fa51f9c6211f947",
      config: serializeServiceConfig({
        ports: [{ name: "web", containerPort: 4173, hostPort: 4173 }],
        envs: [{ key: "VITE_USE_MOCK", value: "true" }],
      }),
      createdBy: opsLead.id,
      createdAt: timestampMinutesAgo(60 * 24),
    }),
  );

  const consoleVersionPreview = upsertItem(
    db.serviceVersions,
    (item) => item.serviceId === consoleService.id && item.version === "preview-login-fix",
    (): ServiceVersion => ({
      id: "service-version-console-preview",
      serviceId: consoleService.id,
      version: "preview-login-fix",
      image: "aegisops/console",
      imageTag: "preview-login-fix",
      imageDigest: "sha256:0e2aabff37511ac60760ed6906d6c0df78f074e5104266fe0ce99f6d8a5d534d",
      config: serializeServiceConfig({
        ports: [{ name: "web", containerPort: 4173, hostPort: 4173 }],
        envs: [
          { key: "VITE_USE_MOCK", value: "true" },
          { key: "FEATURE_LOGIN_FIX", value: "true" },
        ],
      }),
      createdBy: opsLead.id,
      createdAt: timestampMinutesAgo(95),
    }),
  );

  const workerVersion = upsertItem(
    db.serviceVersions,
    (item) => item.serviceId === workerService.id && item.version === "v0.2.0",
    (): ServiceVersion => ({
      id: "service-version-worker-v020",
      serviceId: workerService.id,
      version: "v0.2.0",
      image: "aegisops/worker",
      imageTag: "v0.2.0",
      imageDigest: "sha256:28fd7c13ebefb4ee0ad72bf0b6d5348182bd2d3fd2bf3c5cda6f8a66c44a49fa",
      config: serializeServiceConfig({ envs: [{ key: "WORKER_CONCURRENCY", value: "6" }] }),
      createdBy: admin.id,
      createdAt: timestampMinutesAgo(60 * 12),
    }),
  );

  upsertItem(
    db.serviceInstances,
    (item) => item.serviceId === apiService.id && item.name === "aegisops-api",
    (): ServiceInstance => ({
      id: "service-instance-api-prod",
      serviceId: apiService.id,
      versionId: apiVersionV3.id,
      version: apiVersionV3.version,
      image: "aegisops/api",
      imageTag: apiVersionV3.imageTag,
      dockerNodeId: prodNode.id,
      containerId: "container-aegis-api",
      name: "aegisops-api",
      status: "RUNNING",
      lastError: "",
      startedAt: timestampMinutesAgo(34),
      createdAt: timestampMinutesAgo(34),
      updatedAt: timestampMinutesAgo(12),
    }),
  );

  upsertItem(
    db.serviceInstances,
    (item) => item.serviceId === consoleService.id && item.name === "aegisops-console",
    (): ServiceInstance => ({
      id: "service-instance-console-staging",
      serviceId: consoleService.id,
      versionId: consoleVersionPreview.id,
      version: consoleVersionPreview.version,
      image: "aegisops/console",
      imageTag: consoleVersionPreview.imageTag,
      dockerNodeId: stagingNode.id,
      containerId: "container-console-web",
      name: "aegisops-console",
      status: "FAILED",
      lastError: "staging node offline: websocket preview route unavailable",
      startedAt: timestampMinutesAgo(120),
      stoppedAt: timestampMinutesAgo(48),
      createdAt: timestampMinutesAgo(120),
      updatedAt: timestampMinutesAgo(48),
    }),
  );

  upsertItem(
    db.serviceInstances,
    (item) => item.serviceId === workerService.id && item.name === "aegisops-worker-sync",
    (): ServiceInstance => ({
      id: "service-instance-worker-prod",
      serviceId: workerService.id,
      versionId: workerVersion.id,
      version: workerVersion.version,
      image: "aegisops/worker",
      imageTag: workerVersion.imageTag,
      dockerNodeId: workerNode.id,
      containerId: "container-worker-sync",
      name: "aegisops-worker-sync",
      status: "RUNNING",
      lastError: "",
      startedAt: timestampMinutesAgo(60 * 10),
      createdAt: timestampMinutesAgo(60 * 10),
      updatedAt: timestampMinutesAgo(32),
    }),
  );

  const apiReleaseTask = ensureTask(
    "task-service-release-api-v030",
    {
      type: "SERVICE_RELEASE",
      target: "service:aegisops-api",
      resourceType: "service",
      resourceId: apiService.id,
      initiatedBy: opsLead.displayName,
      summary: "Release Aegis API to production v0.3.0",
      steps: ["Validate service spec", "Lock image digest", "Switch production instance"],
    },
    {
      status: "SUCCESS",
      startedMinutesAgo: 38,
      finishedMinutesAgo: 34,
      logs: [
        { level: "INFO", message: "Locked production service aegisops-api." },
        { level: "INFO", message: "Image digest for aegisops/api:v0.3.0 verified." },
        { level: "INFO", message: "Production instance switched successfully." },
      ],
    },
  );

  const consoleReleaseTask = ensureTask(
    "task-service-release-console-preview",
    {
      type: "SERVICE_RELEASE",
      target: "service:aegisops-console",
      resourceType: "service",
      resourceId: consoleService.id,
      initiatedBy: opsLead.displayName,
      summary: "Release staging console preview-login-fix",
      steps: ["Pull preview image", "Start staging preview instance", "Run login and users-page smoke checks"],
    },
    {
      status: "FAILED",
      startedMinutesAgo: 96,
      finishedMinutesAgo: 92,
      logs: [
        { level: "INFO", message: "Pulled aegisops/console:preview-login-fix from harbor-staging." },
        { level: "INFO", message: "Staging preview instance started on docker-staging-01." },
        { level: "ERROR", message: "Smoke check failed after login: the /system/users page did not finish rendering." },
      ],
      stepDetails: [
        "Resolved image tag preview-login-fix from harbor-staging and verified manifest digest.",
        "Preview container booted on port 4173, but response latency stayed above the staging baseline.",
        "After login the user management page stayed degraded, so the preview release was marked failed.",
      ],
    },
  );

  const consoleRollbackTask = ensureTask(
    "task-service-rollback-console-preview",
    {
      type: "SERVICE_ROLLBACK",
      target: "service:aegisops-console",
      resourceType: "service",
      resourceId: consoleService.id,
      initiatedBy: opsLead.displayName,
      summary: "Rollback staging console preview-login-fix",
      steps: ["Confirm target version", "Update instance status", "Record rollback result"],
    },
    {
      status: "FAILED",
      startedMinutesAgo: 92,
      finishedMinutesAgo: 88,
      logs: [
        { level: "INFO", message: "Detected login-page regression on staging." },
        { level: "ERROR", message: "docker-staging-01 is offline, rollback could not finish." },
      ],
      stepDetails: [
        "Selected main-20260515 as the rollback target for the staging console.",
        "The failed preview instance remained paused and no healthy replacement container could be scheduled.",
        "Rollback was blocked because docker-staging-01 stopped responding before container reconciliation finished.",
      ],
    },
  );

  ensureTask(
    "task-docker-node-test-staging-console",
    {
      type: "DOCKER_NODE_TEST",
      target: stagingNode.name,
      resourceType: "docker-node",
      resourceId: stagingNode.id,
      initiatedBy: opsLead.displayName,
      summary: "Verify staging Docker node after console release incident",
      steps: ["Open TLS connection", "Inspect node heartbeat", "Check container runtime state"],
    },
    {
      status: "FAILED",
      startedMinutesAgo: 52,
      finishedMinutesAgo: 48,
      logs: [
        { level: "INFO", message: "Loaded docker-prod-tls bundle for staging endpoint." },
        { level: "WARN", message: "Remote API handshake retried twice before timing out." },
        { level: "ERROR", message: "docker-staging-01 did not return container runtime metadata." },
      ],
      stepDetails: [
        "Attempted TLS handshake against tcp://10.10.12.33:2376 with the shared remote Docker certificate.",
        "Node heartbeat could not be refreshed and the cached status stayed OFFLINE.",
        "Container runtime inspection never returned, so the incident drill escalated to host-level SSH diagnostics.",
      ],
    },
  );

  const registrySyncTask = ensureTask(
    "task-registry-sync-harbor-prod",
    {
      type: "REGISTRY_SYNC",
      target: "registry:harbor-prod",
      resourceType: "registry",
      resourceId: harborProd.id,
      initiatedBy: admin.displayName,
      summary: "Sync Harbor digests into local index",
      steps: ["Read repository catalog", "Sync tags", "Refresh version cache"],
    },
    {
      status: "RUNNING",
      startedMinutesAgo: 9,
      progress: 64,
      logs: [
        { level: "INFO", message: "Syncing Harbor production catalog." },
        { level: "WARN", message: "Console repository contains many tags, sync is slower than usual." },
      ],
      stepDetails: [
        "Production Harbor catalog returned 4 repositories for reconciliation.",
        "Tag sync is still running for the console repository because preview tags are more fragmented than usual.",
        "Version cache refresh will continue after all tag pages are folded into the local index.",
      ],
    },
  );

  ensureTask(
    "task-host-test-staging-console",
    {
      type: "SSH_TEST",
      target: stagingHost.name,
      resourceType: "host",
      resourceId: stagingHost.id,
      initiatedBy: opsLead.displayName,
      summary: "Test SSH to staging-console-01",
      steps: ["Open network connection", "Authenticate secret", "Wait for remote response"],
    },
    {
      status: "FAILED",
      startedMinutesAgo: 50,
      finishedMinutesAgo: 47,
      logs: [
        { level: "INFO", message: "Loaded staging deploy credential." },
        { level: "ERROR", message: "SSH handshake timeout for 10.10.12.33." },
      ],
      stepDetails: [
        "Routing to 10.10.12.33:22 succeeded only intermittently during the incident window.",
        "The deploy credential was accepted locally, but the remote side never completed the SSH handshake.",
        "Host reachability check failed, so no manual rollback command could be executed on the staging machine.",
      ],
    },
  );

  ensureTask(
    "task-container-restart-nginx",
    {
      type: "CONTAINER_RESTART",
      target: "gateway-nginx",
      resourceType: "container",
      resourceId: "container-gateway-nginx",
      initiatedBy: admin.displayName,
      summary: "Restart production gateway container",
      steps: ["Inspect container state", "Send restart command", "Verify new state"],
    },
    {
      status: "SUCCESS",
      startedMinutesAgo: 70,
      finishedMinutesAgo: 68,
      logs: [
        { level: "INFO", message: "Health checks were green before restart." },
        { level: "INFO", message: "gateway-nginx restarted and resumed traffic." },
      ],
      stepDetails: [
        "The production gateway had no active 5xx burst before the maintenance restart.",
        "Restart completed within the planned window and existing upstream sockets were drained cleanly.",
        "Traffic and health probes recovered immediately after the new process accepted connections.",
      ],
    },
  );

  ensureTask(
    "task-registry-test-legacy",
    {
      type: "REGISTRY_TEST",
      target: legacyRegistry.name,
      resourceType: "registry",
      resourceId: legacyRegistry.id,
      initiatedBy: opsLead.displayName,
      summary: "Test legacy offline registry",
      steps: ["Load registry config", "Call registry api", "Record result"],
    },
    {
      status: "FAILED",
      startedMinutesAgo: 62,
      finishedMinutesAgo: 58,
      logs: [
        { level: "INFO", message: "Loaded failure-drill registry config." },
        { level: "ERROR", message: "dial tcp timeout while contacting legacy registry." },
      ],
      stepDetails: [
        "Legacy registry credentials were loaded for the failure drill environment.",
        "The remote registry endpoint never returned a catalog response within the timeout budget.",
        "The failure was recorded as a network timeout instead of an authentication error.",
      ],
    },
  );

  setTaskTimeline(apiReleaseTask, {
    status: "SUCCESS",
    startedMinutesAgo: 38,
    finishedMinutesAgo: 34,
    logs: [
      { level: "INFO", message: "Locked production service aegisops-api." },
      { level: "INFO", message: "Image digest for aegisops/api:v0.3.0 verified." },
      { level: "INFO", message: "Production instance switched successfully." },
    ],
    stepDetails: [
      "Validated CPU, memory, and port bindings against the active production service definition.",
      "Pinned the v0.3.0 digest before rollout so the release stayed reproducible across environments.",
      "The production API instance was replaced cleanly and health checks passed after cutover.",
    ],
  });

  const releases: ServiceReleaseRecord[] = [
    {
      id: "release-api-v010",
      serviceId: apiService.id,
      taskId: apiReleaseTask.id,
      action: "RELEASE",
      fromVersionId: "",
      fromVersion: "",
      targetVersionId: apiVersionV1.id,
      targetVersion: apiVersionV1.version,
      status: "SUCCESS",
      message: "First production release for Aegis API.",
      createdBy: admin.id,
      createdAt: timestampMinutesAgo(60 * 24 * 9),
      updatedAt: timestampMinutesAgo(60 * 24 * 9),
    },
    {
      id: "release-api-v020",
      serviceId: apiService.id,
      taskId: apiReleaseTask.id,
      action: "UPGRADE",
      fromVersionId: apiVersionV1.id,
      fromVersion: apiVersionV1.version,
      targetVersionId: apiVersionV2.id,
      targetVersion: apiVersionV2.version,
      status: "SUCCESS",
      message: "Upgrade API service to v0.2.0.",
      createdBy: admin.id,
      createdAt: timestampMinutesAgo(60 * 24 * 4),
      updatedAt: timestampMinutesAgo(60 * 24 * 4),
    },
    {
      id: "release-api-v030",
      serviceId: apiService.id,
      taskId: apiReleaseTask.id,
      action: "UPGRADE",
      fromVersionId: apiVersionV2.id,
      fromVersion: apiVersionV2.version,
      targetVersionId: apiVersionV3.id,
      targetVersion: apiVersionV3.version,
      status: "SUCCESS",
      message: "Upgrade API service to v0.3.0.",
      createdBy: opsLead.id,
      createdAt: timestampMinutesAgo(34),
      updatedAt: timestampMinutesAgo(34),
    },
    {
      id: "release-console-preview",
      serviceId: consoleService.id,
      taskId: consoleReleaseTask.id,
      action: "UPGRADE",
      fromVersionId: consoleVersionMain.id,
      fromVersion: consoleVersionMain.version,
      targetVersionId: consoleVersionPreview.id,
      targetVersion: consoleVersionPreview.version,
      status: "FAILED",
      message: "Staging console preview failed validation and needs rollback.",
      createdBy: opsLead.id,
      createdAt: timestampMinutesAgo(92),
      updatedAt: timestampMinutesAgo(92),
    },
    {
      id: "release-console-rollback-blocked",
      serviceId: consoleService.id,
      taskId: consoleRollbackTask.id,
      action: "ROLLBACK",
      fromVersionId: consoleVersionPreview.id,
      fromVersion: consoleVersionPreview.version,
      targetVersionId: consoleVersionMain.id,
      targetVersion: consoleVersionMain.version,
      status: "FAILED",
      message: "Automatic rollback was blocked because the staging Docker node went offline.",
      createdBy: opsLead.id,
      createdAt: timestampMinutesAgo(88),
      updatedAt: timestampMinutesAgo(88),
    },
    {
      id: "release-worker-v020",
      serviceId: workerService.id,
      taskId: registrySyncTask.id,
      action: "RELEASE",
      fromVersionId: "",
      fromVersion: "",
      targetVersionId: workerVersion.id,
      targetVersion: workerVersion.version,
      status: "SUCCESS",
      message: "Worker service attached to production job node.",
      createdBy: admin.id,
      createdAt: timestampMinutesAgo(60 * 12),
      updatedAt: timestampMinutesAgo(60 * 12),
    },
  ];

  for (const release of releases) {
    upsertItem(db.serviceReleases, (item) => item.id === release.id, () => release, (existing) => {
      Object.assign(existing, release);
    });
  }

  ensureAudit({
    actor: admin.displayName,
    action: "admin.setup",
    resourceType: "system",
    resourceId: "bootstrap",
    resourceName: "bootstrap",
    result: "SUCCESS",
    summary: "Initialized admin and injected demo dataset.",
    minutesAgo: 60 * 24 * 14,
  });
  ensureAudit({
    actor: admin.displayName,
    action: "registry.create",
    resourceType: "registry",
    resourceId: harborProd.id,
    resourceName: harborProd.name,
    result: "SUCCESS",
    summary: "Injected production Harbor demo registry.",
    minutesAgo: 60 * 24 * 14 - 3,
  });
  ensureAudit({
    actor: opsLead.displayName,
    action: "service.release",
    resourceType: "service",
    resourceId: apiService.id,
    resourceName: apiService.code,
    result: "SUCCESS",
    summary: "Released service to version v0.3.0.",
    minutesAgo: 34,
  });
  ensureAudit({
    actor: opsLead.displayName,
    action: "service.release",
    resourceType: "service",
    resourceId: consoleService.id,
    resourceName: consoleService.code,
    result: "FAILED",
    summary: "Preview release failed during post-login smoke verification on the users page.",
    minutesAgo: 92,
  });
  ensureAudit({
    actor: opsLead.displayName,
    action: "service.rollback",
    resourceType: "service",
    resourceId: consoleService.id,
    resourceName: consoleService.code,
    result: "FAILED",
    summary: "Rollback to main-20260515 was blocked because docker-staging-01 was offline.",
    minutesAgo: 88,
  });
  ensureAudit({
    actor: opsLead.displayName,
    action: "host.test_ssh",
    resourceType: "host",
    resourceId: stagingHost.id,
    resourceName: stagingHost.name,
    result: "FAILED",
    summary: "SSH test failed because the staging host is unreachable.",
    minutesAgo: 47,
  });
  ensureAudit({
    actor: opsLead.displayName,
    action: "docker.node.test",
    resourceType: "docker-node",
    resourceId: stagingNode.id,
    resourceName: stagingNode.name,
    result: "FAILED",
    summary: "Docker node connectivity test failed while validating the staging rollback environment.",
    minutesAgo: 48,
  });
  ensureAudit({
    actor: opsLead.displayName,
    action: "registry.test",
    resourceType: "registry",
    resourceId: legacyRegistry.id,
    resourceName: legacyRegistry.name,
    result: "FAILED",
    summary: "Registry connectivity test failed with dial tcp timeout.",
    minutesAgo: 58,
  });
  ensureAudit({
    actor: admin.displayName,
    action: "docker.container.restart",
    resourceType: "container",
    resourceId: "container-gateway-nginx",
    resourceName: "gateway-nginx",
    result: "SUCCESS",
    summary: "Production gateway container restarted successfully.",
    minutesAgo: 68,
  });
  ensureAudit({
    actor: auditor.displayName,
    action: "audit.review",
    resourceType: "service",
    resourceId: consoleService.id,
    resourceName: consoleService.code,
    result: "SUCCESS",
    summary: "Reviewed staging rollback trail and task logs.",
    minutesAgo: 54,
  });

  upsertItem(
    db.terminalSessions,
    (item) => item.id === "terminal-session-prod-api",
    () => ({
      id: "terminal-session-prod-api",
      hostId: prodHost.id,
      hostName: prodHost.name,
      status: "CONNECTED",
      createdAt: timestampMinutesAgo(16),
      welcomeLines: [
        `Connected to ${prodHost.name} (${prodHost.address}:${prodHost.port})`,
        "Last login: Fri May 15 20:33:41 CST 2026 from 10.23.8.1",
        "Release logs are available under /var/log/aegisops/",
      ],
    }),
  );

  db.tasks.sort((left, right) => right.createdAt.localeCompare(left.createdAt));
  db.audits.sort((left, right) => right.createdAt.localeCompare(left.createdAt));
  db.serviceVersions.sort((left, right) => right.createdAt.localeCompare(left.createdAt));
  db.serviceReleases.sort((left, right) => right.createdAt.localeCompare(left.createdAt));
  db.serviceInstances.sort((left, right) => right.updatedAt.localeCompare(left.updatedAt));
  db.demoVersion = DEMO_DATASET_VERSION;
}

function filterByKeyword<T>(items: T[], keyword: string, mapper: (item: T) => string) {
  if (!keyword) {
    return items;
  }
  const normalized = keyword.trim().toLowerCase();
  return items.filter((item) => mapper(item).toLowerCase().includes(normalized));
}

function matchesResourceType(actual?: string, expected?: string) {
  if (!expected) {
    return true;
  }
  return (actual ?? "").trim().toLowerCase() === expected.trim().toLowerCase();
}

function matchesResourceId(haystack: string, resourceId?: string) {
  if (!resourceId) {
    return true;
  }
  return haystack.toLowerCase().includes(resourceId.trim().toLowerCase());
}

function getContainerLogs(container: ContainerItem) {
  if (container.name === "aegisops-console") {
    return [
      `[${timestampMinutesAgo(96)}] INFO Pulling release tag preview-login-fix from harbor-staging`,
      `[${timestampMinutesAgo(95)}] INFO Preview container started on 0.0.0.0:4173`,
      `[${timestampMinutesAgo(93)}] WARN Post-login smoke test exceeded latency budget on /system/users`,
      `[${timestampMinutesAgo(92)}] ERROR UI verification failed and the preview release was marked as an incident`,
      `[${timestampMinutesAgo(88)}] ERROR Automatic rollback could not continue because docker-staging-01 went offline`,
    ];
  }

  if (container.name === "gateway-nginx") {
    return [
      `[${timestampMinutesAgo(70)}] INFO Preparing planned gateway restart`,
      `[${timestampMinutesAgo(69)}] INFO Existing upstream connections drained successfully`,
      `[${timestampMinutesAgo(68)}] INFO nginx master process restarted and traffic resumed`,
      `[${timestampMinutesAgo(67)}] INFO Health probes recovered within the maintenance window`,
    ];
  }

  if (container.name === "aegisops-api") {
    return [
      `[${timestampMinutesAgo(38)}] INFO Starting production rollout for aegisops/api:v0.3.0`,
      `[${timestampMinutesAgo(36)}] INFO Image digest sha256:70d98f5397b1 verified`,
      `[${timestampMinutesAgo(34)}] INFO New API container accepted traffic on :8080`,
      `[${timestampMinutesAgo(14)}] INFO Background health probes remain stable`,
    ];
  }

  if (container.name === "aegisops-worker-sync") {
    return [
      `[${timestampMinutesAgo(11)}] INFO Harbor catalog sync started`,
      `[${timestampMinutesAgo(10)}] INFO Repository aegisops/api tag list refreshed`,
      `[${timestampMinutesAgo(9)}] WARN Console repository contains slow preview-tag pages`,
      `[${timestampMinutesAgo(8)}] INFO Waiting for remaining tag pages before cache refresh`,
    ];
  }

  return [
    `[${now()}] INFO Boot sequence ready for ${container.name}`,
    `[${now()}] INFO Listening on 0.0.0.0`,
    `[${now()}] WARN Health probe latency 220ms`,
    `[${now()}] INFO Last deploy completed`,
  ];
}

function validateRegistryPayload(payload: RegistryInput) {
  const fieldErrors: FieldErrors = {};
  if (!payload.name.trim()) {
    fieldErrors.name = "请输入 Registry 名称";
  }
  if (!payload.url.trim()) {
    fieldErrors.url = "请输入 Registry 地址";
  }
  if (payload.authType !== "NONE" && !(payload.secretId ?? "").trim()) {
    fieldErrors.secretId = "当前认证方式需要绑定凭证";
  }
  if (Object.keys(fieldErrors).length) {
    throw new ApiError({
      status: 422,
      code: "VALIDATION_ERROR",
      message: "请补全 Registry 信息。",
      traceId: traceId(),
      fieldErrors,
    });
  }
}

function validateServicePayload(payload: ServiceDefinitionInput) {
  const fieldErrors: FieldErrors = {};
  if (!payload.name.trim()) {
    fieldErrors.name = "请输入服务名称";
  }
  if (!payload.code.trim()) {
    fieldErrors.code = "请输入服务编码";
  }
  if (!payload.image.trim()) {
    fieldErrors.image = "请输入镜像仓库路径";
  }
  if (!payload.registryId.trim()) {
    fieldErrors.registryId = "请选择关联 Registry";
  }
  if (!payload.targetId.trim()) {
    fieldErrors.targetId = "请选择发布目标";
  }
  if (Object.keys(fieldErrors).length) {
    throw new ApiError({
      status: 422,
      code: "VALIDATION_ERROR",
      message: "请补全服务定义信息。",
      traceId: traceId(),
      fieldErrors,
    });
  }
}

function matchRegistryError(message: string) {
  if (message.includes("auth")) {
    return {
      status: 401,
      code: "REGISTRY_AUTH_FAILED",
      message,
    };
  }
  if (message.includes("network")) {
    return {
      status: 502,
      code: "REGISTRY_NETWORK_ERROR",
      message,
    };
  }
  if (message.includes("not found")) {
    return {
      status: 404,
      code: "REGISTRY_RESOURCE_NOT_FOUND",
      message,
    };
  }
  return {
    status: 400,
    code: "REGISTRY_ERROR",
    message,
  };
}

function getRegistryOrThrow(db: MockDb, registryId: string) {
  const registry = db.registries.find((item) => item.id === registryId);
  if (!registry) {
    throw new ApiError({
      status: 404,
      code: "NOT_FOUND",
      message: "Registry 不存在。",
      traceId: traceId(),
    });
  }
  return registry;
}

function getRegistryCatalogOrThrow(db: MockDb, registryId: string) {
  const catalog = db.registryCatalogs[registryId];
  if (!catalog) {
    throw new ApiError({
      status: 404,
      code: "NOT_FOUND",
      message: "Registry 目录不存在。",
      traceId: traceId(),
    });
  }
  return catalog;
}

function bindSecretUsage(db: MockDb, secretId: string | undefined, resourceName: string, enabled: boolean) {
  if (!secretId) {
    return;
  }
  const secret = db.secrets.find((item) => item.id === secretId);
  if (!secret) {
    return;
  }
  const next = new Set(secret.usedBy);
  if (enabled) {
    next.add(resourceName);
  } else {
    next.delete(resourceName);
  }
  secret.usedBy = Array.from(next);
}

function getServiceOrThrow(db: MockDb, serviceId: string) {
  const service = db.services.find((item) => item.id === serviceId);
  if (!service) {
    throw new ApiError({
      status: 404,
      code: "NOT_FOUND",
      message: "服务定义不存在。",
      traceId: traceId(),
    });
  }
  return service;
}

function serializeServiceConfig(value: unknown) {
  return JSON.stringify(value ?? {});
}

export const mockService = {
  async health() {
    return delay({ status: "ok", mode: "mock" as const });
  },

  async getSetupStatus() {
    const db = readDb();
    return delay({ initialized: db.users.length > 0 });
  },

  async initAdmin(payload: AdminSetupInput) {
    const db = readDb();
    if (db.users.length > 0) {
      throw new ApiError({
        status: 409,
        code: "ALREADY_INITIALIZED",
        message: "管理员已初始化，无需重复执行。",
        traceId: traceId(),
      });
    }

    const createdAt = now();
    const admin: StoredUser = {
      id: crypto.randomUUID(),
      username: payload.username,
      displayName: payload.displayName,
      email: payload.email,
      status: "ACTIVE",
      roleIds: ["role-admin"],
      password: payload.password,
      createdAt,
      lastLoginAt: createdAt,
    };
    db.users.push(admin);
    seedDemoResources(db, admin);
    upgradeDemoResources(db, admin);
    writeDb(db);
    return delay({ created: true });
  },

  async login(payload: LoginInput) {
    const db = readDb();
    const user = db.users.find((item) => item.username === payload.username);
    if (!user || user.password !== payload.password) {
      throw new ApiError({
        status: 422,
        code: "INVALID_CREDENTIALS",
        message: "用户名或密码不正确。",
        traceId: traceId(),
        fieldErrors: { username: "请检查用户名和密码", password: "请检查用户名和密码" },
      });
    }
    if (user.status !== "ACTIVE") {
      throw new ApiError({
        status: 403,
        code: "ACCOUNT_DISABLED",
        message: "账号已被禁用，请联系管理员。",
        traceId: traceId(),
      });
    }

    user.lastLoginAt = now();
    writeDb(db);
    appendAudit(db, {
      actor: user.displayName,
      action: "auth.login",
      resourceType: "user",
      resourceId: user.id,
      resourceName: user.username,
      result: "SUCCESS",
      summary: "用户登录成功。",
    });
    writeDb(db);

    return delay({
      token: `mock-token:${user.id}:${Date.now()}`,
      user: publicUser(user),
      permissions: getPermissions(db, user),
    });
  },

  async me(token: string | null) {
    const db = readDb();
    const user = requireAuth(token, db);
    return delay({
      user: publicUser(user),
      permissions: getPermissions(db, user),
    });
  },

  async logout(token: string | null) {
    const db = readDb();
    const actor = getUserFromToken(token, db);
    if (actor) {
      appendAudit(db, {
        actor: actor.displayName,
        action: "auth.logout",
        resourceType: "user",
        resourceId: actor.id,
        resourceName: actor.username,
        result: "SUCCESS",
        summary: "用户主动退出登录。",
      });
      writeDb(db);
    }
    return delay({ ok: true });
  },

  async dashboardSummary(token: string | null) {
    const db = readDb();
    requirePermission(token, "dashboard.view", db);
    const unhealthyResourceCount =
      db.hosts.filter((item) => item.status !== "HEALTHY").length +
      db.dockerNodes.filter((item) => item.status !== "ONLINE").length;
    const summary: DashboardSummary = {
      userCount: db.users.length,
      hostCount: db.hosts.length,
      dockerNodeCount: db.dockerNodes.length,
      containerCount: db.containers.length,
      unhealthyResourceCount,
      recentTasks: db.tasks.slice(0, 5),
      recentAudits: db.audits.slice(0, 6),
    };
    return delay(summary);
  },

  async listUsers(token: string | null, keyword = "") {
    const db = readDb();
    requirePermission(token, "users.view", db);
    return delay(
      filterByKeyword(db.users.map(publicUser), keyword, (item) => {
        return `${item.username} ${item.displayName} ${item.email}`;
      }),
    );
  },

  async saveUser(token: string | null, payload: UserInput) {
    const db = readDb();
    const actor = requirePermission(token, "users.manage", db);
    const fieldErrors: FieldErrors = {};
    if (!payload.username.trim()) {
      fieldErrors.username = "用户名不能为空";
    }
    if (!payload.displayName.trim()) {
      fieldErrors.displayName = "姓名不能为空";
    }
    if (!payload.roleIds.length) {
      fieldErrors.roleIds = "至少绑定一个角色";
    }
    if (!payload.id && !payload.password) {
      fieldErrors.password = "新建用户时需要设置密码";
    }
    if (Object.keys(fieldErrors).length) {
      throw new ApiError({
        status: 422,
        code: "VALIDATION_ERROR",
        message: "请补全用户信息。",
        traceId: traceId(),
        fieldErrors,
      });
    }

    if (payload.id) {
      const target = db.users.find((item) => item.id === payload.id);
      if (!target) {
        throw new ApiError({
          status: 404,
          code: "NOT_FOUND",
          message: "用户不存在。",
          traceId: traceId(),
        });
      }
      if (actor.id === target.id && payload.status === "DISABLED") {
        throw new ApiError({
          status: 409,
          code: "SELF_DISABLE_BLOCKED",
          message: "当前登录用户不能禁用自己。",
          traceId: traceId(),
        });
      }
      assertUniqueName(
        !db.users.some((item) => item.username === payload.username && item.id !== payload.id),
        "用户名已存在。",
        { username: "用户名已存在" },
      );
      target.username = payload.username;
      target.displayName = payload.displayName;
      target.email = payload.email;
      target.status = payload.status;
      target.roleIds = payload.roleIds;
      if (payload.password) {
        target.password = payload.password;
      }
      appendAudit(db, {
        actor: actor.displayName,
        action: "user.update",
        resourceType: "user",
        resourceId: target.id,
        resourceName: target.username,
        result: "SUCCESS",
        summary: "更新用户信息与角色绑定。",
      });
      writeDb(db);
      return delay(publicUser(target));
    }

    assertUniqueName(
      !db.users.some((item) => item.username === payload.username),
      "用户名已存在。",
      { username: "用户名已存在" },
    );

    const created: StoredUser = {
      id: crypto.randomUUID(),
      username: payload.username,
      displayName: payload.displayName,
      email: payload.email,
      status: payload.status,
      roleIds: payload.roleIds,
      password: payload.password ?? "ChangeMe123!",
      createdAt: now(),
    };
    db.users.unshift(created);
    appendAudit(db, {
      actor: actor.displayName,
      action: "user.create",
      resourceType: "user",
      resourceId: created.id,
      resourceName: created.username,
      result: "SUCCESS",
      summary: "创建用户并绑定角色。",
    });
    writeDb(db);
    return delay(publicUser(created));
  },

  async listRoles(token: string | null) {
    const db = readDb();
    requirePermission(token, "roles.view", db);
    return delay(db.roles);
  },

  async saveRole(token: string | null, payload: RoleInput) {
    const db = readDb();
    const actor = requirePermission(token, "roles.manage", db);
    if (!payload.name.trim()) {
      throw new ApiError({
        status: 422,
        code: "VALIDATION_ERROR",
        message: "角色名称不能为空。",
        traceId: traceId(),
        fieldErrors: { name: "角色名称不能为空" },
      });
    }
    if (payload.id) {
      const role = db.roles.find((item) => item.id === payload.id);
      if (!role) {
        throw new ApiError({
          status: 404,
          code: "NOT_FOUND",
          message: "角色不存在。",
          traceId: traceId(),
        });
      }
      if (role.builtIn && role.id === "role-admin") {
        throw new ApiError({
          status: 409,
          code: "BUILTIN_ROLE_LOCKED",
          message: "平台管理员角色默认锁定，不建议直接修改。",
          traceId: traceId(),
        });
      }
      role.name = payload.name;
      role.description = payload.description;
      role.permissions = payload.permissions;
      appendAudit(db, {
        actor: actor.displayName,
        action: "role.update",
        resourceType: "role",
        resourceId: role.id,
        resourceName: role.name,
        result: "SUCCESS",
        summary: "更新角色权限配置。",
      });
      writeDb(db);
      return delay(role);
    }

    assertUniqueName(
      !db.roles.some((item) => item.name === payload.name),
      "角色名称已存在。",
      { name: "角色名称已存在" },
    );
    const role: Role = {
      id: crypto.randomUUID(),
      name: payload.name,
      description: payload.description,
      permissions: payload.permissions,
      createdAt: now(),
    };
    db.roles.unshift(role);
    appendAudit(db, {
      actor: actor.displayName,
      action: "role.create",
      resourceType: "role",
      resourceId: role.id,
      resourceName: role.name,
      result: "SUCCESS",
      summary: "创建角色并分配权限。",
    });
    writeDb(db);
    return delay(role);
  },

  async listSecrets(token: string | null, keyword = "") {
    const db = readDb();
    requirePermission(token, "secrets.view", db);
    return delay(
      filterByKeyword(db.secrets, keyword, (item) => `${item.name} ${item.type} ${item.username ?? ""}`),
    );
  },

  async saveSecret(token: string | null, payload: SecretInputPayload) {
    const db = readDb();
    const actor = requirePermission(token, "secrets.manage", db);
    if (!payload.secretValue.trim()) {
      throw new ApiError({
        status: 422,
        code: "VALIDATION_ERROR",
        message: "凭证内容不能为空。",
        traceId: traceId(),
        fieldErrors: { secretValue: "请输入凭证内容" },
      });
    }
    if (payload.id) {
      const secret = db.secrets.find((item) => item.id === payload.id);
      if (!secret) {
        throw new ApiError({
          status: 404,
          code: "NOT_FOUND",
          message: "凭证不存在。",
          traceId: traceId(),
        });
      }
      secret.name = payload.name;
      secret.type = payload.type;
      secret.username = payload.username;
      secret.description = payload.description;
      secret.secretValue = payload.secretValue;
      secret.valueMasked = maskSecret(payload.secretValue);
      secret.updatedAt = now();
      appendAudit(db, {
        actor: actor.displayName,
        action: "secret.update",
        resourceType: "secret",
        resourceId: secret.id,
        resourceName: secret.name,
        result: "SUCCESS",
        summary: "更新凭证并重新脱敏存储。",
      });
      writeDb(db);
      return delay(secret);
    }
    const created: StoredSecret = {
      id: crypto.randomUUID(),
      name: payload.name,
      type: payload.type,
      username: payload.username,
      description: payload.description,
      secretValue: payload.secretValue,
      valueMasked: maskSecret(payload.secretValue),
      usedBy: [],
      updatedAt: now(),
    };
    db.secrets.unshift(created);
    appendAudit(db, {
      actor: actor.displayName,
      action: "secret.create",
      resourceType: "secret",
      resourceId: created.id,
      resourceName: created.name,
      result: "SUCCESS",
      summary: "创建新的凭证记录。",
    });
    writeDb(db);
    return delay(created);
  },

  async listHosts(token: string | null, keyword = "") {
    const db = readDb();
    requirePermission(token, "hosts.view", db);
    return delay(
      filterByKeyword(db.hosts, keyword, (item) => {
        return `${item.name} ${item.address} ${item.tags.join(" ")}`;
      }),
    );
  },

  async saveHost(token: string | null, payload: HostInput) {
    const db = readDb();
    const actor = requirePermission(token, "hosts.manage", db);
    if (payload.id) {
      const host = db.hosts.find((item) => item.id === payload.id);
      if (!host) {
        throw new ApiError({
          status: 404,
          code: "NOT_FOUND",
          message: "主机不存在。",
          traceId: traceId(),
        });
      }
      host.name = payload.name;
      host.address = payload.address;
      host.port = payload.port;
      host.secretId = payload.secretId;
      host.tags = payload.tags;
      host.description = payload.description;
      appendAudit(db, {
        actor: actor.displayName,
        action: "host.update",
        resourceType: "host",
        resourceId: host.id,
        resourceName: host.name,
        result: "SUCCESS",
        summary: "更新主机信息。",
      });
      writeDb(db);
      return delay(host);
    }
    const host: Host = {
      id: crypto.randomUUID(),
      name: payload.name,
      address: payload.address,
      port: payload.port,
      secretId: payload.secretId,
      status: "UNKNOWN",
      tags: payload.tags,
      description: payload.description,
    };
    db.hosts.unshift(host);
    appendAudit(db, {
      actor: actor.displayName,
      action: "host.create",
      resourceType: "host",
      resourceId: host.id,
      resourceName: host.name,
      result: "SUCCESS",
      summary: "创建新的主机资产。",
    });
    writeDb(db);
    return delay(host);
  },

  async listRegistries(token: string | null, keyword = "") {
    const db = readDb();
    requirePermission(token, "registries.view", db);
    return delay(
      filterByKeyword(db.registries, keyword, (item) => `${item.name} ${item.url} ${item.description ?? ""}`),
    );
  },

  async getRegistry(token: string | null, registryId: string) {
    const db = readDb();
    requirePermission(token, "registries.view", db);
    return delay(getRegistryOrThrow(db, registryId));
  },

  async listServices(token: string | null, keyword = "", status = "") {
    const db = readDb();
    requirePermission(token, "services.view", db);
    const items = filterByKeyword(db.services, keyword, (item) => {
      return `${item.name} ${item.code} ${item.image} ${item.group} ${item.tags.join(" ")}`;
    }).filter((item) => !status || item.status === status);
    return delay(items);
  },

  async getService(token: string | null, serviceId: string) {
    const db = readDb();
    requirePermission(token, "services.view", db);
    return delay(getServiceOrThrow(db, serviceId));
  },

  async saveService(token: string | null, payload: ServiceDefinitionInput) {
    const db = readDb();
    const actor = requirePermission(token, "services.manage", db);
    validateServicePayload(payload);
    if (!db.registries.some((item) => item.id === payload.registryId)) {
      throw new ApiError({
        status: 422,
        code: "VALIDATION_ERROR",
        message: "关联的 Registry 不存在。",
        traceId: traceId(),
        fieldErrors: { registryId: "关联的 Registry 不存在" },
      });
    }
    if (!db.dockerNodes.some((item) => item.id === payload.targetId)) {
      throw new ApiError({
        status: 422,
        code: "VALIDATION_ERROR",
        message: "发布目标不存在。",
        traceId: traceId(),
        fieldErrors: { targetId: "发布目标不存在" },
      });
    }
    assertUniqueName(
      !db.services.some((item) => item.code === payload.code && item.id !== payload.id),
      "服务编码已存在。",
      { code: "服务编码已存在" },
    );

    if (payload.id) {
      const service = getServiceOrThrow(db, payload.id);
      service.name = payload.name.trim();
      service.group = payload.group.trim();
      service.tags = payload.tags;
      service.description = payload.description?.trim();
      service.registryId = payload.registryId;
      service.image = payload.image.trim();
      service.defaultTag = payload.defaultTag.trim() || "latest";
      service.ports = payload.ports;
      service.envs = payload.envs;
      service.mounts = payload.mounts;
      service.resourceLimits = payload.resourceLimits;
      service.targetType = payload.targetType;
      service.targetId = payload.targetId;
      service.status = payload.status;
      service.updatedBy = actor.id;
      service.updatedAt = now();
      appendAudit(db, {
        actor: actor.displayName,
        action: "service.update",
        resourceType: "service",
        resourceId: service.id,
        resourceName: service.code,
        result: "SUCCESS",
        summary: "更新服务定义配置。",
      });
      writeDb(db);
      return delay(service);
    }

    const created: ServiceDefinition = {
      id: crypto.randomUUID(),
      name: payload.name.trim(),
      code: payload.code.trim(),
      group: payload.group.trim(),
      tags: payload.tags,
      description: payload.description?.trim(),
      registryId: payload.registryId,
      image: payload.image.trim(),
      defaultTag: payload.defaultTag.trim() || "latest",
      ports: payload.ports,
      envs: payload.envs,
      mounts: payload.mounts,
      resourceLimits: payload.resourceLimits,
      targetType: payload.targetType,
      targetId: payload.targetId,
      status: payload.status,
      currentVersion: "",
      createdBy: actor.id,
      updatedBy: actor.id,
      createdAt: now(),
      updatedAt: now(),
    };
    db.services.unshift(created);
    appendAudit(db, {
      actor: actor.displayName,
      action: "service.create",
      resourceType: "service",
      resourceId: created.id,
      resourceName: created.code,
      result: "SUCCESS",
      summary: "创建新的服务定义。",
    });
    writeDb(db);
    return delay(created);
  },

  async deleteService(token: string | null, serviceId: string) {
    const db = readDb();
    const actor = requirePermission(token, "services.manage", db);
    const service = getServiceOrThrow(db, serviceId);
    if (db.serviceInstances.some((item) => item.serviceId === serviceId)) {
      throw new ApiError({
        status: 409,
        code: "SERVICE_HAS_INSTANCES",
        message: "服务已有实例，无法删除。",
        traceId: traceId(),
      });
    }
    db.services = db.services.filter((item) => item.id !== serviceId);
    db.serviceVersions = db.serviceVersions.filter((item) => item.serviceId !== serviceId);
    db.serviceReleases = db.serviceReleases.filter((item) => item.serviceId !== serviceId);
    appendAudit(db, {
      actor: actor.displayName,
      action: "service.delete",
      resourceType: "service",
      resourceId: service.id,
      resourceName: service.code,
      result: "SUCCESS",
      summary: "删除服务定义。",
    });
    writeDb(db);
    return delay({ deleted: true });
  },

  async listServiceInstances(token: string | null, serviceId: string) {
    const db = readDb();
    requirePermission(token, "services.view", db);
    getServiceOrThrow(db, serviceId);
    return delay(db.serviceInstances.filter((item) => item.serviceId === serviceId));
  },

  async listServiceReleases(token: string | null, serviceId: string) {
    const db = readDb();
    requirePermission(token, "services.view", db);
    getServiceOrThrow(db, serviceId);
    return delay(db.serviceReleases.filter((item) => item.serviceId === serviceId));
  },

  async listServiceHistory(token: string | null, serviceId: string) {
    const db = readDb();
    requirePermission(token, "services.view", db);
    getServiceOrThrow(db, serviceId);
    return delay(db.serviceReleases.filter((item) => item.serviceId === serviceId));
  },

  async listServiceVersions(token: string | null, serviceId: string) {
    const db = readDb();
    requirePermission(token, "services.view", db);
    getServiceOrThrow(db, serviceId);
    return delay(db.serviceVersions.filter((item) => item.serviceId === serviceId));
  },

  async releaseService(token: string | null, serviceId: string, payload: ServiceReleaseInput): Promise<ServiceReleaseResult> {
    const db = readDb();
    const actor = requirePermission(token, "services.release", db);
    const service = getServiceOrThrow(db, serviceId);
    const versionId = crypto.randomUUID();
    const task = createTask(db, {
      type: "SERVICE_RELEASE",
      target: `service:${service.code}`,
      resourceType: "service",
      resourceId: service.id,
      initiatedBy: actor.displayName,
      summary: `发布服务 ${service.code}:${payload.version || payload.imageTag}`,
      steps: ["校验发布参数", "固化镜像版本", "写入实例状态"],
    });
    finishTask(db, task.id, "服务发布完成。");

    const version = payload.version || payload.imageTag;
    const targetNodeId = payload.targetId || service.targetId;
    db.serviceVersions.unshift({
      id: versionId,
      serviceId,
      version,
      image: service.image,
      imageTag: payload.imageTag,
      imageDigest: payload.imageDigest ?? "",
      config: serializeServiceConfig({
        ports: service.ports,
        envs: service.envs,
        mounts: service.mounts,
        resourceLimits: service.resourceLimits,
      }),
      createdBy: actor.id,
      createdAt: now(),
    });
    db.serviceInstances.unshift({
      id: crypto.randomUUID(),
      serviceId,
      versionId,
      version,
      image: service.image,
      imageTag: payload.imageTag,
      dockerNodeId: targetNodeId,
      containerId: `container-${service.code}-${version}`,
      name: service.code,
      status: "RUNNING",
      lastError: "",
      startedAt: now(),
      createdAt: now(),
      updatedAt: now(),
    });
    const release: ServiceReleaseRecord = {
      id: crypto.randomUUID(),
      serviceId,
      taskId: task.id,
      action: "RELEASE",
      fromVersionId: "",
      fromVersion: service.currentVersion,
      targetVersionId: versionId,
      targetVersion: version,
      status: "SUCCESS",
      message: "完成一次服务发布。",
      createdBy: actor.id,
      createdAt: now(),
      updatedAt: now(),
    };
    db.serviceReleases.unshift(release);
    service.currentVersion = version;
    service.status = "ACTIVE";
    service.targetId = targetNodeId;
    service.updatedBy = actor.id;
    service.updatedAt = now();
    appendAudit(db, {
      actor: actor.displayName,
      action: "service.release",
      resourceType: "service",
      resourceId: service.id,
      resourceName: service.code,
      result: "SUCCESS",
      summary: `发布服务到版本 ${version}。`,
    });
    writeDb(db);
    return delay({ taskId: task.id, releaseId: release.id });
  },

  async upgradeService(token: string | null, serviceId: string, payload: ServiceReleaseInput): Promise<ServiceReleaseResult> {
    const result = await this.releaseService(token, serviceId, payload);
    const db = readDb();
    const release = db.serviceReleases.find((item) => item.id === result.releaseId);
    if (release) {
      release.action = "UPGRADE";
      release.message = "完成一次服务升级。";
      writeDb(db);
    }
    return result;
  },

  async rollbackService(token: string | null, serviceId: string, payload: ServiceRollbackInput): Promise<ServiceReleaseResult> {
    const db = readDb();
    const actor = requirePermission(token, "services.release", db);
    const service = getServiceOrThrow(db, serviceId);
    const targetVersion = db.serviceVersions.find((item) => item.id === payload.versionId && item.serviceId === serviceId);
    if (!targetVersion) {
      throw new ApiError({
        status: 404,
        code: "NOT_FOUND",
        message: "目标版本不存在。",
        traceId: traceId(),
      });
    }
    const task = createTask(db, {
      type: "SERVICE_ROLLBACK",
      target: `service:${service.code}`,
      resourceType: "service",
      resourceId: service.id,
      initiatedBy: actor.displayName,
      summary: `回滚服务 ${service.code} 到 ${targetVersion.version}`,
      steps: ["确认目标版本", "更新实例状态", "记录回滚结果"],
    });
    finishTask(db, task.id, "服务回滚完成。");
    db.serviceInstances.unshift({
      id: crypto.randomUUID(),
      serviceId,
      versionId: targetVersion.id,
      version: targetVersion.version,
      image: targetVersion.image,
      imageTag: targetVersion.imageTag,
      dockerNodeId: service.targetId,
      containerId: `container-${service.code}-${targetVersion.version}-rollback`,
      name: service.code,
      status: "ROLLBACK",
      lastError: "",
      startedAt: now(),
      createdAt: now(),
      updatedAt: now(),
    });
    const release: ServiceReleaseRecord = {
      id: crypto.randomUUID(),
      serviceId,
      taskId: task.id,
      action: "ROLLBACK",
      fromVersionId: "",
      fromVersion: service.currentVersion,
      targetVersionId: targetVersion.id,
      targetVersion: targetVersion.version,
      status: "SUCCESS",
      message: "完成一次服务回滚。",
      createdBy: actor.id,
      createdAt: now(),
      updatedAt: now(),
    };
    db.serviceReleases.unshift(release);
    service.currentVersion = targetVersion.version;
    service.updatedBy = actor.id;
    service.updatedAt = now();
    appendAudit(db, {
      actor: actor.displayName,
      action: "service.rollback",
      resourceType: "service",
      resourceId: service.id,
      resourceName: service.code,
      result: "SUCCESS",
      summary: `回滚服务到版本 ${targetVersion.version}。`,
    });
    writeDb(db);
    return delay({ taskId: task.id, releaseId: release.id });
  },

  async saveRegistry(token: string | null, payload: RegistryInput) {
    const db = readDb();
    const actor = requirePermission(token, "registries.manage", db);
    validateRegistryPayload(payload);
    if (
      payload.authType !== "NONE" &&
      payload.secretId &&
      !db.secrets.some((item) => item.id === payload.secretId)
    ) {
      throw new ApiError({
        status: 422,
        code: "VALIDATION_ERROR",
        message: "绑定的凭证不存在。",
        traceId: traceId(),
        fieldErrors: { secretId: "绑定的凭证不存在" },
      });
    }
    assertUniqueName(
      !db.registries.some((item) => item.name === payload.name && item.id !== payload.id),
      "Registry 名称已存在。",
      { name: "Registry 名称已存在" },
    );

    if (payload.id) {
      const registry = getRegistryOrThrow(db, payload.id);
      bindSecretUsage(db, registry.secretId, registry.name, false);
      registry.name = payload.name.trim();
      registry.url = payload.url.trim();
      registry.authType = payload.authType;
      registry.secretId = payload.authType === "NONE" ? "" : (payload.secretId ?? "").trim();
      registry.description = payload.description?.trim();
      registry.status = "UNKNOWN";
      registry.lastTestAt = undefined;
      registry.updatedBy = actor.id;
      registry.updatedAt = now();
      bindSecretUsage(db, registry.secretId, registry.name, true);
      db.registryCatalogs[registry.id] = db.registryCatalogs[registry.id] ?? {
        repositories: [],
        tags: {},
        manifests: {},
      };
      appendAudit(db, {
        actor: actor.displayName,
        action: "registry.update",
        resourceType: "registry",
        resourceId: registry.id,
        resourceName: registry.name,
        result: "SUCCESS",
        summary: "更新 Registry 配置。",
      });
      writeDb(db);
      return delay(registry);
    }

    const registryId = crypto.randomUUID();
    const created: Registry = {
      id: registryId,
      name: payload.name.trim(),
      url: payload.url.trim(),
      authType: payload.authType,
      secretId: payload.authType === "NONE" ? "" : (payload.secretId ?? "").trim(),
      description: payload.description?.trim(),
      status: "UNKNOWN",
      createdBy: actor.id,
      updatedBy: actor.id,
      createdAt: now(),
      updatedAt: now(),
    };
    db.registries.unshift(created);
    db.registryCatalogs[registryId] = {
      repositories: [],
      tags: {},
      manifests: {},
    };
    bindSecretUsage(db, created.secretId, created.name, true);
    appendAudit(db, {
      actor: actor.displayName,
      action: "registry.create",
      resourceType: "registry",
      resourceId: created.id,
      resourceName: created.name,
      result: "SUCCESS",
      summary: "创建新的 Registry。",
    });
    writeDb(db);
    return delay(created);
  },

  async deleteRegistry(token: string | null, registryId: string) {
    const db = readDb();
    const actor = requirePermission(token, "registries.manage", db);
    const registry = getRegistryOrThrow(db, registryId);
    bindSecretUsage(db, registry.secretId, registry.name, false);
    db.registries = db.registries.filter((item) => item.id !== registryId);
    delete db.registryCatalogs[registryId];
    appendAudit(db, {
      actor: actor.displayName,
      action: "registry.delete",
      resourceType: "registry",
      resourceId: registry.id,
      resourceName: registry.name,
      result: "SUCCESS",
      summary: "删除 Registry。",
    });
    writeDb(db);
    return delay({ deleted: true });
  },

  async testRegistry(token: string | null, registryId: string) {
    const db = readDb();
    const actor = requirePermission(token, "registries.manage", db);
    const registry = getRegistryOrThrow(db, registryId);
    const secret = registry.secretId ? db.secrets.find((item) => item.id === registry.secretId) : undefined;
    let errorMessage = "";
    if (registry.url.includes("offline") || registry.url.includes("timeout")) {
      errorMessage = "registry network error: dial tcp timeout";
    } else if (registry.authType !== "NONE" && !secret) {
      errorMessage = "registry authentication failed: bound secret not found";
    } else if (
      registry.authType === "BASIC" &&
      secret &&
      (!secret.secretValue.includes(":") || secret.secretValue.startsWith("invalid"))
    ) {
      errorMessage = "registry authentication failed: basic secret must use username:password format";
    }

    registry.lastTestAt = now();
    registry.updatedBy = actor.id;
    registry.updatedAt = registry.lastTestAt;

    if (errorMessage) {
      registry.status = "OFFLINE";
      appendAudit(db, {
        actor: actor.displayName,
        action: "registry.test",
        resourceType: "registry",
        resourceId: registry.id,
        resourceName: registry.name,
        result: "FAILED",
        summary: errorMessage,
      });
      writeDb(db);
      const matched = matchRegistryError(errorMessage);
      throw new ApiError({
        status: matched.status,
        code: matched.code,
        message: matched.message,
        traceId: traceId(),
      });
    }

    registry.status = "ONLINE";
    appendAudit(db, {
      actor: actor.displayName,
      action: "registry.test",
      resourceType: "registry",
      resourceId: registry.id,
      resourceName: registry.name,
      result: "SUCCESS",
      summary: "执行 Registry 连通性测试并返回成功。",
    });
    writeDb(db);
    return delay({ connected: true });
  },

  async listRegistryRepositories(token: string | null, registryId: string): Promise<RegistryRepositoriesResult> {
    const db = readDb();
    requirePermission(token, "registries.view", db);
    getRegistryOrThrow(db, registryId);
    const catalog = getRegistryCatalogOrThrow(db, registryId);
    return delay({ repositories: [...catalog.repositories] });
  },

  async listRegistryTags(
    token: string | null,
    registryId: string,
    repository: string,
  ): Promise<RegistryTagsResult> {
    const db = readDb();
    requirePermission(token, "registries.view", db);
    getRegistryOrThrow(db, registryId);
    const catalog = getRegistryCatalogOrThrow(db, registryId);
    const tags = catalog.tags[repository];
    if (!tags) {
      const matched = matchRegistryError("registry resource not found: repository not found");
      throw new ApiError({
        status: matched.status,
        code: matched.code,
        message: matched.message,
        traceId: traceId(),
      });
    }
    return delay({
      name: repository,
      tags: [...tags],
    });
  },

  async getRegistryManifest(
    token: string | null,
    registryId: string,
    repository: string,
    reference: string,
  ): Promise<RegistryManifestResult> {
    const db = readDb();
    requirePermission(token, "registries.view", db);
    getRegistryOrThrow(db, registryId);
    const catalog = getRegistryCatalogOrThrow(db, registryId);
    const manifest = catalog.manifests[repository]?.[reference];
    if (!manifest) {
      const matched = matchRegistryError("registry resource not found: manifest not found");
      throw new ApiError({
        status: matched.status,
        code: matched.code,
        message: matched.message,
        traceId: traceId(),
      });
    }
    return delay({
      repository,
      reference,
      digest: manifest.digest,
      contentType: manifest.contentType,
      manifest: manifest.manifest,
    });
  },

  async testHostSsh(token: string | null, hostId: string) {
    const db = readDb();
    const actor = requirePermission(token, "hosts.manage", db);
    const host = db.hosts.find((item) => item.id === hostId);
    if (!host) {
      throw new ApiError({
        status: 404,
        code: "NOT_FOUND",
        message: "主机不存在。",
        traceId: traceId(),
      });
    }
    host.status = "TESTING";
    const task = createTask(db, {
      type: "SSH_TEST",
      target: host.name,
      resourceType: "host",
      resourceId: host.id,
      initiatedBy: actor.displayName,
      summary: `测试主机 ${host.name} 的 SSH 连通性`,
      steps: ["建立连接", "认证凭证", "等待远端响应"],
    });
    writeDb(db);
    window.setTimeout(() => {
      const next = readDb();
      const currentHost = next.hosts.find((item) => item.id === hostId);
      if (currentHost) {
        currentHost.status = "HEALTHY";
        currentHost.lastCheckedAt = now();
      }
      finishTask(next, task.id, "SSH 测试通过，远端主机可连接。");
      appendAudit(next, {
        actor: actor.displayName,
        action: "host.test_ssh",
        resourceType: "host",
        resourceId: host.id,
        resourceName: host.name,
        result: "SUCCESS",
        summary: "执行 SSH 测试并返回成功。",
      });
      writeDb(next);
    }, TASK_DELAY);
    return delay(task);
  },

  async createTerminalSession(token: string | null, hostId: string) {
    const db = readDb();
    const actor = requirePermission(token, "terminal.open", db);
    const host = db.hosts.find((item) => item.id === hostId);
    if (!host) {
      throw new ApiError({
        status: 404,
        code: "NOT_FOUND",
        message: "目标主机不存在。",
        traceId: traceId(),
      });
    }
    const session: TerminalSession = {
      id: crypto.randomUUID(),
      hostId: host.id,
      hostName: host.name,
      status: "CONNECTED",
      createdAt: now(),
      welcomeLines: [
        `Connected to ${host.name} (${host.address}:${host.port})`,
        `Welcome ${actor.displayName}, session trace ${traceId().slice(0, 8)}`,
        "Type `help` to inspect demo commands.",
      ],
    };
    db.terminalSessions.unshift(session);
    appendAudit(db, {
      actor: actor.displayName,
      action: "terminal.open",
      resourceType: "host",
      resourceId: host.id,
      resourceName: host.name,
      result: "SUCCESS",
      summary: "创建 WebSSH 会话。",
    });
    writeDb(db);
    return delay(session);
  },

  async getTerminalSession(token: string | null, sessionId: string) {
    const db = readDb();
    requirePermission(token, "terminal.open", db);
    const session = db.terminalSessions.find((item) => item.id === sessionId);
    if (!session) {
      throw new ApiError({
        status: 404,
        code: "NOT_FOUND",
        message: "终端会话不存在或已关闭。",
        traceId: traceId(),
      });
    }
    return delay(session);
  },

  async listDockerNodes(token: string | null) {
    const db = readDb();
    requirePermission(token, "docker.view", db);
    return delay(db.dockerNodes);
  },

  async saveDockerNode(token: string | null, payload: DockerNodeInput) {
    const db = readDb();
    const actor = requirePermission(token, "docker.manage", db);
    if (payload.id) {
      const node = db.dockerNodes.find((item) => item.id === payload.id);
      if (!node) {
        throw new ApiError({
          status: 404,
          code: "NOT_FOUND",
          message: "Docker 节点不存在。",
          traceId: traceId(),
        });
      }
      node.name = payload.name;
      node.endpoint = payload.endpoint;
      node.tlsEnabled = payload.tlsEnabled;
      node.description = payload.description;
      appendAudit(db, {
        actor: actor.displayName,
        action: "docker.node.update",
        resourceType: "docker-node",
        resourceId: node.id,
        resourceName: node.name,
        result: "SUCCESS",
        summary: "更新 Docker 节点配置。",
      });
      writeDb(db);
      return delay(node);
    }
    const node: DockerNode = {
      id: crypto.randomUUID(),
      name: payload.name,
      endpoint: payload.endpoint,
      tlsEnabled: payload.tlsEnabled,
      description: payload.description,
      status: "UNKNOWN",
      containerCount: 0,
    };
    db.dockerNodes.unshift(node);
    appendAudit(db, {
      actor: actor.displayName,
      action: "docker.node.create",
      resourceType: "docker-node",
      resourceId: node.id,
      resourceName: node.name,
      result: "SUCCESS",
      summary: "新建 Docker 节点。",
    });
    writeDb(db);
    return delay(node);
  },

  async testDockerNode(token: string | null, nodeId: string) {
    const db = readDb();
    const actor = requirePermission(token, "docker.manage", db);
    const node = db.dockerNodes.find((item) => item.id === nodeId);
    if (!node) {
      throw new ApiError({
        status: 404,
        code: "NOT_FOUND",
        message: "Docker 节点不存在。",
        traceId: traceId(),
      });
    }
    node.status = "TESTING";
    const task = createTask(db, {
      type: "DOCKER_NODE_TEST",
      target: node.name,
      resourceType: "docker-node",
      resourceId: node.id,
      initiatedBy: actor.displayName,
      summary: `测试 Docker 节点 ${node.name} 连接`,
      steps: ["建立 TLS 通道", "查询节点信息", "检查容器清单"],
    });
    writeDb(db);
    window.setTimeout(() => {
      const next = readDb();
      const currentNode = next.dockerNodes.find((item) => item.id === nodeId);
      if (currentNode) {
        currentNode.status = "ONLINE";
        currentNode.lastCheckedAt = now();
        currentNode.containerCount = next.containers.filter((item) => item.nodeId === nodeId).length;
      }
      finishTask(next, task.id, "Docker 节点连通性验证通过。");
      appendAudit(next, {
        actor: actor.displayName,
        action: "docker.node.test",
        resourceType: "docker-node",
        resourceId: node.id,
        resourceName: node.name,
        result: "SUCCESS",
        summary: "Docker 节点测试成功。",
      });
      writeDb(next);
    }, TASK_DELAY);
    return delay(task);
  },

  async getDockerNode(token: string | null, nodeId: string) {
    const db = readDb();
    requirePermission(token, "docker.view", db);
    const node = db.dockerNodes.find((item) => item.id === nodeId);
    if (!node) {
      throw new ApiError({
        status: 404,
        code: "NOT_FOUND",
        message: "Docker 节点不存在。",
        traceId: traceId(),
      });
    }
    return delay(node);
  },

  async listContainers(token: string | null, nodeId: string) {
    const db = readDb();
    requirePermission(token, "docker.view", db);
    return delay(db.containers.filter((item) => item.nodeId === nodeId));
  },

  async getContainerLogs(token: string | null, containerId: string) {
    const db = readDb();
    requirePermission(token, "docker.view", db);
    const container = db.containers.find((item) => item.id === containerId);
    if (!container) {
      throw new ApiError({
        status: 404,
        code: "NOT_FOUND",
        message: "容器不存在。",
        traceId: traceId(),
      });
    }
    return delay(getContainerLogs(container));
  },

  async performContainerAction(
    token: string | null,
    nodeId: string,
    containerId: string,
    action: "start" | "stop" | "restart",
  ) {
    const db = readDb();
    const actor = requirePermission(token, "docker.manage", db);
    const node = db.dockerNodes.find((item) => item.id === nodeId);
    const container = db.containers.find((item) => item.id === containerId && item.nodeId === nodeId);
    if (!node || !container) {
      throw new ApiError({
        status: 404,
        code: "NOT_FOUND",
        message: "容器或节点不存在。",
        traceId: traceId(),
      });
    }
    const actionLabel = {
      start: "启动",
      stop: "停止",
      restart: "重启",
    }[action];
    const task = createTask(db, {
      type: `CONTAINER_${action.toUpperCase()}`,
      target: container.name,
      resourceType: "container",
      resourceId: container.id,
      initiatedBy: actor.displayName,
      summary: `${actionLabel}容器 ${container.name}`,
      steps: ["检查容器状态", `执行${actionLabel}命令`, "确认新状态"],
    });
    writeDb(db);
    window.setTimeout(() => {
      const next = readDb();
      const current = next.containers.find((item) => item.id === containerId && item.nodeId === nodeId);
      if (current) {
        if (action === "start") {
          current.status = "running";
        }
        if (action === "stop") {
          current.status = "exited";
        }
        if (action === "restart") {
          current.status = "running";
          current.restartCount += 1;
        }
      }
      finishTask(next, task.id, `${actionLabel}动作已完成。`);
      appendAudit(next, {
        actor: actor.displayName,
        action: `docker.container.${action}`,
        resourceType: "container",
        resourceId: container.id,
        resourceName: container.name,
        result: "SUCCESS",
        summary: `${actionLabel}容器操作执行完成。`,
      });
      writeDb(next);
    }, TASK_DELAY);
    return delay(task);
  },

  async listTasks(
    token: string | null,
    filters?: {
      status?: string;
      keyword?: string;
      resourceType?: string;
      resourceId?: string;
    },
  ) {
    const db = readDb();
    requirePermission(token, "tasks.view", db);
    let items = [...db.tasks];
    if (filters?.status) {
      items = items.filter((task) => task.status === filters.status);
    }
    if (filters?.resourceType) {
      items = items.filter((task) => matchesResourceType(task.resourceType, filters.resourceType));
    }
    if (filters?.resourceId) {
      items = items.filter((task) =>
        matchesResourceId(`${task.resourceId ?? ""} ${task.target} ${task.summary ?? ""}`, filters.resourceId),
      );
    }
    items = filterByKeyword(items, filters?.keyword ?? "", (task) => {
      return `${task.type} ${task.target} ${task.initiatedBy} ${task.summary ?? ""} ${task.resourceId ?? ""}`;
    });
    return delay(items);
  },

  async getTask(token: string | null, taskId: string) {
    const db = readDb();
    requirePermission(token, "tasks.view", db);
    const task = db.tasks.find((item) => item.id === taskId);
    if (!task) {
      throw new ApiError({
        status: 404,
        code: "NOT_FOUND",
        message: "任务不存在。",
        traceId: traceId(),
      });
    }
    return delay(task);
  },

  async listAudits(
    token: string | null,
    filters?: {
      username?: string;
      action?: string;
      resourceType?: string;
      resourceId?: string;
      result?: string;
      keyword?: string;
    },
  ) {
    const db = readDb();
    requirePermission(token, "audits.view", db);
    let items = [...db.audits];
    if (filters?.username) {
      items = items.filter((audit) => audit.actor.toLowerCase().includes(filters.username!.trim().toLowerCase()));
    }
    if (filters?.action) {
      items = items.filter((audit) => audit.action.toLowerCase().includes(filters.action!.trim().toLowerCase()));
    }
    if (filters?.resourceType) {
      items = items.filter((audit) => matchesResourceType(audit.resourceType, filters.resourceType));
    }
    if (filters?.resourceId) {
      items = items.filter((audit) =>
        matchesResourceId(
          `${audit.resourceId ?? ""} ${audit.resourceName} ${audit.summary} ${audit.traceId}`,
          filters.resourceId,
        ),
      );
    }
    if (filters?.result) {
      items = items.filter((audit) => audit.result === filters.result);
    }
    items = filterByKeyword(items, filters?.keyword ?? "", (audit) => {
      return `${audit.actor} ${audit.action} ${audit.resourceName} ${audit.summary} ${audit.traceId} ${audit.resourceId ?? ""}`;
    });
    return delay(items);
  },
};
