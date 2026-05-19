import { Button, Card, Input, Select, Space, Tag, Typography } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";

import { DataTable } from "../DataTable";
import { PermissionActionButton } from "../PermissionActionButton";
import { ResourceDetailPanel } from "../resource/ResourceDetailPanel";
import { StatusBadge } from "../StatusBadge";
import { taskDispatchesApi } from "../../lib/api";
import { getErrorMessage } from "../../lib/forms";
import { formatDateTime } from "../../lib/format";
import { queryKeys } from "../../lib/queryKeys";
import { formatTaskExecutionPolicy, getTaskDispatchSourceMeta } from "../../lib/taskPresentation";
import type { TaskDispatch, TaskDispatchStatus } from "../../types/models";

type TaskDispatchWorkbenchProps = {
  scopeTitle: string;
  scopeDescription: string;
  queryScope: string;
  jobId?: string;
  taskId?: string;
  defaultSource?: string;
  onMessage: (kind: "success" | "error", text: string) => void;
};

const dispatchStatusOptions: Array<{ label: string; value: TaskDispatchStatus | "" }> = [
  { label: "全部状态", value: "" },
  { label: "待执行", value: "PENDING" },
  { label: "已分发", value: "DISPATCHED" },
  { label: "执行中", value: "RUNNING" },
  { label: "成功", value: "SUCCESS" },
  { label: "失败", value: "FAILED" },
  { label: "已取消", value: "CANCELED" },
  { label: "超时", value: "TIMEOUT" },
];

const dispatchSourceOptions = [
  { label: "全部来源", value: "" },
  { label: "手动", value: "MANUAL" },
  { label: "系统", value: "SYSTEM" },
  { label: "调度", value: "SCHEDULED" },
];

function timeline(dispatch: TaskDispatch) {
  return [
    dispatch.queuedAt ? `入队 ${formatDateTime(dispatch.queuedAt)}` : "",
    dispatch.startedAt ? `开始 ${formatDateTime(dispatch.startedAt)}` : "",
    dispatch.finishedAt ? `结束 ${formatDateTime(dispatch.finishedAt)}` : "",
  ]
    .filter(Boolean)
    .join(" / ") || "等待调度";
}

function canCancel(status?: string) {
  return status === "PENDING" || status === "DISPATCHED" || status === "RUNNING";
}

function canRetry(status?: string) {
  return status === "FAILED" || status === "TIMEOUT" || status === "CANCELED";
}

