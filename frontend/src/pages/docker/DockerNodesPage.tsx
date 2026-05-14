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
import { auditsApi, dockerApi, tasksApi } from "../../lib/api";
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
import { TaskStatus } from "../../components/TaskStatus";
import type { DockerNode, DockerNodeInput } from "../../types/models";

type NodeFormValues = {
  name: string;
  endpoint: string;
  tlsEnabled: boolean;
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
  const tasksQuery = useQuery({
    queryKey: queryKeys.tasks,
    queryFn: tasksApi.list,
  });
  const auditsQuery = useQuery({
    queryKey: queryKeys.audits,
    queryFn: auditsApi.list,
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

  const saveMutation = useMutation({
    mutationFn: dockerApi.saveNode,
    onSuccess: async () => {
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
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.dockerNodes }),
        selectedNodeId ? queryClient.invalidateQueries({ queryKey: queryKeys.dockerNode(selectedNodeId) }) : Promise.resolve(),
        selectedNodeId ? queryClient.invalidateQueries({ queryKey: queryKeys.containers(selectedNodeId) }) : Promise.resolve(),
        queryClient.invalidateQueries({ queryKey: queryKeys.tasks }),
      ]);
      setLatestActionText("已发起一次 Docker 节点连通性检测。");
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
    return (tasksQuery.data ?? [])
      .filter((task) => task.resourceId === selectedNode.id || task.target.includes(selectedNode.id) || task.target.includes(selectedNode.name))
      .slice(0, 5);
  }, [selectedNode, tasksQuery.data]);
  const relatedAudits = useMemo(() => {
    if (!selectedNode) {
      return [];
    }
    return (auditsQuery.data ?? [])
      .filter((audit) => {
        const haystack = `${audit.resourceType} ${audit.resourceName} ${audit.summary}`.toLowerCase();
        return haystack.includes(selectedNode.id.toLowerCase()) || haystack.includes(selectedNode.name.toLowerCase());
      })
      .slice(0, 5);
  }, [auditsQuery.data, selectedNode]);
  const primaryAction = selectedNode?.status === "OFFLINE" || selectedNode?.status === "UNKNOWN" ? "test" : "containers";
  const containerCount = containersQuery.data?.length ?? selectedNode?.containerCount ?? 0;

  return (
    <PermissionGuard permission="docker.view" forbiddenPage>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <PageHeader
          title="Docker 节点"
          description="MVP 先把节点接入、连通性检测和容器列表这三件事做好。"
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
              subtitle={selectedNode?.endpoint}
              status={selectedNode ? <StatusBadge status={selectedNode.status} /> : undefined}
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
                  <Space wrap>
                    <PermissionGuard permission="docker.manage">
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
                    <PermissionGuard permission="docker.manage">
                      <Button
                        onClick={() => {
                          setEditingNode(selectedNode);
                          form.setFieldsValue({
                            name: selectedNode.name,
                            endpoint: selectedNode.endpoint,
                            tlsEnabled: selectedNode.tlsEnabled,
                            description: selectedNode.description,
                          });
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
                actionLabel={selectedNode ? "进入任务中心" : undefined}
                onActionClick={selectedNode ? () => navigate("/tasks") : undefined}
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
                actionLabel={selectedNode ? "查看全部审计" : undefined}
                onActionClick={selectedNode ? () => navigate("/audits") : undefined}
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
          </Form>
        </FormDrawer>
      </Space>
    </PermissionGuard>
  );
}
