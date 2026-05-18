import { App as AntApp, Button, Card, Input, Select, Space, Tag, Typography } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { DataTable } from "../../components/DataTable";
import { DangerConfirm } from "../../components/DangerConfirm";
import { PageHeader } from "../../components/PageHeader";
import { PermissionActionButton } from "../../components/PermissionActionButton";
import { PermissionGuard } from "../../components/PermissionGuard";
import { TaskStatus } from "../../components/TaskStatus";
import { tasksApi } from "../../lib/api";
import { getErrorMessage } from "../../lib/forms";
import { formatDateTime } from "../../lib/format";
import { buildResourcePath, formatTaskResourceName, getResourceTypeLabel, normalizeResourceType } from "../../lib/resourceNavigation";
import { formatTaskExecutionPolicy, getTaskDispatchSourceMeta, taskDispatchSourceOptions } from "../../lib/taskPresentation";
import { queryKeys } from "../../lib/queryKeys";
import type { Task } from "../../types/models";

const statusOptions: Array<{ label: string; value: Task["status"] }> = [
  { label: "运行中", value: "RUNNING" },
  { label: "待执行", value: "PENDING" },
  { label: "成功", value: "SUCCESS" },
  { label: "失败", value: "FAILED" },
  { label: "已取消", value: "CANCELED" },
];

