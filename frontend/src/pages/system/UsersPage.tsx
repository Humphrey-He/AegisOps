import { App as AntApp, Alert, Button, Card, Form, Input, Select, Space, Tag, Typography } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { rolesApi, usersApi } from "../../lib/api";
import { queryKeys } from "../../lib/queryKeys";
import { applyFormErrors, getErrorMessage } from "../../lib/forms";
import { DataTable } from "../../components/DataTable";
import { FormDrawer } from "../../components/FormDrawer";
import { PageHeader } from "../../components/PageHeader";
import { PermissionActionButton } from "../../components/PermissionActionButton";
import { PermissionGuard } from "../../components/PermissionGuard";
import { StatusBadge } from "../../components/StatusBadge";
import { useSessionStore } from "../../store/sessionStore";
import type { User, UserInput } from "../../types/models";

type UserFormValues = {
  username: string;
  displayName: string;
  email: string;
  status: "ACTIVE" | "DISABLED";
  roleIds: string[];
  password?: string;
};

export function UsersPage() {
  const { message } = AntApp.useApp();
  const [form] = Form.useForm<UserFormValues>();
  const [keyword, setKeyword] = useState("");
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editingUser, setEditingUser] = useState<User | null>(null);
  const currentUser = useSessionStore((state) => state.user);
  const queryClient = useQueryClient();

  const usersQuery = useQuery({
    queryKey: queryKeys.users(keyword),
    queryFn: () => usersApi.list(keyword),
  });
  const rolesQuery = useQuery({
    queryKey: queryKeys.roles,
    queryFn: rolesApi.list,
  });

  const roleOptions = useMemo(
    () => (rolesQuery.data ?? []).map((item) => ({ label: item.name, value: item.id })),
    [rolesQuery.data],
  );
  const roleNameMap = useMemo(
    () => new Map((rolesQuery.data ?? []).map((role) => [role.id, role.name])),
    [rolesQuery.data],
  );

  const saveMutation = useMutation({
    mutationFn: usersApi.save,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["users"] });
      await queryClient.invalidateQueries({ queryKey: queryKeys.audits });
      await message.success(editingUser ? "用户已更新" : "用户已创建，可使用用户名和初始密码登录");
      setDrawerOpen(false);
      setEditingUser(null);
      form.resetFields();
    },
    onError: (error) => {
      applyFormErrors(form, error);
      void message.error(getErrorMessage(error));
    },
  });

  return (
    <PermissionGuard permission="users.view" forbiddenPage>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <PageHeader
          title="用户管理"
          description="管理员在这里新增用户、重置密码、启停账号并绑定角色。当前列表只展示你有权查看的用户。"
          extra={
            <PermissionActionButton
              type="primary"
              permission="users.manage"
              permissionReason="当前账号缺少 users.manage 权限，无法新增或编辑用户。"
              onClick={() => {
                setEditingUser(null);
                form.resetFields();
                form.setFieldsValue({ status: "ACTIVE", roleIds: [], password: "" });
                setDrawerOpen(true);
              }}
            >
              新增用户
            </PermissionActionButton>
          }
        />

        <Alert
          type="info"
          showIcon
          message="新增用户如何进入系统"
          description="管理员创建用户并设置初始密码后，把登录地址、用户名和初始密码交给该用户。用户使用普通登录页进入系统；能看到哪些菜单取决于这里绑定的角色权限。"
        />

        <Card className="page-card">
          <Input.Search allowClear placeholder="搜索用户名、姓名或邮箱" style={{ width: 320 }} onSearch={setKeyword} />
        </Card>

        <Card className="page-card">
          <DataTable
            rowKey="id"
            loading={usersQuery.isLoading || rolesQuery.isLoading}
            dataSource={usersQuery.data}
            columns={[
              { title: "用户名", dataIndex: "username" },
              { title: "姓名", dataIndex: "displayName" },
              { title: "邮箱", dataIndex: "email" },
              { title: "状态", dataIndex: "status", render: (value) => <StatusBadge status={value} /> },
              {
                title: "角色",
                dataIndex: "roleIds",
                render: (roleIds: string[]) =>
                  roleIds.length ? (
                    <Space wrap>
                      {roleIds.map((roleId) => (
                        <Tag key={roleId}>{roleNameMap.get(roleId) ?? roleId}</Tag>
                      ))}
                    </Space>
                  ) : (
                    <Typography.Text type="secondary">未绑定</Typography.Text>
                  ),
              },
              {
                title: "操作",
                key: "actions",
                render: (_, user) => (
                  <PermissionActionButton
                    size="small"
                    permission="users.manage"
                    permissionReason="当前账号缺少 users.manage 权限，无法编辑账号、角色或密码。"
                    onClick={() => {
                      setEditingUser(user);
                      form.setFieldsValue({
                        username: user.username,
                        displayName: user.displayName,
                        email: user.email,
                        status: user.status,
                        roleIds: user.roleIds,
                        password: "",
                      });
                      setDrawerOpen(true);
                    }}
                  >
                    {currentUser?.id === user.id ? "编辑自己" : "编辑/重置密码"}
                  </PermissionActionButton>
                ),
              },
            ]}
          />
        </Card>

        <FormDrawer
          open={drawerOpen}
          title={editingUser ? "编辑用户" : "新增用户"}
          onClose={() => {
            setDrawerOpen(false);
            setEditingUser(null);
          }}
          onSubmit={() => form.submit()}
          loading={saveMutation.isPending}
        >
          <Form
            form={form}
            layout="vertical"
            onFinish={(values) =>
              saveMutation.mutate({
                id: editingUser?.id,
                username: values.username,
                displayName: values.displayName,
                email: values.email,
                status: values.status,
                roleIds: values.roleIds,
                password: values.password?.trim() || undefined,
              } satisfies UserInput)
            }
          >
            <Form.Item label="用户名" name="username" rules={[{ required: true, message: "请输入用户名" }]}>
              <Input disabled={Boolean(editingUser)} />
            </Form.Item>
            <Form.Item label="姓名" name="displayName" rules={[{ required: true, message: "请输入姓名" }]}>
              <Input />
            </Form.Item>
            <Form.Item label="邮箱" name="email" rules={[{ required: true, message: "请输入邮箱" }, { type: "email", message: "邮箱格式不正确" }]}>
              <Input />
            </Form.Item>
            <Form.Item label="状态" name="status" rules={[{ required: true, message: "请选择状态" }]}>
              <Select options={[{ label: "启用", value: "ACTIVE" }, { label: "禁用", value: "DISABLED" }]} />
            </Form.Item>
            <Form.Item label="绑定角色" name="roleIds" rules={[{ required: true, message: "请选择角色" }]}>
              <Select mode="multiple" options={roleOptions} placeholder="选择后决定用户可访问的菜单和操作" />
            </Form.Item>
            <Form.Item
              label={editingUser ? "重置密码" : "初始密码"}
              name="password"
              extra={editingUser ? "留空则不修改密码；填写后用户下次可使用新密码登录。" : "创建后用户使用该密码在登录页进入系统。"}
              rules={editingUser ? [] : [{ required: true, message: "请输入初始密码" }, { min: 8, message: "密码至少 8 位" }]}
            >
              <Input.Password placeholder={editingUser ? "留空不修改" : "至少 8 位"} />
            </Form.Item>
          </Form>
        </FormDrawer>
      </Space>
    </PermissionGuard>
  );
}
