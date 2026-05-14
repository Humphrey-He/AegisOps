import { Card, Descriptions, List, Space } from "antd";
import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router-dom";
import { tasksApi } from "../../lib/api";
import { queryKeys } from "../../lib/queryKeys";
import { formatDateTime } from "../../lib/format";
import { ErrorState } from "../../components/ErrorState";
import { LogViewer } from "../../components/LogViewer";
import { PageHeader } from "../../components/PageHeader";
import { PermissionGuard } from "../../components/PermissionGuard";
import { StatusBadge } from "../../components/StatusBadge";

export function TaskDetailPage() {
  const { taskId = "" } = useParams();
  const taskQuery = useQuery({
    queryKey: queryKeys.task(taskId),
    queryFn: () => tasksApi.detail(taskId),
    enabled: Boolean(taskId),
    refetchInterval: 3000,
  });

  if (taskQuery.isError) {
    return <ErrorState message={taskQuery.error.message} onRetry={() => void taskQuery.refetch()} />;
  }

  const task = taskQuery.data;

  return (
    <PermissionGuard permission="tasks.view" forbiddenPage>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <PageHeader title="任务详情" description="任务步骤与日志是一期前端最关键的追踪面板之一。" />

        <Card className="page-card" loading={taskQuery.isLoading}>
          {task ? (
            <Descriptions bordered size="small" column={4}>
              <Descriptions.Item label="类型">{task.type}</Descriptions.Item>
              <Descriptions.Item label="目标">{task.target}</Descriptions.Item>
              <Descriptions.Item label="状态">
                <StatusBadge status={task.status} />
              </Descriptions.Item>
              <Descriptions.Item label="进度">{task.progress}%</Descriptions.Item>
              <Descriptions.Item label="发起人">{task.initiatedBy}</Descriptions.Item>
              <Descriptions.Item label="创建时间">{formatDateTime(task.createdAt)}</Descriptions.Item>
              <Descriptions.Item label="开始时间">{formatDateTime(task.startedAt)}</Descriptions.Item>
              <Descriptions.Item label="结束时间">{formatDateTime(task.finishedAt)}</Descriptions.Item>
            </Descriptions>
          ) : null}
        </Card>

        <Card className="page-card" title="步骤" loading={taskQuery.isLoading}>
          <List
            dataSource={task?.steps ?? []}
            renderItem={(step) => (
              <List.Item>
                <List.Item.Meta title={step.title} description={step.detail ?? `${formatDateTime(step.startedAt)} -> ${formatDateTime(step.finishedAt)}`} />
                <StatusBadge status={step.status} />
              </List.Item>
            )}
          />
        </Card>

        <Card className="page-card" title="执行日志" loading={taskQuery.isLoading}>
          <LogViewer lines={task?.logs ?? []} />
        </Card>
      </Space>
    </PermissionGuard>
  );
}
