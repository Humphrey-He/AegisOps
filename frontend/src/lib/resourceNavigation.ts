import type { AuditLog, Task } from "../types/models";

type ResourceFilter = {
  resourceType?: string;
  resourceId?: string;
};

type PathFilter = ResourceFilter & {
  keyword?: string;
  status?: string;
  result?: string;
};

type AlertPathFilter = ResourceFilter & {
  status?: string;
  eventType?: string;
  selected?: string;
};

function setIfPresent(params: URLSearchParams, key: string, value?: string) {
  if (value) {
    params.set(key, value);
  }
}

export function normalizeResourceType(value?: string) {
  return (value ?? "")
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^_+|_+$/g, "");
}

export function getResourceTypeLabel(resourceType?: string) {
  switch (normalizeResourceType(resourceType)) {
    case "host":
      return "主机";
    case "docker_node":
      return "Docker 节点";
    case "nginx_node":
      return "Nginx 节点";
    case "notification_channel":
      return "通知通道";
    case "alert_rule":
      return "告警规则";
    case "alert_event":
      return "告警事件";
    case "container":
      return "容器";
    case "registry":
      return "Registry";
    case "service":
      return "服务";
    case "secret":
      return "凭证";
    case "user":
      return "用户";
    case "role":
      return "角色";
    case "auth":
      return "认证";
    case "system":
      return "系统";
    default:
      return resourceType || "资源";
  }
}

export function formatAuditActor(actor?: string) {
  return actor?.trim() || "系统";
}

export function formatAuditSummary(summary?: string, action?: string) {
  return summary?.trim() || action?.trim() || "--";
}

export function formatAuditResourceName(audit: Pick<AuditLog, "resourceType" | "resourceId" | "resourceName">) {
  if (audit.resourceName?.trim()) {
    return audit.resourceName.trim();
  }
  if (audit.resourceId?.trim()) {
    return audit.resourceId.trim();
  }
  return getResourceTypeLabel(audit.resourceType);
}

export function formatTaskResourceName(task: Pick<Task, "resourceType" | "resourceId" | "target">) {
  if (task.resourceId?.trim()) {
    return `${getResourceTypeLabel(task.resourceType)} · ${task.resourceId.trim()}`;
  }
  return task.target?.trim() || "--";
}

export function buildTasksPath(filters: PathFilter = {}) {
  const params = new URLSearchParams();
  setIfPresent(params, "keyword", filters.keyword);
  setIfPresent(params, "status", filters.status);
  setIfPresent(params, "resourceType", filters.resourceType);
  setIfPresent(params, "resourceId", filters.resourceId);
  const query = params.toString();
  return query ? `/tasks?${query}` : "/tasks";
}

export function buildAuditsPath(filters: PathFilter = {}) {
  const params = new URLSearchParams();
  setIfPresent(params, "keyword", filters.keyword);
  setIfPresent(params, "result", filters.result);
  setIfPresent(params, "resourceType", filters.resourceType);
  setIfPresent(params, "resourceId", filters.resourceId);
  const query = params.toString();
  return query ? `/audits?${query}` : "/audits";
}

export function buildAlertEventsPath(filters: AlertPathFilter = {}) {
  const params = new URLSearchParams();
  setIfPresent(params, "status", filters.status);
  setIfPresent(params, "eventType", filters.eventType);
  setIfPresent(params, "resourceType", filters.resourceType);
  setIfPresent(params, "resourceId", filters.resourceId);
  setIfPresent(params, "selected", filters.selected);
  const query = params.toString();
  return query ? `/alerts/events?${query}` : "/alerts/events";
}

export function buildResourcePath(resourceType?: string, resourceId?: string) {
  const normalized = normalizeResourceType(resourceType);
  switch (normalized) {
    case "host":
      return resourceId ? `/assets/hosts?selected=${encodeURIComponent(resourceId)}` : "/assets/hosts";
    case "docker_node":
      return resourceId ? `/docker/nodes?selected=${encodeURIComponent(resourceId)}` : "/docker/nodes";
    case "nginx_node":
      return resourceId ? `/nginx/nodes?selected=${encodeURIComponent(resourceId)}` : "/nginx/nodes";
    case "registry":
      return resourceId ? `/delivery/registries?selected=${encodeURIComponent(resourceId)}` : "/delivery/registries";
    case "service":
      return resourceId ? `/delivery/services?selected=${encodeURIComponent(resourceId)}` : "/delivery/services";
    case "notification_channel":
      return resourceId ? `/settings/notifications?selected=${encodeURIComponent(resourceId)}` : "/settings/notifications";
    case "alert_rule":
      return resourceId ? `/settings/alert-rules?selected=${encodeURIComponent(resourceId)}` : "/settings/alert-rules";
    case "alert_event":
      return resourceId ? `/alerts/events?selected=${encodeURIComponent(resourceId)}` : "/alerts/events";
    case "secret":
      return "/assets/secrets";
    case "user":
      return "/system/users";
    case "role":
      return "/system/roles";
    case "container":
      return "/docker/nodes";
    default:
      return null;
  }
}

export function taskMatchesResource(
  task: Pick<Task, "resourceType" | "resourceId" | "target" | "summary">,
  resourceType: string,
  resourceId: string,
  aliases: string[] = [],
) {
  const normalizedTaskType = normalizeResourceType(task.resourceType);
  const normalizedTargetType = normalizeResourceType(resourceType);
  if (task.resourceId && task.resourceId === resourceId) {
    return !normalizedTaskType || normalizedTaskType === normalizedTargetType;
  }
  const haystack = `${task.target} ${task.summary ?? ""} ${task.resourceId ?? ""}`.toLowerCase();
  return [resourceId, ...aliases].some((item) => item && haystack.includes(item.toLowerCase()));
}

export function auditMatchesResource(
  audit: Pick<AuditLog, "resourceType" | "resourceId" | "resourceName" | "summary">,
  resourceType: string,
  resourceId: string,
  aliases: string[] = [],
) {
  const normalizedAuditType = normalizeResourceType(audit.resourceType);
  const normalizedTargetType = normalizeResourceType(resourceType);
  if (audit.resourceId && audit.resourceId === resourceId) {
    return !normalizedAuditType || normalizedAuditType === normalizedTargetType;
  }
  const haystack = `${audit.resourceId ?? ""} ${audit.resourceName} ${audit.summary}`.toLowerCase();
  return [resourceId, ...aliases].some((item) => item && haystack.includes(item.toLowerCase()));
}
