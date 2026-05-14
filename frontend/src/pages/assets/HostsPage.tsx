import {
  App as AntApp,
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
import { auditsApi, hostsApi, secretsApi, tasksApi } from "../../lib/api";
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
  const tasksQuery = useQuery({
    queryKey: queryKeys.tasks,
    queryFn: tasksApi.list,
  });
  const auditsQuery = useQuery({
    queryKey: queryKeys.audits,
    queryFn: auditsApi.list,
  });
  const hostDetailQuery = useQuery({
    queryKey: queryKeys.host(selectedHostId),
    queryFn: () => hostsApi.detail(selectedHostId),
    enabled: Boolean(selectedHostId),
  });

  const saveMutation = useMutation({
    mutationFn: hostsApi.save,
    onSuccess: async () => {
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
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["hosts"] }),
        selectedHostId ? queryClient.invalidateQueries({ queryKey: queryKeys.host(selectedHostId) }) : Promise.resolve(),
        queryClient.invalidateQueries({ queryKey: queryKeys.tasks }),
      ]);
      setLatestActionText("已发起一次 SSH 连通性检测。");
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
    return (tasksQuery.data ?? [])
      .filter((task) => task.resourceId === selectedHost.id || task.target.includes(selectedHost.id) || task.target.includes(selectedHost.name))
      .slice(0, 5);
  }, [selectedHost, tasksQuery.data]);
  const relatedAudits = useMemo(() => {
    if (!selectedHost) {
      return [];
    }
    return (auditsQuery.data ?? [])
      .filter((audit) => {
        const haystack = `${audit.resourceType} ${audit.resourceName} ${audit.summary}`.toLowerCase();
        return haystack.includes(selectedHost.id.toLowerCase()) || haystack.includes(selectedHost.name.toLowerCase());
      })
      .slice(0, 5);
  }, [auditsQuery.data, selectedHost]);
  const primaryAction = selectedHost?.status === "HEALTHY" ? "terminal" : "test";

  if (hostsQuery.isError) {
    return <ErrorState message={hostsQuery.error.message} onRetry={() => void hostsQuery.refetch()} />;
  }

  return (
    <PermissionGuard permission="hosts.view" forbiddenPage>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <PageHeader
          title="主机"
          description="一期里主机页负责资产接入、SSH 测试和进入 WebSSH 终端。"
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
                      description="先绑定 SSH 凭证，再把主机接入控制台。"
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
              subtitle={selectedHost ? `${selectedHost.address}:${selectedHost.port}` : undefined}
              status={selectedHost ? <StatusBadge status={selectedHost.status} /> : undefined}
              meta={
                selectedHost
                  ? [
                      { label: "绑定凭证", value: selectedSecretName },
                      { label: "最近检测", value: selectedHost.lastCheckedAt ? formatDateTime(selectedHost.lastCheckedAt) : "--" },
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
                  <Space wrap>
                    <PermissionGuard permission="hosts.manage">
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
                  </Space>
                ) : undefined
              }
            >
              {latestActionText ? (
                <div className="resource-detail-section">
                  <Typography.Text type="secondary">{latestActionText}</Typography.Text>
                </div>
              ) : null}

              <ResourceActivityList
                title="最近任务"
                actionLabel={selectedHost ? "进入任务中心" : undefined}
                onActionClick={selectedHost ? () => navigate("/tasks") : undefined}
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
                actionLabel={selectedHost ? "查看全部审计" : undefined}
                onActionClick={selectedHost ? () => navigate("/audits") : undefined}
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
    const result = await import("../../lib/api").then(({ terminalApi }) => terminalApi.create(hostId));
    return result.id;
  }
}
