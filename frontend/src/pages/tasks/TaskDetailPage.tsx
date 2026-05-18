import { App as AntApp, Alert, Button, Card, Descriptions, List, Space, Tag, Typography } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { ErrorState } from "../../components/ErrorState";
import { LogViewer } from "../../components/LogViewer";
import { PageHeader } from "../../components/PageHeader";
import { PermissionActionButton } from "../../components/PermissionActionButton";
import { PermissionGuard } from "../../components/PermissionGuard";
import { StatusBadge } from "../../components/StatusBadge";
import { TaskStatus } from "../../components/TaskStatus";
import { ResourceActivityList } from "../../components/resource/ResourceActivityList";
import { resourcesApi, tasksApi } from "../../lib/api";
import { getErrorMessage } from "../../lib/forms";
import { formatDateTime } from "../../lib/format";
import { queryKeys } from "../../lib/queryKeys";
import {
  buildAlertEventsPath,
  buildAuditsPath,
  buildResourcePath,
  buildTasksPath,
  formatTaskResourceName,
  getResourceTypeLabel,
} from "../../lib/resourceNavigation";
import { formatTaskExecutionPolicy, getTaskDispatchSourceMeta } from "../../lib/taskPresentation";
import type { Task } from "../../types/models";

export function TaskDetailPage() {
  const { message } = AntApp.useApp();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { taskId = "" } = useParams();

  const taskQuery = useQuery({
    queryKey: queryKeys.task(taskId),
    queryFn: () => tasksApi.detail(taskId),
    enabled: Boolean(taskId),
    refetchInterval: ({ state }) => {
      const task = state.data as Task | undefined;
      return task && ["SUCCESS", "FAILED", "CANCELED"].includes(task.status) ? false : 3000;
    },
  });

  const task = taskQuery.data;
  const resourceType = task?.resourceType;
  const resourceId = task?.resourceId;

  const resourceContextQuery = useQuery({
    queryKey: queryKeys.resourceContext(resourceType ?? "", resourceId ?? ""),
    queryFn: () => resourcesApi.context(resourceType ?? "", resourceId ?? ""),
    enabled: Boolean(resourceType && resourceId),
  });

  const cancelMutation = useMutation({
    mutationFn: () => tasksApi.cancel(taskId),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.tasks }),
        queryClient.invalidateQueries({ queryKey: queryKeys.task(taskId) }),
      ]);
      await message.success("任务已取消");
    },
    onError: async (error) => {
      void message.error(getErrorMessage(error, "取消任务失败"));
    },
  });

  const retryMutation = useMutation({
    mutationFn: () => tasksApi.retry(taskId),
    onSuccess: async (nextTask) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.tasks });
      await message.success("已创建重试任务");
      navigate(`/tasks/${nextTask.id}`);
    },
    onError: async (error) => {
      void message.error(getErrorMessage(error, "重试任务失败"));
    },
  });

  const relatedTasks = useMemo(
    () => (resourceContextQuery.data?.recentTasks ?? []).filter((item) => item.id !== taskId).slice(0, 5),
    [resourceContextQuery.data?.recentTasks, taskId],
  );
  const relatedAudits = useMemo(() => (resourceContextQuery.data?.recentAudits ?? []).slice(0, 6), [resourceContextQuery.data?.recentAudits]);
  const relatedAlerts = useMemo(() => (resourceContextQuery.data?.recentAlerts ?? []).slice(0, 6), [resourceContextQuery.data?.recentAlerts]);

  if (taskQuery.isError) {
    return <ErrorState message={taskQuery.error.message} onRetry={() => void taskQuery.refetch()} />;
  }

  const resourcePath = buildResourcePath(resourceType, resourceId);
  const canCancel = task?.status === "PENDING" || task?.status === "RUNNING";
  const canRetry = task?.status === "FAILED" || task?.status === "CANCELED";
  const hasResourceContext = Boolean(resourceType && resourceId);
  const latestAlert = relatedAlerts[0];

  return (
    <PermissionGuard permission="tasks.view" forbiddenPage>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <PageHeader
          title="任务详情"
          description="查看调度来源、执行步骤、日志，并把关联资源的最近动作一起放回同一视图里。"
          eyebrow="执行链路 / 单任务上下文"
          extra={
            <Space wrap>
              <PermissionActionButton
                permission="tasks.retry"
                permissionReason="当前账号缺少 tasks.retry 权限，无法重新调度失败任务。"
                disabled={!canRetry}
                disabledReason="只有失败或已取消的任务才可以重试。"
                loading={retryMutation.isPending}
                onClick={() => retryMutation.mutate()}
              >
                重试任务
              </PermissionActionButton>
              <PermissionActionButton
                danger
                permission="tasks.cancel"
                permissionReason="当前账号缺少 tasks.cancel 权限，无法中止执行中的任务。"
                disabled={!canCancel}
                disabledReason="只有待执行或执行中的任务才可以取消。"
                loading={cancelMutation.isPending}
                onClick={() => cancelMutation.mutate()}
              >
                取消任务
              </PermissionActionButton>
              {resourcePath ? <Button onClick={() => navigate(resourcePath)}>回到资源</Button> : null}
              <Button onClick={() => navigate("/tasks")}>返回任务中心</Button>
            </Space>
          }
        />

        {latestAlert?.status === "OPEN" ? (
          <Card className="page-card">
            <Alert
              type="warning"
              showIcon
              message={`当前关联资源存在待处理告警：${latestAlert.summary || latestAlert.eventType}`}
              description={latestAlert.detail || "建议结合最近任务、审计和资源状态一起判断是否需要回滚或进一步排查。"}
              action={
                <Button
                  size="small"
                  onClick={() =>
                    navigate(
                      buildAlertEventsPath({
                        resourceType,
                        resourceId,
                        selected: latestAlert.id,
                      }),
                    )
                  }
                >
                  查看告警
                </Button>
              }
            />
          </Card>
        ) : null}

        <Card className="page-card" loading={taskQuery.isLoading}>
          {task ? (
            <Descriptions bordered size="small" column={4}>
              <Descriptions.Item label="类型">{task.type}</Descriptions.Item>
              <Descriptions.Item label="资源">{formatTaskResourceName(task)}</Descriptions.Item>
              <Descriptions.Item label="状态">
                <StatusBadge status={task.status} />
              </Descriptions.Item>
              <Descriptions.Item label="进度">{task.progress}%</Descriptions.Item>
              <Descriptions.Item label="触发来源">
                {task.dispatchSource ? (
                  <Tag color={getTaskDispatchSourceMeta(task.dispatchSource).color}>
                    {getTaskDispatchSourceMeta(task.dispatchSource).label}
                  </Tag>
                ) : (
                  "--"
                )}
              </Descriptions.Item>
              <Descriptions.Item label="调度状态">
                {task.dispatchStatus ? <StatusBadge status={task.dispatchStatus} /> : "--"}
              </Descriptions.Item>
              <Descriptions.Item label="执行策略" span={2}>
                {formatTaskExecutionPolicy(task)}
              </Descriptions.Item>
              <Descriptions.Item label="资源类型">
                {task.resourceType ? getResourceTypeLabel(task.resourceType) : "--"}
              </Descriptions.Item>
              <Descriptions.Item label="发起人">{task.initiatedBy || "--"}</Descriptions.Item>
              <Descriptions.Item label="创建时间">{formatDateTime(task.createdAt)}</Descriptions.Item>
              <Descriptions.Item label="开始时间">{formatDateTime(task.startedAt)}</Descriptions.Item>
              <Descriptions.Item label="入队时间">{formatDateTime(task.queuedAt)}</Descriptions.Item>
              <Descriptions.Item label="结束时间">{formatDateTime(task.finishedAt)}</Descriptions.Item>
            </Descriptions>
          ) : null}
        </Card>

        {hasResourceContext ? (
          <Card className="page-card">
            <div className="page-toolbar">
              <Space direction="vertical" size={2}>
                <Typography.Text strong>关联资源上下文</Typography.Text>
                <Typography.Text type="secondary">
                  这里补充当前任务对应资源的最近任务、审计和告警，方便判断问题是单次失败还是持续性异常。
                </Typography.Text>
              </Space>
              <Space wrap>
                <Button onClick={() => navigate(buildTasksPath({ resourceType, resourceId }))}>查看该资源全部任务</Button>
                <Button onClick={() => navigate(buildAuditsPath({ resourceType, resourceId }))}>查看该资源全部审计</Button>
              </Space>
            </div>

            <div className="task-context-grid" style={{ marginTop: 12 }}>
              <ResourceActivityList
                title="最近任务"
                helper="优先看同一资源最近是否连续出现失败、取消或重复重试。"
                items={relatedTasks.map((item) => ({
                  key: item.id,
                  title: item.type,
                  description: item.summary ?? item.target,
                  meta: `${item.initiatedBy} · ${formatDateTime(item.createdAt)}`,
                  extra: <TaskStatus task={item} />,
                }))}
                emptyText="当前资源没有更多关联任务。"
              />

              <ResourceActivityList
                title="最近审计"
                helper="对齐最近变更发生时间，判断异常是否紧随配置或发布动作出现。"
                items={relatedAudits.map((audit) => ({
                  key: audit.id,
                  title: audit.action,
                  description: audit.summary,
                  meta: `${audit.actor} · ${formatDateTime(audit.createdAt)}`,
                  extra: <StatusBadge status={audit.result} />,
                }))}
                emptyText="当前资源还没有关联审计记录。"
              />
            </div>
          </Card>
        ) : null}

        {hasResourceContext ? (
          <Card className="page-card">
            <ResourceActivityList
              title="最近告警"
              helper="如果当前资源最近有告警，这里能直接看到严重级别、处理状态和回滚建议。"
              actionLabel="进入告警中心"
              onActionClick={() => navigate(buildAlertEventsPath({ resourceType, resourceId }))}
              items={relatedAlerts.map((event) => ({
                key: event.id,
                title: event.summary || event.eventType,
                description: event.detail || event.resourceName || "--",
                meta: `${formatDateTime(event.lastTriggeredAt)}${event.suggestedRollbackVersion ? ` · 回滚建议 ${event.suggestedRollbackVersion}` : ""}`,
                extra: (
                  <Space size={8}>
                    <StatusBadge status={event.severity} />
                    <StatusBadge status={event.status} />
                  </Space>
                ),
              }))}
              emptyText="当前资源最近没有告警事件。"
            />
          </Card>
        ) : null}

        <Card className="page-card" title="执行步骤" loading={taskQuery.isLoading}>
          <List
            dataSource={task?.steps ?? []}
            locale={{ emptyText: "暂无步骤信息" }}
            renderItem={(step) => {
              const detail = step.detail?.trim();
              const timeline = [formatDateTime(step.startedAt), formatDateTime(step.finishedAt)]
                .filter((value) => value && value !== "--")
                .join(" -> ");
              const description = [detail, timeline].filter(Boolean).join(" · ");
              return (
                <List.Item>
                  <List.Item.Meta
                    title={step.title}
                    description={
                      <Typography.Text type={step.status === "FAILED" ? "danger" : "secondary"}>
                        {description || "--"}
                      </Typography.Text>
                    }
                  />
                  <StatusBadge status={step.status} />
                </List.Item>
              );
            }}
          />
        </Card>

        <Card className="page-card" title="执行日志" loading={taskQuery.isLoading}>
          <LogViewer lines={task?.logs ?? []} />
        </Card>
      </Space>
    </PermissionGuard>
  );
}
