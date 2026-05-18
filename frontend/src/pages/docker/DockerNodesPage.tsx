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
import { dockerApi, resourcesApi, secretsApi } from "../../lib/api";
import { queryKeys } from "../../lib/queryKeys";
import { applyFormErrors, getErrorMessage } from "../../lib/forms";
import { DataTable } from "../../components/DataTable";
import { FormDrawer } from "../../components/FormDrawer";
import { PageHeader } from "../../components/PageHeader";
import { PermissionGuard } from "../../components/PermissionGuard";
import { ResourceActivityList } from "../../components/resource/ResourceActivityList";
import { ResourceDetailPanel } from "../../components/resource/ResourceDetailPanel";
import { StatusBadge } from "../../components/StatusBadge";
import { formatDateTime } from "../../lib/format";
import { buildAuditsPath, buildTasksPath } from "../../lib/resourceNavigation";
import { TaskStatus } from "../../components/TaskStatus";
import type { DockerNode, DockerNodeInput } from "../../types/models";

type NodeFormValues = {
  name: string;
  endpoint: string;
  tlsEnabled: boolean;
  secretId?: string;
  description?: string;
};

export function DockerNodesPage() {
  const { message } = AntApp.useApp();
  const [form] = Form.useForm<NodeFormValues>();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editingNode, setEditingNode] = useState<DockerNode | null>(null);
  const [latestActionText, setLatestActionText] = useState<string | null>(null);
  const [searchParams, setSearchParams] = useSearchParams();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const selectedNodeId = searchParams.get("selected") ?? "";

  const nodesQuery = useQuery({
    queryKey: queryKeys.dockerNodes,
    queryFn: dockerApi.listNodes,
  });
  const dockerTlsSecretsQuery = useQuery({
    queryKey: queryKeys.secrets(""),
    queryFn: () => secretsApi.list(""),
  });
  const nodeDetailQuery = useQuery({
    queryKey: queryKeys.dockerNode(selectedNodeId),
    queryFn: () => dockerApi.getNode(selectedNodeId),
    enabled: Boolean(selectedNodeId),
  });
  const containersQuery = useQuery({
    queryKey: queryKeys.containers(selectedNodeId),
    queryFn: () => dockerApi.listContainers(selectedNodeId),
    enabled: Boolean(selectedNodeId),
  });
  const resourceContextQuery = useQuery({
    queryKey: queryKeys.resourceContext("docker-node", selectedNodeId),
    queryFn: () => resourcesApi.context("docker-node", selectedNodeId),
    enabled: Boolean(selectedNodeId),
  });

  const saveMutation = useMutation({
    mutationFn: dockerApi.saveNode,
    onSuccess: async (result) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.dockerNodes });
      await message.success(editingNode ? "节点已更新" : "节点已创建");
      setDrawerOpen(false);
      setEditingNode(null);
      form.resetFields();
    },
    onError: (error) => {
      applyFormErrors(form, error);
      void message.error(getErrorMessage(error));
    },
  });

  const testMutation = useMutation({
    mutationFn: dockerApi.testNode,
    onSuccess: async (result) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.dockerNodes }),
        selectedNodeId ? queryClient.invalidateQueries({ queryKey: queryKeys.dockerNode(selectedNodeId) }) : Promise.resolve(),
        selectedNodeId ? queryClient.invalidateQueries({ queryKey: queryKeys.containers(selectedNodeId) }) : Promise.resolve(),
        queryClient.invalidateQueries({ queryKey: queryKeys.resourceContext("docker-node", selectedNodeId) }),
      ]);
      setLatestActionText(
        result.taskId
          ? `已发起一次 Docker 节点连通性检测，任务 ${result.taskId} 可在当前资源任务中继续跟踪。`
          : "已发起一次 Docker 节点连通性检测。",
      );
      await message.success("节点检测已完成");
    },
    onError: (error) => {
      void message.error(getErrorMessage(error));
    },
  });
  const selectedNode = nodeDetailQuery.data ?? (nodesQuery.data ?? []).find((item) => item.id === selectedNodeId) ?? null;
  const relatedTasks = useMemo(() => {
    if (!selectedNode) {
      return [];
    }
    return (resourceContextQuery.data?.recentTasks ?? []).slice(0, 5);
  }, [resourceContextQuery.data?.recentTasks, selectedNode]);
  const relatedAudits = useMemo(() => {
    if (!selectedNode) {
      return [];
    }
    return (resourceContextQuery.data?.recentAudits ?? []).slice(0, 5);
  }, [resourceContextQuery.data?.recentAudits, selectedNode]);
  const primaryAction = selectedNode?.status === "OFFLINE" || selectedNode?.status === "UNKNOWN" ? "test" : "containers";
  const containerCount = containersQuery.data?.length ?? selectedNode?.containerCount ?? 0;
  const nodeSummaryItems = [
    {
      label: "节点总数",
      value: nodesQuery.data?.length ?? 0,
      helper: "当前已纳入管理的 Docker 节点",
    },
    {
      label: "在线节点",
      value: (nodesQuery.data ?? []).filter((node) => node.status === "ONLINE").length,
      helper: "已通过连通性检测",
    },
    {
      label: "离线 / 异常",
      value: (nodesQuery.data ?? []).filter((node) => node.status === "OFFLINE" || node.status === "UNKNOWN").length,
      helper: "建议优先测试连接并核查证书",
    },
    {
      label: "TLS 接入",
      value: (nodesQuery.data ?? []).filter((node) => node.tlsEnabled).length,
      helper: "已启用 TLS 的节点数量",
    },
  ];

  return (
    <PermissionGuard permission="docker.view" forbiddenPage>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <PageHeader
          title="Docker 节点"
          description="统一管理 Docker 节点接入、连接检测与容器视图。"
          eyebrow="运行资源 / Docker 工作台"
          extra={
            <PermissionGuard permission="docker.manage">
              <Button
                type="primary"
                onClick={() => {
                  setEditingNode(null);
                  form.resetFields();
                  form.setFieldsValue({ tlsEnabled: true });
                  setDrawerOpen(true);
                }}
              >
                新增节点
              </Button>
            </PermissionGuard>
          }
        />

        <Card className="page-card">
          <div className="workbench-summary-grid">
            {nodeSummaryItems.map((item) => (
              <div key={item.label} className="workbench-summary-card">
                <Typography.Text className="workbench-summary-label">{item.label}</Typography.Text>
                <div className="workbench-summary-value">{item.value}</div>
                <div className="workbench-summary-helper">{item.helper}</div>
              </div>
            ))}
          </div>
        </Card>

        <div className="resource-workbench">
          <div className="resource-list-pane">
            <Card className="page-card">
              <DataTable
                rowKey="id"
                loading={nodesQuery.isLoading}
                dataSource={nodesQuery.data}
                rowClassName={(node) => (node.id === selectedNodeId ? "resource-row-selected" : "")}
                onRow={(node) => ({
                  onClick: () => {
                    setSearchParams((previous) => {
                      const next = new URLSearchParams(previous);
                      next.set("selected", node.id);
                      return next;
                    });
                    setLatestActionText(null);
                  },
                })}
                columns={[
                  { title: "名称", dataIndex: "name" },
                  { title: "Endpoint", dataIndex: "endpoint" },
                  {
                    title: "TLS",
                    dataIndex: "tlsEnabled",
                    render: (value) => (value ? "已启用" : "未启用"),
                  },
                  {
                    title: "状态",
                    dataIndex: "status",
                    render: (value) => <StatusBadge status={value} />,
                  },
                  {
                    title: "容器数",
                    dataIndex: "containerCount",
                    render: (value) => value ?? 0,
                  },
                ]}
              />
            </Card>
          </div>

          <div className="resource-detail-pane">
            <ResourceDetailPanel
              title={selectedNode?.name}
              kicker={selectedNode ? "节点上下文" : undefined}
              subtitle={selectedNode?.endpoint}
              status={selectedNode ? <StatusBadge status={selectedNode.status} /> : undefined}
              helper={
                selectedNode
                  ? "围绕当前节点集中查看接入方式、连接状态、容器规模和最近动作，避免在节点列表与容器页之间反复切换。"
                  : undefined
              }
              highlights={
                selectedNode
                  ? [
                      {
                        label: "容器总数",
                        value: containerCount,
                        helper: selectedNode.status === "ONLINE" ? "节点在线时可直接进入容器视图" : "节点异常时容器视图可能不完整",
                      },
                      {
                        label: "最近检测",
                        value: selectedNode.lastCheckedAt ? formatDateTime(selectedNode.lastCheckedAt) : "--",
                        helper: selectedNode.status === "ONLINE" ? "最近一次检测成功" : "建议重新测试连接",
                      },
                      {
                        label: "关联任务",
                        value: relatedTasks.length,
                        helper: relatedTasks[0] ? "这里优先保留最近 5 条关联任务" : "当前还没有关联任务",
                      },
                    ]
                  : []
              }
              meta={
                selectedNode
                  ? [
                      {
                        label: "认证方式",
                        value: <Tag color={selectedNode.tlsEnabled ? "blue" : "default"}>{selectedNode.tlsEnabled ? "TLS" : "NONE"}</Tag>,
                      },
                      { label: "容器数", value: containerCount },
                      { label: "最近检测", value: selectedNode.lastCheckedAt ? formatDateTime(selectedNode.lastCheckedAt) : "--" },
                      { label: "说明", value: selectedNode.description || "--" },
                    ]
                  : []
              }
              actions={
                selectedNode ? (
                  <>
                    <div className="resource-action-group">
                      <PermissionGuard permission="docker.test">
                        <Button
                          type={primaryAction === "test" ? "primary" : "default"}
                          loading={testMutation.isPending}
                          onClick={() => {
                            setLatestActionText("正在执行 Docker 节点连通性检测...");
                            testMutation.mutate(selectedNode.id);
                          }}
                        >
                          测试连接
                        </Button>
                      </PermissionGuard>
                      <Button
                        type={primaryAction === "containers" ? "primary" : "default"}
                        onClick={() => navigate(`/docker/nodes/${selectedNode.id}`)}
                      >
                        查看容器
                      </Button>
                    </div>
                    <div className="resource-action-group">
                      <Button onClick={() => navigate(buildTasksPath({ resourceType: "docker-node", resourceId: selectedNode.id }))}>
                        查看任务
                      </Button>
                      <Button onClick={() => navigate(buildAuditsPath({ resourceType: "docker-node", resourceId: selectedNode.id }))}>
                        查看审计
                      </Button>
                      <PermissionGuard permission="docker.manage">
                        <Button
                          onClick={() => {
                            setEditingNode(selectedNode);
                            form.setFieldsValue({
                              name: selectedNode.name,
                              endpoint: selectedNode.endpoint,
                              tlsEnabled: selectedNode.tlsEnabled,
                              secretId: selectedNode.secretId,
                              description: selectedNode.description,
                            });
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
              {selectedNode && (selectedNode.status === "OFFLINE" || selectedNode.status === "UNKNOWN") ? (
                <div className="resource-detail-section">
                  <Alert
                    type="warning"
                    showIcon
                    message="当前节点未处于稳定在线状态"
                    description="建议先测试连接，确认 Endpoint、TLS Secret 和目标主机网络连通性，再继续查看容器细节。"
                    action={
                      <Button
                        size="small"
                        onClick={() => {
                          setLatestActionText("正在执行 Docker 节点连通性检测...");
                          testMutation.mutate(selectedNode.id);
                        }}
                      >
                        立即测试
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

              <ResourceActivityList
                title="最近任务"
                helper="先看节点接入、连接测试和容器动作相关任务，判断最近一次运维动作是否收口。"
                actionLabel={selectedNode ? "进入任务中心" : undefined}
                onActionClick={
                  selectedNode
                    ? () =>
                        navigate(
                          buildTasksPath({
                            resourceType: "docker-node",
                            resourceId: selectedNode.id,
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
                emptyText="当前节点还没有关联任务。"
              />

              <ResourceActivityList
                title="最近审计"
                helper="审计能帮助你回看是谁变更了节点配置，以及连接异常是否发生在变更之后。"
                actionLabel={selectedNode ? "查看全部审计" : undefined}
                onActionClick={
                  selectedNode
                    ? () =>
                        navigate(
                          buildAuditsPath({
                            resourceType: "docker-node",
                            resourceId: selectedNode.id,
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
                emptyText="当前节点还没有关联审计记录。"
              />
            </ResourceDetailPanel>
          </div>
        </div>

        <FormDrawer
          open={drawerOpen}
          title={editingNode ? "编辑节点" : "新增节点"}
          onClose={() => {
            setDrawerOpen(false);
            setEditingNode(null);
          }}
          onSubmit={() => form.submit()}
          loading={saveMutation.isPending}
        >
          <Form
            layout="vertical"
            form={form}
            onFinish={(values) =>
              saveMutation.mutate({
                id: editingNode?.id,
                name: values.name,
                endpoint: values.endpoint,
                tlsEnabled: values.tlsEnabled,
                secretId: values.secretId,
                description: values.description,
              } satisfies DockerNodeInput)
            }
          >
            <Form.Item label="节点名称" name="name" rules={[{ required: true, message: "请输入节点名称" }]}>
              <Input />
            </Form.Item>
            <Form.Item label="Docker Endpoint" name="endpoint" rules={[{ required: true, message: "请输入 Endpoint" }]}>
              <Input placeholder="tcp://10.0.0.1:2376" />
            </Form.Item>
            <Form.Item label="TLS" name="tlsEnabled" rules={[{ required: true, message: "请选择 TLS 选项" }]}>
              <Select options={[{ label: "启用", value: true }, { label: "关闭", value: false }]} />
            </Form.Item>
            <Form.Item label="说明" name="description">
              <Input.TextArea rows={4} />
            </Form.Item>
            <Form.Item shouldUpdate={(previous, current) => previous.tlsEnabled !== current.tlsEnabled} noStyle>
              {({ getFieldValue }) =>
                getFieldValue("tlsEnabled") ? (
                  <Form.Item label="TLS Secret" name="secretId" rules={[{ required: true, message: "请选择 Docker TLS Secret" }]}>
                    <Select
                      loading={dockerTlsSecretsQuery.isLoading}
                      options={(dockerTlsSecretsQuery.data ?? [])
                        .filter((secret) => secret.type === "DOCKER_TLS")
                        .map((secret) => ({ label: secret.name, value: secret.id }))}
                    />
                  </Form.Item>
                ) : null
              }
            </Form.Item>
          </Form>
        </FormDrawer>
      </Space>
    </PermissionGuard>
  );
}
