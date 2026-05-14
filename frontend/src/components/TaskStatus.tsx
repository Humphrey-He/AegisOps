import { Progress, Space } from "antd";
import type { Task } from "../types/models";
import { StatusBadge } from "./StatusBadge";

type TaskStatusProps = {
  task: Pick<Task, "status" | "progress">;
};

export function TaskStatus({ task }: TaskStatusProps) {
  return (
    <Space direction="vertical" size={4} style={{ width: 180 }}>
      <StatusBadge status={task.status} />
      <Progress percent={task.progress} size="small" showInfo={false} />
    </Space>
  );
}
