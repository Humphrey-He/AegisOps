import { App as AntApp, Button, Card, Checkbox, Form, Input, Select, Space, Tag, Typography } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";

import { alertsApi, hostsApi, nginxApi, tasksApi } from "../../lib/api";
import { queryKeys } from "../../lib/queryKeys";
import { applyFormErrors, getErrorMessage } from "../../lib/forms";
import { DataTable } from "../../components/DataTable";
import { FormDrawer } from "../../components/FormDrawer";
import { PageHeader } from "../../components/PageHeader";
import { PermissionGuard } from "../../components/PermissionGuard";
import { ResourceActivityList } from "../../components/resource/ResourceActivityList";
import { ResourceDetailPanel } from "../../components/resource/ResourceDetailPanel";
import { StatusBadge } from "../../components/StatusBadge";
import { TaskStatus } from "../../components/TaskStatus";
import { formatDateTime } from "../../lib/format";
import { buildTasksPath, taskMatchesResource } from "../../lib/resourceNavigation";
import type { NginxConfigInput, NginxConfigVersion, NginxNode, NginxNodeInput } from "../../types/models";

type NodeFormValues = {
  name: string;
  hostId: string;
  configPath?: string;
  testCommand?: string;
  reloadCommand?: string;
  description?: string;
};

type ConfigFormValues = {
  version: string;
  content: string;
  message?: string;
  activate?: boolean;
};

