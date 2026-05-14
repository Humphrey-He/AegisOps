import { Space, Typography } from "antd";
import type { ReactNode } from "react";

type PageHeaderProps = {
  title: string;
  description?: string;
  extra?: ReactNode;
};

export function PageHeader({ title, description, extra }: PageHeaderProps) {
  return (
    <div className="page-toolbar">
      <Space direction="vertical" size={4}>
        <Typography.Title level={3} style={{ margin: 0 }}>
          {title}
        </Typography.Title>
        {description ? <Typography.Text type="secondary">{description}</Typography.Text> : null}
      </Space>
      {extra ? <div className="page-toolbar-end">{extra}</div> : null}
    </div>
  );
}
