import { Button, Card, Form, Input, Space, Typography, App as AntApp } from "antd";
import { useMutation } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { authApi } from "../../lib/api";
import { applyFormErrors, getErrorMessage } from "../../lib/forms";
import { useSessionStore } from "../../store/sessionStore";

type SetupFormValues = {
  username: string;
  password: string;
  confirmPassword: string;
  displayName: string;
  email: string;
};

export function SetupAdminPage() {
  const { message } = AntApp.useApp();
  const [form] = Form.useForm<SetupFormValues>();
  const navigate = useNavigate();
  const setInitialized = useSessionStore((state) => state.setInitialized);

  const setupMutation = useMutation({
    mutationFn: authApi.initAdmin,
    onSuccess: async () => {
      setInitialized(true);
      await message.success("管理员初始化完成，请登录控制台。");
      navigate("/login", { replace: true });
    },
    onError: (error) => {
      applyFormErrors(form, error);
      void message.error(getErrorMessage(error));
    },
  });

  return (
    <div className="fullscreen-center">
      <Card className="page-card" style={{ width: 520 }}>
        <Space direction="vertical" size={24} style={{ width: "100%" }}>
          <Space direction="vertical" size={6}>
            <Typography.Title level={2} style={{ margin: 0 }}>
              初始化 AegisOps
            </Typography.Title>
            <Typography.Text type="secondary">
              一期 MVP 先完成管理员初始化，再进入登录与权限配置闭环。
            </Typography.Text>
          </Space>

          <Form
            layout="vertical"
            form={form}
            initialValues={{
              username: "admin",
              password: "AegisOps123!",
              confirmPassword: "AegisOps123!",
              displayName: "平台管理员",
              email: "admin@aegisops.local",
            }}
            onFinish={(values) =>
              setupMutation.mutate({
                username: values.username,
                password: values.password,
                displayName: values.displayName,
                email: values.email,
              })
            }
          >
            <Form.Item label="管理员账号" name="username" rules={[{ required: true, message: "请输入管理员账号" }]}>
              <Input autoFocus />
            </Form.Item>
            <Form.Item label="显示名称" name="displayName" rules={[{ required: true, message: "请输入显示名称" }]}>
              <Input />
            </Form.Item>
            <Form.Item label="邮箱" name="email" rules={[{ required: true, message: "请输入邮箱" }]}>
              <Input />
            </Form.Item>
            <Form.Item label="密码" name="password" rules={[{ required: true, message: "请输入密码" }]}>
              <Input.Password />
            </Form.Item>
            <Form.Item
              label="确认密码"
              name="confirmPassword"
              dependencies={["password"]}
              rules={[
                { required: true, message: "请再次输入密码" },
                ({ getFieldValue }) => ({
                  validator(_, value) {
                    if (!value || getFieldValue("password") === value) {
                      return Promise.resolve();
                    }
                    return Promise.reject(new Error("两次输入的密码不一致"));
                  },
                }),
              ]}
            >
              <Input.Password />
            </Form.Item>
            <Button type="primary" htmlType="submit" block loading={setupMutation.isPending}>
              初始化管理员
            </Button>
          </Form>
        </Space>
      </Card>
    </div>
  );
}
