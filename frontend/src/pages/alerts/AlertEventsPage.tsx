import { App as AntApp, Button, Card, Select, Space, Typography } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";

import { DataTable } from "../../components/DataTable";
import { ErrorState } from "../../components/ErrorState";
import { PageHeader } from "../../components/PageHeader";
import { PermissionGuard } from "../../components/PermissionGuard";
import { StatusBadge } from "../../components/StatusBadge";
import { ResourceActivityList } from "../../components/resource/ResourceActivityList";
import { ResourceDetailPanel } from "../../components/resource/ResourceDetailPanel";
import { alertsApi, resourcesApi } from "../../lib/api";
import { formatDateTime } from "../../lib/format";
import { queryKeys } from "../../lib/queryKeys";
import {
  buildAlertEventsPath,
  buildAuditsPath,
  buildResourcePath,
  buildTasksPath,
  getResourceTypeLabel,
} from "../../lib/resourceNavigation";
import { TaskStatus } from "../../components/TaskStatus";

const statusOptions = [
  { label: "全部状态", value: "" },
  { label: "待处理", value: "OPEN" },
  { label: "已确认", value: "ACKED" },
  { label: "已关闭", value: "RESOLVED" },
];

const eventTypeOptions = [
  { label: "全部事件", value: "" },
  { label: "服务发布失败", value: "service_release_failed" },
  { label: "服务健康检查失败", value: "service_health_check_failed" },
  { label: "Nginx 重载失败", value: "nginx_reload_failed" },
  { label: "Nginx 配置发布失败", value: "nginx_publish_failed" },
  { label: "主机离线", value: "host_offline" },
  { label: "主机恢复", value: "host_recovered" },
];

function getEventTypeLabel(value: string) {
  return eventTypeOptions.find((item) => item.value === value)?.label ?? value;
}