export function TaskDispatchWorkbench({
  scopeTitle,
  scopeDescription,
  queryScope,
  jobId,
  taskId,
  defaultSource,
  onMessage,
}: TaskDispatchWorkbenchProps) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [statusFilter, setStatusFilter] = useState<TaskDispatchStatus | "">("");
  const [sourceFilter, setSourceFilter] = useState(defaultSource ?? "");
  const [keyword, setKeyword] = useState("");
  const [selectedDispatchId, setSelectedDispatchId] = useState("");

  const dispatchesQuery = useQuery({
    queryKey: queryKeys.taskDispatches(queryScope),
    queryFn: () =>
      taskDispatchesApi.list({
        status: statusFilter || undefined,
        source: sourceFilter || undefined,
        jobId,
        taskId,
      }),
    refetchInterval: ({ state }) => {
      const items = state.data as TaskDispatch[] | undefined;
      return items?.some((item) => canCancel(item.status)) ? 3000 : false;
    },
  });

  const filteredData = useMemo(() => {
    let items = dispatchesQuery.data ?? [];
    if (keyword.trim()) {
      const value = keyword.trim().toLowerCase();
      items = items.filter(
        (item) =>
          item.id.toLowerCase().includes(value) ||
          item.taskId.toLowerCase().includes(value) ||
          (item.concurrencyKey ?? "").toLowerCase().includes(value) ||
          (item.leaseOwner ?? "").toLowerCase().includes(value),
      );
    }
    return items;
  }, [dispatchesQuery.data, keyword]);

  const selectedDispatch = filteredData.find((item) => item.id === selectedDispatchId) ?? filteredData[0] ?? null;

  const refreshRelatedQueries = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: queryKeys.taskDispatches(queryScope) }),
      queryClient.invalidateQueries({ queryKey: queryKeys.tasks }),
      queryClient.invalidateQueries({ queryKey: queryKeys.scheduledJobs }),
      queryClient.invalidateQueries({ queryKey: queryKeys.scheduledJobDispatches(jobId ?? "") }),
      taskId ? queryClient.invalidateQueries({ queryKey: queryKeys.task(taskId) }) : Promise.resolve(),
      selectedDispatch?.taskId ? queryClient.invalidateQueries({ queryKey: queryKeys.task(selectedDispatch.taskId) }) : Promise.resolve(),
    ]);
  };

  const cancelMutation = useMutation({
    mutationFn: taskDispatchesApi.cancel,
    onSuccess: async () => {
      await refreshRelatedQueries();
      onMessage("success", "分发实例已取消");
    },
    onError: (error) => {
      onMessage("error", getErrorMessage(error, "取消分发实例失败"));
    },
  });

  const retryMutation = useMutation({
    mutationFn: taskDispatchesApi.retry,
    onSuccess: async () => {
      await refreshRelatedQueries();
      onMessage("success", "分发实例已重新入队");
    },
    onError: (error) => {
      onMessage("error", getErrorMessage(error, "重试分发实例失败"));
    },
  });

  return (
    <div className="resource-detail-section">
      <Card className="page-card">
        <div className="page-toolbar">
          <Space direction="vertical" size={2}>
            <Typography.Text strong>{scopeTitle}</Typography.Text>
            <Typography.Text type="secondary">{scopeDescription}</Typography.Text>
          </Space>
        </div>
        <div className="page-toolbar" style={{ marginTop: 12 }}>
          <div className="page-toolbar-start">
            <Input.Search
              allowClear
              placeholder="搜索 dispatch / task / lease owner"
              style={{ width: 280 }}
              value={keyword}
              onChange={(event) => setKeyword(event.target.value)}
              onSearch={setKeyword}
            />
            <Select
              style={{ width: 160 }}
              value={statusFilter}
              options={dispatchStatusOptions}
              onChange={(value) => setStatusFilter((value ?? "") as TaskDispatchStatus | "")}
            />
            <Select
              style={{ width: 160 }}
              value={sourceFilter}
              options={dispatchSourceOptions}
              onChange={(value) => setSourceFilter(value ?? "")}
            />
          </div>
        </div>
      </Card>

      <div className="resource-workbench" style={{ marginTop: 16 }}>
        <div className="resource-list-pane">
          <Card className="page-card">
            <DataTable
              rowKey="id"
              loading={dispatchesQuery.isLoading}
              dataSource={filteredData}
              rowClassName={(item) => (item.id === selectedDispatch?.id ? "resource-row-selected" : "")}
              onRow={(item) => ({
                onClick: () => setSelectedDispatchId(item.id),
              })}
              columns={[
                {
                  title: "分发实例",
                  key: "dispatch",
                  render: (_, record) => (
                    <Space direction="vertical" size={2}>
                      <Typography.Text strong>{record.id}</Typography.Text>
                      <Typography.Text type="secondary">{record.taskId}</Typography.Text>
                    </Space>
                  ),
                },
                {
                  title: "来源",
                  key: "source",
                  render: (_, record) =>
                    record.source ? <Tag color={getTaskDispatchSourceMeta(record.source).color}>{getTaskDispatchSourceMeta(record.source).label}</Tag> : "--",
                },
                { title: "状态", dataIndex: "status", render: (value: string) => <StatusBadge status={value} /> },
                { title: "重试", key: "retry", render: (_, record) => `${record.retryCount ?? 0}/${record.maxRetry ?? 0}` },
                { title: "入队时间", dataIndex: "queuedAt", render: (value?: string) => formatDateTime(value) },
              ]}
            />
          </Card>
        </div>

        <div className="resource-detail-pane">
          <ResourceDetailPanel
            title={selectedDispatch?.id}
            subtitle={selectedDispatch ? `任务 ${selectedDispatch.taskId}` : undefined}
            status={selectedDispatch?.status ? <StatusBadge status={selectedDispatch.status} /> : undefined}
            emptyTitle="选择一条分发实例"
            emptyDescription="这里会展示 lease、重试、超时和可执行操作，方便把调度计划追到执行实例。"
            highlights={
              selectedDispatch
                ? [
                    {
                      label: "执行策略",
                      value: formatTaskExecutionPolicy(selectedDispatch),
                      helper: timeline(selectedDispatch),
                    },
                    {
                      label: "Lease Owner",
                      value: selectedDispatch.leaseOwner || "--",
                      helper: selectedDispatch.leaseExpiresAt ? `到期 ${formatDateTime(selectedDispatch.leaseExpiresAt)}` : "当前未持有 lease",
                    },
                  ]
                : []
            }
            meta={
              selectedDispatch
                ? [
                    { label: "并发键", value: selectedDispatch.concurrencyKey || "--" },
                    { label: "重试计数", value: `${selectedDispatch.retryCount ?? 0}/${selectedDispatch.maxRetry ?? 0}` },
                    { label: "超时", value: selectedDispatch.timeoutSeconds ? `${selectedDispatch.timeoutSeconds}s` : "--" },
                    { label: "创建时间", value: formatDateTime(selectedDispatch.createdAt) },
                    { label: "更新时间", value: formatDateTime(selectedDispatch.updatedAt) },
                  ]
                : []
            }
            actions={
              selectedDispatch ? (
                <Space wrap>
                  <Button onClick={() => navigate(`/tasks/${selectedDispatch.taskId}`)}>查看任务</Button>
                  <PermissionActionButton
                    permission="tasks.retry"
                    permissionReason="当前账号缺少 tasks.retry 权限，无法重试分发实例。"
                    disabled={!canRetry(selectedDispatch.status)}
                    disabledReason="仅失败、超时或已取消的分发实例可重试。"
                    loading={retryMutation.isPending}
                    onClick={() => retryMutation.mutate(selectedDispatch.id)}
                  >
                    重试分发
                  </PermissionActionButton>
                  <PermissionActionButton
                    danger
                    permission="tasks.cancel"
                    permissionReason="当前账号缺少 tasks.cancel 权限，无法取消分发实例。"
                    disabled={!canCancel(selectedDispatch.status)}
                    disabledReason="仅待执行、已分发或执行中的分发实例可取消。"
                    loading={cancelMutation.isPending}
                    onClick={() => cancelMutation.mutate(selectedDispatch.id)}
                  >
                    取消分发
                  </PermissionActionButton>
                </Space>
              ) : undefined
            }
          />
        </div>
      </div>
    </div>
  );
}
