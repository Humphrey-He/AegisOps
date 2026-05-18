import { Typography } from "antd";
import type { ReactNode } from "react";

type PageHeaderProps = {
  title: string;
  description?: string;
  eyebrow?: ReactNode;
  extra?: ReactNode;
};

export function PageHeader({ title, description, eyebrow, extra }: PageHeaderProps) {
  return (
    <div className="page-header">
      <div className="page-header-main">
        {eyebrow ? <div className="page-header-eyebrow">{eyebrow}</div> : null}
        <Typography.Title level={3} className="page-header-title">
          {title}
        </Typography.Title>
        {description ? (
          <Typography.Text type="secondary" className="page-header-description" style={{ display: "block" }}>
            {description}
          </Typography.Text>
        ) : null}
      </div>
      {extra ? <div className="page-header-extra">{extra}</div> : null}
    </div>
  );
}
