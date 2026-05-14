import {
  App as AntApp,
  Button,
  Card,
  Form,
  Input,
  Select,
  Space,
} from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { dockerApi } from "../../lib/api";
import { queryKeys } from "../../lib/queryKeys";
import { applyFormErrors, getErrorMessage } from "../../lib/forms";
import { DataTable } from "../../components/DataTable";
import { FormDrawer } from "../../components/FormDrawer";
import { PageHeader } from "../../components/PageHeader";
import { PermissionGuard } from "../../components/PermissionGuard";
import { StatusBadge } from "../../components/StatusBadge";
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
  const queryClient = useQueryClient();
  const navigate = useNavigate();

  const nodesQuery = useQuery({
    queryKey: queryKeys.dockerNodes,
    queryFn: dockerApi.listNodes,
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
        queryClient.invalidateQueries({ queryKey: queryKeys.tasks }),
      ]);
      await message.success("节点检测已完成");
    },
    onError: (error) => {
      void message.error(getErrorMessage(error));
    },
  });

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

        <Card className="page-card">
          <DataTable
            rowKey="id"
            loading={nodesQuery.isLoading}
            dataSource={nodesQuery.data}
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
              },
              {
                title: "操作",
                key: "actions",
                width: 280,
                render: (_, node) => (
                  <Space wrap>
                    <Button size="small" type="link" onClick={() => navigate(`/docker/nodes/${node.id}`)}>
                      查看容器
                    </Button>
                    <PermissionGuard permission="docker.manage">
                      <Button size="small" onClick={() => testMutation.mutate(node.id)} loading={testMutation.isPending}>
                        测试连接
                      </Button>
                    </PermissionGuard>
                    <PermissionGuard permission="docker.manage">
                      <Button
                        size="small"
                        onClick={() => {
                          setEditingNode(node);
                          form.setFieldsValue({
                            name: node.name,
                            endpoint: node.endpoint,
                            tlsEnabled: node.tlsEnabled,
                            description: node.description,
                          });
                          setDrawerOpen(true);
                        }}
                      >
                        编辑
                      </Button>
                    </PermissionGuard>
                  </Space>
                ),
              },
            ]}
          />
        </Card>

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
