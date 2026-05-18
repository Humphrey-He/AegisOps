import { LockOutlined, SafetyCertificateOutlined, UserOutlined } from "@ant-design/icons";
import { Alert, App as AntApp, Button, Card, Form, Input, Space, Typography } from "antd";
import { useMutation } from "@tanstack/react-query";
import { useLocation, useNavigate } from "react-router-dom";
import { authApi } from "../../lib/api";
import { applyFormErrors, getErrorMessage } from "../../lib/forms";
import { useSessionStore } from "../../store/sessionStore";
import { AuthLayout } from "./AuthLayout";

type LoginFormValues = {
  username: string;
  password: string;
};

export function LoginPage() {
  const { message } = AntApp.useApp();
  const [form] = Form.useForm<LoginFormValues>();
  const navigate = useNavigate();
  const location = useLocation();
  const initialized = useSessionStore((state) => state.initialized);
  const setSession = useSessionStore((state) => state.setSession);
  const targetPath = (location.state as { from?: string } | undefined)?.from ?? "/dashboard";
  const shouldAutoFocus = typeof window !== "undefined" && window.matchMedia("(min-width: 961px)").matches;

  const loginMutation = useMutation({
    mutationFn: authApi.login,
    onSuccess: async (result) => {
      setSession(result);
      await message.success(`欢迎回来，${result.user.displayName}`);
      navigate(targetPath, { replace: true });
    },
    onError: (error) => {
      applyFormErrors(form, error);
      void message.error(getErrorMessage(error, "登录失败，请检查账号或密码。"));
    },
  });

  return (
    <AuthLayout>
      <Card className="page-card auth-card">
        <Space direction="vertical" size={22} style={{ width: "100%" }}>
          <div>
            <Typography.Title level={2} className="auth-card-title">
              登录 AegisOps
            </Typography.Title>
            <Typography.Text type="secondary">使用管理员或授权账号进入控制台。</Typography.Text>
          </div>

          {loginMutation.isError ? (
            <Alert type="error" showIcon message={getErrorMessage(loginMutation.error, "登录失败，请检查账号或密码。")} />
          ) : null}

          <Form
            layout="vertical"
            form={form}
            onFinish={(values) => loginMutation.mutate(values)}
            onValuesChange={() => loginMutation.reset()}
          >
            <Form.Item label="用户名" name="username" rules={[{ required: true, message: "请输入用户名" }]}>
              <Input
                autoFocus={shouldAutoFocus}
                autoComplete="username"
                prefix={<UserOutlined />}
                placeholder="请输入用户名"
              />
            </Form.Item>
            <Form.Item label="密码" name="password" rules={[{ required: true, message: "请输入密码" }]}>
              <Input.Password
                autoComplete="current-password"
                prefix={<LockOutlined />}
                placeholder="请输入密码"
                onPressEnter={() => form.submit()}
              />
            </Form.Item>
            <Button type="primary" htmlType="submit" block loading={loginMutation.isPending}>
              登录控制台
            </Button>
          </Form>

          {!initialized ? (
            <div className="auth-card-footer">
              <Typography.Text type="secondary">首次部署 AegisOps？</Typography.Text>
              <Button type="link" onClick={() => navigate("/setup/admin")}>
                初始化系统管理员
              </Button>
            </div>
          ) : null}

          <div className="auth-security-note">
            <Space size={8} align="start">
              <SafetyCertificateOutlined />
              <Typography.Text type="secondary">
                登录后操作将进入审计记录，请使用本人账号访问控制台。
              </Typography.Text>
            </Space>
          </div>
        </Space>
      </Card>
    </AuthLayout>
  );
}
