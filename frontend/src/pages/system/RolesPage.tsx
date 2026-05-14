import {
  App as AntApp,
  Button,
  Card,
  Checkbox,
  Form,
  Input,
  Space,
  Tag,
  Typography,
} from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { rolesApi } from "../../lib/api";
import { permissionGroups } from "../../lib/permissions";
import { queryKeys } from "../../lib/queryKeys";
import { applyFormErrors, getErrorMessage } from "../../lib/forms";
import { DataTable } from "../../components/DataTable";
import { FormDrawer } from "../../components/FormDrawer";
import { PageHeader } from "../../components/PageHeader";
import { PermissionGuard } from "../../components/PermissionGuard";
import type { Role, RoleInput } from "../../types/models";

type RoleFormValues = {
  name: string;
  description: string;
  permissions: string[];
};

export function RolesPage() {
  const { message } = AntApp.useApp();
  const [form] = Form.useForm<RoleFormValues>();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editingRole, setEditingRole] = useState<Role | null>(null);
  const queryClient = useQueryClient();

  const rolesQuery = useQuery({
    queryKey: queryKeys.roles,
    queryFn: rolesApi.list,
  });

  const saveMutation = useMutation({
    mutationFn: rolesApi.save,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.roles });
      await queryClient.invalidateQueries({ queryKey: queryKeys.audits });
      await message.success(editingRole ? "角色已更新" : "角色已创建");
      setDrawerOpen(false);
      setEditingRole(null);
      form.resetFields();
    },
    onError: (error) => {
      applyFormErrors(form, error);
      void message.error(getErrorMessage(error));
    },
  });

  return (
    <PermissionGuard permission="roles.view" forbiddenPage>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <PageHeader
          title="角色管理"
          description="一期不单独做权限页，直接在角色里配置简化 RBAC。"
          extra={
            <PermissionGuard permission="roles.manage">
              <Button
                type="primary"
                onClick={() => {
                  setEditingRole(null);
                  form.resetFields();
                  form.setFieldsValue({ permissions: [] });
                  setDrawerOpen(true);
                }}
              >
                新增角色
              </Button>
            </PermissionGuard>
          }
        />

        <Card className="page-card">
          <DataTable
            rowKey="id"
            loading={rolesQuery.isLoading}
            dataSource={rolesQuery.data}
            columns={[
              {
                title: "角色",
                dataIndex: "name",
                render: (_, role) => (
                  <Space>
                    <span>{role.name}</span>
                    {role.builtIn ? <Tag color="blue">内置</Tag> : null}
                  </Space>
                ),
              },
              { title: "描述", dataIndex: "description" },
              {
                title: "权限数",
                dataIndex: "permissions",
                render: (permissions: string[]) => permissions.length,
              },
              {
                title: "操作",
                key: "actions",
                render: (_, role) => (
                  <PermissionGuard permission="roles.manage">
                    <Button
                      size="small"
                      onClick={() => {
                        setEditingRole(role);
                        form.setFieldsValue({
                          name: role.name,
                          description: role.description,
                          permissions: role.permissions,
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
          title={editingRole ? "编辑角色" : "新增角色"}
          width={700}
          onClose={() => {
            setDrawerOpen(false);
            setEditingRole(null);
          }}
          onSubmit={() => form.submit()}
          loading={saveMutation.isPending}
        >
          <Form
            form={form}
            layout="vertical"
            onFinish={(values) =>
              saveMutation.mutate({
                id: editingRole?.id,
                name: values.name,
                description: values.description,
                permissions: values.permissions,
              } satisfies RoleInput)
            }
          >
            <Form.Item label="角色名称" name="name" rules={[{ required: true, message: "请输入角色名称" }]}>
              <Input />
            </Form.Item>
            <Form.Item label="描述" name="description">
              <Input.TextArea rows={3} />
            </Form.Item>
            <Form.Item label="权限" name="permissions">
              <Checkbox.Group style={{ width: "100%" }}>
                <Space direction="vertical" size={16} style={{ width: "100%" }}>
                  {Object.entries(permissionGroups).map(([group, items]) => (
                    <Card key={group} size="small">
                      <Space direction="vertical" size={12} style={{ width: "100%" }}>
                        <Typography.Text strong>{group}</Typography.Text>
                        <Space direction="vertical" size={8} style={{ width: "100%" }}>
                          {items.map((item) => (
                            <Checkbox key={item.key} value={item.key}>
                              {item.label}
                              <Typography.Text type="secondary"> {item.description}</Typography.Text>
                            </Checkbox>
                          ))}
                        </Space>
                      </Space>
                    </Card>
                  ))}
                </Space>
              </Checkbox.Group>
            </Form.Item>
          </Form>
        </FormDrawer>
      </Space>
    </PermissionGuard>
  );
}
