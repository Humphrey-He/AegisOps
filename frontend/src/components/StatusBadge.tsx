import { Badge } from "antd";

const statusMap: Record<string, { color: string; text: string }> = {
  ACTIVE: { color: "green", text: "启用" },
  DISABLED: { color: "default", text: "禁用" },
  HEALTHY: { color: "green", text: "正常" },
  UNREACHABLE: { color: "red", text: "异常" },
  UNKNOWN: { color: "default", text: "未知" },
  TESTING: { color: "processing", text: "测试中" },
  ONLINE: { color: "green", text: "在线" },
  OFFLINE: { color: "red", text: "离线" },
  DRAFT: { color: "default", text: "草稿" },
  ARCHIVED: { color: "orange", text: "已归档" },
  PENDING: { color: "default", text: "待执行" },
  RUNNING: { color: "processing", text: "执行中" },
  STOPPED: { color: "default", text: "已停止" },
  ROLLBACK: { color: "orange", text: "回滚中" },
  SUCCESS: { color: "green", text: "成功" },
  FAILED: { color: "red", text: "失败" },
  CANCELED: { color: "orange", text: "已取消" },
  CONNECTED: { color: "green", text: "已连接" },
  DISCONNECTED: { color: "red", text: "已断开" },
  running: { color: "green", text: "运行中" },
  exited: { color: "default", text: "已退出" },
  paused: { color: "orange", text: "已暂停" },
};

type StatusBadgeProps = {
  status: string;
};

export function StatusBadge({ status }: StatusBadgeProps) {
  const mapped = statusMap[status] ?? { color: "default", text: status };
  return <Badge color={mapped.color} text={mapped.text} />;
}
