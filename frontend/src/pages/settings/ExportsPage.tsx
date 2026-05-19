import { App as AntApp, Button, Card, Form, Input, Radio, Select, Space, Switch, Typography } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";

import { DataTable } from "../../components/DataTable";
import { ErrorState } from "../../components/ErrorState";
import { FormDrawer } from "../../components/FormDrawer";
import { PageHeader } from "../../components/PageHeader";
import { PermissionActionButton } from "../../components/PermissionActionButton";
import { PermissionGuard } from "../../components/PermissionGuard";
import { ResourceDetailPanel } from "../../components/resource/ResourceDetailPanel";
import { StatusBadge } from "../../components/StatusBadge";
import { exportsApi } from "../../lib/api";
import { applyFormErrors, getErrorMessage } from "../../lib/forms";
import { formatBytes, formatDateTime } from "../../lib/format";
import { queryKeys } from "../../lib/queryKeys";
import type { ExportJob, ExportJobInput } from "../../types/models";

type ExportFormValues = {
  kind: ExportJobInput["kind"];
  resourceType?: string;
  resourceId?: string;
  recordType?: string;
  format?: "json" | "csv";
  taskId?: string;
  releaseId?: string;
  eventId?: string;
  masked: boolean;
};

const createOptions = [
  { label: "资源快照", value: "resource" },
  { label: "记录导出", value: "records" },
  { label: "事件包", value: "incident" },
] as const;

const recordTypeOptions = [
  { label: "审计记录", value: "audit_logs" },
  { label: "告警事件", value: "alert_events" },
  { label: "通知记录", value: "notification_records" },
  { label: "任务记录", value: "tasks" },
] as const;

const formatOptions = [
  { label: "JSON", value: "json" },
  { label: "CSV", value: "csv" },
] as const;

function buildFormValues(): ExportFormValues {
  return {
    kind: "resource",
    resourceType: "",
    resourceId: "",
    recordType: "audit_logs",
    format: "json",
    taskId: "",
    releaseId: "",
    eventId: "",
    masked: true,
  };
}

function formatExportTarget(job?: ExportJob | null) {
  if (!job) {
    return "--";
  }
  if (job.type === "resource") {
    return [job.resourceType || "resource", job.resourceId || "all"].join(" / ");
  }
  if (job.type === "incident") {
    return job.resourceId || "--";
  }
  return job.resourceType || job.filtersJson || "--";
}

