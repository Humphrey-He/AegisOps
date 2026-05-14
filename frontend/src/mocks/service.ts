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
  Secret,
  SecretInputPayload,
  Task,
  TaskStep,
  TerminalSession,
  User,
  UserInput,
} from "../types/models";

const STORAGE_KEY = "aegisops-mvp-db";
const RESPONSE_DELAY = 260;
const TASK_DELAY = 1600;

type StoredUser = User & {
  password: string;
};

type StoredSecret = Secret & {
  secretValue: string;
};

type MockDb = {
  users: StoredUser[];
  roles: Role[];
  secrets: StoredSecret[];
  hosts: Host[];
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

function maskSecret(secretValue: string) {
  if (secretValue.length <= 8) {
    return "*".repeat(secretValue.length);
  }
  return `${secretValue.slice(0, 2)}${"*".repeat(Math.max(4, secretValue.length - 4))}${secretValue.slice(-2)}`;
}

function createStoredDb(): MockDb {
  return {
    users: [],
    roles: createDefaultRoles(),
    secrets: [],
    hosts: [],
    dockerNodes: [],
    containers: [],
    tasks: [],
    audits: [],
    terminalSessions: [],
  };
}

function readDb(): MockDb {
  const raw = window.localStorage.getItem(STORAGE_KEY);
  if (!raw) {
    const initial = createStoredDb();
    writeDb(initial);
    return initial;
  }

  try {
    return JSON.parse(raw) as MockDb;
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
  payload: Pick<Task, "type" | "target" | "initiatedBy" | "summary"> & { steps: string[] },
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

  const task = createTask(db, {
    type: "DOCKER_RESTART",
    target: "gateway-nginx",
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
}

function filterByKeyword<T>(items: T[], keyword: string, mapper: (item: T) => string) {
  if (!keyword) {
    return items;
  }
  const normalized = keyword.trim().toLowerCase();
  return items.filter((item) => mapper(item).toLowerCase().includes(normalized));
}

function getContainerLogs(containerName: string) {
  return [
    `[${now()}] INFO Boot sequence ready for ${containerName}`,
    `[${now()}] INFO Listening on 0.0.0.0`,
    `[${now()}] WARN Health probe latency 220ms`,
    `[${now()}] INFO Last deploy completed`,
  ];
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
      resourceName: host.name,
      result: "SUCCESS",
      summary: "创建新的主机资产。",
    });
    writeDb(db);
    return delay(host);
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
    return delay(getContainerLogs(container.name));
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
        resourceName: container.name,
        result: "SUCCESS",
        summary: `${actionLabel}容器操作执行完成。`,
      });
      writeDb(next);
    }, TASK_DELAY);
    return delay(task);
  },

  async listTasks(token: string | null) {
    const db = readDb();
    requirePermission(token, "tasks.view", db);
    return delay(db.tasks);
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

  async listAudits(token: string | null) {
    const db = readDb();
    requirePermission(token, "audits.view", db);
    return delay(db.audits);
  },
};