export function AlertEventsPage() {
  const { message } = AntApp.useApp();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const status = searchParams.get("status") ?? "";
  const eventType = searchParams.get("eventType") ?? "";
  const selectedId = searchParams.get("selected") ?? "";
  const resourceType = searchParams.get("resourceType") ?? "";
  const resourceId = searchParams.get("resourceId") ?? "";

  const eventsQuery = useQuery({
    queryKey: [...queryKeys.alertEvents, status, eventType, resourceType, resourceId],
    queryFn: () =>
      alertsApi.listEvents({
        status: status || undefined,
        eventType: eventType || undefined,
        resourceType: resourceType || undefined,
        resourceId: resourceId || undefined,
      }),
  });

  const recordsQuery = useQuery({
    queryKey: queryKeys.notificationRecords,
    queryFn: alertsApi.listRecords,
  });

  const selectedEvent = useMemo(() => {
    const items = eventsQuery.data ?? [];
    if (!items.length) {
      return null;
    }
    return items.find((item) => item.id === selectedId) ?? items[0];
  }, [eventsQuery.data, selectedId]);

  const contextQuery = useQuery({
    queryKey: queryKeys.resourceContext(selectedEvent?.resourceType ?? "", selectedEvent?.resourceId ?? ""),
    queryFn: () => resourcesApi.context(selectedEvent?.resourceType ?? "", selectedEvent?.resourceId ?? ""),
    enabled: Boolean(selectedEvent?.resourceType && selectedEvent?.resourceId),
  });

  const ackMutation = useMutation({
    mutationFn: alertsApi.ackEvent,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.alertEvents });
      await message.success("告警已确认");
    },
    onError: (error) => {
      void message.error(error instanceof Error ? error.message : "确认告警失败");
    },
  });

  const resolveMutation = useMutation({
    mutationFn: alertsApi.resolveEvent,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.alertEvents });
      await message.success("告警已关闭");
    },
    onError: (error) => {
      void message.error(error instanceof Error ? error.message : "关闭告警失败");
    },
  });

  const relatedRecordMap = useMemo(() => {
    const map = new Map<string, string>();
    for (const record of recordsQuery.data ?? []) {
      if (record.eventId && !map.has(record.eventId)) {
        map.set(record.eventId, record.status);
      }
    }
    return map;
  }, [recordsQuery.data]);

  const relatedTasks = useMemo(() => {
    const currentId = selectedEvent?.taskId;
    return (contextQuery.data?.recentTasks ?? []).filter((task) => task.id !== currentId).slice(0, 5);
  }, [contextQuery.data?.recentTasks, selectedEvent?.taskId]);

  const relatedAudits = useMemo(() => (contextQuery.data?.recentAudits ?? []).slice(0, 6), [contextQuery.data?.recentAudits]);
  const relatedAlerts = useMemo(
    () => (contextQuery.data?.recentAlerts ?? []).filter((event) => event.id !== selectedEvent?.id).slice(0, 6),
    [contextQuery.data?.recentAlerts, selectedEvent?.id],
  );

  if (eventsQuery.isError) {
    return <ErrorState message={eventsQuery.error.message} onRetry={() => void eventsQuery.refetch()} />;
  }

  return (
    <PermissionGuard permission="alerts.view" forbiddenPage>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <PageHeader
          title="告警事件"
          description="集中查看发布失败、健康检查失败、主机离线与 Nginx 异常，并在同一页继续处理。"
          eyebrow="稳定性 / 告警工作台"
          extra={
            selectedEvent?.resourceType && selectedEvent?.resourceId ? (
              <Button
                onClick={() =>
                  navigate(
                    buildResourcePath(selectedEvent.resourceType, selectedEvent.resourceId) ??
                      buildAlertEventsPath({
                        resourceType: selectedEvent.resourceType,
                        resourceId: selectedEvent.resourceId,
                      }),
                  )
                }
              >
                回到资源
              </Button>
            ) : null
          }
        />

        <Card className="page-card">
          <div className="page-toolbar">
            <div className="page-toolbar-start">
              <Select
                style={{ width: 160 }}
                options={statusOptions}
                value={status}
                onChange={(value) =>
                  setSearchParams((previous) => {
                    const next = new URLSearchParams(previous);
                    if (value) {
                      next.set("status", value);
                    } else {
                      next.delete("status");
                    }
                    next.delete("selected");
                    return next;
                  })
                }
              />
              <Select
                style={{ width: 220 }}
                options={eventTypeOptions}
                value={eventType}
                onChange={(value) =>
                  setSearchParams((previous) => {
                    const next = new URLSearchParams(previous);
                    if (value) {
                      next.set("eventType", value);
                    } else {
                      next.delete("eventType");
                    }
                    next.delete("selected");
                    return next;
                  })
                }
              />
            </div>
          </div>
        </Card>

        <div className="resource-workbench">
          <div className="resource-list-pane">
            <Card className="page-card">
              <DataTable
                rowKey="id"
                loading={eventsQuery.isLoading}
                dataSource={eventsQuery.data}
                rowClassName={(event) => (event.id === selectedEvent?.id ? "resource-row-selected" : "")}
                onRow={(event) => ({
                  onClick: () => {
                    setSearchParams((previous) => {
                      const next = new URLSearchParams(previous);
                      next.set("selected", event.id);
                      return next;
                    });
                  },
                })}
                columns={[
                  {
                    title: "事件",
                    dataIndex: "summary",
                    render: (_, event) => (
                      <Space direction="vertical" size={2}>
                        <Typography.Text strong>{event.summary || getEventTypeLabel(event.eventType)}</Typography.Text>
                        <Typography.Text type="secondary">{event.resourceName || event.resourceId || "--"}</Typography.Text>
                      </Space>
                    ),
                  },
                  {
                    title: "等级",
                    dataIndex: "severity",
                    render: (value: string) => <StatusBadge status={value} />,
                  },
                  {
                    title: "状态",
                    dataIndex: "status",
                    render: (value: string) => <StatusBadge status={value} />,
                  },
                  {
                    title: "通知",
                    key: "notification",
                    render: (_, event) => {
                      const recordStatus = event.notificationStatus || relatedRecordMap.get(event.id);
                      return recordStatus ? <StatusBadge status={recordStatus} /> : "--";
                    },
                  },
                  {
                    title: "触发时间",
                    dataIndex: "lastTriggeredAt",
                    render: (value: string) => formatDateTime(value),
                  },
                ]}
              />
            </Card>
          </div>

          <div className="resource-detail-pane">
            <ResourceDetailPanel
              title={selectedEvent?.summary || (selectedEvent ? getEventTypeLabel(selectedEvent.eventType) : undefined)}
              kicker={selectedEvent ? "告警上下文" : undefined}
              subtitle={
                selectedEvent
                  ? `${getResourceTypeLabel(selectedEvent.resourceType)} · ${selectedEvent.resourceName || selectedEvent.resourceId || "--"}`
                  : undefined
              }
              status={
                selectedEvent ? (
                  <Space size={8} wrap>
                    <StatusBadge status={selectedEvent.severity} />
                    <StatusBadge status={selectedEvent.status} />
                  </Space>
                ) : undefined
              }
              helper={
                selectedEvent
                  ? "在这里集中查看当前告警的资源背景、最近任务、审计和相关告警，并直接完成确认或关闭。"
                  : undefined
              }
              highlights={
                selectedEvent
                  ? [
                      {
                        label: "事件类型",
                        value: getEventTypeLabel(selectedEvent.eventType),
                        helper: selectedEvent.dedupeKey ? `去重键 ${selectedEvent.dedupeKey}` : "当前事件没有额外去重信息",
                      },
                      {
                        label: "最新触发",
                        value: formatDateTime(selectedEvent.lastTriggeredAt),
                        helper: selectedEvent.firstTriggeredAt ? `首次触发 ${formatDateTime(selectedEvent.firstTriggeredAt)}` : "仅记录了最新触发时间",
                      },
                      {
                        label: "关联任务",
                        value: selectedEvent.taskId ? 1 : 0,
                        helper: selectedEvent.taskId ? "当前告警已关联到一条任务链路" : "当前告警没有直接关联任务",
                      },
                    ]
                  : []
              }
              meta={
                selectedEvent
                  ? [
                      { label: "资源类型", value: getResourceTypeLabel(selectedEvent.resourceType) },
                      { label: "资源标识", value: selectedEvent.resourceName || selectedEvent.resourceId || "--" },
                      { label: "通知状态", value: selectedEvent.notificationStatus ? <StatusBadge status={selectedEvent.notificationStatus} /> : "--" },
                      { label: "任务 ID", value: selectedEvent.taskId || "--" },
                      { label: "建议回滚版本", value: selectedEvent.suggestedRollbackVersion || selectedEvent.suggestedRollbackVersionId || "--" },
                      { label: "关闭时间", value: formatDateTime(selectedEvent.resolvedAt) },
                    ]
                  : []
              }
              actions={
                selectedEvent ? (
                  <>
                    <div className="resource-action-group">
                      {selectedEvent.status === "OPEN" ? (
                        <PermissionGuard permission="alerts.ack">
                          <Button type="primary" loading={ackMutation.isPending} onClick={() => ackMutation.mutate(selectedEvent.id)}>
                            确认告警
                          </Button>
                        </PermissionGuard>
                      ) : null}
                      {selectedEvent.status !== "RESOLVED" ? (
                        <PermissionGuard permission="alerts.ack">
                          <Button loading={resolveMutation.isPending} onClick={() => resolveMutation.mutate(selectedEvent.id)}>
                            关闭告警
                          </Button>
                        </PermissionGuard>
                      ) : null}
                    </div>
                    <div className="resource-action-group">
                      {selectedEvent.taskId ? (
                        <Button onClick={() => navigate(`/tasks/${selectedEvent.taskId}`)}>查看任务</Button>
                      ) : null}
                      {selectedEvent.resourceType && selectedEvent.resourceId ? (
                        <Button
                          onClick={() =>
                            navigate(buildTasksPath({ resourceType: selectedEvent.resourceType, resourceId: selectedEvent.resourceId }))
                          }
                        >
                          查看该资源任务
                        </Button>
                      ) : null}
                      {selectedEvent.resourceType && selectedEvent.resourceId ? (
                        <Button
                          onClick={() =>
                            navigate(buildAuditsPath({ resourceType: selectedEvent.resourceType, resourceId: selectedEvent.resourceId }))
                          }
                        >
                          查看该资源审计
                        </Button>
                      ) : null}
                    </div>
                  </>
                ) : undefined
              }
            >
              {selectedEvent ? (
                <div className="resource-detail-section resource-callout">
                  <Space direction="vertical" size={6}>
                    <Typography.Text strong>详情说明</Typography.Text>
                    <Typography.Text type="secondary">{selectedEvent.detail || "当前告警未返回更详细的说明。"}</Typography.Text>
                  </Space>
                </div>
              ) : null}

              <ResourceActivityList
                title="最近任务"
                helper="优先看最近的失败、重试和恢复动作，判断这次告警是否已经进入修复链路。"
                items={relatedTasks.map((task) => ({
                  key: task.id,
                  title: task.type,
                  description: task.summary ?? task.target,
                  meta: `${task.initiatedBy} · ${formatDateTime(task.createdAt)}`,
                  extra: <TaskStatus task={task} />,
                }))}
                emptyText="当前资源没有更多关联任务。"
              />

              <ResourceActivityList
                title="最近审计"
                helper="对齐最近变更动作，帮助判断是否是发布、配置修改或资源调整带来的异常。"
                items={relatedAudits.map((audit) => ({
                  key: audit.id,
                  title: audit.action,
                  description: audit.summary,
                  meta: `${audit.actor} · ${formatDateTime(audit.createdAt)}`,
                  extra: <StatusBadge status={audit.result} />,
                }))}
                emptyText="当前资源还没有关联审计记录。"
              />

              <ResourceActivityList
                title="相关告警"
                helper="这里保留同一资源最近的其它告警，便于判断是单点问题还是连续性异常。"
                items={relatedAlerts.map((event) => ({
                  key: event.id,
                  title: event.summary || getEventTypeLabel(event.eventType),
                  description: event.detail || event.resourceName || "--",
                  meta: `${formatDateTime(event.lastTriggeredAt)}${event.suggestedRollbackVersion ? ` · 回滚建议 ${event.suggestedRollbackVersion}` : ""}`,
                  extra: (
                    <Space size={8}>
                      <StatusBadge status={event.severity} />
                      <StatusBadge status={event.status} />
                    </Space>
                  ),
                }))}
                emptyText="当前资源最近没有更多相关告警。"
              />
            </ResourceDetailPanel>
          </div>
        </div>
      </Space>
    </PermissionGuard>
  );
}
