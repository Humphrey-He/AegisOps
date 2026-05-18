type StatusTone = "success" | "processing" | "warning" | "danger" | "neutral";

type StatusMeta = {
  tone: StatusTone;
  text: string;
};

const statusMap: Record<string, StatusMeta> = {
  ACTIVE: { tone: "success", text: "启用" },
  HEALTHY: { tone: "success", text: "正常" },
  ONLINE: { tone: "success", text: "在线" },
  SUCCESS: { tone: "success", text: "成功" },
  RESOLVED: { tone: "success", text: "已关闭" },
  CONNECTED: { tone: "success", text: "已连接" },
  running: { tone: "success", text: "运行中" },

  TESTING: { tone: "processing", text: "测试中" },
  PENDING: { tone: "processing", text: "待执行" },
  DISPATCHED: { tone: "processing", text: "已分发" },
  RUNNING: { tone: "processing", text: "执行中" },
  ROLLBACK: { tone: "processing", text: "回滚中" },
  CONNECTING: { tone: "processing", text: "连接中" },

  WARNING: { tone: "warning", text: "警告" },
  WARN: { tone: "warning", text: "警告" },
  ACKED: { tone: "warning", text: "已确认" },
  CANCELED: { tone: "warning", text: "已取消" },
  INFO: { tone: "warning", text: "信息" },
  paused: { tone: "warning", text: "已暂停" },

  UNREACHABLE: { tone: "danger", text: "异常" },
  OFFLINE: { tone: "danger", text: "离线" },
  FAILED: { tone: "danger", text: "失败" },
  TIMEOUT: { tone: "danger", text: "超时" },
  OPEN: { tone: "danger", text: "待处理" },
  CRITICAL: { tone: "danger", text: "严重" },
  DISCONNECTED: { tone: "danger", text: "已断开" },
  ERROR: { tone: "danger", text: "错误" },

  DISABLED: { tone: "neutral", text: "禁用" },
  UNKNOWN: { tone: "neutral", text: "未知" },
  DRAFT: { tone: "neutral", text: "草稿" },
  ARCHIVED: { tone: "neutral", text: "已归档" },
  STOPPED: { tone: "neutral", text: "已停止" },
  exited: { tone: "neutral", text: "已退出" },
};

type StatusBadgeProps = {
  status?: string | null;
};

export function StatusBadge({ status }: StatusBadgeProps) {
  const normalizedStatus = status?.trim() || "UNKNOWN";
  const mapped = statusMap[normalizedStatus] ?? { tone: "neutral" as const, text: normalizedStatus };

  return (
    <span className={`status-badge status-badge--${mapped.tone}`} title={normalizedStatus} role="status">
      <span className="status-badge__dot" aria-hidden />
      <span>{mapped.text}</span>
    </span>
  );
}
