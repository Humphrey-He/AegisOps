import {
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
import { useNavigate, useParams } from "react-router-dom";
import { dockerApi } from "../../lib/api";
import { queryKeys } from "../../lib/queryKeys";
import { DataTable } from "../../components/DataTable";
import { DangerConfirm } from "../../components/DangerConfirm";
import { LogViewer } from "../../components/LogViewer";
import { PageHeader } from "../../components/PageHeader";
import { PermissionGuard } from "../../components/PermissionGuard";
import { StatusBadge } from "../../components/StatusBadge";
import type { ContainerItem } from "../../types/models";

export function DockerNodeDetailPage() {
  const { message } = AntApp.useApp();
  const { nodeId = "" } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [selectedContainer, setSelectedContainer] = useState<ContainerItem | null>(null);
  const [dangerAction, setDangerAction] = useState<{
    action: "start" | "stop" | "restart";
    container: ContainerItem;
  } | null>(null);

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

  const actionMutation = useMutation({
    mutationFn: ({ containerId, action }: { containerId: string; action: "start" | "stop" | "restart" }) =>
      dockerApi.runContainerAction(nodeId, containerId, action),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.containers(nodeId) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.tasks }),
      ]);
      setDangerAction(null);
      await message.success("容器动作已执行");
    },
  });

  const nodeDescription = useMemo(() => nodeQuery.data, [nodeQuery.data]);

  return (
    <PermissionGuard permission="docker.view" forbiddenPage>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <PageHeader
          title="节点详情"
          description="容器列表、日志和启停重启操作都先收拢在节点详情页。"
          extra={<Button onClick={() => navigate("/docker/nodes")}>返回节点列表</Button>}
        />

        <Card className="page-card" loading={nodeQuery.isLoading}>
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
                    <Button size="small" onClick={() => setSelectedContainer(container)}>
                      查看日志
                    </Button>
                    <PermissionGuard permission="docker.manage">
                      <Button size="small" onClick={() => setDangerAction({ action: "start", container })}>
                        启动
                      </Button>
                    </PermissionGuard>
                    <PermissionGuard permission="docker.manage">
                      <Button size="small" onClick={() => setDangerAction({ action: "stop", container })}>
                        停止
                      </Button>
                    </PermissionGuard>
                    <PermissionGuard permission="docker.manage">
                      <Button size="small" type="primary" ghost onClick={() => setDangerAction({ action: "restart", container })}>
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
              <Typography.Text type="secondary">
                镜像 {selectedContainer.image} · 状态 {selectedContainer.status}
              </Typography.Text>
              <LogViewer title="容器运行日志" lines={logsQuery.data ?? []} />
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