export function TasksPage() {
  const { message } = AntApp.useApp();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [searchParams, setSearchParams] = useSearchParams();
  const [cancelTarget, setCancelTarget] = useState<Task | null>(null);
  const keyword = searchParams.get("keyword") ?? "";
  const statusFilter = searchParams.get("status") ?? "";
  const taskTypeFilter = searchParams.get("taskType") ?? "";
  const resourceTypeFilter = searchParams.get("resourceType") ?? "";
  const resourceIdFilter = searchParams.get("resourceId") ?? "";
  const dispatchSourceFilter = searchParams.get("dispatchSource") ?? "";

  const tasksQuery = useQuery({
    queryKey: ["tasks", statusFilter, resourceTypeFilter, resourceIdFilter, dispatchSourceFilter],
    queryFn: () =>
      tasksApi.list({
        status: statusFilter || undefined,
        resourceType: resourceTypeFilter || undefined,
        resourceId: resourceIdFilter || undefined,
      }),
    refetchInterval: ({ state }) => {
      const tasks = state.data as Task[] | undefined;
      return tasks?.some((task) => task.status === "RUNNING" || task.status === "PENDING") ? 3000 : false;
    },
  });

  const cancelMutation = useMutation({
    mutationFn: (taskId: string) => tasksApi.cancel(taskId),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.tasks });
      setCancelTarget(null);
      await message.success("任务已取消");
    },
    onError: async (error) => {
      void message.error(getErrorMessage(error, "取消任务失败"));
    },
  });

  const retryMutation = useMutation({
    mutationFn: (taskId: string) => tasksApi.retry(taskId),
    onSuccess: async (task) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.tasks });
      await message.success("已创建重试任务");
      navigate(`/tasks/${task.id}`);
    },
    onError: async (error) => {
      void message.error(getErrorMessage(error, "重试任务失败"));
    },
  });

  const taskTypeOptions = useMemo(() => {
    return Array.from(new Set((tasksQuery.data ?? []).map((task) => task.type).filter(Boolean))).map((type) => ({
      label: type,
      value: type,
    }));
  }, [tasksQuery.data]);

  const resourceTypeOptions = useMemo(() => {
    const items = new Map<string, string>();
    for (const task of tasksQuery.data ?? []) {
      const normalized = normalizeResourceType(task.resourceType);
      if (!normalized) {
        continue;
      }
      items.set(normalized, task.resourceType || normalized);
    }
    return Array.from(items.entries()).map(([value, original]) => ({
      label: getResourceTypeLabel(original),
      value,
    }));
  }, [tasksQuery.data]);

  const filteredData = useMemo(() => {
    let items = tasksQuery.data ?? [];

    if (statusFilter) {
      items = items.filter((task) => task.status === statusFilter);
    }
    if (taskTypeFilter) {
      items = items.filter((task) => task.type === taskTypeFilter);
    }
    if (resourceTypeFilter) {
      items = items.filter(
        (task) => normalizeResourceType(task.resourceType) === normalizeResourceType(resourceTypeFilter),
      );
    }
    if (resourceIdFilter) {
      const normalizedResourceId = resourceIdFilter.toLowerCase();
      items = items.filter((task) =>
        `${task.resourceId ?? ""} ${task.target} ${task.summary ?? ""}`.toLowerCase().includes(normalizedResourceId),
      );
    }
    if (dispatchSourceFilter) {
      items = items.filter((task) => task.dispatchSource === dispatchSourceFilter);
    }
    if (!keyword.trim()) {
      return items;
    }
    const normalizedKeyword = keyword.trim().toLowerCase();
    return items.filter((task) =>
      `${task.type} ${task.target} ${task.initiatedBy} ${task.summary ?? ""} ${task.resourceId ?? ""}`
        .toLowerCase()
        .includes(normalizedKeyword),
    );
  }, [dispatchSourceFilter, keyword, resourceIdFilter, resourceTypeFilter, statusFilter, taskTypeFilter, tasksQuery.data]);

  const runningCount = filteredData.filter((task) => task.status === "RUNNING" || task.status === "PENDING").length;
  const failedCount = filteredData.filter((task) => task.status === "FAILED" || task.status === "CANCELED").length;
  const scheduledCount = filteredData.filter((task) => task.dispatchSource === "SCHEDULED").length;
  const systemCount = filteredData.filter((task) => task.dispatchSource === "SYSTEM").length;

  const resourcePath = buildResourcePath(resourceTypeFilter, resourceIdFilter);

  function setFilter(key: string, value?: string) {
    setSearchParams((previous) => {
      const next = new URLSearchParams(previous);
      const normalized = value?.trim() ?? "";
      if (normalized) {
        next.set(key, normalized);
      } else {
        next.delete(key);
      }
      return next;
    });
  }

  function clearFilters() {
    setSearchParams((previous) => {
      const next = new URLSearchParams(previous);
      next.delete("keyword");
      next.delete("status");
      next.delete("taskType");
      next.delete("resourceType");
      next.delete("resourceId");
      next.delete("dispatchSource");
      return next;
    });
  }

  function canCancel(task: Task) {
    return task.status === "PENDING" || task.status === "RUNNING";
  }

  function canRetry(task: Task) {
    return task.status === "FAILED" || task.status === "CANCELED";
  }

  return (
    <PermissionGuard permission="tasks.view" forbiddenPage>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <PageHeader
          title="任务中心"
          description="在这里按来源、状态、资源维度追踪执行链路，并直接处理取消与重试。"
          extra={
            <Space wrap>
              {resourcePath ? <Button onClick={() => navigate(resourcePath)}>回到资源</Button> : null}
              <Button onClick={clearFilters}>清空筛选</Button>
            </Space>
          }
        />

        <Card className="page-card">
          <div className="metric-grid" style={{ marginBottom: 16 }}>
            <Card size="small">
              <Typography.Text type="secondary">当前结果集</Typography.Text>
              <Typography.Title level={4} style={{ margin: "8px 0 0" }}>
                {filteredData.length}
              </Typography.Title>
            </Card>
            <Card size="small">
              <Typography.Text type="secondary">运行中 / 待执行</Typography.Text>
              <Typography.Title level={4} style={{ margin: "8px 0 0" }}>
                {runningCount}
              </Typography.Title>
            </Card>
            <Card size="small">
              <Typography.Text type="secondary">失败 / 已取消</Typography.Text>
              <Typography.Title level={4} style={{ margin: "8px 0 0" }}>
                {failedCount}
              </Typography.Title>
            </Card>
            <Card size="small">
              <Typography.Text type="secondary">定时 / 系统触发</Typography.Text>
              <Typography.Title level={4} style={{ margin: "8px 0 0" }}>
                {scheduledCount} / {systemCount}
              </Typography.Title>
            </Card>
          </div>
          <div className="page-toolbar">
            <div className="page-toolbar-start">
              <Input.Search
                allowClear
                placeholder="搜索任务类型、资源、发起人或摘要"
                style={{ width: 320 }}
                value={keyword}
                onChange={(event) => setFilter("keyword", event.target.value)}
                onSearch={(value) => setFilter("keyword", value)}
              />
              <Select
                allowClear
                placeholder="状态"
                style={{ width: 140 }}
                value={statusFilter || undefined}
                options={statusOptions}
                onChange={(value) => setFilter("status", value)}
              />
              <Select
                allowClear
                placeholder="触发来源"
                style={{ width: 160 }}
                value={dispatchSourceFilter || undefined}
                options={taskDispatchSourceOptions}
                onChange={(value) => setFilter("dispatchSource", value)}
              />
              <Select
                allowClear
                placeholder="任务类型"
                style={{ width: 180 }}
                value={taskTypeFilter || undefined}
                options={taskTypeOptions}
                onChange={(value) => setFilter("taskType", value)}
              />
              <Select
                allowClear
                placeholder="资源类型"
                style={{ width: 160 }}
                value={resourceTypeFilter || undefined}
                options={resourceTypeOptions}
                onChange={(value) => setFilter("resourceType", value)}
              />
              <Input
                allowClear
                placeholder="资源 ID"
                style={{ width: 220 }}
                value={resourceIdFilter}
                onChange={(event) => setFilter("resourceId", event.target.value)}
              />
            </div>
          </div>
        </Card>

        <Card className="page-card">
          <DataTable
            rowKey="id"
            loading={tasksQuery.isLoading}
            dataSource={filteredData}
            columns={[
              {
                title: "任务",
                dataIndex: "type",
                render: (_, task) => (
                  <Space direction="vertical" size={2}>
                    <Space wrap size={[8, 4]}>
                      <span>{task.type}</span>
                      {task.dispatchSource ? (
                        <Tag color={getTaskDispatchSourceMeta(task.dispatchSource).color}>
                          {getTaskDispatchSourceMeta(task.dispatchSource).label}
                        </Tag>
                      ) : null}
                      {task.dispatchStatus ? <Tag>{task.dispatchStatus}</Tag> : null}
                    </Space>
                    <span style={{ color: "#64748b" }}>{task.summary ?? "--"}</span>
                  </Space>
                ),
              },
              {
                title: "资源",
                key: "resource",
                render: (_, task) => (
                  <Space direction="vertical" size={4}>
                    <span>{formatTaskResourceName(task)}</span>
                    {task.resourceType ? <Tag>{getResourceTypeLabel(task.resourceType)}</Tag> : null}
                  </Space>
                ),
              },
              {
                title: "发起人",
                render: (_, task) => (
                  <Space direction="vertical" size={2}>
                    <span>{task.initiatedBy || "--"}</span>
                    <Typography.Text type="secondary">
                      {task.queuedAt ? `入队 ${formatDateTime(task.queuedAt)}` : "未记录入队时间"}
                    </Typography.Text>
                  </Space>
                ),
              },
              {
                title: "创建时间",
                dataIndex: "createdAt",
                render: (value: string) => formatDateTime(value),
              },
              {
                title: "执行策略",
                key: "policy",
                render: (_, task) => (
                  <Space direction="vertical" size={2}>
                    <span>{formatTaskExecutionPolicy(task)}</span>
                    <Typography.Text type="secondary">
                      {task.dispatchStatus ? `调度状态 ${task.dispatchStatus}` : "未记录调度状态"}
                    </Typography.Text>
                  </Space>
                ),
              },
              {
                title: "状态",
                key: "status",
                render: (_, task) => <TaskStatus task={task} />,
              },
              {
                title: "操作",
                key: "actions",
                render: (_, task) => {
                  const targetPath = buildResourcePath(task.resourceType, task.resourceId);
                  return (
                    <Space size={0} wrap>
                      <Button type="link" onClick={() => navigate(`/tasks/${task.id}`)}>
                        详情
                      </Button>
                      {targetPath ? (
                        <Button type="link" onClick={() => navigate(targetPath)}>
                          资源
                        </Button>
                      ) : null}
                      <PermissionActionButton
                        type="link"
                        permission="tasks.retry"
                        permissionReason="当前账号缺少 tasks.retry 权限，无法重新调度失败任务。"
                        disabled={!canRetry(task)}
                        disabledReason="只有失败或已取消的任务才可以重试。"
                        loading={retryMutation.isPending}
                        onClick={() => retryMutation.mutate(task.id)}
                      >
                        重试
                      </PermissionActionButton>
                      <PermissionActionButton
                        type="link"
                        danger
                        permission="tasks.cancel"
                        permissionReason="当前账号缺少 tasks.cancel 权限，无法中止执行中的任务。"
                        disabled={!canCancel(task)}
                        disabledReason="只有待执行或执行中的任务才可以取消。"
                        onClick={() => setCancelTarget(task)}
                      >
                        取消
                      </PermissionActionButton>
                    </Space>
                  );
                },
              },
            ]}
          />
        </Card>

        <DangerConfirm
          open={Boolean(cancelTarget)}
          title="取消任务"
          description={`任务 ${cancelTarget?.type ?? ""} 仍在执行链路中，取消后会将其标记为已取消。`}
          confirmText={cancelTarget?.id}
          loading={cancelMutation.isPending}
          onCancel={() => setCancelTarget(null)}
          onConfirm={() => {
            if (cancelTarget) {
              cancelMutation.mutate(cancelTarget.id);
            }
          }}
        />
      </Space>
    </PermissionGuard>
  );
}
