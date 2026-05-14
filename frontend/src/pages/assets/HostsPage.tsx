import {
  App as AntApp,
  Button,
  Card,
  Form,
  Input,
  Select,
  Space,
  Tag,
} from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { hostsApi, secretsApi } from "../../lib/api";
import { queryKeys } from "../../lib/queryKeys";
import { applyFormErrors, getErrorMessage } from "../../lib/forms";
import { DataTable } from "../../components/DataTable";
import { EmptyState } from "../../components/EmptyState";
import { ErrorState } from "../../components/ErrorState";
import { FormDrawer } from "../../components/FormDrawer";
import { PageHeader } from "../../components/PageHeader";
import { PermissionGuard } from "../../components/PermissionGuard";
import { StatusBadge } from "../../components/StatusBadge";
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
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const hostsQuery = useQuery({
    queryKey: queryKeys.hosts(keyword),
    queryFn: () => hostsApi.list(keyword),
  });
  const secretsQuery = useQuery({
    queryKey: queryKeys.secrets(""),
    queryFn: () => secretsApi.list(""),
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
        queryClient.invalidateQueries({ queryKey: queryKeys.tasks }),
      ]);
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
                onSearch={setKeyword}
              />
            </div>
          </div>
        </Card>

        <Card className="page-card">
          <DataTable
            rowKey="id"
            loading={hostsQuery.isLoading}
            dataSource={hostsQuery.data}
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
                render: (value) => value ?? "--",
              },
              {
                title: "操作",
                key: "actions",
                width: 280,
                render: (_, host) => (
                  <Space wrap>
                    <PermissionGuard permission="hosts.manage">
                      <Button size="small" onClick={() => testMutation.mutate(host.id)} loading={testMutation.isPending}>
                        SSH 测试
                      </Button>
                    </PermissionGuard>
                    <PermissionGuard permission="terminal.open">
                      <Button
                        size="small"
                        type="primary"
                        onClick={async () => {
                          const session = await hostsToTerminal(host.id);
                          navigate(`/terminal/${session}`);
                        }}
                      >
                        打开终端
                      </Button>
                    </PermissionGuard>
                    <PermissionGuard permission="hosts.manage">
                      <Button
                        size="small"
                        onClick={() => {
                          setEditingHost(host);
                          form.setFieldsValue(host);
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
