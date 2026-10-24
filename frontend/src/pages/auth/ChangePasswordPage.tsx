import { LockOutlined, SafetyCertificateOutlined } from "@ant-design/icons";
import { Alert, App as AntApp, Button, Card, Form, Input, Space, Typography } from "antd";
import { useMutation } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { authApi } from "../../lib/api";
import { getErrorMessage } from "../../lib/forms";
import { useSessionStore } from "../../store/sessionStore";
import { AuthLayout } from "./AuthLayout";

type ChangePasswordFormValues = {
  oldPassword: string;
  newPassword: string;
  confirmPassword: string;
};

export function ChangePasswordPage() {
  const { message } = AntApp.useApp();
  const [form] = Form.useForm<ChangePasswordFormValues>();
  const navigate = useNavigate();
  const clearSession = useSessionStore((state) => state.clearSession);

  const changeMutation = useMutation({
    mutationFn: (values: ChangePasswordFormValues) =>
      authApi.changePassword({ oldPassword: values.oldPassword, newPassword: values.newPassword }),
    onSuccess: () => {
      clearSession();
      void message.success("密码已更新，请使用新密码重新登录");
      navigate("/login", { replace: true });
    },
    onError: (error) => {
      void message.error(getErrorMessage(error, "修改密码失败，请检查旧密码是否正确。"));
    },
  });

  return (
    <AuthLayout>
      <Card className="page-card auth-card">
        <Space direction="vertical" size={22} style={{ width: "100%" }}>
          <div>
            <Typography.Title level={2} className="auth-card-title">
              修改初始密码
            </Typography.Title>
            <Typography.Text type="secondary">
              初始密码是公开信息，首次登录后必须修改才能继续使用控制台。
            </Typography.Text>
          </div>

          {changeMutation.isError ? (
            <Alert
              type="error"
              showIcon
              message={getErrorMessage(changeMutation.error, "修改密码失败，请检查旧密码是否正确。")}
            />
          ) : null}

          <Form
            layout="vertical"
            form={form}
            onFinish={(values) => changeMutation.mutate(values)}
            onValuesChange={() => changeMutation.reset()}
          >
            <Form.Item label="当前密码" name="oldPassword" rules={[{ required: true, message: "请输入当前密码" }]}>
              <Input.Password
                autoFocus
                autoComplete="current-password"
                prefix={<LockOutlined />}
                placeholder="请输入当前密码"
              />
            </Form.Item>
            <Form.Item
              label="新密码"
              name="newPassword"
              rules={[
                { required: true, message: "请输入新密码" },
                { min: 8, message: "新密码至少 8 位" },
              ]}
            >
              <Input.Password autoComplete="new-password" prefix={<LockOutlined />} placeholder="至少 8 位" />
            </Form.Item>
            <Form.Item
              label="确认新密码"
              name="confirmPassword"
              dependencies={["newPassword"]}
              rules={[
                { required: true, message: "请再次输入新密码" },
                ({ getFieldValue }) => ({
                  validator: (_rule, value) =>
                    !value || value === getFieldValue("newPassword")
                      ? Promise.resolve()
                      : Promise.reject(new Error("两次输入的新密码不一致")),
                }),
              ]}
            >
              <Input.Password autoComplete="new-password" prefix={<LockOutlined />} placeholder="再次输入新密码" />
            </Form.Item>
            <Button type="primary" htmlType="submit" block loading={changeMutation.isPending}>
              更新密码并重新登录
            </Button>
          </Form>

          <div className="auth-security-note">
            <Space size={8} align="start">
              <SafetyCertificateOutlined />
              <Typography.Text type="secondary">修改成功后需要使用新密码重新登录。</Typography.Text>
            </Space>
          </div>
        </Space>
      </Card>
    </AuthLayout>
  );
}
