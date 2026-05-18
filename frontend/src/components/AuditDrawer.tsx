import { Descriptions, Drawer } from "antd";
import {
  formatAuditActor,
  formatAuditResourceName,
  formatAuditSummary,
  getResourceTypeLabel,
} from "../lib/resourceNavigation";
import { formatDateTime } from "../lib/format";
import type { AuditLog } from "../types/models";
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
          <Descriptions.Item label="操作人">{formatAuditActor(audit.actor)}</Descriptions.Item>
          <Descriptions.Item label="动作">{audit.action}</Descriptions.Item>
          <Descriptions.Item label="资源类型">{getResourceTypeLabel(audit.resourceType)}</Descriptions.Item>
          <Descriptions.Item label="资源">{formatAuditResourceName(audit)}</Descriptions.Item>
          <Descriptions.Item label="结果">
            <StatusBadge status={audit.result} />
          </Descriptions.Item>
          <Descriptions.Item label="摘要">{formatAuditSummary(audit.summary, audit.action)}</Descriptions.Item>
          <Descriptions.Item label="Trace ID">{audit.traceId || "--"}</Descriptions.Item>
          <Descriptions.Item label="时间">{formatDateTime(audit.createdAt)}</Descriptions.Item>
        </Descriptions>
      ) : null}
    </Drawer>
  );
}
