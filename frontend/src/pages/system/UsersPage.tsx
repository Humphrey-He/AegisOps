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
import { useMemo, useState } from "react";
import { rolesApi, usersApi } from "../../lib/api";
import { queryKeys } from "../../lib/queryKeys";
import { applyFormErrors, getErrorMessage } from "../../lib/forms";
import { DataTable } from "../../components/DataTable";
import { FormDrawer } from "../../components/FormDrawer";
import { PageHeader } from "../../components/PageHeader";
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

  const saveMutation = useMutation({
    mutationFn: usersApi.save,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["users"] });
      await queryClient.invalidateQueries({ queryKey: queryKeys.audits });
      await message.success(editingUser ? "用户已更新" : "用户已创建");
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
          description="管理员在这里完成人员接入、启停控制和角色绑定。"
          extra={
            <PermissionGuard permission="users.manage">
              <Button
                type="primary"
                onClick={() => {
                  setEditingUser(null);
                  form.resetFields();
                  form.setFieldsValue({ status: "ACTIVE", roleIds: [] });
                  setDrawerOpen(true);
                }}
              >
                新增用户
              </Button>
            </PermissionGuard>
          }
        />

        <Card className="page-card">
          <Input.Search allowClear placeholder="搜索用户名、姓名或邮箱" style={{ width: 320 }} onSearch={setKeyword} />
        </Card>

        <Card className="page-card">
          <DataTable
            rowKey="id"
            loading={usersQuery.isLoading}
            dataSource={usersQuery.data}
            columns={[
              { title: "用户名", dataIndex: "username" },
              { title: "姓名", dataIndex: "displayName" },
              { title: "邮箱", dataIndex: "email" },
              { title: "状态", dataIndex: "status", render: (value) => <StatusBadge status={value} /> },
              {
                title: "角色数",
                dataIndex: "roleIds",
                render: (roleIds: string[]) => roleIds.length,
              },
              {
                title: "操作",
                key: "actions",
                render: (_, user) => (
                  <PermissionGuard permission="users.manage">
                    <Button
                      size="small"
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
                      {currentUser?.id === user.id ? "编辑自己" : "编辑"}
                    </Button>
                  </PermissionGuard>
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
                password: values.password,
              } satisfies UserInput)
            }
          >
            <Form.Item label="用户名" name="username" rules={[{ required: true, message: "请输入用户名" }]}>
              <Input />
            </Form.Item>
            <Form.Item label="姓名" name="displayName" rules={[{ required: true, message: "请输入姓名" }]}>
              <Input />
            </Form.Item>
            <Form.Item label="邮箱" name="email" rules={[{ required: true, message: "请输入邮箱" }]}>
              <Input />
            </Form.Item>
            <Form.Item label="状态" name="status" rules={[{ required: true, message: "请选择状态" }]}>
              <Select options={[{ label: "启用", value: "ACTIVE" }, { label: "禁用", value: "DISABLED" }]} />
            </Form.Item>
            <Form.Item label="绑定角色" name="roleIds" rules={[{ required: true, message: "请选择角色" }]}>
              <Select mode="multiple" options={roleOptions} />
            </Form.Item>
            <Form.Item label={editingUser ? "重置密码" : "初始密码"} name="password">
              <Input.Password placeholder={editingUser ? "留空则不修改" : "请输入初始密码"} />
            </Form.Item>
          </Form>
        </FormDrawer>
      </Space>
    </PermissionGuard>
  );
}
