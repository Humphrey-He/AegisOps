import { Space, Switch, Typography } from "antd";
import { useEffect, useMemo, useRef, useState } from "react";
import type { TaskLog } from "../types/models";

type LogViewerProps = {
  lines: string[] | TaskLog[];
  title?: string;
};

export function LogViewer({ lines, title }: LogViewerProps) {
  const [autoScroll, setAutoScroll] = useState(true);
  const ref = useRef<HTMLDivElement | null>(null);

  const rendered = useMemo(
    () =>
      lines.map((item) =>
        typeof item === "string"
          ? {
              key: item,
              text: item,
              level: "INFO",
            }
          : {
              key: item.id,
              text: `${item.timestamp} [${item.level}] ${item.message}`,
              level: item.level,
            },
      ),
    [lines],
  );

  useEffect(() => {
    if (autoScroll && ref.current) {
      ref.current.scrollTop = ref.current.scrollHeight;
    }
  }, [autoScroll, rendered]);

  return (
    <Space direction="vertical" size={12} style={{ width: "100%" }}>
      <div className="page-toolbar">
        <Typography.Text strong>{title ?? "日志"}</Typography.Text>
        <Space>
          <Typography.Text type="secondary">自动滚动</Typography.Text>
          <Switch checked={autoScroll} onChange={setAutoScroll} />
        </Space>
      </div>
      <div className="log-viewer" ref={ref}>
        {rendered.map((item) => (
          <div
            key={item.key}
            className={`log-line${item.level === "ERROR" ? " log-line--error" : item.level === "WARN" ? " log-line--warn" : ""}`}
          >
            {item.text}
          </div>
        ))}
      </div>
    </Space>
  );
}
