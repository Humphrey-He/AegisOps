import { App as AntApp, Button, Card, Descriptions, List, Space, Tag, Typography } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { ErrorState } from "../../components/ErrorState";
import { LogViewer } from "../../components/LogViewer";
import { PageHeader } from "../../components/PageHeader";
import { PermissionActionButton } from "../../components/PermissionActionButton";
import { PermissionGuard } from "../../components/PermissionGuard";
import { StatusBadge } from "../../components/StatusBadge";
import { tasksApi } from "../../lib/api";
import { getErrorMessage } from "../../lib/forms";
import { formatDateTime } from "../../lib/format";
import { queryKeys } from "../../lib/queryKeys";
import { buildResourcePath, formatTaskResourceName, getResourceTypeLabel } from "../../lib/resourceNavigation";
import { formatTaskExecutionPolicy, getTaskDispatchSourceMeta } from "../../lib/taskPresentation";
import type { Task } from "../../types/models";

export function TaskDetailPage() {
  const { message } = AntApp.useApp();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { taskId = "" } = useParams();
  const taskQuery = useQuery({
    queryKey: queryKeys.task(taskId),
    queryFn: () => tasksApi.detail(taskId),
    enabled: Boolean(taskId),
    refetchInterval: ({ state }) => {
      const task = state.data as Task | undefined;
      return task && ["SUCCESS", "FAILED", "CANCELED"].includes(task.status) ? false : 3000;
    },
  });

  const cancelMutation = useMutation({
    mutationFn: () => tasksApi.cancel(taskId),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.tasks }),
        queryClient.invalidateQueries({ queryKey: queryKeys.task(taskId) }),
      ]);
      await message.success("任务已取消");
    },
    onError: async (error) => {
      void message.error(getErrorMessage(error, "取消任务失败"));
    },
  });

  const retryMutation = useMutation({
    mutationFn: () => tasksApi.retry(taskId),
    onSuccess: async (task) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.tasks });
      await message.success("已创建重试任务");
      navigate(`/tasks/${task.id}`);
    },
    onError: async (error) => {
      void message.error(getErrorMessage(error, "重试任务失败"));
    },
  });

  if (taskQuery.isError) {
    return <ErrorState message={taskQuery.error.message} onRetry={() => void taskQuery.refetch()} />;
  }

  const task = taskQuery.data;
  const resourcePath = buildResourcePath(task?.resourceType, task?.resourceId);
  const canCancel = task?.status === "PENDING" || task?.status === "RUNNING";
  const canRetry = task?.status === "FAILED" || task?.status === "CANCELED";

  return (
    <PermissionGuard permission="tasks.view" forbiddenPage>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <PageHeader
          title="任务详情"
          description="查看调度来源、执行步骤、日志，并在这里直接处理取消或重试。"
          extra={
            <Space wrap>
              <PermissionActionButton
                permission="tasks.retry"
                permissionReason="当前账号缺少 tasks.retry 权限，无法重新调度失败任务。"
                disabled={!canRetry}
                disabledReason="只有失败或已取消的任务才可以重试。"
                loading={retryMutation.isPending}
                onClick={() => retryMutation.mutate()}
              >
                重试任务
              </PermissionActionButton>
              <PermissionActionButton
                danger
                permission="tasks.cancel"
                permissionReason="当前账号缺少 tasks.cancel 权限，无法中止执行中的任务。"
                disabled={!canCancel}
                disabledReason="只有待执行或执行中的任务才可以取消。"
                loading={cancelMutation.isPending}
                onClick={() => cancelMutation.mutate()}
              >
                取消任务
              </PermissionActionButton>
              {resourcePath ? <Button onClick={() => navigate(resourcePath)}>回到资源</Button> : null}
              <Button onClick={() => navigate("/tasks")}>返回任务中心</Button>
            </Space>
          }
        />

        <Card className="page-card" loading={taskQuery.isLoading}>
          {task ? (
            <Descriptions bordered size="small" column={4}>
              <Descriptions.Item label="类型">{task.type}</Descriptions.Item>
              <Descriptions.Item label="资源">{formatTaskResourceName(task)}</Descriptions.Item>
              <Descriptions.Item label="状态">
                <StatusBadge status={task.status} />
              </Descriptions.Item>
              <Descriptions.Item label="进度">{task.progress}%</Descriptions.Item>
              <Descriptions.Item label="触发来源">
                {task.dispatchSource ? (
                  <Tag color={getTaskDispatchSourceMeta(task.dispatchSource).color}>
                    {getTaskDispatchSourceMeta(task.dispatchSource).label}
                  </Tag>
                ) : (
                  "--"
                )}
              </Descriptions.Item>
              <Descriptions.Item label="调度状态">
                {task.dispatchStatus ? <StatusBadge status={task.dispatchStatus} /> : "--"}
              </Descriptions.Item>
              <Descriptions.Item label="执行策略" span={2}>
                {formatTaskExecutionPolicy(task)}
              </Descriptions.Item>
              <Descriptions.Item label="资源类型">
                {task.resourceType ? getResourceTypeLabel(task.resourceType) : "--"}
              </Descriptions.Item>
              <Descriptions.Item label="发起人">{task.initiatedBy || "--"}</Descriptions.Item>
              <Descriptions.Item label="创建时间">{formatDateTime(task.createdAt)}</Descriptions.Item>
              <Descriptions.Item label="开始时间">{formatDateTime(task.startedAt)}</Descriptions.Item>
              <Descriptions.Item label="入队时间">{formatDateTime(task.queuedAt)}</Descriptions.Item>
              <Descriptions.Item label="结束时间">{formatDateTime(task.finishedAt)}</Descriptions.Item>
            </Descriptions>
          ) : null}
        </Card>

        <Card className="page-card" title="执行步骤" loading={taskQuery.isLoading}>
          <List
            dataSource={task?.steps ?? []}
            locale={{ emptyText: "暂无步骤信息" }}
            renderItem={(step) => {
              const detail = step.detail?.trim();
              const timeline = [formatDateTime(step.startedAt), formatDateTime(step.finishedAt)]
                .filter((value) => value && value !== "--")
                .join(" -> ");
              const description = [detail, timeline].filter(Boolean).join(" · ");
              return (
                <List.Item>
                  <List.Item.Meta
                    title={step.title}
                    description={
                      <Typography.Text type={step.status === "FAILED" ? "danger" : "secondary"}>
                        {description || "--"}
                      </Typography.Text>
                    }
                  />
                  <StatusBadge status={step.status} />
                </List.Item>
              );
            }}
          />
        </Card>

        <Card className="page-card" title="执行日志" loading={taskQuery.isLoading}>
          <LogViewer lines={task?.logs ?? []} />
        </Card>
      </Space>
    </PermissionGuard>
  );
}
