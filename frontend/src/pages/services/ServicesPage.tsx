import {
  Alert,
  App as AntApp,
  Button,
  Card,
  Empty,
  Form,
  Input,
  InputNumber,
  Select,
  Space,
  Tag,
  Typography,
} from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { DangerConfirm } from "../../components/DangerConfirm";
import { DataTable } from "../../components/DataTable";
import { EmptyState } from "../../components/EmptyState";
import { ErrorState } from "../../components/ErrorState";
import { FormDrawer } from "../../components/FormDrawer";
import { PageHeader } from "../../components/PageHeader";
import { PermissionActionButton } from "../../components/PermissionActionButton";
import { PermissionGuard } from "../../components/PermissionGuard";
import { StatusBadge } from "../../components/StatusBadge";
import { TaskStatus } from "../../components/TaskStatus";
import { ResourceActivityList } from "../../components/resource/ResourceActivityList";
import { ResourceDetailPanel } from "../../components/resource/ResourceDetailPanel";
import { alertsApi, auditsApi, dockerApi, exportsApi, registriesApi, servicesApi, tasksApi } from "../../lib/api";
import { applyFormErrors, getErrorMessage } from "../../lib/forms";
import { formatDateTime } from "../../lib/format";
import { queryKeys } from "../../lib/queryKeys";
import { auditMatchesResource, buildAuditsPath, buildTasksPath, taskMatchesResource } from "../../lib/resourceNavigation";
import { useSessionStore } from "../../store/sessionStore";
import type {
  AlertEvent,
  DockerNode,
  Registry,
  ServiceDefinition,
  ServiceDefinitionInput,
  ServiceEnvVar,
  ServiceHealthCheck,
  ServiceMount,
  ServicePort,
  ServiceReleaseRecord,
  ServiceReleaseInput,
  ServiceVersion,
} from "../../types/models";

type ServiceFormValues = {
  name: string;
  code: string;
  group: string;
  tags: string[];
  description?: string;
  registryId: string;
  image: string;
  defaultTag: string;
  status: ServiceDefinition["status"];
  targetId: string;
  ports: ServicePort[];
  envs: ServiceEnvVar[];
  mounts: ServiceMount[];
  cpu?: string;
  memory?: string;
};

type ReleaseFormValues = {
  version: string;
  imageTag: string;
  imageDigest?: string;
  targetId?: string;
};

type ReleaseMode = "release" | "upgrade" | "rollback";

type ValidationIssue = {
  name: (string | number)[];
  errors: string[];
};

const statusOptions: Array<{ label: string; value: ServiceDefinition["status"] }> = [
  { label: "草稿", value: "DRAFT" },
  { label: "启用", value: "ACTIVE" },
  { label: "归档", value: "ARCHIVED" },
];

const targetTypeLabelMap: Record<ServiceDefinition["targetType"], string> = {
  DOCKER_NODE: "Docker 节点",
};

const releaseActionLabelMap = {
  RELEASE: "发布",
  UPGRADE: "升级",
  ROLLBACK: "回滚",
} as const;

const healthStrategyLabelMap: Record<ServiceHealthCheck["strategyType"], string> = {
  HTTP: "HTTP",
  TCP: "TCP",
  COMMAND: "命令",
};

const serviceHealthStatusMetaMap: Record<
  ServiceHealthCheck["status"],
  { color: string; label: string }
> = {
  PENDING: { color: "default", label: "待执行" },
  RUNNING: { color: "blue", label: "执行中" },
  SUCCESS: { color: "green", label: "通过" },
  FAILED: { color: "red", label: "失败" },
};

const notificationStatusMetaMap = {
  PENDING: { color: "gold", label: "待发送" },
  SUCCESS: { color: "green", label: "已发送" },
  FAILED: { color: "red", label: "发送失败" },
} as const;

const alertEventLabelMap: Record<AlertEvent["eventType"], string> = {
  service_release_failed: "服务发布失败",
  service_health_check_failed: "健康检查失败",
  nginx_reload_failed: "Nginx 重载失败",
  nginx_publish_failed: "Nginx 配置发布失败",
  host_offline: "主机离线",
  host_recovered: "主机恢复",
};

const alertSeverityMetaMap: Record<AlertEvent["severity"], { color: string; label: string }> = {
  INFO: { color: "default", label: "信息" },
  WARN: { color: "gold", label: "警告" },
  WARNING: { color: "gold", label: "警告" },
  CRITICAL: { color: "red", label: "严重" },
};

function buildServiceFormValues(service?: ServiceDefinition | null): ServiceFormValues {
  return {
    name: service?.name ?? "",
    code: service?.code ?? "",
    group: service?.group ?? "",
    tags: service?.tags ?? [],
    description: service?.description ?? "",
    registryId: service?.registryId ?? "",
    image: service?.image ?? "",
    defaultTag: service?.defaultTag ?? "latest",
    status: service?.status ?? "DRAFT",
    targetId: service?.targetId ?? "",
    ports: service?.ports?.length ? service.ports : [{ name: "http", containerPort: 8080, protocol: "TCP" }],
    envs: service?.envs?.length ? service.envs : [{ key: "GIN_MODE", value: "release" }],
    mounts: service?.mounts?.length ? service.mounts : [],
    cpu: service?.resourceLimits.cpu ?? "",
    memory: service?.resourceLimits.memory ?? "",
  };
}

function buildReleaseFormValues(service: ServiceDefinition | null): ReleaseFormValues {
  return {
    version: "",
    imageTag: service?.defaultTag ?? "latest",
    imageDigest: "",
    targetId: service?.targetId ?? "",
  };
}

function versionsForRollbackOptions(versions: ServiceVersion[]) {
  return versions.map((version) => ({
    label: `${version.version} · ${version.imageTag}`,
    value: version.id,
  }));
}

function normalizeValue(value?: string) {
  return value?.trim() ?? "";
}

function truncateMiddle(value?: string, head = 14, tail = 8) {
  const text = normalizeValue(value);
  if (!text) {
    return "--";
  }
  if (text.length <= head + tail + 3) {
    return text;
  }
  return `${text.slice(0, head)}...${text.slice(-tail)}`;
}

function truncateText(value?: string, maxLength = 88) {
  const text = normalizeValue(value);
  if (!text) {
    return "--";
  }
  if (text.length <= maxLength) {
    return text;
  }
  return `${text.slice(0, maxLength)}...`;
}

function getReleaseTargetSummary(record: ServiceReleaseRecord) {
  if (record.fromVersion && record.targetVersion && record.fromVersion !== record.targetVersion) {
    return `${record.fromVersion} -> ${record.targetVersion}`;
  }
  return record.targetVersion || record.fromVersion || "--";
}

function getReleaseOutcomeSummary(record: ServiceReleaseRecord) {
  if (normalizeValue(record.failureSummary)) {
    return record.failureSummary;
  }
  if (normalizeValue(record.message)) {
    return record.message;
  }
  if (record.rollbackSuggested && record.suggestedRollbackVersion) {
    return `建议回滚到 ${record.suggestedRollbackVersion}`;
  }
  return "--";
}

function getHealthCheckSummary(record: ServiceHealthCheck) {
  if (normalizeValue(record.errorMessage)) {
    return record.errorMessage;
  }
  if (normalizeValue(record.output)) {
    return record.output;
  }
  const details = [];
  if (record.httpStatus) {
    details.push(`HTTP ${record.httpStatus}`);
  }
  if (record.latencyMs) {
    details.push(`${record.latencyMs} ms`);
  }
  return details.length ? details.join(" · ") : "--";
}

function getAlertSummary(event: AlertEvent) {
  if (normalizeValue(event.detail)) {
    return event.detail;
  }
  return event.summary || "--";
}

function mergeValidationIssues(issues: ValidationIssue[]) {
  const merged = new Map<string, ValidationIssue>();
  issues.forEach((issue) => {
    const key = JSON.stringify(issue.name);
    const existing = merged.get(key);
    if (existing) {
      existing.errors = Array.from(new Set([...existing.errors, ...issue.errors]));
      return;
    }
    merged.set(key, {
      name: issue.name,
      errors: Array.from(new Set(issue.errors)),
    });
  });
  return Array.from(merged.values());
}

