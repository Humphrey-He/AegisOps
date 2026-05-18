import { Button, Result, Space, Typography } from "antd";

type ErrorStateProps = {
  title?: string;
  message: string;
  traceId?: string;
  onRetry?: () => void;
};

export function ErrorState({ title = "加载失败", message, traceId, onRetry }: ErrorStateProps) {
  return (
    <div className="error-state">
      <Result
        status="error"
        title={title}
        subTitle={
          <Space direction="vertical" size={8}>
            <Typography.Text>{message}</Typography.Text>
            {traceId ? (
              <Typography.Text type="secondary" copyable={{ text: traceId }}>
                Trace ID: {traceId}
              </Typography.Text>
            ) : (
              <Typography.Text type="secondary">请稍后重试；如果问题持续，请联系管理员。</Typography.Text>
            )}
          </Space>
        }
        extra={
          onRetry ? (
            <Button type="primary" onClick={onRetry}>
              重试
            </Button>
          ) : undefined
        }
      />
    </div>
  );
}
