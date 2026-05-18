import { Button, Card, Form, Input, Space, Typography, App as AntApp } from "antd";
import { useMutation } from "@tanstack/react-query";
import { useLocation, useNavigate } from "react-router-dom";
import { authApi } from "../../lib/api";
import { applyFormErrors, getErrorMessage } from "../../lib/forms";
import { useSessionStore } from "../../store/sessionStore";

type LoginFormValues = {
  username: string;
  password: string;
};

export function LoginPage() {
  const { message } = AntApp.useApp();
  const [form] = Form.useForm<LoginFormValues>();
  const navigate = useNavigate();
  const location = useLocation();
  const setSession = useSessionStore((state) => state.setSession);
  const targetPath = (location.state as { from?: string } | undefined)?.from ?? "/dashboard";

  const loginMutation = useMutation({
    mutationFn: authApi.login,
    onSuccess: async (result) => {
      setSession(result);
      await message.success(`欢迎回来，${result.user.displayName}`);
      navigate(targetPath, { replace: true });
    },
    onError: (error) => {
      applyFormErrors(form, error);
      void message.error(getErrorMessage(error, "登录失败，请检查账号信息。"));
    },
  });

  return (
    <div className="fullscreen-center">
      <Card className="page-card" style={{ width: 460 }}>
        <Space direction="vertical" size={24} style={{ width: "100%" }}>
          <Space direction="vertical" size={6}>
            <Typography.Title level={2} style={{ margin: 0 }}>
              登录 AegisOps
            </Typography.Title>
            <Typography.Text type="secondary">
              登录后进入运维控制台，查看资产、服务、任务与审计记录。
            </Typography.Text>
          </Space>

          <Form
            layout="vertical"
            form={form}
            initialValues={{ username: "admin", password: "admin123456" }}
            onFinish={(values) => loginMutation.mutate(values)}
          >
            <Form.Item label="用户名" name="username" rules={[{ required: true, message: "请输入用户名" }]}>
              <Input autoFocus />
            </Form.Item>
            <Form.Item label="密码" name="password" rules={[{ required: true, message: "请输入密码" }]}>
              <Input.Password onPressEnter={() => form.submit()} />
            </Form.Item>
            <Button type="primary" htmlType="submit" block loading={loginMutation.isPending}>
              登录控制台
            </Button>
          </Form>

          <Typography.Text type="secondary">
            如尚未初始化系统管理员，请先完成系统初始化。
          </Typography.Text>
        </Space>
      </Card>
    </div>
  );
}
