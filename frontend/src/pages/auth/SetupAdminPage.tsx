import { LockOutlined, MailOutlined, SafetyCertificateOutlined, UserOutlined } from "@ant-design/icons";
import { Alert, App as AntApp, Button, Card, Form, Input, Space, Typography } from "antd";
import { useMutation } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { authApi } from "../../lib/api";
import { applyFormErrors, getErrorMessage } from "../../lib/forms";
import { useSessionStore } from "../../store/sessionStore";
import { AuthLayout } from "./AuthLayout";

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
  const shouldAutoFocus = typeof window !== "undefined" && window.matchMedia("(min-width: 961px)").matches;

  const setupMutation = useMutation({
    mutationFn: authApi.initAdmin,
    onSuccess: async () => {
      setInitialized(true);
      await message.success("管理员初始化完成，请登录控制台。");
      navigate("/login", { replace: true });
    },
    onError: (error) => {
      applyFormErrors(form, error);
      void message.error(getErrorMessage(error, "初始化失败，请检查输入信息。"));
    },
  });

  return (
    <AuthLayout
      panelTitle="首次部署初始化"
      panelDescription="创建第一位系统管理员后，即可进入 AegisOps 配置角色权限、资源接入和审计策略。"
    >
      <Card className="page-card auth-card auth-card--wide">
        <Space direction="vertical" size={22} style={{ width: "100%" }}>
          <div>
            <Typography.Title level={2} className="auth-card-title">
              初始化 AegisOps
            </Typography.Title>
            <Typography.Text type="secondary">创建第一位系统管理员，并继续完成访问权限配置。</Typography.Text>
          </div>

          {setupMutation.isError ? (
            <Alert type="error" showIcon message={getErrorMessage(setupMutation.error, "初始化失败，请检查输入信息。")} />
          ) : null}

          <Form
            layout="vertical"
            form={form}
            onValuesChange={() => setupMutation.reset()}
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
              <Input
                autoFocus={shouldAutoFocus}
                autoComplete="username"
                prefix={<UserOutlined />}
                placeholder="请输入管理员账号"
              />
            </Form.Item>
            <Form.Item label="显示名称" name="displayName" rules={[{ required: true, message: "请输入显示名称" }]}>
              <Input prefix={<UserOutlined />} placeholder="请输入显示名称" />
            </Form.Item>
            <Form.Item label="邮箱" name="email" rules={[{ required: true, message: "请输入邮箱" }]}>
              <Input autoComplete="email" prefix={<MailOutlined />} placeholder="请输入邮箱" />
            </Form.Item>
            <Form.Item label="密码" name="password" rules={[{ required: true, message: "请输入密码" }]}>
              <Input.Password autoComplete="new-password" prefix={<LockOutlined />} placeholder="请输入密码" />
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
              <Input.Password autoComplete="new-password" prefix={<LockOutlined />} placeholder="请再次输入密码" />
            </Form.Item>
            <Space direction="vertical" size={12} style={{ width: "100%" }}>
              <Button type="primary" htmlType="submit" block loading={setupMutation.isPending}>
                初始化管理员
              </Button>
              <Button block onClick={() => navigate("/login")}>
                返回登录
              </Button>
            </Space>
          </Form>

          <div className="auth-security-note">
            <Space size={8} align="start">
              <SafetyCertificateOutlined />
              <Typography.Text type="secondary">
                初始化完成后，后续登录、权限分配与关键变更都会纳入审计记录。
              </Typography.Text>
            </Space>
          </div>
        </Space>
      </Card>
    </AuthLayout>
  );
}
