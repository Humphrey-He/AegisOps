import { Descriptions, Drawer } from "antd";
import type { AuditLog } from "../types/models";
import { formatDateTime } from "../lib/format";
import { StatusBadge } from "./StatusBadge";

type AuditDrawerProps = {
  open: boolean;
  audit: AuditLog | null;
  onClose: () => void;
};

export function AuditDrawer({ open, audit, onClose }: AuditDrawerProps) {
  return (
    <Drawer open={open} title="审计详情" width={560} onClose={onClose}>
      {audit ? (
        <Descriptions column={1} bordered size="small">
          <Descriptions.Item label="操作者">{audit.actor}</Descriptions.Item>
          <Descriptions.Item label="动作">{audit.action}</Descriptions.Item>
          <Descriptions.Item label="资源类型">{audit.resourceType}</Descriptions.Item>
          <Descriptions.Item label="资源名称">{audit.resourceName}</Descriptions.Item>
          <Descriptions.Item label="结果">
            <StatusBadge status={audit.result} />
          </Descriptions.Item>
          <Descriptions.Item label="摘要">{audit.summary}</Descriptions.Item>
          <Descriptions.Item label="Trace ID">{audit.traceId}</Descriptions.Item>
          <Descriptions.Item label="时间">{formatDateTime(audit.createdAt)}</Descriptions.Item>
        </Descriptions>
      ) : null}
    </Drawer>
  );
}
