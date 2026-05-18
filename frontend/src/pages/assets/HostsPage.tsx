import {
  App as AntApp,
  Alert,
  Button,
  Card,
  Form,
  Input,
  Select,
  Space,
  Tag,
  Typography,
} from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { hostAvailabilityApi, hostsApi, resourcesApi, secretsApi, terminalApi } from "../../lib/api";
import { queryKeys } from "../../lib/queryKeys";
import { applyFormErrors, getErrorMessage } from "../../lib/forms";
import { DataTable } from "../../components/DataTable";
import { EmptyState } from "../../components/EmptyState";
import { ErrorState } from "../../components/ErrorState";
import { FormDrawer } from "../../components/FormDrawer";
import { PageHeader } from "../../components/PageHeader";
import { PermissionGuard } from "../../components/PermissionGuard";
import { StatusBadge } from "../../components/StatusBadge";
import { ResourceActivityList } from "../../components/resource/ResourceActivityList";
import { ResourceDetailPanel } from "../../components/resource/ResourceDetailPanel";
import { formatDateTime } from "../../lib/format";
import { buildAuditsPath, buildTasksPath } from "../../lib/resourceNavigation";
import { TaskStatus } from "../../components/TaskStatus";
import type { Host, HostInput } from "../../types/models";

type HostFormValues = {
  name: string;
  address: string;
  port: number;
  secretId: string;
  tags: string[];
  description?: string;
};