export function NginxNodesPage() {
  const { message } = AntApp.useApp();
  const [nodeForm] = Form.useForm<NodeFormValues>();
  const [configForm] = Form.useForm<ConfigFormValues>();
  const [keyword, setKeyword] = useState("");
  const [nodeDrawerOpen, setNodeDrawerOpen] = useState(false);
  const [configDrawerOpen, setConfigDrawerOpen] = useState(false);
  const [editingNode, setEditingNode] = useState<NginxNode | null>(null);
  const [searchParams, setSearchParams] = useSearchParams();
  const selectedNodeId = searchParams.get("selected") ?? "";
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const nodesQuery = useQuery({
    queryKey: queryKeys.nginxNodes(keyword),
    queryFn: () => nginxApi.listNodes(keyword),
  });
  const hostsQuery = useQuery({
    queryKey: queryKeys.hosts(""),
    queryFn: () => hostsApi.list(""),
  });
  const tasksQuery = useQuery({
    queryKey: queryKeys.tasks,
    queryFn: () => tasksApi.list(),
  });
  const nodeDetailQuery = useQuery({
    queryKey: queryKeys.nginxNode(selectedNodeId),
    queryFn: () => nginxApi.detailNode(selectedNodeId),
    enabled: Boolean(selectedNodeId),
  });
  const configsQuery = useQuery({
    queryKey: queryKeys.nginxConfigs(selectedNodeId),
    queryFn: () => nginxApi.listConfigs(selectedNodeId),
    enabled: Boolean(selectedNodeId),
  });
  const alertEventsQuery = useQuery({
    queryKey: [...queryKeys.alertEvents, "nginx", selectedNodeId],
    queryFn: () => alertsApi.listEvents(),
    enabled: Boolean(selectedNodeId),
  });

  const selectedNode = nodeDetailQuery.data ?? (nodesQuery.data ?? []).find((item) => item.id === selectedNodeId) ?? null;
  const activeConfig = useMemo(
    () => (configsQuery.data ?? []).find((item) => item.status === "ACTIVE") ?? null,
    [configsQuery.data],
  );
  const relatedTasks = useMemo(() => {
    if (!selectedNode) {
      return [];
    }
    return (tasksQuery.data ?? [])
      .filter((task) => taskMatchesResource(task, "nginx_node", selectedNode.id, [selectedNode.name]))
      .slice(0, 5);
  }, [selectedNode, tasksQuery.data]);
  const recentOperations = useMemo(() => {
    return relatedTasks.filter((task) =>
      ["nginx.node.test", "nginx.node.reload", "nginx.config.publish", "nginx.config.rollback"].includes(task.type),
    );
  }, [relatedTasks]);
  const nodeAlertEvents = useMemo(() => {
    if (!selectedNode) {
      return [];
    }
    return (alertEventsQuery.data ?? []).filter(
      (event) => event.resourceType === "nginx_node" && event.resourceId === selectedNode.id,
    );
  }, [alertEventsQuery.data, selectedNode]);
  const latestOperation = recentOperations[0] ?? null;
  const latestFailureOperation = recentOperations.find((item) => item.status === "FAILED") ?? null;
  const latestNodeAlert = nodeAlertEvents[0] ?? null;

  const saveNodeMutation = useMutation({
    mutationFn: nginxApi.saveNode,
    onSuccess: async (node) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.nginxNodes(keyword) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.nginxNode(node.id) }),
      ]);
      setNodeDrawerOpen(false);
      setEditingNode(null);
      nodeForm.resetFields();
      setSearchParams((previous) => {
        const next = new URLSearchParams(previous);
        next.set("selected", node.id);
        return next;
      });
      await message.success(editingNode ? "Nginx 节点已更新" : "Nginx 节点已创建");
    },
    onError: (error) => {
      applyFormErrors(nodeForm, error);
      void message.error(getErrorMessage(error));
    },
  });

  const saveConfigMutation = useMutation({
    mutationFn: (payload: NginxConfigInput) => nginxApi.saveConfig(selectedNodeId, payload),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.nginxConfigs(selectedNodeId) });
      setConfigDrawerOpen(false);
      configForm.resetFields();
      await message.success("配置版本已保存");
    },
    onError: (error) => {
      applyFormErrors(configForm, error);
      void message.error(getErrorMessage(error));
    },
  });

  const runAction = useMutation({
    mutationFn: async ({ action, configId }: { action: "test" | "reload" | "publish" | "rollback"; configId?: string }) => {
      if (!selectedNodeId) {
        throw new Error("请选择 Nginx 节点");
      }
      if (action === "test") {
        return nginxApi.testNode(selectedNodeId);
      }
      if (action === "reload") {
        return nginxApi.reloadNode(selectedNodeId);
      }
      if (!configId) {
        throw new Error("请选择配置版本");
      }
      return action === "publish"
        ? nginxApi.publishConfig(selectedNodeId, configId)
        : nginxApi.rollback(selectedNodeId, configId);
    },
    onSuccess: async (result) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.nginxNodes(keyword) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.nginxNode(selectedNodeId) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.nginxConfigs(selectedNodeId) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.tasks }),
      ]);
      if (result.taskId) {
        navigate(`/tasks/${result.taskId}`);
      }
      await message.success("动作已进入任务中心");
    },
    onError: (error) => {
      void message.error(getErrorMessage(error));
    },
  });

  const hostOptions = (hostsQuery.data ?? []).map((host) => ({ label: `${host.name} (${host.address})`, value: host.id }));

  return (
    <PermissionGuard permission="nginx.view" forbiddenPage>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <PageHeader
          title="Nginx"
          description="管理 Nginx 节点、配置版本，并将测试、重载、回滚串入任务中心。"
          extra={
            <PermissionGuard permission="nginx.manage">
              <Button
                type="primary"
                onClick={() => {
                  setEditingNode(null);
                  nodeForm.setFieldsValue({
                    configPath: "/etc/nginx/nginx.conf",
                    testCommand: "nginx -t",
                    reloadCommand: "nginx -s reload",
                  });
                  setNodeDrawerOpen(true);
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
              <Space direction="vertical" size={12} style={{ width: "100%" }}>
                <Input.Search placeholder="搜索节点、配置路径" allowClear onSearch={setKeyword} />
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
                    },
                  })}
                  columns={[
                    { title: "名称", dataIndex: "name" },
                    { title: "主机", dataIndex: "hostName", render: (_, node) => node.hostName || node.hostId },
                    { title: "状态", dataIndex: "status", render: (value) => <StatusBadge status={value} /> },
                  ]}
                />
              </Space>
            </Card>
          </div>

          <div className="resource-detail-pane">
            <ResourceDetailPanel
              title={selectedNode?.name}
              subtitle={selectedNode?.configPath}
              status={selectedNode ? <StatusBadge status={selectedNode.status} /> : undefined}
              meta={
                selectedNode
                  ? [
                      { label: "主机", value: selectedNode.hostName || selectedNode.hostId },
                      { label: "测试命令", value: <Typography.Text code>{selectedNode.testCommand}</Typography.Text> },
                      { label: "重载命令", value: <Typography.Text code>{selectedNode.reloadCommand}</Typography.Text> },
                      { label: "最近测试", value: selectedNode.lastTestAt ? formatDateTime(selectedNode.lastTestAt) : "--" },
                      {
                        label: "最近操作",
                        value: latestOperation ? `${latestOperation.type} · ${formatDateTime(latestOperation.createdAt)}` : "--",
                      },
                      {
                        label: "最近失败原因",
                        value: latestFailureOperation?.summary || latestNodeAlert?.detail || "--",
                      },
                      {
                        label: "最近通知状态",
                        value: latestNodeAlert?.notificationStatus ? (
                          <StatusBadge status={latestNodeAlert.notificationStatus} />
                        ) : (
                          "--"
                        ),
                      },
                      { label: "当前版本", value: activeConfig ? <Tag color="green">{activeConfig.version}</Tag> : "--" },
                    ]
                  : []
              }
              actions={
                selectedNode ? (
                  <Space wrap>
                    <PermissionGuard permission="nginx.test">
                      <Button loading={runAction.isPending} onClick={() => runAction.mutate({ action: "test" })}>
                        测试配置
                      </Button>
                    </PermissionGuard>
                    <PermissionGuard permission="nginx.reload">
                      <Button type="primary" loading={runAction.isPending} onClick={() => runAction.mutate({ action: "reload" })}>
                        重载
                      </Button>
                    </PermissionGuard>
                    <PermissionGuard permission="nginx.manage">
                      <Button
                        onClick={() => {
                          setEditingNode(selectedNode);
                          nodeForm.setFieldsValue(selectedNode);
                          setNodeDrawerOpen(true);
                        }}
                      >
                        编辑
                      </Button>
                      <Button
                        onClick={() => {
                          configForm.setFieldsValue({
                            version: `v${(configsQuery.data?.length ?? 0) + 1}`,
                            content: activeConfig?.content ?? "events {}\nhttp {\n  server { listen 80; }\n}\n",
                            activate: true,
                          });
                          setConfigDrawerOpen(true);
                        }}
                      >
                        新增配置
                      </Button>
                    </PermissionGuard>
                  </Space>
                ) : undefined
              }
            >
              <ResourceActivityList
                title="最近操作"
                items={recentOperations.map((task) => ({
                  key: task.id,
                  title: task.type,
                  description: task.summary ?? task.target,
                  meta: `${task.initiatedBy} · ${formatDateTime(task.createdAt)}`,
                  extra: <TaskStatus task={task} />,
                }))}
                emptyText="当前节点还没有最近操作。"
              />

              <ResourceActivityList
                title="配置版本"
                items={(configsQuery.data ?? []).map((config: NginxConfigVersion) => ({
                  key: config.id,
                  title: config.version,
                  description: config.message || config.checksum.slice(0, 12),
                  meta: formatDateTime(config.createdAt),
                  extra: (
                    <Space>
                      <StatusBadge status={config.status} />
                      <PermissionGuard permission="nginx.publish">
                        <Button size="small" type="primary" loading={runAction.isPending} onClick={() => runAction.mutate({ action: "publish", configId: config.id })}>
                          发布
                        </Button>
                      </PermissionGuard>
                      <PermissionGuard permission="nginx.rollback">
                        <Button size="small" loading={runAction.isPending} onClick={() => runAction.mutate({ action: "rollback", configId: config.id })}>
                          回滚
                        </Button>
                      </PermissionGuard>
                    </Space>
                  ),
                }))}
                emptyText="当前节点还没有配置版本。"
              />

              <ResourceActivityList
                title="最近任务"
                actionLabel={selectedNode ? "进入任务中心" : undefined}
                onActionClick={selectedNode ? () => navigate(buildTasksPath({ resourceType: "nginx_node", resourceId: selectedNode.id })) : undefined}
                items={relatedTasks.map((task) => ({
                  key: task.id,
                  title: task.type,
                  description: task.summary ?? task.target,
                  meta: `${task.initiatedBy} · ${formatDateTime(task.createdAt)}`,
                  extra: <TaskStatus task={task} />,
                }))}
                emptyText="当前节点还没有关联任务。"
              />
            </ResourceDetailPanel>
          </div>
        </div>

        <FormDrawer
          open={nodeDrawerOpen}
          title={editingNode ? "编辑 Nginx 节点" : "新增 Nginx 节点"}
          onClose={() => {
            setNodeDrawerOpen(false);
            setEditingNode(null);
          }}
          onSubmit={() => nodeForm.submit()}
          loading={saveNodeMutation.isPending}
        >
          <Form
            layout="vertical"
            form={nodeForm}
            onFinish={(values) =>
              saveNodeMutation.mutate({
                id: editingNode?.id,
                ...values,
              } satisfies NginxNodeInput)
            }
          >
            <Form.Item label="名称" name="name" rules={[{ required: true, message: "请输入节点名称" }]}>
              <Input />
            </Form.Item>
            <Form.Item label="关联主机" name="hostId" rules={[{ required: true, message: "请选择主机" }]}>
              <Select loading={hostsQuery.isLoading} options={hostOptions} />
            </Form.Item>
            <Form.Item label="配置路径" name="configPath">
              <Input />
            </Form.Item>
            <Form.Item label="测试命令" name="testCommand">
              <Input />
            </Form.Item>
            <Form.Item label="重载命令" name="reloadCommand">
              <Input />
            </Form.Item>
            <Form.Item label="说明" name="description">
              <Input.TextArea rows={3} />
            </Form.Item>
          </Form>
        </FormDrawer>

        <FormDrawer
          open={configDrawerOpen}
          title="新增配置版本"
          width={760}
          onClose={() => setConfigDrawerOpen(false)}
          onSubmit={() => configForm.submit()}
          loading={saveConfigMutation.isPending}
        >
          <Form
            layout="vertical"
            form={configForm}
            onFinish={(values) => saveConfigMutation.mutate(values)}
          >
            <Form.Item label="版本号" name="version" rules={[{ required: true, message: "请输入版本号" }]}>
              <Input />
            </Form.Item>
            <Form.Item label="配置内容" name="content" rules={[{ required: true, message: "请输入配置内容" }]}>
              <Input.TextArea rows={14} style={{ fontFamily: "monospace" }} />
            </Form.Item>
            <Form.Item label="说明" name="message">
              <Input.TextArea rows={3} />
            </Form.Item>
            <Form.Item name="activate" valuePropName="checked">
              <Checkbox>保存后设为当前版本</Checkbox>
            </Form.Item>
          </Form>
        </FormDrawer>
      </Space>
    </PermissionGuard>
  );
}
