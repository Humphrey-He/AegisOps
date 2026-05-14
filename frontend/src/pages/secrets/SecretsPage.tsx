import {
  App as AntApp,
  Button,
  Card,
  Form,
  Input,
  Select,
  Space,
  Typography,
} from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { secretsApi } from "../../lib/api";
import { queryKeys } from "../../lib/queryKeys";
import { applyFormErrors, getErrorMessage } from "../../lib/forms";
import { DataTable } from "../../components/DataTable";
import { FormDrawer } from "../../components/FormDrawer";
import { PageHeader } from "../../components/PageHeader";
import { PermissionGuard } from "../../components/PermissionGuard";
import { SecretInput } from "../../components/SecretInput";
import type { Secret, SecretInputPayload } from "../../types/models";

type SecretFormValues = {
  name: string;
  type: Secret["type"];
  username?: string;
  description?: string;
  secretValue: string;
};

export function SecretsPage() {
  const { message } = AntApp.useApp();
  const [form] = Form.useForm<SecretFormValues>();
  const [keyword, setKeyword] = useState("");
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editingSecret, setEditingSecret] = useState<Secret | null>(null);
  const queryClient = useQueryClient();

  const secretsQuery = useQuery({
    queryKey: queryKeys.secrets(keyword),
    queryFn: () => secretsApi.list(keyword),
  });

  const saveMutation = useMutation({
    mutationFn: secretsApi.save,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["secrets"] });
      await queryClient.invalidateQueries({ queryKey: ["audits"] });
      await message.success(editingSecret ? "凭证已更新" : "凭证已创建");
      setDrawerOpen(false);
      setEditingSecret(null);
      form.resetFields();
    },
    onError: (error) => {
      applyFormErrors(form, error);
      void message.error(getErrorMessage(error));
    },
  });

  return (
    <PermissionGuard permission="secrets.view" forbiddenPage>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <PageHeader
          title="凭证"
          description="一期只把 SSH 凭证打磨扎实，Docker Token 类型先保留结构。"
          extra={
            <PermissionGuard permission="secrets.manage">
              <Button
                type="primary"
                onClick={() => {
                  setEditingSecret(null);
                  form.resetFields();
                  form.setFieldsValue({ type: "SSH_PRIVATE_KEY" });
                  setDrawerOpen(true);
                }}
              >
                新增凭证
              </Button>
            </PermissionGuard>
          }
        />

        <Card className="page-card">
          <Input.Search allowClear placeholder="搜索凭证名称、类型或用户名" style={{ width: 320 }} onSearch={setKeyword} />
        </Card>

        <Card className="page-card">
          <DataTable
            rowKey="id"
            loading={secretsQuery.isLoading}
            dataSource={secretsQuery.data}
            columns={[
              {
                title: "名称",
                dataIndex: "name",
              },
              {
                title: "类型",
                dataIndex: "type",
              },
              {
                title: "用户名",
                dataIndex: "username",
                render: (value) => value ?? "--",
              },
              {
                title: "脱敏内容",
                dataIndex: "valueMasked",
                render: (value) => <Typography.Text code>{value}</Typography.Text>,
              },
              {
                title: "引用资源",
                dataIndex: "usedBy",
                render: (usedBy: string[]) => (usedBy.length ? usedBy.join(", ") : "--"),
              },
              {
                title: "操作",
                key: "actions",
                render: (_, secret) => (
                  <PermissionGuard permission="secrets.manage">
                    <Button
                      size="small"
                      onClick={() => {
                        setEditingSecret(secret);
                        form.setFieldsValue({
                          name: secret.name,
                          type: secret.type,
                          username: secret.username,
                          description: secret.description,
                          secretValue: "",
                        });
                        setDrawerOpen(true);
                      }}
                    >
                      编辑
                    </Button>
                  </PermissionGuard>
                ),
              },
            ]}
          />
        </Card>

        <FormDrawer
          open={drawerOpen}
          title={editingSecret ? "编辑凭证" : "新增凭证"}
          onClose={() => {
            setDrawerOpen(false);
            setEditingSecret(null);
          }}
          onSubmit={() => form.submit()}
          loading={saveMutation.isPending}
        >
          <Form
            form={form}
            layout="vertical"
            onFinish={(values) =>
              saveMutation.mutate({
                id: editingSecret?.id,
                name: values.name,
                type: values.type,
                username: values.username,
                description: values.description,
                secretValue: values.secretValue,
              } satisfies SecretInputPayload)
            }
          >
            <Form.Item label="名称" name="name" rules={[{ required: true, message: "请输入凭证名称" }]}>
              <Input />
            </Form.Item>
            <Form.Item label="类型" name="type" rules={[{ required: true, message: "请选择凭证类型" }]}>
              <Select
                options={[
                  { label: "SSH 私钥", value: "SSH_PRIVATE_KEY" },
                  { label: "SSH 密码", value: "SSH_PASSWORD" },
                  { label: "Docker Token", value: "DOCKER_TOKEN" },
                ]}
              />
            </Form.Item>
            <Form.Item label="用户名" name="username">
              <Input />
            </Form.Item>
            <Form.Item
              label={editingSecret ? "新的凭证内容" : "凭证内容"}
              name="secretValue"
              rules={[{ required: true, message: "请输入凭证内容" }]}
            >
              <SecretInput />
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
