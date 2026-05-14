import { Button, Empty, Space, Typography } from "antd";
import type { ReactNode } from "react";

export type ResourceActivityItem = {
  key: string;
  title: ReactNode;
  description?: ReactNode;
  meta?: ReactNode;
  extra?: ReactNode;
};

type ResourceActivityListProps = {
  title: string;
  actionLabel?: string;
  onActionClick?: () => void;
  items: ResourceActivityItem[];
  emptyText: string;
};

export function ResourceActivityList({
  title,
  actionLabel,
  onActionClick,
  items,
  emptyText,
}: ResourceActivityListProps) {
  return (
    <div className="resource-detail-section">
      <div className="page-toolbar">
        <Typography.Text strong>{title}</Typography.Text>
        {actionLabel && onActionClick ? (
          <Button type="link" size="small" onClick={onActionClick}>
            {actionLabel}
          </Button>
        ) : null}
      </div>

      {items.length ? (
        <div className="resource-activity-list">
          {items.map((item) => (
            <div key={item.key} className="resource-activity-item">
              <div className="resource-activity-main">
                <Space direction="vertical" size={4} style={{ width: "100%" }}>
                  <Typography.Text strong>{item.title}</Typography.Text>
                  {item.description ? (
                    <Typography.Text type="secondary">{item.description}</Typography.Text>
                  ) : null}
                  {item.meta ? <div className="resource-activity-meta">{item.meta}</div> : null}
                </Space>
              </div>
              {item.extra ? <div className="resource-activity-extra">{item.extra}</div> : null}
            </div>
          ))}
        </div>
      ) : (
        <div className="resource-activity-empty">
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={emptyText} />
        </div>
      )}
    </div>
  );
}
