import { Card, Empty, Space, Typography } from "antd";
import type { ReactNode } from "react";

type ResourceMetaItem = {
  label: string;
  value: ReactNode;
};

type ResourceHighlightItem = {
  label: string;
  value: ReactNode;
  helper?: ReactNode;
};

type ResourceDetailPanelProps = {
  title?: string;
  subtitle?: string;
  kicker?: ReactNode;
  status?: ReactNode;
  helper?: ReactNode;
  actions?: ReactNode;
  highlights?: ResourceHighlightItem[];
  meta?: ResourceMetaItem[];
  emptyTitle?: string;
  emptyDescription?: string;
  children?: ReactNode;
};

export function ResourceDetailPanel({
  title,
  subtitle,
  kicker,
  status,
  helper,
  actions,
  highlights = [],
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
        <div className="resource-detail-summary">
          {kicker ? <div className="resource-detail-kicker">{kicker}</div> : null}

          <div className="resource-detail-hero">
            <div className="resource-detail-hero-main">
              <Typography.Title level={4} className="resource-detail-title">
                {title}
              </Typography.Title>
              {subtitle ? (
                <Typography.Text type="secondary" className="resource-detail-subtitle" style={{ display: "block" }}>
                  {subtitle}
                </Typography.Text>
              ) : null}
            </div>
            {status ? <div className="resource-detail-status resource-detail-status-cluster">{status}</div> : null}
          </div>

          {helper ? <div className="resource-detail-helper">{helper}</div> : null}
        </div>

        {actions ? <div className="resource-detail-actions">{actions}</div> : null}

        {highlights.length ? (
          <div className="resource-detail-highlights">
            {highlights.map((item) => (
              <div key={item.label} className="resource-highlight-card">
                <Typography.Text className="resource-highlight-label">{item.label}</Typography.Text>
                <div className="resource-highlight-value">{item.value}</div>
                {item.helper ? <div className="resource-highlight-helper">{item.helper}</div> : null}
              </div>
            ))}
          </div>
        ) : null}

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
