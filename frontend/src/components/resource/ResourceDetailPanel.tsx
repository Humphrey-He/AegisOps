import { Card, Empty, Space, Typography } from "antd";
import type { ReactNode } from "react";

type ResourceMetaItem = {
  label: string;
  value: ReactNode;
};

type ResourceDetailPanelProps = {
  title?: string;
  subtitle?: string;
  status?: ReactNode;
  actions?: ReactNode;
  meta?: ResourceMetaItem[];
  emptyTitle?: string;
  emptyDescription?: string;
  children?: ReactNode;
};

export function ResourceDetailPanel({
  title,
  subtitle,
  status,
  actions,
  meta = [],
  emptyTitle = "选择一个资源",
  emptyDescription = "点击左侧列表中的资源后，这里会显示它的上下文信息、操作入口和最近活动。",
  children,
}: ResourceDetailPanelProps) {
  if (!title) {
    return (
      <Card className="page-card resource-detail-card">
        <div className="resource-detail-empty">
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={
              <Space direction="vertical" size={4}>
                <Typography.Text strong>{emptyTitle}</Typography.Text>
                <Typography.Text type="secondary">{emptyDescription}</Typography.Text>
              </Space>
            }
          />
        </div>
      </Card>
    );
  }

  return (
    <Card className="page-card resource-detail-card">
      <Space direction="vertical" size={20} style={{ width: "100%" }}>
        <div className="resource-detail-header">
          <Space direction="vertical" size={4}>
            <Typography.Title level={4} style={{ margin: 0 }}>
              {title}
            </Typography.Title>
            {subtitle ? <Typography.Text type="secondary">{subtitle}</Typography.Text> : null}
          </Space>
          {status ? <div className="resource-detail-status">{status}</div> : null}
        </div>

        {actions ? <div className="resource-detail-actions">{actions}</div> : null}

        {meta.length ? (
          <div className="resource-detail-metadata">
            {meta.map((item) => (
              <div key={item.label} className="resource-detail-metadata-item">
                <Typography.Text type="secondary" className="resource-detail-metadata-label">
                  {item.label}
                </Typography.Text>
                <div className="resource-detail-metadata-value">{item.value}</div>
              </div>
            ))}
          </div>
        ) : null}

        {children}
      </Space>
    </Card>
  );
}