function applyServiceValidationIssues(
  form: ReturnType<typeof Form.useForm<ServiceFormValues>>[0],
  values: ServiceFormValues,
  issues: ValidationIssue[],
) {
  const clearFields: Array<(string | number)[]> = [];
  (values.ports ?? []).forEach((_, index) => {
    clearFields.push(["ports", index, "name"]);
    clearFields.push(["ports", index, "containerPort"]);
    clearFields.push(["ports", index, "hostPort"]);
  });
  (values.envs ?? []).forEach((_, index) => {
    clearFields.push(["envs", index, "key"]);
    clearFields.push(["envs", index, "value"]);
  });
  (values.mounts ?? []).forEach((_, index) => {
    clearFields.push(["mounts", index, "source"]);
    clearFields.push(["mounts", index, "target"]);
  });
  form.setFields([
    ...clearFields.map((name) => ({ name: name as never, errors: [] })),
    ...issues.map((issue) => ({ name: issue.name as never, errors: issue.errors })),
  ] as never);
}

function validateServiceDefinition(values: ServiceFormValues) {
  const issues: ValidationIssue[] = [];
  const portNameMap = new Map<string, number>();
  const containerPortMap = new Map<string, number>();
  const hostPortMap = new Map<string, number>();

  (values.ports ?? []).forEach((port, index) => {
    const portName = normalizeValue(port.name);
    const protocol = (port.protocol ?? "TCP").toUpperCase();
    const hasAnyValue = Boolean(portName || port.containerPort || port.hostPort);
    if (!hasAnyValue) {
      return;
    }
    if (!portName) {
      issues.push({ name: ["ports", index, "name"], errors: ["请输入端口名称"] });
    }
    if (!port.containerPort) {
      issues.push({ name: ["ports", index, "containerPort"], errors: ["请输入容器端口"] });
    }
    if (portName) {
      const nameKey = portName.toLowerCase();
      if (portNameMap.has(nameKey)) {
        issues.push({ name: ["ports", index, "name"], errors: ["端口名称重复"] });
      } else {
        portNameMap.set(nameKey, index);
      }
    }
    if (port.containerPort) {
      const containerKey = `${protocol}:${port.containerPort}`;
      if (containerPortMap.has(containerKey)) {
        issues.push({ name: ["ports", index, "containerPort"], errors: ["容器端口与协议组合重复"] });
      } else {
        containerPortMap.set(containerKey, index);
      }
    }
    if (port.hostPort) {
      const hostKey = `${protocol}:${port.hostPort}`;
      if (hostPortMap.has(hostKey)) {
        issues.push({ name: ["ports", index, "hostPort"], errors: ["主机端口与协议组合重复"] });
      } else {
        hostPortMap.set(hostKey, index);
      }
    }
  });

  const envKeyMap = new Map<string, number>();
  (values.envs ?? []).forEach((env, index) => {
    const key = normalizeValue(env.key);
    const value = normalizeValue(env.value);
    if (!key && !value) {
      return;
    }
    if (!key) {
      issues.push({ name: ["envs", index, "key"], errors: ["请输入环境变量 Key"] });
      return;
    }
    const mapKey = key.toUpperCase();
    if (envKeyMap.has(mapKey)) {
      issues.push({ name: ["envs", index, "key"], errors: ["环境变量 Key 重复"] });
    } else {
      envKeyMap.set(mapKey, index);
    }
  });

  (values.mounts ?? []).forEach((mount, index) => {
    const source = normalizeValue(mount.source);
    const target = normalizeValue(mount.target);
    if (!source && !target) {
      return;
    }
    if (!source) {
      issues.push({ name: ["mounts", index, "source"], errors: ["请输入挂载源"] });
    }
    if (!target) {
      issues.push({ name: ["mounts", index, "target"], errors: ["请输入挂载目标"] });
    }
    if (source.includes(" ")) {
      issues.push({ name: ["mounts", index, "source"], errors: ["挂载源不能包含空格"] });
    }
    if (target.includes(" ")) {
      issues.push({ name: ["mounts", index, "target"], errors: ["挂载目标不能包含空格"] });
    }
    if (target && !target.startsWith("/")) {
      issues.push({ name: ["mounts", index, "target"], errors: ["挂载目标需使用绝对路径"] });
    }
  });

  return mergeValidationIssues(issues);
}

function isServiceReleaseTask(taskType?: string) {
  const normalized = (taskType ?? "").toUpperCase();
  return normalized.startsWith("SERVICE_") && ["RELEASE", "UPGRADE", "ROLLBACK"].some((item) => normalized.includes(item));
}

function getReleaseTaskLabel(taskType?: string) {
  const normalized = (taskType ?? "").toUpperCase();
  if (normalized.includes("ROLLBACK")) {
    return "回滚";
  }
  if (normalized.includes("UPGRADE")) {
    return "升级";
  }
  return "发布";
}

function buildReleasePreview(
  mode: ReleaseMode,
  service: ServiceDefinition | null,
  values: Partial<ReleaseFormValues>,
  rollbackVersion: ServiceVersion | null,
  targetNode: DockerNode | null,
) {
  const imageTag =
    mode === "rollback"
      ? rollbackVersion?.imageTag || "--"
      : normalizeValue(values.imageTag) || service?.defaultTag || "--";
  const targetVersion =
    mode === "rollback" ? rollbackVersion?.version || "--" : normalizeValue(values.version) || "--";
  return [
    { label: "动作", value: releaseActionLabelMap[mode.toUpperCase() as keyof typeof releaseActionLabelMap] },
    { label: "当前版本", value: service?.currentVersion || "未发布" },
    { label: "目标版本", value: targetVersion },
    { label: "镜像 Tag", value: imageTag },
    {
      label: "目标节点",
      value: targetNode ? `${targetNode.name} · ${targetNode.endpoint}` : service?.targetId || "--",
    },
    {
      label: "镜像",
      value: service ? `${service.image}:${imageTag === "--" ? service.defaultTag || "latest" : imageTag}` : "--",
    },
  ];
}

