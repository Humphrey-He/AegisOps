import {
  Alert,
  App as AntApp,
  Button,
  Card,
  Descriptions,
  Drawer,
  Space,
  Typography,
} from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import { dockerApi, servicesApi } from "../../lib/api";
import { queryKeys } from "../../lib/queryKeys";
import { DataTable } from "../../components/DataTable";
import { DangerConfirm } from "../../components/DangerConfirm";
import { ErrorState } from "../../components/ErrorState";
import { LogViewer } from "../../components/LogViewer";
import { PageHeader } from "../../components/PageHeader";
import { PermissionGuard } from "../../components/PermissionGuard";
import { StatusBadge } from "../../components/StatusBadge";
import type { ContainerItem, ServiceDefinition, ServiceInstance } from "../../types/models";

export function DockerNodeDetailPage() {
  const { message } = AntApp.useApp();
  const { nodeId = "" } = useParams();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const queryClient = useQueryClient();
  const [selectedContainer, setSelectedContainer] = useState<ContainerItem | null>(null);
  const [dangerAction, setDangerAction] = useState<{
    action: "start" | "stop" | "restart";
    container: ContainerItem;
  } | null>(null);
  const [pendingContainerAction, setPendingContainerAction] = useState<{
    containerId: string;
    action: "start" | "stop" | "restart";
  } | null>(null);
  const selectedParam = searchParams.get("selected") ?? nodeId;
  const backToListPath = `/docker/nodes?selected=${encodeURIComponent(selectedParam)}`;

  const nodeQuery = useQuery({
    queryKey: queryKeys.dockerNode(nodeId),
    queryFn: () => dockerApi.getNode(nodeId),
    enabled: Boolean(nodeId),
  });

  const containersQuery = useQuery({
    queryKey: queryKeys.containers(nodeId),
    queryFn: () => dockerApi.listContainers(nodeId),
    enabled: Boolean(nodeId),
  });

  const logsQuery = useQuery({
    queryKey: queryKeys.containerLogs(selectedContainer?.id ?? ""),
    queryFn: () => dockerApi.getContainerLogs(nodeId, selectedContainer!.id),
    enabled: Boolean(selectedContainer?.id),
  });

  const servicesQuery = useQuery({
    queryKey: queryKeys.services("", ""),
    queryFn: () => servicesApi.list("", ""),
  });

  const serviceInstancesQuery = useQuery({
    queryKey: ["serviceInstancesByNode", nodeId, servicesQuery.data?.length ?? 0],
    enabled: Boolean(nodeId) && Boolean(servicesQuery.data?.length),
    queryFn: async () => {
      const services = servicesQuery.data ?? [];
      const instances = await Promise.all(
        services.map(async (service) => ({
          service,
          instances: await servicesApi.instances(service.id),
        })),
      );
      return instances.flatMap(({ service, instances }) =>
        instances
          .filter((instance) => instance.dockerNodeId === nodeId)
          .map((instance) => ({ service, instance })),
      );
    },
  });

  const actionMutation = useMutation({
    mutationFn: ({ containerId, action }: { containerId: string; action: "start" | "stop" | "restart" }) =>
      dockerApi.runContainerAction(nodeId, containerId, action),
    onMutate: ({ containerId, action }) => {
      setPendingContainerAction({ containerId, action });
    },
    onSuccess: async (result) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.containers(nodeId) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.tasks }),
      ]);
      setDangerAction(null);
      setPendingContainerAction(null);
      if (result.taskId) {
        navigate(`/tasks/${result.taskId}`);
      }
      await message.success("容器动作已执行");
    },
    onError: async (error) => {
      setPendingContainerAction(null);
      void message.error(error instanceof Error ? error.message : "容器动作执行失败");
    },
  });

  const nodeDescription = useMemo(() => nodeQuery.data, [nodeQuery.data]);
  const nodeServiceLinks = useMemo(() => {
    const links = serviceInstancesQuery.data ?? [];
    const unique = new Map<string, { service: ServiceDefinition; instance: ServiceInstance }>();
    links.forEach((item) => {
      if (!unique.has(item.service.id)) {
        unique.set(item.service.id, item);
      }
    });
    return Array.from(unique.values());
  }, [serviceInstancesQuery.data]);
  const isNodeOffline = nodeDescription?.status === "OFFLINE" || nodeDescription?.status === "UNKNOWN";

  if (nodeQuery.isError) {
    return <ErrorState message={nodeQuery.error.message} onRetry={() => void nodeQuery.refetch()} />;
  }

  return (
    <PermissionGuard permission="docker.view" forbiddenPage>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <PageHeader
          title="节点详情"
          description="容器列表、日志和启停重启操作都先收拢在节点详情页。"
          extra={<Button onClick={() => navigate(backToListPath)}>返回节点列表</Button>}
        />

        <Card className="page-card" loading={nodeQuery.isLoading}>
          <Space direction="vertical" size={16} style={{ width: "100%" }}>
            {isNodeOffline ? (
              <Alert
                type="warning"
                showIcon
                message="节点当前离线，建议先验证连接"
                description="离线或未知状态下，容器日志与容器动作可能无法获得稳定结果。建议先返回节点工作台执行连接测试。"
                action={
                  <Button size="small" onClick={() => navigate(backToListPath)}>
                    回到节点工作台
                  </Button>
                }
              />
            ) : null}
            {nodeDescription ? (
              <Descriptions bordered size="small" column={4}>
                <Descriptions.Item label="名称">{nodeDescription.name}</Descriptions.Item>
                <Descriptions.Item label="状态">
                  <StatusBadge status={nodeDescription.status} />
                </Descriptions.Item>
                <Descriptions.Item label="Endpoint">{nodeDescription.endpoint}</Descriptions.Item>
                <Descriptions.Item label="容器数">{nodeDescription.containerCount}</Descriptions.Item>
              </Descriptions>
            ) : null}
          </Space>
        </Card>

        <Card
          className="page-card"
          title="关联服务实例"
          loading={servicesQuery.isLoading || serviceInstancesQuery.isLoading}
        >
          <DataTable
            rowKey={(record) => `${record.service.id}-${record.instance.id}`}
            pagination={false}
            dataSource={nodeServiceLinks}
            locale={{ emptyText: "当前节点还没有关联服务实例" }}
            columns={[
              {
                title: "服务",
                key: "service",
                render: (_, record) => (
                  <Space direction="vertical" size={2}>
                    <Typography.Text>{record.service.name}</Typography.Text>
                    <Typography.Text type="secondary">{record.service.code}</Typography.Text>
                  </Space>
                ),
              },
              {
                title: "实例",
                key: "instance",
                render: (_, record) => (
                  <Space direction="vertical" size={2}>
                    <Typography.Text>{record.instance.name}</Typography.Text>
                    <Typography.Text type="secondary">{record.instance.version || "--"}</Typography.Text>
                  </Space>
                ),
              },
              {
                title: "状态",
                key: "status",
                render: (_, record) => <StatusBadge status={record.instance.status} />,
              },
              {
                title: "跳转",
                key: "link",
                render: (_, record) => (
                  <Button
                    size="small"
                    type="link"
                    onClick={() => navigate(`/delivery/services?selected=${encodeURIComponent(record.service.id)}`)}
                  >
                    查看服务工作台
                  </Button>
                ),
              },
            ]}
          />
        </Card>

        <Card className="page-card" title="容器列表" loading={containersQuery.isLoading}>
          <DataTable
            rowKey="id"
            dataSource={containersQuery.data}
            columns={[
              { title: "名称", dataIndex: "name" },
              { title: "镜像", dataIndex: "image" },
              { title: "状态", dataIndex: "status", render: (value) => <StatusBadge status={value} /> },
              { title: "端口", dataIndex: "ports", render: (ports: string[]) => ports.join(", ") },
              { title: "重启次数", dataIndex: "restartCount" },
              {
                title: "操作",
                key: "actions",
                width: 320,
                render: (_, container) => (
                  <Space wrap>
                    <Button
                      size="small"
                      type={isNodeOffline ? "default" : "primary"}
                      ghost={!isNodeOffline}
                      onClick={() => setSelectedContainer(container)}
                    >
                      查看日志
                    </Button>
                    <PermissionGuard permission="docker.manage">
                      <Button
                        size="small"
                        loading={
                          actionMutation.isPending &&
                          pendingContainerAction?.containerId === container.id &&
                          pendingContainerAction.action === "start"
                        }
                        disabled={isNodeOffline || actionMutation.isPending}
                        onClick={() => setDangerAction({ action: "start", container })}
                      >
                        启动
                      </Button>
                    </PermissionGuard>
                    <PermissionGuard permission="docker.manage">
                      <Button
                        size="small"
                        loading={
                          actionMutation.isPending &&
                          pendingContainerAction?.containerId === container.id &&
                          pendingContainerAction.action === "stop"
                        }
                        disabled={isNodeOffline || actionMutation.isPending}
                        onClick={() => setDangerAction({ action: "stop", container })}
                      >
                        停止
                      </Button>
                    </PermissionGuard>
                    <PermissionGuard permission="docker.manage">
                      <Button
                        size="small"
                        type="primary"
                        ghost
                        loading={
                          actionMutation.isPending &&
                          pendingContainerAction?.containerId === container.id &&
                          pendingContainerAction.action === "restart"
                        }
                        disabled={isNodeOffline || actionMutation.isPending}
                        onClick={() => setDangerAction({ action: "restart", container })}
                      >
                        重启
                      </Button>
                    </PermissionGuard>
                  </Space>
                ),
              },
            ]}
          />
        </Card>

        <Drawer
          open={Boolean(selectedContainer)}
          title={selectedContainer ? `${selectedContainer.name} 容器日志` : "容器日志"}
          width={760}
          onClose={() => setSelectedContainer(null)}
        >
          {selectedContainer ? (
            <Space direction="vertical" size={16} style={{ width: "100%" }}>
              <div className="page-toolbar">
                <Typography.Text type="secondary">
                  镜像 {selectedContainer.image} · 状态 {selectedContainer.status}
                </Typography.Text>
                <Button size="small" loading={logsQuery.isFetching} onClick={() => void logsQuery.refetch()}>
                  刷新日志
                </Button>
              </div>
              {logsQuery.isError ? (
                <Alert
                  type="error"
                  showIcon
                  message="容器日志读取失败"
                  description={logsQuery.error.message}
                />
              ) : (
                <LogViewer title="容器运行日志" lines={logsQuery.data ?? []} />
              )}
            </Space>
          ) : null}
        </Drawer>

        <DangerConfirm
          open={Boolean(dangerAction)}
          title="确认容器操作"
          description={
            dangerAction
              ? `即将对容器 ${dangerAction.container.name} 执行 ${toActionLabel(dangerAction.action)} 操作。`
              : ""
          }
          confirmText={dangerAction?.container.name}
          loading={actionMutation.isPending}
          onCancel={() => setDangerAction(null)}
          onConfirm={() => {
            if (!dangerAction) {
              return;
            }
            actionMutation.mutate({
              containerId: dangerAction.container.id,
              action: dangerAction.action,
            });
          }}
        />
      </Space>
    </PermissionGuard>
  );
}

function toActionLabel(action: "start" | "stop" | "restart") {
  if (action === "start") return "启动";
  if (action === "stop") return "停止";
  return "重启";
}
