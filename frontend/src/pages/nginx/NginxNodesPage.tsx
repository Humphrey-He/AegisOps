import { Alert, App as AntApp, Button, Card, Checkbox, Form, Input, Select, Space, Tag, Typography } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";

import { PermissionActionButton } from "../../components/PermissionActionButton";
import { exportsApi, hostsApi, nginxApi, resourcesApi } from "../../lib/api";
import { queryKeys } from "../../lib/queryKeys";
import { applyFormErrors, getErrorMessage } from "../../lib/forms";
import { DataTable } from "../../components/DataTable";
import { EmptyState } from "../../components/EmptyState";
import { FormDrawer } from "../../components/FormDrawer";
import { PageHeader } from "../../components/PageHeader";
import { PermissionGuard } from "../../components/PermissionGuard";
import { ResourceActivityList } from "../../components/resource/ResourceActivityList";
import { ResourceDetailPanel } from "../../components/resource/ResourceDetailPanel";
import { StatusBadge } from "../../components/StatusBadge";
import { TaskStatus } from "../../components/TaskStatus";
import { formatDateTime } from "../../lib/format";
import { buildTasksPath } from "../../lib/resourceNavigation";
import { useSessionStore } from "../../store/sessionStore";
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
  const permissions = useSessionStore((state) => state.permissions);
  const canViewExports = permissions.includes("*") || permissions.includes("exports.view");
  const [nodeForm] = Form.useForm<NodeFormValues>();
  const [configForm] = Form.useForm<ConfigFormValues>();
  const [keyword, setKeyword] = useState("");
  const [nodeDrawerOpen, setNodeDrawerOpen] = useState(false);
  const [configDrawerOpen, setConfigDrawerOpen] = useState(false);
  const [editingNode, setEditingNode] = useState<NginxNode | null>(null);
  const [latestActionText, setLatestActionText] = useState<string | null>(null);
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
  const resourceContextQuery = useQuery({
    queryKey: queryKeys.resourceContext("nginx_node", selectedNodeId),
    queryFn: () => resourcesApi.context("nginx_node", selectedNodeId),
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
    return (resourceContextQuery.data?.recentTasks ?? []).slice(0, 5);
  }, [resourceContextQuery.data?.recentTasks, selectedNode]);
  const recentOperations = useMemo(() => {
    return relatedTasks.filter((task) =>
      ["nginx.node.test", "nginx.node.reload", "nginx.config.publish", "nginx.config.rollback"].includes(task.type),
    );
  }, [relatedTasks]);
  const nodeAlertEvents = useMemo(() => {
    if (!selectedNode) {
      return [];
    }
    return resourceContextQuery.data?.recentAlerts ?? [];
  }, [resourceContextQuery.data?.recentAlerts, selectedNode]);
  const latestOperation = recentOperations[0] ?? null;
  const latestFailureOperation = recentOperations.find((item) => item.status === "FAILED") ?? null;
  const latestNodeAlert = nodeAlertEvents[0] ?? null;
  const unresolvedAlertCount = useMemo(
    () => nodeAlertEvents.filter((event) => event.status !== "RESOLVED").length,
    [nodeAlertEvents],
  );
  const primaryAction = selectedNode?.status === "ONLINE" ? "reload" : "test";
  const nodeSummaryItems = [
    {
      label: "节点总数",
      value: nodesQuery.data?.length ?? 0,
      helper: keyword ? `当前按关键词 "${keyword}" 过滤` : "当前纳管的 Nginx 节点",
    },
    {
      label: "在线节点",
      value: (nodesQuery.data ?? []).filter((node) => node.status === "ONLINE").length,
      helper: "可直接执行配置测试、发布和重载动作",
    },
    {
      label: "异常节点",
      value: (nodesQuery.data ?? []).filter((node) => node.status === "OFFLINE" || node.status === "UNKNOWN").length,
      helper: "建议优先核查主机可达性与 Nginx 命令链路",
    },
    {
      label: "已测试节点",
      value: (nodesQuery.data ?? []).filter((node) => Boolean(node.lastTestAt)).length,
      helper: "至少保留过一条配置测试记录",
    },
  ];

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
        queryClient.invalidateQueries({ queryKey: queryKeys.resourceContext("nginx_node", selectedNodeId) }),
      ]);
      setLatestActionText(
        result.taskId
          ? `动作已提交，任务 ${result.taskId} 可在当前节点任务中继续跟踪测试、发布或回滚结果。`
          : "动作已提交，可继续在当前节点上下文查看后续变化。",
      );
      await message.success("动作已进入任务中心");
    },
    onError: (error) => {
      setLatestActionText(null);
      void message.error(getErrorMessage(error));
    },
  });

  const exportConfigMutation = useMutation({
    mutationFn: ({ configId }: { configId: string }) => exportsApi.exportNginxConfig(configId),
    onSuccess: async (job) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.exports });
      await message.success("Nginx 配置导出已创建");
      if (canViewExports) {
        navigate(`/settings/exports?selected=${encodeURIComponent(job.id)}`);
      }
    },
    onError: (error) => {
      void message.error(getErrorMessage(error, "创建 Nginx 配置导出失败"));
    },
  });

  const hostOptions = (hostsQuery.data ?? []).map((host) => ({ label: `${host.name} (${host.address})`, value: host.id }));

  return (
    <PermissionGuard permission="nginx.view" forbiddenPage>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <PageHeader
          title="Nginx"
          description="管理 Nginx 节点、配置版本，并将测试、重载、回滚串入任务中心。"
          eyebrow="边缘入口 / Nginx 工作台"
          extra={
            <PermissionGuard permission="nginx.manage">
              <Button
                type="primary"
                onClick={() => {
                  setEditingNode(null);
                  nodeForm.resetFields();
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

        <Card className="page-card">
          <div className="page-toolbar">
            <div className="page-toolbar-start">
              <Input.Search
                allowClear
                placeholder="搜索节点名称、主机或配置路径"
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
                locale={{
                  emptyText: (
                    <EmptyState
                      title="还没有 Nginx 节点"
                      description="先绑定主机并录入测试、重载命令，后续就可以在这里完成配置版本管理与发布动作。"
                      action={
                        <PermissionGuard permission="nginx.manage">
                          <Button
                            type="primary"
                            onClick={() => {
                              setEditingNode(null);
                              nodeForm.resetFields();
                              nodeForm.setFieldsValue({
                                configPath: "/etc/nginx/nginx.conf",
                                testCommand: "nginx -t",
                                reloadCommand: "nginx -s reload",
                              });
                              setNodeDrawerOpen(true);
                            }}
                          >
                            新增第一个节点
                          </Button>
                        </PermissionGuard>
                      }
                    />
                  ),
                }}
                columns={[
                  {
                    title: "节点",
                    dataIndex: "name",
                    render: (_, node) => (
                      <Space direction="vertical" size={2}>
                        <span>{node.name}</span>
                        <span style={{ color: "#64748b" }}>{node.configPath}</span>
                      </Space>
                    ),
                  },
                  {
                    title: "主机",
                    dataIndex: "hostName",
                    render: (_, node) => node.hostName || node.hostId,
                  },
                  { title: "状态", dataIndex: "status", render: (value) => <StatusBadge status={value} /> },
                  {
                    title: "最近测试",
                    dataIndex: "lastTestAt",
                    render: (value?: string) => (value ? formatDateTime(value) : "--"),
                  },
                ]}
              />
            </Card>
          </div>

          <div className="resource-detail-pane">
            <ResourceDetailPanel
              title={selectedNode?.name}
              kicker={selectedNode ? "配置发布工作台" : undefined}
              subtitle={selectedNode?.configPath}
              status={selectedNode ? <StatusBadge status={selectedNode.status} /> : undefined}
              helper={
                selectedNode
                  ? "围绕当前节点集中查看配置路径、测试与重载链路、版本记录和最近异常，减少在节点列表、任务中心与发布动作之间反复切换。"
                  : undefined
              }
              highlights={
                selectedNode
                  ? [
                      {
                        label: "最近测试",
                        value: selectedNode.lastTestAt ? formatDateTime(selectedNode.lastTestAt) : "--",
                        helper:
                          selectedNode.status === "ONLINE"
                            ? "节点在线时可直接继续发布配置或执行重载"
                            : "建议先执行配置测试，确认节点与命令链路可用",
                      },
                      {
                        label: "配置版本",
                        value: configsQuery.data?.length ?? 0,
                        helper: activeConfig ? `当前激活版本 ${activeConfig.version}` : "尚未记录可回滚的激活版本",
                      },
                      {
                        label: "待处理信号",
                        value: unresolvedAlertCount,
                        helper: latestNodeAlert
                          ? `最近告警触发于 ${formatDateTime(latestNodeAlert.lastTriggeredAt)}`
                          : "当前没有打开中的节点告警",
                      },
                    ]
                  : []
              }
              meta={
                selectedNode
                  ? [
                      { label: "主机", value: selectedNode.hostName || selectedNode.hostId },
                      { label: "测试命令", value: <Typography.Text code>{selectedNode.testCommand}</Typography.Text> },
                      { label: "重载命令", value: <Typography.Text code>{selectedNode.reloadCommand}</Typography.Text> },
                      { label: "最近测试", value: selectedNode.lastTestAt ? formatDateTime(selectedNode.lastTestAt) : "--" },
                      { label: "最近重载", value: selectedNode.lastReloadAt ? formatDateTime(selectedNode.lastReloadAt) : "--" },
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
                      { label: "说明", value: selectedNode.description || "--" },
                    ]
                  : []
              }
              actions={
                selectedNode ? (
                  <>
                    <div className="resource-action-group">
                      <PermissionGuard permission="nginx.test">
                        <Button
                          type={primaryAction === "test" ? "primary" : "default"}
                          loading={runAction.isPending}
                          onClick={() => {
                            setLatestActionText("正在执行 Nginx 配置测试...");
                            runAction.mutate({ action: "test" });
                          }}
                        >
                          测试配置
                        </Button>
                      </PermissionGuard>
                      <PermissionGuard permission="nginx.reload">
                        <Button
                          type={primaryAction === "reload" ? "primary" : "default"}
                          loading={runAction.isPending}
                          onClick={() => {
                            setLatestActionText("正在提交 Nginx 重载任务...");
                            runAction.mutate({ action: "reload" });
                          }}
                        >
                          重载
                        </Button>
                      </PermissionGuard>
                    </div>
                    <div className="resource-action-group">
                      <PermissionGuard permission="nginx.manage">
                        <Button
                          onClick={() => {
                            configForm.resetFields();
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
                      <Button
                        onClick={() =>
                          navigate(buildTasksPath({ resourceType: "nginx_node", resourceId: selectedNode.id }))
                        }
                      >
                        查看任务
                      </Button>
                      <PermissionGuard permission="nginx.manage">
                        <Button
                          onClick={() => {
                            setEditingNode(selectedNode);
                            nodeForm.setFieldsValue(selectedNode);
                            setNodeDrawerOpen(true);
                          }}
                        >
                          编辑节点
                        </Button>
                      </PermissionGuard>
                    </div>
                  </>
                ) : undefined
              }
            >
              {selectedNode && latestNodeAlert?.status === "OPEN" ? (
                <div className="resource-detail-section">
                  <Alert
                    type="warning"
                    showIcon
                    message={`当前节点存在待处理告警：${latestNodeAlert.summary || latestNodeAlert.eventType}`}
                    description={
                      latestNodeAlert.detail ||
                      "建议先确认配置测试和重载链路是否正常，再结合最近任务判断是否需要回滚到上一版配置。"
                    }
                    action={
                      <Button
                        size="small"
                        onClick={() => navigate(buildTasksPath({ resourceType: "nginx_node", resourceId: selectedNode.id }))}
                      >
                        查看任务
                      </Button>
                    }
                  />
                </div>
              ) : null}

              {selectedNode && !latestNodeAlert && (selectedNode.status === "OFFLINE" || selectedNode.status === "UNKNOWN") ? (
                <div className="resource-detail-section">
                  <Alert
                    type="warning"
                    showIcon
                    message="当前节点未处于稳定状态"
                    description="建议先执行配置测试，确认主机关联、配置路径和 Nginx 命令链路都可达，再继续发布或重载。"
                    action={
                      <PermissionGuard permission="nginx.test">
                        <Button
                          size="small"
                          onClick={() => {
                            setLatestActionText("正在执行 Nginx 配置测试...");
                            runAction.mutate({ action: "test" });
                          }}
                        >
                          立即测试
                        </Button>
                      </PermissionGuard>
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
                title="最近操作"
                helper="优先查看配置测试、重载、发布与回滚动作，先判断当前节点最近经历了什么。"
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
                helper="这里保留当前节点的配置版本与状态，方便在测试后直接发布新版本或回滚到历史版本。"
                items={(configsQuery.data ?? []).map((config: NginxConfigVersion) => ({
                  key: config.id,
                  title: config.version,
                  description: config.message || config.checksum.slice(0, 12),
                  meta: formatDateTime(config.createdAt),
                  extra: (
                    <Space>
                      <PermissionActionButton
                        size="small"
                        permission="exports.create"
                        permissionReason="当前账号缺少 exports.create 权限，无法创建配置导出。"
                        loading={exportConfigMutation.isPending}
                        onClick={() => exportConfigMutation.mutate({ configId: config.id })}
                      >
                        导出
                      </PermissionActionButton>
                      <StatusBadge status={config.status} />
                      <PermissionGuard permission="nginx.publish">
                        <Button
                          size="small"
                          type="primary"
                          loading={runAction.isPending}
                          onClick={() => {
                            setLatestActionText(`正在提交配置版本 ${config.version} 的发布任务...`);
                            runAction.mutate({ action: "publish", configId: config.id });
                          }}
                        >
                          发布
                        </Button>
                      </PermissionGuard>
                      <PermissionGuard permission="nginx.rollback">
                        <Button
                          size="small"
                          loading={runAction.isPending}
                          onClick={() => {
                            setLatestActionText(`正在提交配置版本 ${config.version} 的回滚任务...`);
                            runAction.mutate({ action: "rollback", configId: config.id });
                          }}
                        >
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
                helper="当测试、重载或版本发布进入异步执行后，可以在任务中心继续跟踪结果与失败原因。"
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