export function ServicesPage() {
  const { message } = AntApp.useApp();
  const permissions = useSessionStore((state) => state.permissions);
  const canViewExports = permissions.includes("*") || permissions.includes("exports.view");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [searchParams, setSearchParams] = useSearchParams();
  const [keyword, setKeyword] = useState("");
  const [statusFilter, setStatusFilter] = useState<string>("");
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [releaseDrawerOpen, setReleaseDrawerOpen] = useState(false);
  const [editingService, setEditingService] = useState<ServiceDefinition | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<ServiceDefinition | null>(null);
  const [latestActionText, setLatestActionText] = useState<string | null>(null);
  const [releaseMode, setReleaseMode] = useState<ReleaseMode>("release");
  const [serviceForm] = Form.useForm<ServiceFormValues>();
  const [releaseForm] = Form.useForm<ReleaseFormValues>();
  const selectedServiceId = searchParams.get("selected") ?? "";
  const releaseVersionValue = Form.useWatch("version", releaseForm);
  const releaseImageTagValue = Form.useWatch("imageTag", releaseForm);
  const releaseTargetIdValue = Form.useWatch("targetId", releaseForm);
  const releaseDigestValue = Form.useWatch("imageDigest", releaseForm);

  const servicesQuery = useQuery({
    queryKey: queryKeys.services(keyword, statusFilter),
    queryFn: () => servicesApi.list(keyword, statusFilter),
  });
  const registriesQuery = useQuery({
    queryKey: queryKeys.registries(""),
    queryFn: () => registriesApi.list(""),
  });
  const dockerNodesQuery = useQuery({
    queryKey: queryKeys.dockerNodes,
    queryFn: dockerApi.listNodes,
  });
  const tasksQuery = useQuery({
    queryKey: queryKeys.tasks,
    queryFn: () => tasksApi.list(),
  });
  const auditsQuery = useQuery({
    queryKey: queryKeys.audits,
    queryFn: () => auditsApi.list(),
  });
  const serviceDetailQuery = useQuery({
    queryKey: queryKeys.service(selectedServiceId),
    queryFn: () => servicesApi.detail(selectedServiceId),
    enabled: Boolean(selectedServiceId),
  });
  const serviceInstancesQuery = useQuery({
    queryKey: queryKeys.serviceInstances(selectedServiceId),
    queryFn: () => servicesApi.instances(selectedServiceId),
    enabled: Boolean(selectedServiceId),
  });
  const serviceReleasesQuery = useQuery({
    queryKey: queryKeys.serviceReleases(selectedServiceId),
    queryFn: () => servicesApi.releases(selectedServiceId),
    enabled: Boolean(selectedServiceId),
  });
  const serviceVersionsQuery = useQuery({
    queryKey: queryKeys.serviceVersions(selectedServiceId),
    queryFn: () => servicesApi.versions(selectedServiceId),
    enabled: Boolean(selectedServiceId),
  });
  const serviceHealthChecksQuery = useQuery({
    queryKey: queryKeys.serviceHealthChecks(selectedServiceId),
    queryFn: () => servicesApi.healthChecks(selectedServiceId),
    enabled: Boolean(selectedServiceId),
  });
  const rollbackSuggestionQuery = useQuery({
    queryKey: queryKeys.serviceRollbackSuggestion(selectedServiceId),
    queryFn: () => servicesApi.rollbackSuggestion(selectedServiceId),
    enabled: Boolean(selectedServiceId),
  });
  const alertEventsQuery = useQuery({
    queryKey: [...queryKeys.alertEvents, "service", selectedServiceId],
    queryFn: () => alertsApi.listEvents(),
    enabled: Boolean(selectedServiceId),
  });

  const selectedService =
    serviceDetailQuery.data ?? (servicesQuery.data ?? []).find((item) => item.id === selectedServiceId) ?? null;

  const selectedRegistry = useMemo<Registry | null>(() => {
    if (!selectedService?.registryId) {
      return null;
    }
    return (registriesQuery.data ?? []).find((item) => item.id === selectedService.registryId) ?? null;
  }, [registriesQuery.data, selectedService?.registryId]);

  const selectedNode = useMemo<DockerNode | null>(() => {
    if (!selectedService?.targetId) {
      return null;
    }
    return (dockerNodesQuery.data ?? []).find((item) => item.id === selectedService.targetId) ?? null;
  }, [dockerNodesQuery.data, selectedService?.targetId]);

  const serviceTasks = useMemo(() => {
    if (!selectedService) {
      return [];
    }
    return (tasksQuery.data ?? []).filter((task) =>
      taskMatchesResource(task, "service", selectedService.id, [selectedService.code, selectedService.name]),
    );
  }, [selectedService, tasksQuery.data]);

  const relatedTasks = useMemo(() => serviceTasks.slice(0, 5), [serviceTasks]);

  const serviceAudits = useMemo(() => {
    if (!selectedService) {
      return [];
    }
    return (auditsQuery.data ?? []).filter((audit) =>
      auditMatchesResource(audit, "service", selectedService.id, [selectedService.code, selectedService.name]),
    );
  }, [auditsQuery.data, selectedService]);

  const relatedAudits = useMemo(() => serviceAudits.slice(0, 6), [serviceAudits]);
  const serviceHealthChecks = serviceHealthChecksQuery.data ?? [];
  const latestHealthCheck = serviceHealthChecks[0] ?? null;

  const serviceAlertEvents = useMemo(() => {
    if (!selectedService) {
      return [];
    }
    return (alertEventsQuery.data ?? []).filter(
      (event) => event.resourceType === "service" && event.resourceId === selectedService.id,
    );
  }, [alertEventsQuery.data, selectedService]);
  const latestServiceAlert = serviceAlertEvents[0] ?? null;

  const registryOptions = useMemo(
    () =>
      (registriesQuery.data ?? []).map((item) => ({
        label: `${item.name} · ${item.url}`,
        value: item.id,
      })),
    [registriesQuery.data],
  );

  const dockerNodeOptions = useMemo(
    () =>
      (dockerNodesQuery.data ?? []).map((item) => ({
        label: `${item.name} · ${item.endpoint}`,
        value: item.id,
      })),
    [dockerNodesQuery.data],
  );

  const rollbackVersionOptions = useMemo(
    () => versionsForRollbackOptions(serviceVersionsQuery.data ?? []),
    [serviceVersionsQuery.data],
  );

  const rollbackTargetVersion = useMemo(
    () => (serviceVersionsQuery.data ?? []).find((item) => item.id === releaseVersionValue) ?? null,
    [releaseVersionValue, serviceVersionsQuery.data],
  );

  const releaseTargetNode = useMemo(() => {
    const targetId = releaseMode === "rollback" ? selectedService?.targetId : releaseTargetIdValue || selectedService?.targetId;
    if (!targetId) {
      return null;
    }
    return (dockerNodesQuery.data ?? []).find((item) => item.id === targetId) ?? null;
  }, [dockerNodesQuery.data, releaseMode, releaseTargetIdValue, selectedService?.targetId]);

  const releasePreviewItems = useMemo(
    () =>
      buildReleasePreview(
        releaseMode,
        selectedService,
        {
          version: releaseVersionValue,
          imageTag: releaseImageTagValue,
          imageDigest: releaseDigestValue,
          targetId: releaseTargetIdValue,
        },
        rollbackTargetVersion,
        releaseTargetNode,
      ),
    [
      releaseDigestValue,
      releaseImageTagValue,
      releaseMode,
      releaseTargetIdValue,
      releaseTargetNode,
      releaseVersionValue,
      rollbackTargetVersion,
      selectedService,
    ],
  );

  const activeReleaseTask = useMemo(
    () =>
      serviceTasks.find((task) => isServiceReleaseTask(task.type) && ["PENDING", "RUNNING"].includes(task.status)) ?? null,
    [serviceTasks],
  );
  const serviceSummaryItems = [
    {
      label: "服务总数",
      value: servicesQuery.data?.length ?? 0,
      helper: keyword ? `当前按关键词“${keyword}”检索` : "当前服务定义清单",
    },
    {
      label: "已启用",
      value: (servicesQuery.data ?? []).filter((item) => item.status === "ACTIVE").length,
      helper: "允许进入发布和运行编排的服务",
    },
    {
      label: "草稿 / 归档",
      value: (servicesQuery.data ?? []).filter((item) => item.status === "DRAFT" || item.status === "ARCHIVED").length,
      helper: "仍在准备或已退出交付面的服务",
    },
    {
      label: "待处理异常",
      value: serviceAlertEvents.filter((event) => event.status !== "RESOLVED").length,
      helper: selectedService ? "当前选中服务的异常信号" : "选中服务后展示其告警数量",
    },
  ];

  const versionReleaseTaskMap = useMemo(() => {
    const map = new Map<string, string>();
    (serviceReleasesQuery.data ?? []).forEach((release) => {
      if (release.targetVersionId && release.taskId && !map.has(release.targetVersionId)) {
        map.set(release.targetVersionId, release.taskId);
      }
    });
    return map;
  }, [serviceReleasesQuery.data]);

  const saveMutation = useMutation({
    mutationFn: servicesApi.save,
    onSuccess: async (service) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["services"] }),
        queryClient.invalidateQueries({ queryKey: queryKeys.audits }),
      ]);
      setDrawerOpen(false);
      setEditingService(null);
      serviceForm.resetFields();
      setSearchParams((previous) => {
        const next = new URLSearchParams(previous);
        next.set("selected", service.id);
        return next;
      });
      setLatestActionText(editingService ? "服务定义已更新，工作台信息已刷新。" : "服务定义已创建，可继续执行首次发布。");
      await message.success(editingService ? "服务定义已更新" : "服务定义已创建");
    },
    onError: (error) => {
      applyFormErrors(serviceForm, error);
      void message.error(getErrorMessage(error));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: servicesApi.remove,
    onSuccess: async () => {
      const deletingId = deleteTarget?.id ?? "";
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["services"] }),
        queryClient.invalidateQueries({ queryKey: queryKeys.audits }),
      ]);
      setDeleteTarget(null);
      if (selectedServiceId === deletingId) {
        setSearchParams((previous) => {
          const next = new URLSearchParams(previous);
          next.delete("selected");
          return next;
        });
      }
      setLatestActionText("服务定义已删除，关联工作台已退出当前条目。");
      await message.success("服务定义已删除");
    },
    onError: (error) => {
      void message.error(getErrorMessage(error, "删除服务定义失败"));
    },
  });

  const releaseMutation = useMutation({
    mutationFn: async (values: ReleaseFormValues) => {
      if (!selectedService) {
        throw new Error("请先选择服务定义");
      }
      if (releaseMode === "rollback") {
        return servicesApi.rollback(selectedService.id, {
          versionId: values.version,
        });
      }
      const payload: ServiceReleaseInput = {
        version: values.version,
        imageTag: values.imageTag,
        imageDigest: values.imageDigest,
        targetId: values.targetId,
      };
      return releaseMode === "release"
        ? servicesApi.release(selectedService.id, payload)
        : servicesApi.upgrade(selectedService.id, payload);
    },
    onSuccess: async (result) => {
      if (!selectedService) {
        return;
      }
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["services"] }),
        queryClient.invalidateQueries({ queryKey: queryKeys.service(selectedService.id) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.serviceInstances(selectedService.id) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.serviceReleases(selectedService.id) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.serviceVersions(selectedService.id) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.serviceHealthChecks(selectedService.id) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.serviceRollbackSuggestion(selectedService.id) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.tasks }),
        queryClient.invalidateQueries({ queryKey: queryKeys.audits }),
      ]);
      setReleaseDrawerOpen(false);
      releaseForm.resetFields();
      setLatestActionText(
        `${releaseActionLabelMap[releaseMode.toUpperCase() as keyof typeof releaseActionLabelMap]} 已提交，任务 ${result.taskId} 可在任务中心继续跟踪。`,
      );
      await message.success(
        `${releaseActionLabelMap[releaseMode.toUpperCase() as keyof typeof releaseActionLabelMap]} 已触发`,
      );
    },
    onError: (error) => {
      applyFormErrors(releaseForm, error);
      void message.error(getErrorMessage(error));
    },
  });

  const exportMutation = useMutation({
    mutationFn: (serviceId: string) => exportsApi.exportService(serviceId),
    onSuccess: async (job) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.exports });
      await message.success("服务导出已创建");
      if (canViewExports) {
        navigate(`/settings/exports?selected=${encodeURIComponent(job.id)}`);
      }
    },
    onError: (error) => {
      void message.error(getErrorMessage(error, "创建服务导出失败"));
    },
  });

  if (servicesQuery.isError) {
    return <ErrorState message={servicesQuery.error.message} onRetry={() => void servicesQuery.refetch()} />;
  }

  return (
    <PermissionGuard permission="services.view" forbiddenPage>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <PageHeader
          title="服务定义"
          description="统一管理服务定义、版本、实例与发布记录。"
          eyebrow="应用交付 / 服务工作台"
          extra={
            <PermissionGuard permission="services.manage">
              <Button
                type="primary"
                onClick={() => {
                  setEditingService(null);
                  serviceForm.setFieldsValue(buildServiceFormValues(null));
                  setDrawerOpen(true);
                }}
              >
                新增服务
              </Button>
            </PermissionGuard>
          }
        />

        <Card className="page-card">
          <div className="workbench-summary-grid">
            {serviceSummaryItems.map((item) => (
              <div key={item.label} className="workbench-summary-card">
                <Typography.Text className="workbench-summary-label">{item.label}</Typography.Text>
                <div className="workbench-summary-value">{item.value}</div>
                <div className="workbench-summary-helper">{item.helper}</div>
              </div>
            ))}
          </div>
        </Card>

        <Card className="page-card">
          <div className="page-toolbar">
            <div className="page-toolbar-start">
              <Input.Search
                allowClear
                placeholder="搜索服务名、编码、镜像或分组"
                style={{ width: 320 }}
                onSearch={(value) => setKeyword(value)}
              />
              <Select
                allowClear
                placeholder="按状态筛选"
                style={{ width: 160 }}
                value={statusFilter || undefined}
                options={[{ label: "全部状态", value: "" }, ...statusOptions]}
                onChange={(value) => setStatusFilter(value ?? "")}
              />
            </div>
          </div>
        </Card>

        <div className="resource-workbench service-workbench">
          <div className="resource-list-pane">
            <Card className="page-card">
              <DataTable
                rowKey="id"
                loading={servicesQuery.isLoading}
                dataSource={servicesQuery.data}
                rowClassName={(service) => (service.id === selectedServiceId ? "resource-row-selected" : "")}
                onRow={(service) => ({
                  onClick: () => {
                    setSearchParams((previous) => {
                      const next = new URLSearchParams(previous);
                      next.set("selected", service.id);
                      return next;
                    });
                    setLatestActionText(null);
                  },
                })}
                locale={{
                  emptyText: (
                    <EmptyState
                      title="还没有服务定义"
                      description="配置镜像来源、目标节点和运行参数后，即可创建服务并发起发布。"
                      action={
                        <Button
                          type="primary"
                          onClick={() => {
                            setEditingService(null);
                            serviceForm.setFieldsValue(buildServiceFormValues(null));
                            setDrawerOpen(true);
                          }}
                        >
                          创建第一条服务
                        </Button>
                      }
                    />
                  ),
                }}
                columns={[
                  {
                    title: "服务",
                    dataIndex: "name",
                    render: (_, service) => (
                      <Space direction="vertical" size={2}>
                        <span>{service.name}</span>
                        <span style={{ color: "#64748b" }}>{service.code}</span>
                      </Space>
                    ),
                  },
                  {
                    title: "镜像",
                    dataIndex: "image",
                    render: (_, service) => (
                      <Space direction="vertical" size={2}>
                        <Typography.Text>{service.image}</Typography.Text>
                        <Typography.Text type="secondary">{service.defaultTag}</Typography.Text>
                      </Space>
                    ),
                  },
                  {
                    title: "状态",
                    dataIndex: "status",
                    render: (value: ServiceDefinition["status"]) => <StatusBadge status={value} />,
                  },
                  {
                    title: "当前版本",
                    dataIndex: "currentVersion",
                    render: (value: string) => value || "--",
                  },
                ]}
              />
            </Card>
          </div>

          <div className="resource-detail-pane">
            <ResourceDetailPanel
              title={selectedService?.name}
              kicker={selectedService ? "服务上下文" : undefined}
              subtitle={selectedService ? `${selectedService.code} · ${selectedService.image}` : undefined}
              status={selectedService ? <StatusBadge status={selectedService.status} /> : undefined}
              helper={
                selectedService
                  ? "围绕当前服务集中查看版本、实例、发布流水、健康检查和异常闭环，让交付动作和运行信号始终留在同一个上下文里。"
                  : undefined
              }
              highlights={
                selectedService
                  ? [
                      {
                        label: "当前版本",
                        value: selectedService.currentVersion || "--",
                        helper: selectedService.defaultTag ? `默认 Tag ${selectedService.defaultTag}` : "尚未设置默认 Tag",
                      },
                      {
                        label: "运行实例",
                        value: serviceInstancesQuery.data?.length ?? 0,
                        helper: selectedNode ? `目标节点 ${selectedNode.name}` : "尚未识别目标节点",
                      },
                      {
                        label: "待处理异常",
                        value: serviceAlertEvents.filter((event) => event.status !== "RESOLVED").length,
                        helper: latestServiceAlert ? `最近触发 ${formatDateTime(latestServiceAlert.lastTriggeredAt)}` : "当前没有服务相关告警",
                      },
                    ]
                  : []
              }
              meta={
                selectedService
                  ? [
                      {
                        label: "分组",
                        value: selectedService.group || "--",
                      },
                      {
                        label: "镜像仓库",
                        value: selectedRegistry ? `${selectedRegistry.name} · ${selectedRegistry.url}` : "--",
                      },
                      {
                        label: "目标类型",
                        value: targetTypeLabelMap[selectedService.targetType],
                      },
                      {
                        label: "目标节点",
                        value: selectedNode ? `${selectedNode.name} · ${selectedNode.endpoint}` : selectedService.targetId || "--",
                      },
                      {
                        label: "当前版本",
                        value: selectedService.currentVersion || "--",
                      },
                      {
                        label: "最近通知状态",
                        value: latestServiceAlert?.notificationStatus ? (
                          <StatusBadge status={latestServiceAlert.notificationStatus} />
                        ) : (
                          "--"
                        ),
                      },
                      {
                        label: "回滚建议",
                        value: rollbackSuggestionQuery.data?.available
                          ? rollbackSuggestionQuery.data.suggestedVersion ||
                            rollbackSuggestionQuery.data.suggestedImageTag ||
                            "可回滚"
                          : rollbackSuggestionQuery.data?.reason || "--",
                      },
                      {
                        label: "标签",
                        value: selectedService.tags.length ? (
                          <Space wrap>
                            {selectedService.tags.map((tag) => (
                              <Tag key={tag}>{tag}</Tag>
                            ))}
                          </Space>
                        ) : (
                          "--"
                        ),
                      },
                      {
                        label: "默认 Tag",
                        value: selectedService.defaultTag || "--",
                      },
                      {
                        label: "说明",
                        value: selectedService.description || "--",
                      },
                    ]
                  : []
              }
              actions={
                selectedService ? (
                  <>
                    <div className="resource-action-group">
                      <PermissionGuard permission="services.release">
                        <Button
                          type="primary"
                          disabled={Boolean(activeReleaseTask)}
                          onClick={() => {
                            setReleaseMode("release");
                            releaseForm.setFieldsValue(buildReleaseFormValues(selectedService));
                            setReleaseDrawerOpen(true);
                          }}
                        >
                          首次发布
                        </Button>
                      </PermissionGuard>
                      <PermissionGuard permission="services.rollback">
                        <Button
                          disabled={Boolean(activeReleaseTask)}
                          onClick={() => {
                            setReleaseMode("upgrade");
                            releaseForm.setFieldsValue(buildReleaseFormValues(selectedService));
                            setReleaseDrawerOpen(true);
                          }}
                        >
                          升级版本
                        </Button>
                      </PermissionGuard>
                      <PermissionGuard permission="services.release">
                        <Button
                          disabled={Boolean(activeReleaseTask)}
                          onClick={() => {
                            setReleaseMode("rollback");
                            releaseForm.setFieldsValue(buildReleaseFormValues(selectedService));
                            setReleaseDrawerOpen(true);
                          }}
                        >
                          回滚
                        </Button>
                      </PermissionGuard>
                    </div>
                    <div className="resource-action-group">
                      <PermissionActionButton
                        permission="exports.create"
                        permissionReason="当前账号缺少 exports.create 权限，无法创建服务导出。"
                        loading={exportMutation.isPending}
                        onClick={() => exportMutation.mutate(selectedService.id)}
                      >
                        导出服务
                      </PermissionActionButton>
                      <Button onClick={() => navigate(buildTasksPath({ resourceType: "service", resourceId: selectedService.id }))}>
                        查看任务
                      </Button>
                      <Button onClick={() => navigate(buildAuditsPath({ resourceType: "service", resourceId: selectedService.id }))}>
                        查看审计
                      </Button>
                      <PermissionGuard permission="services.manage">
                        <Button
                          onClick={() => {
                            setEditingService(selectedService);
                            serviceForm.setFieldsValue(buildServiceFormValues(selectedService));
                            setDrawerOpen(true);
                          }}
                        >
                          编辑
                        </Button>
                      </PermissionGuard>
                      <PermissionGuard permission="services.manage">
                        <Button danger onClick={() => setDeleteTarget(selectedService)}>
                          删除
                        </Button>
                      </PermissionGuard>
                    </div>
                  </>
                ) : undefined
              }
            >
              {activeReleaseTask ? (
                <div className="resource-detail-section">
                  <Alert
                    type="warning"
                    showIcon
                    message={`存在进行中的服务${getReleaseTaskLabel(activeReleaseTask.type)}任务`}
                    description={`任务 ${activeReleaseTask.id} 正在执行中，由 ${activeReleaseTask.initiatedBy} 于 ${formatDateTime(activeReleaseTask.createdAt)} 发起。为避免重复操作，发布动作已暂时锁定。`}
                    action={
                      <Button size="small" onClick={() => navigate(`/tasks/${activeReleaseTask.id}`)}>
                        查看任务
                      </Button>
                    }
                  />
                </div>
              ) : null}

              {latestActionText ? (
                <div className="resource-detail-section resource-callout">
                  <Typography.Text type="secondary">{latestActionText}</Typography.Text>
                </div>
              ) : null}

              <div className="resource-detail-section">
                <div className="page-toolbar">
                  <Typography.Text strong>运行配置</Typography.Text>
                  <Typography.Text type="secondary">
                    {selectedService ? "发布目标、端口、环境变量与挂载统一在这里收口。" : ""}
                  </Typography.Text>
                </div>
                <div className="two-col-grid" style={{ marginTop: 12 }}>
                  <div className="resource-subpanel">
                    <Typography.Text strong>端口映射</Typography.Text>
                    <DataTable
                      rowKey={(record) => `${record.name}-${record.containerPort}-${record.hostPort ?? 0}`}
                      pagination={false}
                      dataSource={selectedService?.ports ?? []}
                      locale={{ emptyText: "未配置端口" }}
                      columns={[
                        { title: "名称", dataIndex: "name" },
                        { title: "容器端口", dataIndex: "containerPort" },
                        {
                          title: "主机端口",
                          dataIndex: "hostPort",
                          render: (value?: number) => value ?? "--",
                        },
                        {
                          title: "协议",
                          dataIndex: "protocol",
                          render: (value?: string) => value ?? "TCP",
                        },
                      ]}
                    />
                  </div>

                  <div className="resource-subpanel">
                    <Typography.Text strong>资源限制</Typography.Text>
                    <div className="resource-detail-metadata">
                      <div className="resource-detail-metadata-item">
                        <Typography.Text type="secondary" className="resource-detail-metadata-label">
                          CPU
                        </Typography.Text>
                        <div className="resource-detail-metadata-value">
                          {selectedService?.resourceLimits.cpu || "--"}
                        </div>
                      </div>
                      <div className="resource-detail-metadata-item">
                        <Typography.Text type="secondary" className="resource-detail-metadata-label">
                          Memory
                        </Typography.Text>
                        <div className="resource-detail-metadata-value">
                          {selectedService?.resourceLimits.memory || "--"}
                        </div>
                      </div>
                    </div>
                    <Typography.Text strong>环境变量</Typography.Text>
                    <DataTable
                      rowKey={(record) => `${record.key}-${record.value}`}
                      pagination={false}
                      dataSource={selectedService?.envs ?? []}
                      locale={{ emptyText: "未配置环境变量" }}
                      columns={[
                        { title: "Key", dataIndex: "key" },
                        { title: "Value", dataIndex: "value" },
                      ]}
                    />
                    <Typography.Text strong>挂载卷</Typography.Text>
                    <DataTable
                      rowKey={(record) => `${record.source}-${record.target}`}
                      pagination={false}
                      dataSource={selectedService?.mounts ?? []}
                      locale={{ emptyText: "未配置挂载卷" }}
                      columns={[
                        { title: "Source", dataIndex: "source" },
                        { title: "Target", dataIndex: "target" },
                        {
                          title: "只读",
                          dataIndex: "readOnly",
                          render: (value?: boolean) => (value ? "是" : "否"),
                        },
                      ]}
                    />
                  </div>
                </div>
              </div>

              <div className="resource-detail-section">
                <div className="page-toolbar">
                  <Typography.Text strong>版本与发布记录</Typography.Text>
                  <Typography.Text type="secondary">
                    {serviceVersionsQuery.isLoading || serviceReleasesQuery.isLoading
                      ? "正在同步版本数据..."
                      : `${serviceVersionsQuery.data?.length ?? 0} 个版本 · ${serviceReleasesQuery.data?.length ?? 0} 条发布记录`}
                  </Typography.Text>
                </div>
                <div className="service-detail-stack">
                  <div className="resource-subpanel">
                    <div className="service-detail-panel-header">
                      <Space direction="vertical" size={2}>
                        <Typography.Text strong>版本列表</Typography.Text>
                        <Typography.Text type="secondary">
                          以版本号、镜像 Tag 和构建指纹为主，保留最近一次关联任务入口。
                        </Typography.Text>
                      </Space>
                    </div>
                    {serviceVersionsQuery.isLoading ? (
                      <div className="resource-activity-empty">
                        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="正在加载版本记录..." />
                      </div>
                    ) : (serviceVersionsQuery.data?.length ?? 0) > 0 ? (
                      <div className="service-signal-list">
                        {(serviceVersionsQuery.data ?? []).map((version) => {
                          const taskId = versionReleaseTaskMap.get(version.id);
                          return (
                            <div key={version.id} className="service-signal-item">
                              <div className="service-signal-main">
                                <Space direction="vertical" size={6} style={{ width: "100%" }}>
                                  <Space wrap size={[8, 8]}>
                                    <Typography.Text strong>{version.version}</Typography.Text>
                                    <Tag color="blue">{version.imageTag || "--"}</Tag>
                                  </Space>
                                  <Typography.Text type="secondary">
                                    镜像指纹 {version.imageDigest ? truncateMiddle(version.imageDigest, 16, 8) : "--"}
                                  </Typography.Text>
                                  <div className="resource-activity-meta">{formatDateTime(version.createdAt)}</div>
                                </Space>
                              </div>
                              <div className="service-signal-actions">
                                {taskId ? (
                                  <Button size="small" type="link" onClick={() => navigate(`/tasks/${taskId}`)}>
                                    任务详情
                                  </Button>
                                ) : null}
                              </div>
                            </div>
                          );
                        })}
                      </div>
                    ) : (
                      <div className="resource-activity-empty">
                        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="还没有版本记录" />
                      </div>
                    )}
                  </div>

                  <div className="resource-subpanel">
                    <div className="service-detail-panel-header">
                      <Space direction="vertical" size={2}>
                        <Typography.Text strong>发布流水</Typography.Text>
                        <Typography.Text type="secondary">
                          将动作、目标版本、回滚建议和失败原因收口在同一行，方便定位最近一次变更。
                        </Typography.Text>
                      </Space>
                    </div>
                    {serviceReleasesQuery.isLoading ? (
                      <div className="resource-activity-empty">
                        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="正在加载发布记录..." />
                      </div>
                    ) : (serviceReleasesQuery.data?.length ?? 0) > 0 ? (
                      <div className="service-signal-list">
                        {(serviceReleasesQuery.data ?? []).map((record) => (
                          <div key={record.id} className="service-signal-item">
                            <div className="service-signal-main">
                              <Space direction="vertical" size={6} style={{ width: "100%" }}>
                                <Typography.Text strong>
                                  {releaseActionLabelMap[record.action]} · {getReleaseTargetSummary(record)}
                                </Typography.Text>
                                <Space wrap size={[8, 8]}>
                                  <StatusBadge status={record.status} />
                                  {record.healthCheckStatus ? <StatusBadge status={record.healthCheckStatus} /> : null}
                                  {record.notificationStatus ? <StatusBadge status={record.notificationStatus} /> : null}
                                </Space>
                                <Typography.Text type="secondary">
                                  {truncateText(getReleaseOutcomeSummary(record), 112)}
                                </Typography.Text>
                                <div className="resource-activity-meta">
                                  {record.createdBy ? `${record.createdBy} · ` : ""}
                                  {formatDateTime(record.createdAt)}
                                  {record.suggestedRollbackVersion ? ` · 回滚建议 ${record.suggestedRollbackVersion}` : ""}
                                </div>
                              </Space>
                            </div>
                            <div className="service-signal-actions">
                              {record.taskId ? (
                                <Button size="small" type="link" onClick={() => navigate(`/tasks/${record.taskId}`)}>
                                  任务详情
                                </Button>
                              ) : null}
                            </div>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <div className="resource-activity-empty">
                        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="还没有发布记录" />
                      </div>
                    )}
                  </div>
                </div>
              </div>

              <div className="resource-detail-section">
                <div className="page-toolbar">
                  <Typography.Text strong>健康检查与异常闭环</Typography.Text>
                  <Typography.Text type="secondary">
                    {serviceHealthChecksQuery.isLoading
                      ? "正在同步探活结果..."
                      : `${serviceHealthChecks.length} 条健康检查 · ${serviceAlertEvents.length} 条相关告警`}
                  </Typography.Text>
                </div>
                <div className="service-detail-band-grid">
                  <div className="resource-subpanel">
                    <div className="service-detail-panel-header">
                      <Space direction="vertical" size={2}>
                        <Typography.Text strong>最近健康检查</Typography.Text>
                        <Typography.Text type="secondary">先看探活策略、目标和结果摘要，再判断是否需要进入告警闭环。</Typography.Text>
                      </Space>
                    </div>
                    {serviceHealthChecksQuery.isLoading ? (
                      <div className="resource-activity-empty">
                        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="正在加载健康检查记录..." />
                      </div>
                    ) : serviceHealthChecks.length ? (
                      <div className="service-signal-list">
                        {serviceHealthChecks.slice(0, 5).map((record) => (
                          <div key={record.id} className="service-signal-item">
                            <div className="service-signal-main">
                              <Space direction="vertical" size={6} style={{ width: "100%" }}>
                                <Space wrap size={[8, 8]}>
                                  <Tag color={serviceHealthStatusMetaMap[record.status].color}>
                                    {healthStrategyLabelMap[record.strategyType]}
                                  </Tag>
                                  <StatusBadge status={record.status} />
                                </Space>
                                <Typography.Text strong>{record.target}</Typography.Text>
                                <Typography.Text type="secondary">
                                  {truncateText(getHealthCheckSummary(record), 112)}
                                </Typography.Text>
                                <div className="resource-activity-meta">
                                  {formatDateTime(record.startedAt)}
                                  {record.finishedAt ? ` · 结束于 ${formatDateTime(record.finishedAt)}` : " · 执行中"}
                                </div>
                              </Space>
                            </div>
                            <div className="service-signal-actions">
                              {record.taskId ? (
                                <Button size="small" type="link" onClick={() => navigate(`/tasks/${record.taskId}`)}>
                                  任务详情
                                </Button>
                              ) : null}
                            </div>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <div className="resource-activity-empty">
                        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前服务还没有健康检查记录" />
                      </div>
                    )}
                  </div>

                  <div className="resource-subpanel">
                    <div className="service-detail-panel-header">
                      <Space direction="vertical" size={2}>
                        <Typography.Text strong>最近相关告警</Typography.Text>
                        <Typography.Text type="secondary">把事件等级、处理状态和通知结果放在一张工作带里，便于追踪是否真正闭环。</Typography.Text>
                      </Space>
                    </div>
                    {alertEventsQuery.isLoading ? (
                      <div className="resource-activity-empty">
                        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="正在加载相关告警..." />
                      </div>
                    ) : serviceAlertEvents.length ? (
                      <div className="service-signal-list">
                        {serviceAlertEvents.slice(0, 5).map((event) => {
                          const severityMeta = alertSeverityMetaMap[event.severity];
                          const notificationMeta = event.notificationStatus
                            ? notificationStatusMetaMap[event.notificationStatus]
                            : null;
                          return (
                            <div key={event.id} className="service-signal-item">
                              <div className="service-signal-main">
                                <Space direction="vertical" size={6} style={{ width: "100%" }}>
                                  <Space wrap size={[8, 8]}>
                                    <Typography.Text strong>{alertEventLabelMap[event.eventType] ?? event.eventType}</Typography.Text>
                                    <Tag color={severityMeta.color}>{severityMeta.label}</Tag>
                                    <StatusBadge status={event.status} />
                                    {notificationMeta ? <Tag color={notificationMeta.color}>{notificationMeta.label}</Tag> : null}
                                  </Space>
                                  <Typography.Text type="secondary">{truncateText(event.summary, 108)}</Typography.Text>
                                  <div className="resource-activity-meta">
                                    {formatDateTime(event.lastTriggeredAt)}
                                    {event.suggestedRollbackVersion ? ` · 回滚建议 ${event.suggestedRollbackVersion}` : ""}
                                  </div>
                                  {normalizeValue(getAlertSummary(event)) !== normalizeValue(event.summary) ? (
                                    <Typography.Text type="secondary">{truncateText(getAlertSummary(event), 132)}</Typography.Text>
                                  ) : null}
                                </Space>
                              </div>
                              <div className="service-signal-actions">
                                {event.taskId ? (
                                  <Button size="small" type="link" onClick={() => navigate(`/tasks/${event.taskId}`)}>
                                    任务详情
                                  </Button>
                                ) : null}
                              </div>
                            </div>
                          );
                        })}
                      </div>
                    ) : (
                      <div className="resource-activity-empty">
                        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前服务还没有相关告警" />
                      </div>
                    )}
                  </div>
                </div>
              </div>

              <div className="resource-detail-section">
                <div className="page-toolbar">
                  <Typography.Text strong>运行实例</Typography.Text>
                  <Typography.Text type="secondary">
                    {serviceInstancesQuery.isLoading
                      ? "正在读取实例状态..."
                      : `${serviceInstancesQuery.data?.length ?? 0} 个实例`}
                  </Typography.Text>
                </div>
                <div className="resource-subpanel" style={{ marginTop: 12 }}>
                  <DataTable
                    rowKey="id"
                    pagination={false}
                    loading={serviceInstancesQuery.isLoading}
                    dataSource={serviceInstancesQuery.data}
                    locale={{ emptyText: "当前还没有运行实例" }}
                    columns={[
                      { title: "实例名", dataIndex: "name" },
                      { title: "版本", dataIndex: "version" },
                      { title: "镜像 Tag", dataIndex: "imageTag" },
                      {
                        title: "状态",
                        dataIndex: "status",
                        render: (value: string) => <StatusBadge status={value} />,
                      },
                      {
                        title: "启动时间",
                        dataIndex: "startedAt",
                        render: (value?: string) => formatDateTime(value),
                      },
                      {
                        title: "节点",
                        key: "dockerNode",
                        render: (_, instance) =>
                          instance.dockerNodeId ? (
                            <Button
                              size="small"
                              type="link"
                              onClick={() => navigate(`/docker/nodes?selected=${encodeURIComponent(instance.dockerNodeId)}`)}
                            >
                              查看节点
                            </Button>
                          ) : (
                            "--"
                          ),
                      },
                    ]}
                  />
                </div>
              </div>

              <ResourceActivityList
                title="最近任务"
                helper="先看发布、升级、回滚和健康检查相关任务，确认这条交付链路现在卡在哪一步。"
                actionLabel={selectedService ? "进入任务中心" : undefined}
                onActionClick={
                  selectedService
                    ? () =>
                        navigate(
                          buildTasksPath({
                            resourceType: "service",
                            resourceId: selectedService.id,
                          }),
                        )
                    : undefined
                }
                items={relatedTasks.map((task) => ({
                  key: task.id,
                  title: task.type,
                  description: task.summary ?? task.target,
                  meta: `${task.initiatedBy} · ${formatDateTime(task.createdAt)}`,
                  extra: <TaskStatus task={task} />,
                }))}
                emptyText="当前服务还没有关联任务。"
              />

              <ResourceActivityList
                title="最近审计"
                helper="操作审计可以帮助你对齐最近一次服务变更、发布动作和异常出现的时间线。"
                actionLabel={selectedService ? "查看全部审计" : undefined}
                onActionClick={
                  selectedService
                    ? () =>
                        navigate(
                          buildAuditsPath({
                            resourceType: "service",
                            resourceId: selectedService.id,
                          }),
                        )
                    : undefined
                }
                items={relatedAudits.map((audit) => ({
                  key: audit.id,
                  title: audit.action,
                  description: audit.summary,
                  meta: `${audit.actor} · ${formatDateTime(audit.createdAt)}`,
                  extra: <StatusBadge status={audit.result} />,
                }))}
                emptyText="当前服务还没有关联审计记录。"
              />
            </ResourceDetailPanel>
          </div>
        </div>

        <FormDrawer
          open={drawerOpen}
          title={editingService ? "编辑服务定义" : "新增服务定义"}
          width={720}
          loading={saveMutation.isPending}
          onClose={() => {
            setDrawerOpen(false);
            setEditingService(null);
          }}
          onSubmit={() => serviceForm.submit()}
        >
          <Form
            form={serviceForm}
            layout="vertical"
            initialValues={buildServiceFormValues(editingService)}
            onFinish={(values) => {
              const validationIssues = validateServiceDefinition(values);
              applyServiceValidationIssues(serviceForm, values, validationIssues);
              if (validationIssues.length) {
                void message.error("请先修正服务定义中的校验问题");
                return;
              }
              const payload: ServiceDefinitionInput = {
                id: editingService?.id,
                name: values.name,
                code: values.code,
                group: values.group,
                tags: values.tags ?? [],
                description: values.description,
                registryId: values.registryId,
                image: values.image,
                defaultTag: values.defaultTag,
                targetType: "DOCKER_NODE",
                targetId: values.targetId,
                status: values.status,
                ports:
                  values.ports?.filter((item) => item.name || item.containerPort || item.hostPort || item.protocol) ?? [],
                envs: values.envs?.filter((item) => item.key || item.value) ?? [],
                mounts: values.mounts?.filter((item) => item.source || item.target) ?? [],
                resourceLimits: {
                  cpu: values.cpu,
                  memory: values.memory,
                },
              };
              saveMutation.mutate(payload);
            }}
          >
            <div className="two-col-grid">
              <Form.Item label="服务名称" name="name" rules={[{ required: true, message: "请输入服务名称" }]}>
                <Input />
              </Form.Item>
              <Form.Item label="服务编码" name="code" rules={[{ required: true, message: "请输入服务编码" }]}>
                <Input disabled={Boolean(editingService)} />
              </Form.Item>
              <Form.Item label="服务分组" name="group">
                <Input placeholder="例如 core / edge / ops" />
              </Form.Item>
              <Form.Item label="状态" name="status" rules={[{ required: true, message: "请选择状态" }]}>
                <Select options={statusOptions} />
              </Form.Item>
              <Form.Item label="关联 Registry" name="registryId" rules={[{ required: true, message: "请选择 Registry" }]}>
                <Select options={registryOptions} placeholder="选择镜像来源" />
              </Form.Item>
              <Form.Item label="目标节点" name="targetId" rules={[{ required: true, message: "请选择目标节点" }]}>
                <Select options={dockerNodeOptions} placeholder="选择发布目标" />
              </Form.Item>
              <Form.Item label="镜像路径" name="image" rules={[{ required: true, message: "请输入镜像路径" }]}>
                <Input placeholder="例如 aegisops/api" />
              </Form.Item>
              <Form.Item label="默认 Tag" name="defaultTag" rules={[{ required: true, message: "请输入默认 Tag" }]}>
                <Input placeholder="latest" />
              </Form.Item>
            </div>

            <Form.Item label="标签" name="tags">
              <Select mode="tags" placeholder="例如 production / api / core" />
            </Form.Item>
            <Form.Item label="说明" name="description">
              <Input.TextArea rows={3} />
            </Form.Item>

            <Typography.Text strong>端口映射</Typography.Text>
            <Form.List name="ports">
              {(fields, { add, remove }) => (
                <Space direction="vertical" size={12} style={{ width: "100%", marginTop: 12, marginBottom: 16 }}>
                  {fields.map((field) => (
                    <Card key={field.key} size="small">
                      <div className="two-col-grid">
                        <Form.Item label="名称" name={[field.name, "name"]}>
                          <Input placeholder="http" />
                        </Form.Item>
                        <Form.Item label="协议" name={[field.name, "protocol"]}>
                          <Select
                            options={[
                              { label: "TCP", value: "TCP" },
                              { label: "UDP", value: "UDP" },
                            ]}
                          />
                        </Form.Item>
                        <Form.Item label="容器端口" name={[field.name, "containerPort"]}>
                          <InputNumber min={1} max={65535} style={{ width: "100%" }} />
                        </Form.Item>
                        <Form.Item label="主机端口" name={[field.name, "hostPort"]}>
                          <InputNumber min={1} max={65535} style={{ width: "100%" }} />
                        </Form.Item>
                      </div>
                      <Button danger type="link" onClick={() => remove(field.name)}>
                        删除端口映射
                      </Button>
                    </Card>
                  ))}
                  <Button onClick={() => add({ name: "", protocol: "TCP" })}>新增端口</Button>
                </Space>
              )}
            </Form.List>

            <Typography.Text strong>环境变量</Typography.Text>
            <Form.List name="envs">
              {(fields, { add, remove }) => (
                <Space direction="vertical" size={12} style={{ width: "100%", marginTop: 12, marginBottom: 16 }}>
                  {fields.map((field) => (
                    <Card key={field.key} size="small">
                      <div className="two-col-grid">
                        <Form.Item label="Key" name={[field.name, "key"]}>
                          <Input />
                        </Form.Item>
                        <Form.Item label="Value" name={[field.name, "value"]}>
                          <Input />
                        </Form.Item>
                      </div>
                      <Button danger type="link" onClick={() => remove(field.name)}>
                        删除环境变量
                      </Button>
                    </Card>
                  ))}
                  <Button onClick={() => add({ key: "", value: "" })}>新增环境变量</Button>
                </Space>
              )}
            </Form.List>

            <Typography.Text strong>挂载卷</Typography.Text>
            <Form.List name="mounts">
              {(fields, { add, remove }) => (
                <Space direction="vertical" size={12} style={{ width: "100%", marginTop: 12, marginBottom: 16 }}>
                  {fields.map((field) => (
                    <Card key={field.key} size="small">
                      <div className="two-col-grid">
                        <Form.Item label="Source" name={[field.name, "source"]}>
                          <Input />
                        </Form.Item>
                        <Form.Item label="Target" name={[field.name, "target"]}>
                          <Input />
                        </Form.Item>
                        <Form.Item label="只读" name={[field.name, "readOnly"]}>
                          <Select
                            options={[
                              { label: "否", value: false },
                              { label: "是", value: true },
                            ]}
                          />
                        </Form.Item>
                      </div>
                      <Button danger type="link" onClick={() => remove(field.name)}>
                        删除挂载卷
                      </Button>
                    </Card>
                  ))}
                  <Button onClick={() => add({ source: "", target: "", readOnly: false })}>新增挂载卷</Button>
                </Space>
              )}
            </Form.List>

            <Typography.Text strong>资源限制</Typography.Text>
            <div className="two-col-grid" style={{ marginTop: 12 }}>
              <Form.Item label="CPU" name="cpu">
                <Input placeholder="例如 500m" />
              </Form.Item>
              <Form.Item label="Memory" name="memory">
                <Input placeholder="例如 512Mi" />
              </Form.Item>
            </div>
          </Form>
        </FormDrawer>

        <FormDrawer
          open={releaseDrawerOpen}
          title={
            releaseMode === "release" ? "执行首次发布" : releaseMode === "upgrade" ? "执行版本升级" : "执行回滚"
          }
          width={520}
          loading={releaseMutation.isPending}
          onClose={() => {
            setReleaseDrawerOpen(false);
            releaseForm.resetFields();
          }}
          onSubmit={() => releaseForm.submit()}
        >
          <Space direction="vertical" size={16} style={{ width: "100%" }}>
            <Alert
              type="info"
              showIcon
              message="提交后将在任务中心跟踪发布进度"
              description="确认版本、镜像与目标节点后发起发布、升级或回滚，并在记录中查看结果。"
            />
            {activeReleaseTask ? (
              <Alert
                type="warning"
                showIcon
                message="当前已有进行中的发布任务"
                description="建议先跟踪现有任务结果，再继续发起新的发布、升级或回滚。"
              />
            ) : null}
            <div className="resource-subpanel">
              <div className="page-toolbar">
                <Typography.Text strong>提交前摘要</Typography.Text>
                <Typography.Text type="secondary">确认目标版本、镜像与节点后再发起动作。</Typography.Text>
              </div>
              <div className="resource-detail-metadata">
                {releasePreviewItems.map((item) => (
                  <div key={item.label} className="resource-detail-metadata-item">
                    <Typography.Text type="secondary" className="resource-detail-metadata-label">
                      {item.label}
                    </Typography.Text>
                    <div className="resource-detail-metadata-value">{item.value}</div>
                  </div>
                ))}
              </div>
            </div>
            <Form
              form={releaseForm}
              layout="vertical"
              onFinish={(values) => {
                const issues: ValidationIssue[] = [];
                if (releaseMode === "upgrade" && selectedService?.currentVersion && values.version === selectedService.currentVersion) {
                  issues.push({ name: ["version"], errors: ["升级目标版本不能与当前版本一致"] });
                }
                if (
                  releaseMode === "rollback" &&
                  rollbackTargetVersion?.version &&
                  rollbackTargetVersion.version === selectedService?.currentVersion
                ) {
                  issues.push({ name: ["version"], errors: ["当前已经处于该版本，无需重复回滚"] });
                }
                releaseForm.setFields([
                  { name: ["version"], errors: [] },
                  { name: ["imageTag"], errors: [] },
                  ...issues.map((issue) => ({ name: issue.name as never, errors: issue.errors })),
                ] as never);
                if (issues.length) {
                  void message.error("请先修正发布参数");
                  return;
                }
                releaseMutation.mutate(values);
              }}
            >
              {releaseMode === "rollback" ? (
                <Form.Item label="回滚目标版本" name="version" rules={[{ required: true, message: "请选择回滚目标版本" }]}>
                  <Select options={rollbackVersionOptions} placeholder="选择历史版本" />
                </Form.Item>
              ) : (
                <>
                  <Form.Item label="版本号" name="version" rules={[{ required: true, message: "请输入版本号" }]}>
                    <Input placeholder="例如 v0.2.1" />
                  </Form.Item>
                  <Form.Item label="镜像 Tag" name="imageTag" rules={[{ required: true, message: "请输入镜像 Tag" }]}>
                    <Input placeholder="例如 v0.2.1" />
                  </Form.Item>
                  <Form.Item label="镜像 Digest" name="imageDigest">
                    <Input placeholder="例如 sha256:..." />
                  </Form.Item>
                  <Form.Item label="目标节点" name="targetId">
                    <Select options={dockerNodeOptions} placeholder="为空则沿用当前目标节点" allowClear />
                  </Form.Item>
                </>
              )}
            </Form>
          </Space>
        </FormDrawer>

        <DangerConfirm
          open={Boolean(deleteTarget)}
          title="删除服务定义"
          description={`删除后将移除 ${deleteTarget?.name ?? ""} 的服务定义记录。若仍存在关联实例或发布引用，删除请求可能被拒绝。`}
          confirmText={deleteTarget?.code}
          loading={deleteMutation.isPending}
          onCancel={() => setDeleteTarget(null)}
          onConfirm={() => {
            if (deleteTarget) {
              deleteMutation.mutate(deleteTarget.id);
            }
          }}
        />
      </Space>
    </PermissionGuard>
  );
}
