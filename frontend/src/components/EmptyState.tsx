import { Button, Empty, Space, Typography } from "antd";
import type { ReactNode } from "react";

type EmptyStateProps = {
  title: string;
  description?: string;
  action?: ReactNode;
  image?: ReactNode;
};

export function EmptyState({ title, description, action, image }: EmptyStateProps) {
  return (
    <div className="empty-state">
      <Empty
        image={image ?? Empty.PRESENTED_IMAGE_SIMPLE}
        description={
          <Space direction="vertical" size={8}>
            <Typography.Text strong>{title}</Typography.Text>
            {description ? <Typography.Text type="secondary">{description}</Typography.Text> : null}
            {typeof action === "string" ? <Button type="primary">{action}</Button> : action}
          </Space>
        }
      />
    </div>
  );
}
