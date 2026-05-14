import { Button, Result, Typography } from "antd";

type ErrorStateProps = {
  title?: string;
  message: string;
  traceId?: string;
  onRetry?: () => void;
};

export function ErrorState({ title = "加载失败", message, traceId, onRetry }: ErrorStateProps) {
  return (
    <Result
      status="error"
      title={title}
      subTitle={
        <div>
          <div>{message}</div>
          {traceId ? <Typography.Text type="secondary">Trace ID: {traceId}</Typography.Text> : null}
        </div>
      }
      extra={onRetry ? <Button onClick={onRetry}>重试</Button> : undefined}
    />
  );
}