export function HostsPage() {
  const { message } = AntApp.useApp();
  const [form] = Form.useForm<HostFormValues>();
  const [keyword, setKeyword] = useState("");
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editingHost, setEditingHost] = useState<Host | null>(null);
  const [latestActionText, setLatestActionText] = useState<string | null>(null);
  const [searchParams, setSearchParams] = useSearchParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const selectedHostId = searchParams.get("selected") ?? "";

  const hostsQuery = useQuery({
    queryKey: queryKeys.hosts(keyword),
    queryFn: () => hostsApi.list(keyword),
  });
  const secretsQuery = useQuery({
    queryKey: queryKeys.secrets(""),
    queryFn: () => secretsApi.list(""),
  });
  const hostDetailQuery = useQuery({
    queryKey: queryKeys.host(selectedHostId),
    queryFn: () => hostsApi.detail(selectedHostId),
    enabled: Boolean(selectedHostId),
  });
  const hostAvailabilityQuery = useQuery({
    queryKey: queryKeys.hostAvailability(selectedHostId),
    queryFn: () => hostAvailabilityApi.list(selectedHostId),
    enabled: Boolean(selectedHostId),
  });
  const resourceContextQuery = useQuery({
    queryKey: queryKeys.resourceContext("host", selectedHostId),
    queryFn: () => resourcesApi.context("host", selectedHostId),
    enabled: Boolean(selectedHostId),
  });

  const saveMutation = useMutation({
    mutationFn: hostsApi.save,
    onSuccess: async (result) => {
      await queryClient.invalidateQueries({ queryKey: ["hosts"] });
      await message.success(editingHost ? "主机已更新" : "主机已创建");
      setDrawerOpen(false);
      setEditingHost(null);
      form.resetFields();
    },
    onError: (error) => {
      applyFormErrors(form, error);
      void message.error(getErrorMessage(error));
    },
  });

  const testMutation = useMutation({
    mutationFn: hostsApi.testSsh,
    onSuccess: async (result) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["hosts"] }),
        selectedHostId ? queryClient.invalidateQueries({ queryKey: queryKeys.host(selectedHostId) }) : Promise.resolve(),
        queryClient.invalidateQueries({ queryKey: queryKeys.resourceContext("host", selectedHostId) }),
      ]);
      setLatestActionText(
        result.taskId
          ? `已发起一次 SSH 连通性检测，任务 ${result.taskId} 可在当前资源任务中继续跟踪。`
          : "已发起一次 SSH 连通性检测。",
      );
      await message.success("SSH 测试已完成");
    },
    onError: (error) => {
      void message.error(getErrorMessage(error, "SSH 测试提交失败"));
    },
  });

  const secretOptions = useMemo(
    () =>
      (secretsQuery.data ?? []).map((item) => ({
        label: `${item.name} · ${item.username ?? "anonymous"}`,
        value: item.id,
      })),
    [secretsQuery.data],
  );
  const selectedHost = hostDetailQuery.data ?? (hostsQuery.data ?? []).find((item) => item.id === selectedHostId) ?? null;
  const selectedSecretName = useMemo(() => {
    if (!selectedHost) {
      return "--";
    }
    return (secretsQuery.data ?? []).find((item) => item.id === selectedHost.secretId)?.name ?? selectedHost.secretId;
  }, [secretsQuery.data, selectedHost]);
  const relatedTasks = useMemo(() => {
    if (!selectedHost) {
      return [];
    }
    return (resourceContextQuery.data?.recentTasks ?? []).slice(0, 5);
  }, [resourceContextQuery.data?.recentTasks, selectedHost]);
  const relatedAudits = useMemo(() => {
    if (!selectedHost) {
      return [];
    }
    return (resourceContextQuery.data?.recentAudits ?? []).slice(0, 5);
  }, [resourceContextQuery.data?.recentAudits, selectedHost]);
  const hostAvailability = hostAvailabilityQuery.data ?? [];
  const hostAlertEvents = useMemo(() => {
    if (!selectedHost) {
      return [];
    }
    return resourceContextQuery.data?.recentAlerts ?? [];
  }, [resourceContextQuery.data?.recentAlerts, selectedHost]);
  const latestAvailability = hostAvailability[0] ?? null;
  const latestHostAlert = hostAlertEvents[0] ?? null;
  const latestOfflineCheck = hostAvailability.find((item) => item.status === "UNREACHABLE") ?? null;
  const consecutiveFailureCount = useMemo(() => {
    let count = 0;
    for (const item of hostAvailability) {
      if (item.status !== "UNREACHABLE") {
        break;
      }
      count += 1;
    }
    return count;
  }, [hostAvailability]);
  const primaryAction = selectedHost?.status === "HEALTHY" ? "terminal" : "test";
  const hostSummaryItems = [
    {
      label: "主机总数",
      value: hostsQuery.data?.length ?? 0,
      helper: keyword ? `当前按关键词“${keyword}”过滤` : "当前主机资产清单",
    },
    {
      label: "健康主机",
      value: (hostsQuery.data ?? []).filter((host) => host.status === "HEALTHY").length,
      helper: "最近一次检测结果为正常",
    },
    {
      label: "异常主机",
      value: (hostsQuery.data ?? []).filter((host) => host.status === "UNREACHABLE").length,
      helper: "建议优先查看最近离线和告警状态",
    },
    {
      label: "未知 / 测试中",
      value: (hostsQuery.data ?? []).filter((host) => host.status === "UNKNOWN" || host.status === "TESTING").length,
      helper: "通常意味着仍需确认可达性",
    },
  ];

  if (hostsQuery.isError) {
    return <ErrorState message={hostsQuery.error.message} onRetry={() => void hostsQuery.refetch()} />;
  }

  return (
    <PermissionGuard permission="hosts.view" forbiddenPage>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <PageHeader
          title="主机"
          description="统一管理主机接入、SSH 检测与浏览器终端入口。"
          eyebrow="运行资源 / 主机工作台"
          extra={
            <PermissionGuard permission="hosts.manage">
              <Button
                type="primary"
                onClick={() => {
                  setEditingHost(null);
                  form.resetFields();
                  form.setFieldsValue({ port: 22, tags: [] });
                  setDrawerOpen(true);
                }}
              >
                新增主机
              </Button>
            </PermissionGuard>
          }
        />

        <Card className="page-card">
          <div className="workbench-summary-grid">
            {hostSummaryItems.map((item) => (
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
                placeholder="搜索主机名、地址或标签"
                style={{ width: 320 }}
                onSearch={(value) => {
                  setKeyword(value);
                  setLatestActionText(null);
                }}
              />
            </div>
          </div>
        </Card>

        <div className="resource-workbench">
          <div className="resource-list-pane">
            <Card className="page-card">
              <DataTable
                rowKey="id"
                loading={hostsQuery.isLoading}
                dataSource={hostsQuery.data}
                rowClassName={(host) => (host.id === selectedHostId ? "resource-row-selected" : "")}
                onRow={(host) => ({
                  onClick: () => {
                    setSearchParams((previous) => {
                      const next = new URLSearchParams(previous);
                      next.set("selected", host.id);
                      return next;
                    });
                    setLatestActionText(null);
                  },
                })}
                locale={{
                  emptyText: (
                    <EmptyState
                      title="还没有主机资产"
                      description="绑定 SSH 凭证后，即可将主机接入控制台。"
                      action={
                        <Button type="primary" onClick={() => setDrawerOpen(true)}>
                          新增第一台主机
                        </Button>
                      }
                    />
                  ),
                }}
                columns={[
                  {
                    title: "主机",
                    dataIndex: "name",
                    render: (_, host) => (
                      <Space direction="vertical" size={2}>
                        <span>{host.name}</span>
                        <span style={{ color: "#64748b" }}>
                          {host.address}:{host.port}
                        </span>
                      </Space>
                    ),
                  },
                  {
                    title: "状态",
                    dataIndex: "status",
                    render: (value) => <StatusBadge status={value} />,
                  },
                  {
                    title: "标签",
                    dataIndex: "tags",
                    render: (tags: string[]) => (
                      <Space wrap>
                        {tags.map((tag) => (
                          <Tag key={tag}>{tag}</Tag>
                        ))}
                      </Space>
                    ),
                  },
                  {
                    title: "最近检测",
                    dataIndex: "lastCheckedAt",
                    render: (value) => (value ? formatDateTime(value) : "--"),
                  },
                ]}
              />
            </Card>
          </div>

          <div className="resource-detail-pane">
            <ResourceDetailPanel
              title={selectedHost?.name}
              kicker={selectedHost ? "主机上下文" : undefined}
              subtitle={selectedHost ? `${selectedHost.address}:${selectedHost.port}` : undefined}
              status={selectedHost ? <StatusBadge status={selectedHost.status} /> : undefined}
              helper={
                selectedHost
                  ? "围绕当前主机集中查看接入状态、最近检测、告警信号与执行入口，减少在任务、终端和审计之间来回切换。"
                  : undefined
              }
              highlights={
                selectedHost
                  ? [
                      {
                        label: "最近检测",
                        value: selectedHost.lastCheckedAt ? formatDateTime(selectedHost.lastCheckedAt) : "--",
                        helper: latestAvailability ? `最近结果 ${latestAvailability.status}` : "尚未记录检测结果",
                      },
                      {
                        label: "连续失败",
                        value: consecutiveFailureCount || 0,
                        helper: consecutiveFailureCount > 0 ? "建议优先执行 SSH 测试或查看终端" : "当前没有连续失败记录",
                      },
                      {
                        label: "待处理信号",
                        value: hostAlertEvents.filter((item) => item.status !== "RESOLVED").length,
                        helper: latestHostAlert
                          ? `最近告警 ${formatDateTime(latestHostAlert.lastTriggeredAt)}`
                          : "当前没有关联告警",
                      },
                    ]
                  : []
              }
              meta={
                selectedHost
                  ? [
                      { label: "绑定凭证", value: selectedSecretName },
                      {
                        label: "最近可用性",
                        value: latestAvailability ? <StatusBadge status={latestAvailability.status} /> : "--",
                      },
                      {
                        label: "最近离线时间",
                        value: latestOfflineCheck?.startedAt ? formatDateTime(latestOfflineCheck.startedAt) : "--",
                      },
                      {
                        label: "最新告警状态",
                        value: latestHostAlert ? <StatusBadge status={latestHostAlert.status} /> : "--",
                      },
                      {
                        label: "连续失败次数",
                        value: String(consecutiveFailureCount || 0),
                      },
                      {
                        label: "标签",
                        value: selectedHost.tags.length ? (
                          <Space wrap>
                            {selectedHost.tags.map((tag) => (
                              <Tag key={tag}>{tag}</Tag>
                            ))}
                          </Space>
                        ) : (
                          "--"
                        ),
                      },
                      { label: "说明", value: selectedHost.description || "--" },
                    ]
                  : []
              }
              actions={
                selectedHost ? (
                  <>
                    <div className="resource-action-group">
                      <PermissionGuard permission="hosts.test">
                        <Button
                          type={primaryAction === "test" ? "primary" : "default"}
                          loading={testMutation.isPending}
                          onClick={() => {
                            setLatestActionText("正在执行 SSH 连通性检测...");
                            testMutation.mutate(selectedHost.id);
                          }}
                        >
                          SSH 测试
                        </Button>
                      </PermissionGuard>
                      <PermissionGuard permission="terminal.open">
                        <Button
                          type={primaryAction === "terminal" ? "primary" : "default"}
                          onClick={async () => {
                            setLatestActionText("正在创建终端会话...");
                            const session = await hostsToTerminal(selectedHost.id);
                            navigate(`/terminal/${session}`);
                          }}
                        >
                          打开终端
                        </Button>
                      </PermissionGuard>
                    </div>
                    <div className="resource-action-group">
                      <Button onClick={() => navigate(buildTasksPath({ resourceType: "host", resourceId: selectedHost.id }))}>
                        查看任务
                      </Button>
                      <Button onClick={() => navigate(buildAuditsPath({ resourceType: "host", resourceId: selectedHost.id }))}>
                        查看审计
                      </Button>
                      <PermissionGuard permission="hosts.manage">
                        <Button
                          onClick={() => {
                            setEditingHost(selectedHost);
                            form.setFieldsValue(selectedHost);
                            setDrawerOpen(true);
                          }}
                        >
                          编辑
                        </Button>
                      </PermissionGuard>
                    </div>
                  </>
                ) : undefined
              }
            >
              {selectedHost && latestHostAlert?.status === "OPEN" ? (
                <div className="resource-detail-section">
                  <Alert
                    type="warning"
                    showIcon
                    message={`当前主机存在待处理告警：${latestHostAlert.summary || latestHostAlert.eventType}`}
                    description={latestHostAlert.detail || "建议先确认 SSH 可达性，再结合最近任务和审计判断是否为持续性异常。"}
                    action={
                      <Button size="small" onClick={() => navigate("/alerts/events")}>
                        查看告警
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
                  <Typography.Text strong>主机可用性记录</Typography.Text>
                  <Typography.Text type="secondary">
                    {hostAvailabilityQuery.isLoading ? "正在同步..." : `${hostAvailability.length} 条记录`}
                  </Typography.Text>
                </div>
                <div className="resource-subpanel" style={{ marginTop: 12 }}>
                  <DataTable
                    rowKey="id"
                    pagination={false}
                    loading={hostAvailabilityQuery.isLoading}
                    dataSource={hostAvailability.slice(0, 6)}
                    locale={{ emptyText: "当前主机还没有可用性记录" }}
                    columns={[
                      { title: "状态", dataIndex: "status", render: (value: string) => <StatusBadge status={value} /> },
                      {
                        title: "开始时间",
                        dataIndex: "startedAt",
                        render: (value: string) => formatDateTime(value),
                      },
                      {
                        title: "失败原因",
                        dataIndex: "failureReason",
                        render: (value?: string) => value || "--",
                      },
                    ]}
                  />
                </div>
              </div>

              <ResourceActivityList
                title="最近任务"
                helper="优先看失败、执行中和最近触发的任务，判断这台主机刚发生了什么。"
                actionLabel={selectedHost ? "进入任务中心" : undefined}
                onActionClick={
                  selectedHost
                    ? () =>
                        navigate(
                          buildTasksPath({
                            resourceType: "host",
                            resourceId: selectedHost.id,
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
                emptyText="当前资源还没有关联任务。"
              />

              <ResourceActivityList
                title="最近审计"
                helper="操作审计能帮助你确认异常是否由刚刚发生的变更触发。"
                actionLabel={selectedHost ? "查看全部审计" : undefined}
                onActionClick={
                  selectedHost
                    ? () =>
                        navigate(
                          buildAuditsPath({
                            resourceType: "host",
                            resourceId: selectedHost.id,
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
                emptyText="当前资源还没有关联审计记录。"
              />
            </ResourceDetailPanel>
          </div>
        </div>

        <FormDrawer
          open={drawerOpen}
          title={editingHost ? "编辑主机" : "新增主机"}
          onClose={() => {
            setDrawerOpen(false);
            setEditingHost(null);
          }}
          onSubmit={() => form.submit()}
          loading={saveMutation.isPending}
        >
          <Form
            layout="vertical"
            form={form}
            onFinish={(values) =>
              saveMutation.mutate({
                id: editingHost?.id,
                name: values.name,
                address: values.address,
                port: Number(values.port),
                secretId: values.secretId,
                tags: values.tags ?? [],
                description: values.description,
              } satisfies HostInput)
            }
          >
            <Form.Item label="主机名" name="name" rules={[{ required: true, message: "请输入主机名" }]}>
              <Input />
            </Form.Item>
            <Form.Item label="地址" name="address" rules={[{ required: true, message: "请输入主机地址" }]}>
              <Input />
            </Form.Item>
            <Form.Item label="端口" name="port" rules={[{ required: true, message: "请输入 SSH 端口" }]}>
              <Input type="number" />
            </Form.Item>
            <Form.Item label="SSH 凭证" name="secretId" rules={[{ required: true, message: "请选择 SSH 凭证" }]}>
              <Select options={secretOptions} placeholder="请选择绑定凭证" />
            </Form.Item>
            <Form.Item label="标签" name="tags">
              <Select mode="tags" placeholder="例如 production / web" />
            </Form.Item>
            <Form.Item label="说明" name="description">
              <Input.TextArea rows={4} />
            </Form.Item>
          </Form>
        </FormDrawer>
      </Space>
    </PermissionGuard>
  );

  async function hostsToTerminal(hostId: string) {
    const result = await terminalApi.create(hostId);
    return result.id;
  }
}