export function ExportsPage() {
  const { message } = AntApp.useApp();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<ExportFormValues>();
  const [searchParams, setSearchParams] = useSearchParams();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const selectedId = searchParams.get("selected") ?? "";
  const createKind = Form.useWatch("kind", form) ?? "resource";

  const exportsQuery = useQuery({
    queryKey: queryKeys.exports,
    queryFn: exportsApi.list,
  });

  const jobs = exportsQuery.data ?? [];
  const selectedJob = jobs.find((item) => item.id === selectedId) ?? jobs[0] ?? null;

  const createMutation = useMutation({
    mutationFn: exportsApi.create,
    onSuccess: async (job) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.exports });
      setDrawerOpen(false);
      form.resetFields();
      setSearchParams((previous) => {
        const next = new URLSearchParams(previous);
        next.set("selected", job.id);
        return next;
      });
      await message.success("导出任务已创建");
    },
    onError: (error) => {
      applyFormErrors(form, error);
      void message.error(getErrorMessage(error, "创建导出任务失败"));
    },
  });

  const downloadMutation = useMutation({
    mutationFn: exportsApi.download,
    onSuccess: async (result) => {
      await message.success(result.fileName ? `已开始下载 ${result.fileName}` : "已开始下载导出文件");
    },
    onError: (error) => {
      void message.error(getErrorMessage(error, "下载导出文件失败"));
    },
  });

  const runningCount = useMemo(() => jobs.filter((item) => item.status === "RUNNING" || item.status === "PENDING").length, [jobs]);
  const failedCount = useMemo(() => jobs.filter((item) => item.status === "FAILED").length, [jobs]);

  if (exportsQuery.isError) {
    return <ErrorState message={exportsQuery.error.message} onRetry={() => void exportsQuery.refetch()} />;
  }

  return (
    <PermissionGuard permission="exports.view" forbiddenPage>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <PageHeader
          title="导出中心"
          description="统一查看资源快照、记录导出和事件包的生成结果，并从这里追踪是否已经产出可下载文件。"
          extra={
            <PermissionActionButton
              type="primary"
              permission="exports.create"
              permissionReason="当前账号缺少 exports.create 权限，无法创建导出任务。"
              onClick={() => {
                form.setFieldsValue(buildFormValues());
                setDrawerOpen(true);
              }}
            >
              新建导出
            </PermissionActionButton>
          }
        />

        <Card className="page-card">
          <div className="metric-grid">
            <Card size="small">
              <Typography.Text type="secondary">导出总数</Typography.Text>
              <Typography.Title level={4} style={{ margin: "8px 0 0" }}>{jobs.length}</Typography.Title>
            </Card>
            <Card size="small">
              <Typography.Text type="secondary">处理中</Typography.Text>
              <Typography.Title level={4} style={{ margin: "8px 0 0" }}>{runningCount}</Typography.Title>
            </Card>
            <Card size="small">
              <Typography.Text type="secondary">最近失败</Typography.Text>
              <Typography.Title level={4} style={{ margin: "8px 0 0" }}>{failedCount}</Typography.Title>
            </Card>
          </div>
        </Card>

        <div className="resource-workbench">
          <div className="resource-list-pane">
            <Card className="page-card">
              <DataTable
                rowKey="id"
                loading={exportsQuery.isLoading}
                dataSource={jobs}
                rowClassName={(item) => (item.id === selectedJob?.id ? "resource-row-selected" : "")}
                onRow={(item) => ({
                  onClick: () => {
                    setSearchParams((previous) => {
                      const next = new URLSearchParams(previous);
                      next.set("selected", item.id);
                      return next;
                    });
                  },
                })}
                columns={[
                  {
                    title: "导出任务",
                    key: "job",
                    render: (_, job) => (
                      <Space direction="vertical" size={2}>
                        <Typography.Text strong>{job.fileName || job.id}</Typography.Text>
                        <Typography.Text type="secondary">{job.type}</Typography.Text>
                      </Space>
                    ),
                  },
                  { title: "状态", dataIndex: "status", render: (value: string) => <StatusBadge status={value} /> },
                  {
                    title: "目标",
                    key: "target",
                    render: (_, job) => <Typography.Text type="secondary">{formatExportTarget(job)}</Typography.Text>,
                  },
                  { title: "大小", dataIndex: "fileSize", render: (value: number) => formatBytes(value) },
                  { title: "创建时间", dataIndex: "createdAt", render: (value: string) => formatDateTime(value) },
                ]}
              />
            </Card>
          </div>

          <div className="resource-detail-pane">
            <ResourceDetailPanel
              title={selectedJob?.fileName || selectedJob?.id}
              subtitle={selectedJob ? `类型 ${selectedJob.type}` : undefined}
              status={selectedJob ? <StatusBadge status={selectedJob.status} /> : undefined}
              emptyTitle="选择一条导出任务"
              emptyDescription="左侧选中导出记录后，这里会展示文件、范围、错误信息和下载入口。"
              highlights={
                selectedJob
                  ? [
                      {
                        label: "导出大小",
                        value: formatBytes(selectedJob.fileSize),
                        helper: selectedJob.contentType || "等待文件生成",
                      },
                      {
                        label: "脱敏模式",
                        value: selectedJob.masked ? "已脱敏" : "未脱敏",
                        helper: selectedJob.masked ? "下载内容会保留脱敏结果" : "包含完整原始数据",
                      },
                    ]
                  : []
              }
              meta={
                selectedJob
                  ? [
                      { label: "资源类型", value: selectedJob.resourceType || "--" },
                      { label: "资源 ID", value: selectedJob.resourceId || "--" },
                      { label: "创建人", value: selectedJob.createdBy || "--" },
                      { label: "完成时间", value: formatDateTime(selectedJob.finishedAt) },
                    ]
                  : []
              }
              actions={
                selectedJob ? (
                  <Space wrap>
                    <PermissionActionButton
                      permission="exports.download"
                      permissionReason="当前账号缺少 exports.download 权限，无法下载导出文件。"
                      disabled={selectedJob.status !== "SUCCESS"}
                      disabledReason="仅成功生成的导出任务才能下载。"
                      loading={downloadMutation.isPending}
                      onClick={() => downloadMutation.mutate(selectedJob.id)}
                    >
                      下载文件
                    </PermissionActionButton>
                  </Space>
                ) : undefined
              }
            >
              {selectedJob?.errorMessage ? (
                <Card size="small">
                  <Typography.Text strong>错误信息</Typography.Text>
                  <pre className="service-json-block">{selectedJob.errorMessage}</pre>
                </Card>
              ) : null}
              {selectedJob?.filtersJson ? (
                <Card size="small">
                  <Typography.Text strong>筛选条件</Typography.Text>
                  <pre className="service-json-block">{selectedJob.filtersJson}</pre>
                </Card>
              ) : null}
            </ResourceDetailPanel>
          </div>
        </div>

        <FormDrawer
          open={drawerOpen}
          title="新建导出"
          width={560}
          loading={createMutation.isPending}
          onClose={() => setDrawerOpen(false)}
          onSubmit={() => form.submit()}
        >
          <Form
            form={form}
            layout="vertical"
            onFinish={(values) => {
              if (values.kind === "resource") {
                createMutation.mutate({
                  kind: "resource",
                  resourceType: values.resourceType?.trim() || "",
                  resourceId: values.resourceId?.trim() || "",
                  masked: values.masked,
                });
                return;
              }
              if (values.kind === "records") {
                createMutation.mutate({
                  kind: "records",
                  recordType: values.recordType?.trim() || "audit_logs",
                  format: values.format || "json",
                  masked: values.masked,
                });
                return;
              }
              createMutation.mutate({
                kind: "incident",
                taskId: values.taskId?.trim() || "",
                releaseId: values.releaseId?.trim() || "",
                eventId: values.eventId?.trim() || "",
                masked: values.masked,
              });
            }}
          >
            <Form.Item label="导出类型" name="kind" rules={[{ required: true, message: "请选择导出类型" }]}>
              <Radio.Group
                options={createOptions as unknown as Array<{ label: string; value: string }>}
                optionType="button"
                buttonStyle="solid"
              />
            </Form.Item>

            {createKind === "resource" ? (
              <>
                <Form.Item label="资源类型" name="resourceType" rules={[{ required: true, message: "请输入资源类型" }]}>
                  <Input placeholder="例如 service / host / nginx_config" />
                </Form.Item>
                <Form.Item label="资源 ID" name="resourceId">
                  <Input placeholder="为空时由后端按范围处理" />
                </Form.Item>
              </>
            ) : null}

            {createKind === "records" ? (
              <>
                <Form.Item label="记录类型" name="recordType" rules={[{ required: true, message: "请选择记录类型" }]}>
                  <Select options={recordTypeOptions as unknown as Array<{ label: string; value: string }>} />
                </Form.Item>
                <Form.Item label="文件格式" name="format" rules={[{ required: true, message: "请选择文件格式" }]}>
                  <Select options={formatOptions as unknown as Array<{ label: string; value: string }>} />
                </Form.Item>
              </>
            ) : null}

            {createKind === "incident" ? (
              <>
                <Form.Item label="任务 ID" name="taskId">
                  <Input />
                </Form.Item>
                <Form.Item label="发布 ID" name="releaseId">
                  <Input />
                </Form.Item>
                <Form.Item label="事件 ID" name="eventId">
                  <Input />
                </Form.Item>
              </>
            ) : null}

            <Form.Item label="脱敏导出" name="masked" valuePropName="checked">
              <Switch checkedChildren="脱敏" unCheckedChildren="原始" />
            </Form.Item>
          </Form>
        </FormDrawer>
      </Space>
    </PermissionGuard>
  );
}
