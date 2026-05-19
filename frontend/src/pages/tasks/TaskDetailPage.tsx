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
import { exportsApi, tasksApi } from "../../lib/api";
import { getErrorMessage } from "../../lib/forms";
import { formatDateTime } from "../../lib/format";
import { queryKeys } from "../../lib/queryKeys";
import {
  buildAlertEventsPath,
  buildResourcePath,
  formatTaskResourceName,
  getResourceTypeLabel,
} from "../../lib/resourceNavigation";
import { formatTaskExecutionPolicy, getTaskDispatchSourceMeta } from "../../lib/taskPresentation";
import { useSessionStore } from "../../store/sessionStore";
import type { NotificationRecord, ResourceActionHint, ResourceRisk, Task } from "../../types/models";

const riskLevelMeta: Record<string, { tone: "success" | "warning" | "error"; title: string }> = {
  critical: { tone: "error", title: "当前任务关联资源存在高优先级风险" },
  warning: { tone: "warning", title: "当前任务关联资源存在待跟进风险" },
  normal: { tone: "success", title: "当前任务关联资源整体稳定" },
};

function getRiskAlertType(level?: string) {
  if (level === "critical") {
    return "error" as const;
  }
  if (level === "warning") {
    return "warning" as const;
  }
  return "success" as const;
}

function buildRiskHighlights(risk: ResourceRisk) {
  return [
    {
      label: "待处理告警",
      value: risk.openAlertCount,
      helper: risk.openAlertCount > 0 ? "建议先确认影响范围，再决定是否继续操作。" : "当前没有打开中的告警事件。",
    },
    {
      label: "失败任务",
      value: risk.failedTaskCount,
      helper: risk.failedTaskCount > 0 ? "说明这不是一次孤立执行，建议回看最近失败链路。" : "当前没有新的失败任务堆积。",
    },
    {
      label: "高风险审计",
      value: risk.highRiskAuditCount,
      helper: risk.highRiskAuditCount > 0 ? "建议结合最近变更判断异常是否由操作触发。" : "最近没有突出的高风险操作。",
    },
  ];
}

function buildActionButtonType(action: ResourceActionHint) {
  return action.kind === "primary" ? "primary" : "default";
}

function summarizeNotification(record: NotificationRecord) {
  if (record.errorMessage?.trim()) {
    return record.errorMessage;
  }
  if (record.responseExcerpt?.trim()) {
    return record.responseExcerpt;
  }
  if (record.providerMessageId?.trim()) {
    return `providerMessageId: ${record.providerMessageId}`;
  }
  return "已记录通知发送结果。";
}

