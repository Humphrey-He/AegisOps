import { App as AntApp, Button, Card, Select, Space, Typography } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";

import { DataTable } from "../../components/DataTable";
import { ErrorState } from "../../components/ErrorState";
import { PageHeader } from "../../components/PageHeader";
import { PermissionGuard } from "../../components/PermissionGuard";
import { StatusBadge } from "../../components/StatusBadge";
import { alertsApi } from "../../lib/api";
import { formatDateTime } from "../../lib/format";
import { queryKeys } from "../../lib/queryKeys";
import { buildResourcePath } from "../../lib/resourceNavigation";

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

export function AlertEventsPage() {
  const { message } = AntApp.useApp();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const status = searchParams.get("status") ?? "";
  const eventType = searchParams.get("eventType") ?? "";

  const eventsQuery = useQuery({
    queryKey: [...queryKeys.alertEvents, status, eventType],
    queryFn: () => alertsApi.listEvents({ status: status || undefined, eventType: eventType || undefined }),
  });

  const recordsQuery = useQuery({
    queryKey: queryKeys.notificationRecords,
    queryFn: alertsApi.listRecords,
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

  if (eventsQuery.isError) {
    return <ErrorState message={eventsQuery.error.message} onRetry={() => void eventsQuery.refetch()} />;
  }

  return (
    <PermissionGuard permission="alerts.view" forbiddenPage>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <PageHeader
          title="告警事件"
          description="集中查看发布失败、健康检查失败、主机离线与 Nginx 异常，并继续处理。"
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
                    return next;
                  })
                }
              />
            </div>
          </div>
        </Card>

        <Card className="page-card">
          <DataTable
            rowKey="id"
            loading={eventsQuery.isLoading}
            dataSource={eventsQuery.data}
            columns={[
              {
                title: "事件",
                dataIndex: "summary",
                render: (_, event) => (
                  <Space direction="vertical" size={2}>
                    <Typography.Text strong>{event.summary || event.eventType}</Typography.Text>
                    <Typography.Text type="secondary">{event.detail || event.resourceName || "--"}</Typography.Text>
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
                title: "资源",
                key: "resource",
                render: (_, event) => event.resourceName || event.resourceId || event.resourceType || "--",
              },
              {
                title: "触发时间",
                dataIndex: "lastTriggeredAt",
                render: (value: string) => formatDateTime(value),
              },
              {
                title: "动作",
                key: "actions",
                render: (_, event) => {
                  const targetPath = buildResourcePath(event.resourceType, event.resourceId);
                  return (
                    <Space size={0}>
                      {targetPath ? (
                        <Button type="link" onClick={() => navigate(targetPath)}>
                          资源
                        </Button>
                      ) : null}
                      {event.taskId ? (
                        <Button type="link" onClick={() => navigate(`/tasks/${event.taskId}`)}>
                          任务
                        </Button>
                      ) : null}
                      {event.status === "OPEN" ? (
                        <PermissionGuard permission="alerts.ack">
                          <Button type="link" loading={ackMutation.isPending} onClick={() => ackMutation.mutate(event.id)}>
                            确认
                          </Button>
                        </PermissionGuard>
                      ) : null}
                      {event.status !== "RESOLVED" ? (
                        <PermissionGuard permission="alerts.ack">
                          <Button
                            type="link"
                            loading={resolveMutation.isPending}
                            onClick={() => resolveMutation.mutate(event.id)}
                          >
                            关闭
                          </Button>
                        </PermissionGuard>
                      ) : null}
                    </Space>
                  );
                },
              },
            ]}
            expandable={{
              expandedRowRender: (event) => (
                <Space direction="vertical" size={8} style={{ width: "100%" }}>
                  <Typography.Text type="secondary">详情</Typography.Text>
                  <Typography.Text>{event.detail || "--"}</Typography.Text>
                  {event.suggestedRollbackVersion || event.suggestedRollbackVersionId ? (
                    <>
                      <Typography.Text type="secondary">回滚建议</Typography.Text>
                      <Typography.Text>
                        {event.suggestedRollbackVersion || event.suggestedRollbackVersionId}
                      </Typography.Text>
                    </>
                  ) : null}
                </Space>
              ),
            }}
          />
        </Card>
      </Space>
    </PermissionGuard>
  );
}
