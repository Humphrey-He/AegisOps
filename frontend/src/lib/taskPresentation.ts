import type { Task, TaskDispatchSource } from "../types/models";

const dispatchSourceMeta: Record<TaskDispatchSource, { label: string; color: string }> = {
  MANUAL: { label: "手工触发", color: "blue" },
  SYSTEM: { label: "系统触发", color: "purple" },
  SCHEDULED: { label: "定时触发", color: "gold" },
};

export const taskDispatchSourceOptions = (Object.entries(dispatchSourceMeta) as Array<
  [TaskDispatchSource, { label: string; color: string }]
>).map(([value, meta]) => ({
  label: meta.label,
  value,
}));

export function getTaskDispatchSourceMeta(source?: TaskDispatchSource) {
  return dispatchSourceMeta[source ?? "MANUAL"];
}

export function formatTaskExecutionPolicy(
  task: Pick<Task, "retryCount" | "maxRetry" | "timeoutSeconds" | "concurrencyKey">,
) {
  const parts: string[] = [];

  if (typeof task.maxRetry === "number") {
    parts.push(`重试 ${task.retryCount ?? 0}/${task.maxRetry}`);
  }
  if (typeof task.timeoutSeconds === "number" && task.timeoutSeconds > 0) {
    parts.push(`超时 ${task.timeoutSeconds}s`);
  }
  if (task.concurrencyKey) {
    parts.push(`并发键 ${task.concurrencyKey}`);
  }

  return parts.join(" · ") || "沿用默认执行策略";
}