export function TaskDetailPage() {
  const { message } = AntApp.useApp();
  const permissions = useSessionStore((state) => state.permissions);
  const canViewExports = permissions.includes("*") || permissions.includes("exports.view");
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

  const taskContextQuery = useQuery({
    queryKey: [...queryKeys.task(taskId), "context"],
    queryFn: () => tasksApi.context(taskId),
    enabled: Boolean(taskId),
  });

  const cancelMutation = useMutation({
    mutationFn: () => tasksApi.cancel(taskId),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.tasks }),
        queryClient.invalidateQueries({ queryKey: queryKeys.task(taskId) }),
        queryClient.invalidateQueries({ queryKey: [...queryKeys.task(taskId), "context"] }),
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

  const exportMutation = useMutation({
    mutationFn: () => exportsApi.exportTask(taskId),
    onSuccess: async (job) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.exports });
      await message.success("任务导出已创建");
      if (canViewExports) {
        navigate(`/settings/exports?selected=${encodeURIComponent(job.id)}`);
      }
    },
    onError: async (error) => {
      void message.error(getErrorMessage(error, "创建任务导出失败"));
    },
  });

  const taskContext = taskContextQuery.data;
  const navigation = taskContext?.navigation;
  const risk = taskContext?.risk;
  const relatedTasks = useMemo(
    () => (taskContext?.relatedTasks ?? []).filter((item) => item.id !== taskId).slice(0, 5),
    [taskContext?.relatedTasks, taskId],
  );
  const contextAudits = taskContext?.relatedAudits ?? [];
  const relatedAlerts = taskContext?.relatedAlerts ?? [];
  const relatedNotifications = taskContext?.notifications ?? [];

  if (taskQuery.isError) {
    return <ErrorState message={taskQuery.error.message} onRetry={() => void taskQuery.refetch()} />;
  }
  if (taskContextQuery.isError) {
    return <ErrorState message={taskContextQuery.error.message} onRetry={() => void taskContextQuery.refetch()} />;
  }

  const resourcePath = navigation?.detailPath || buildResourcePath(resourceType, resourceId);
  const canCancel = task?.status === "PENDING" || task?.status === "RUNNING";
  const canRetry = task?.status === "FAILED" || task?.status === "CANCELED";
  const hasResourceContext = Boolean(resourceType && resourceId);
  const latestAlert = relatedAlerts[0];
  const riskMeta = riskLevelMeta[risk?.level ?? "normal"] ?? riskLevelMeta.normal;
  const riskHighlights = risk ? buildRiskHighlights(risk) : [];
  const nextActions = taskContext?.nextActions ?? [];

  return (
    <PermissionGuard permission="tasks.view" forbiddenPage>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <PageHeader
          title="任务详情"
          description="查看调度来源、执行步骤、失败摘要，并把当前任务可继续采取的动作收在同一页里。"
          eyebrow="执行链路 / 单任务上下文"
          extra={
            <Space wrap>
              <PermissionActionButton
                permission="exports.create"
                permissionReason="当前账号缺少 exports.create 权限，无法创建任务导出。"
                loading={exportMutation.isPending}
                onClick={() => exportMutation.mutate()}
              >
                导出任务包
              </PermissionActionButton>
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

        {taskContext?.failureSummary ? (
          <Card className="page-card">
            <Alert
              type={task?.status === "FAILED" ? "error" : "info"}
              showIcon
              message={task?.status === "FAILED" ? "失败摘要" : "当前任务上下文摘要"}
              description={taskContext.failureSummary}
            />
          </Card>
        ) : null}

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

        {risk ? (
          <Card className="page-card" loading={taskContextQuery.isLoading}>
            <Space direction="vertical" size={16} style={{ width: "100%" }}>
              <div className="page-toolbar">
                <Space direction="vertical" size={2}>
                  <Typography.Text strong>风险与处置建议</Typography.Text>
                  <Typography.Text type="secondary">
                    用任务上下文判断这是单次失败、持续性风险，还是由最近变更触发的问题。
                  </Typography.Text>
                </Space>
              </div>

              <Alert
                type={getRiskAlertType(risk.level)}
                showIcon
                message={riskMeta.title}
                description={risk.summary}
              />

              <div className="resource-detail-highlights">
                {riskHighlights.map((item) => (
                  <div key={item.label} className="resource-highlight-card">
                    <Typography.Text className="resource-highlight-label">{item.label}</Typography.Text>
                    <div className="resource-highlight-value">{item.value}</div>
                    <div className="resource-highlight-helper">{item.helper}</div>
                  </div>
                ))}
              </div>

              {risk.lastFailureReason ? (
                <div className="resource-callout">
                  <Space direction="vertical" size={4}>
                    <Typography.Text strong>最近失败原因</Typography.Text>
                    <Typography.Text type="secondary">{risk.lastFailureReason}</Typography.Text>
                  </Space>
                </div>
              ) : null}

              {nextActions.length ? (
                <div className="resource-detail-section">
                  <div className="page-toolbar">
                    <Space direction="vertical" size={2}>
                      <Typography.Text strong>下一步动作</Typography.Text>
                      <Typography.Text type="secondary">
                        这些动作由后端按资源类型和当前状态给出，适合直接继续排查或回到资源工作台。
                      </Typography.Text>
                    </Space>
                  </div>
                  <Space wrap>
                    {nextActions.map((action) => (
                      <PermissionActionButton
                        key={action.key}
                        permission={action.permission}
                        permissionReason={
                          action.permission ? `当前账号缺少 ${action.permission} 权限，无法执行该动作。` : undefined
                        }
                        type={buildActionButtonType(action)}
                        onClick={() => {
                          if (action.path) {
                            navigate(action.path);
                            return;
                          }
                          if (navigation?.detailPath) {
                            navigate(navigation.detailPath);
                          }
                        }}
                      >
                        {action.label}
                      </PermissionActionButton>
                    ))}
                  </Space>
                  <Space direction="vertical" size={8} style={{ width: "100%", marginTop: 12 }}>
                    {nextActions.map((action) => (
                      <Typography.Text key={`${action.key}-reason`} type="secondary">
                        {`${action.label}：${action.reason || "建议在对应资源工作台继续处理。"}`}
                      </Typography.Text>
                    ))}
                  </Space>
                </div>
              ) : null}
            </Space>
          </Card>
        ) : null}

        <Card className="page-card" loading={taskQuery.isLoading || taskContextQuery.isLoading}>
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
                  这里直接展示和当前任务关联的资源、导航入口、最近审计与告警，不再只停留在通用列表页。
                </Typography.Text>
              </Space>
              <Space wrap>
                {navigation?.tasksPath ? <Button onClick={() => navigate(navigation.tasksPath)}>查看该资源全部任务</Button> : null}
                {navigation?.auditsPath ? <Button onClick={() => navigate(navigation.auditsPath)}>查看该资源全部审计</Button> : null}
                {navigation?.alertsPath ? <Button onClick={() => navigate(navigation.alertsPath)}>查看该资源全部告警</Button> : null}
              </Space>
            </div>

            <div className="resource-detail-metadata" style={{ marginTop: 12 }}>
              <div className="resource-detail-metadata-item">
                <Typography.Text type="secondary" className="resource-detail-metadata-label">
                  资源名称
                </Typography.Text>
                <div className="resource-detail-metadata-value">{taskContext?.resource?.name || (task ? formatTaskResourceName(task) : "--")}</div>
              </div>
              <div className="resource-detail-metadata-item">
                <Typography.Text type="secondary" className="resource-detail-metadata-label">
                  资源状态
                </Typography.Text>
                <div className="resource-detail-metadata-value">
                  <StatusBadge status={taskContext?.resource?.status || "UNKNOWN"} />
                </div>
              </div>
              <div className="resource-detail-metadata-item">
                <Typography.Text type="secondary" className="resource-detail-metadata-label">
                  资源入口
                </Typography.Text>
                <div className="resource-detail-metadata-value">{taskContext?.resource?.endpoint || taskContext?.resource?.resourceId || "--"}</div>
              </div>
              <div className="resource-detail-metadata-item">
                <Typography.Text type="secondary" className="resource-detail-metadata-label">
                  最近更新时间
                </Typography.Text>
                <div className="resource-detail-metadata-value">{formatDateTime(taskContext?.resource?.updatedAt)}</div>
              </div>
            </div>
          </Card>
        ) : null}

        {relatedTasks.length ? (
          <Card className="page-card">
            <ResourceActivityList
              title="相关任务"
              helper="从任务上下文直接回看同一资源最近的其它执行链路，判断是否连续失败或已经有恢复动作。"
              actionLabel={navigation?.tasksPath ? "进入任务中心" : undefined}
              onActionClick={navigation?.tasksPath ? () => navigate(navigation.tasksPath) : undefined}
              items={relatedTasks.map((item) => ({
                key: item.id,
                title: item.type,
                description: item.summary ?? item.target,
                meta: `${item.initiatedBy} · ${formatDateTime(item.createdAt)}`,
                extra: <TaskStatus task={item} />,
              }))}
              emptyText="当前资源没有更多关联任务。"
            />
          </Card>
        ) : null}

        {hasResourceContext ? (
          <Card className="page-card">
            <ResourceActivityList
              title="最近告警"
              helper="如果当前资源最近有告警，这里能直接看到严重级别、处理状态和回滚建议。"
              actionLabel="进入告警中心"
              onActionClick={() => navigate(navigation?.alertsPath || buildAlertEventsPath({ resourceType, resourceId }))}
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

        {contextAudits.length ? (
          <Card className="page-card">
            <ResourceActivityList
              title="最近审计"
              helper="对齐最近变更发生时间，判断异常是否紧随配置、发布或权限操作出现。"
              actionLabel={navigation?.auditsPath ? "进入审计中心" : undefined}
              onActionClick={navigation?.auditsPath ? () => navigate(navigation.auditsPath) : undefined}
              items={contextAudits.map((audit) => ({
                key: audit.id,
                title: audit.action,
                description: audit.summary,
                meta: `${audit.actor} · ${formatDateTime(audit.createdAt)}`,
                extra: <StatusBadge status={audit.result} />,
              }))}
              emptyText="当前资源还没有关联审计记录。"
            />
          </Card>
        ) : null}

        {relatedNotifications.length ? (
          <Card className="page-card">
            <ResourceActivityList
              title="通知记录"
              helper="这里保留当前任务关联告警的发送结果，方便确认是否已经真正触达到通道。"
              items={relatedNotifications.map((record) => ({
                key: record.id,
                title: record.channelName ? `${record.channelName} · ${record.channelType}` : record.channelType,
                description: summarizeNotification(record),
                meta: `${formatDateTime(record.createdAt)}${record.finishedAt ? ` · 完成于 ${formatDateTime(record.finishedAt)}` : ""}`,
                extra: <StatusBadge status={record.status} />,
              }))}
              emptyText="当前任务还没有关联通知记录。"
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
