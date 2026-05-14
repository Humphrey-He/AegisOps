import { Button, Empty, Space, Typography } from "antd";
import type { ReactNode } from "react";

type EmptyStateProps = {
  title: string;
  description?: string;
  action?: ReactNode;
};

export function EmptyState({ title, description, action }: EmptyStateProps) {
  return (
    <Empty
      description={
        <Space direction="vertical" size={4}>
          <Typography.Text strong>{title}</Typography.Text>
          {description ? <Typography.Text type="secondary">{description}</Typography.Text> : null}
          {typeof action === "string" ? <Button type="primary">{action}</Button> : action}
        </Space>
      }
    />
  );
}
