import { App as AntApp, Alert, Button, Card, Form, Input, InputNumber, Select, Space, Switch, Tag, Typography } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";

import { DangerConfirm } from "../../components/DangerConfirm";
import { DataTable } from "../../components/DataTable";
import { FormDrawer } from "../../components/FormDrawer";
import { PageHeader } from "../../components/PageHeader";
import { PermissionActionButton } from "../../components/PermissionActionButton";
import { PermissionGuard } from "../../components/PermissionGuard";
import { ResourceDetailPanel } from "../../components/resource/ResourceDetailPanel";
import { StatusBadge } from "../../components/StatusBadge";
import { scheduledJobsApi } from "../../lib/api";
import { applyFormErrors, getErrorMessage } from "../../lib/forms";
import { formatDateTime } from "../../lib/format";
import { queryKeys } from "../../lib/queryKeys";
import type { ScheduledJob, ScheduledJobInput } from "../../types/models";

type ScheduledJobFormValues = ScheduledJobInput;
type ScheduledJobTemplateKey = "blank" | "hostAvailability" | "serviceHealth";

const templateMeta: Record<
  ScheduledJobTemplateKey,
  {
    title: string;
    description: string;
    tag: string;
    color: string;
    values: ScheduledJobFormValues;
  }
> = {
  blank: {
    title: "自定义任务",
    description: "用于接入额外的调度动作，自行填写类型、目标和 payload。",
    tag: "自定义",
    color: "default",
    values: {
      name: "",
      type: "",
      enabled: true,
      cronExpr: "",
      targetType: "",
      targetId: "",
      payloadJson: "",
      retryPolicyJson: "",
      timeoutSeconds: 300,
      concurrencyKey: "",
    },
  },
  hostAvailability: {
    title: "主机可用性巡检",
    description: "按计划触发主机连通性巡检，适合配合离线告警与资产健康面板。",
    tag: "主机巡检",
    color: "blue",
    values: {
      name: "主机可用性巡检",
      type: "host.availability.check",
      enabled: true,
      cronExpr: "*/5 * * * *",
      targetType: "host",
      targetId: "all",
      payloadJson: JSON.stringify({ mode: "availability", target: "all" }, null, 2),
      retryPolicyJson: JSON.stringify({ maxRetry: 1, backoffSeconds: 30 }, null, 2),
      timeoutSeconds: 120,
      concurrencyKey: "scheduler:host:availability:all",
    },
  },
  serviceHealth: {
    title: "服务健康检查",
    description: "按计划触发服务健康检查，适合和发布后探活、告警事件联动使用。",
    tag: "服务巡检",
    color: "purple",
    values: {
      name: "服务健康检查",
      type: "service.health.check",
      enabled: true,
      cronExpr: "*/2 * * * *",
      targetType: "service",
      targetId: "all",
      payloadJson: JSON.stringify({ mode: "health_check", target: "all" }, null, 2),
      retryPolicyJson: JSON.stringify({ maxRetry: 2, backoffSeconds: 20 }, null, 2),
      timeoutSeconds: 180,
      concurrencyKey: "scheduler:service:health:all",
    },
  },
};

function buildFormValues(job?: ScheduledJob | null): ScheduledJobFormValues {
  return {
    name: job?.name ?? "",
    type: job?.type ?? "",
    enabled: job?.enabled ?? true,
    cronExpr: job?.cronExpr ?? "",
    targetType: job?.targetType ?? "",
    targetId: job?.targetId ?? "",
    payloadJson: job?.payloadJson ?? "",
    retryPolicyJson: job?.retryPolicyJson ?? "",
    timeoutSeconds: job?.timeoutSeconds ?? 300,
    concurrencyKey: job?.concurrencyKey ?? "",
  };
}

function validateJsonText(_: unknown, value?: string) {
  const text = value?.trim();
  if (!text) {
    return Promise.resolve();
  }
  try {
    JSON.parse(text);
    return Promise.resolve();
  } catch {
    return Promise.reject(new Error("请输入合法 JSON"));
  }
}

function inferTemplate(job?: Pick<ScheduledJob, "type" | "targetType"> | null): ScheduledJobTemplateKey {
  const type = (job?.type ?? "").toLowerCase();
  const targetType = (job?.targetType ?? "").toLowerCase();
  if (type.includes("health") && targetType.includes("service")) {
    return "serviceHealth";
  }
  if ((type.includes("availability") || type.includes("host")) && targetType.includes("host")) {
    return "hostAvailability";
  }
  return "blank";
}

function formatTarget(job?: Pick<ScheduledJob, "targetType" | "targetId"> | null) {
  if (!job?.targetType && !job?.targetId) {
    return "未指定";
  }
  return [job.targetType || "未指定类型", job.targetId || "all"].join(" · ");
}

function formatPolicy(job?: Pick<ScheduledJob, "timeoutSeconds" | "concurrencyKey" | "retryPolicyJson"> | null) {
  const items: string[] = [];
  if (typeof job?.timeoutSeconds === "number" && job.timeoutSeconds > 0) {
    items.push(`超时 ${job.timeoutSeconds}s`);
  }
  if (job?.concurrencyKey) {
    items.push(`并发键 ${job.concurrencyKey}`);
  }
  if (job?.retryPolicyJson?.trim()) {
    items.push("自定义重试策略");
  }
  return items.join(" · ") || "沿用默认执行策略";
}

function prettyJson(text?: string) {
  if (!text?.trim()) {
    return "--";
  }
  try {
    return JSON.stringify(JSON.parse(text), null, 2);
  } catch {
    return text;
  }
}

export function ScheduledJobsPage() {
  const { message } = AntApp.useApp();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<ScheduledJobFormValues>();
  const [searchParams, setSearchParams] = useSearchParams();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editingJob, setEditingJob] = useState<ScheduledJob | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<ScheduledJob | null>(null);
  const selectedJobId = searchParams.get("selected") ?? "";

  const jobsQuery = useQuery({
    queryKey: queryKeys.scheduledJobs,
    queryFn: scheduledJobsApi.list,
  });

  const jobs = jobsQuery.data ?? [];
  const selectedJob = jobs.find((item) => item.id === selectedJobId) ?? jobs[0] ?? null;
  const enabledCount = jobs.filter((item) => item.enabled).length;
  const hostAvailabilityCount = jobs.filter((item) => inferTemplate(item) === "hostAvailability").length;
  const serviceHealthCount = jobs.filter((item) => inferTemplate(item) === "serviceHealth").length;

  const saveMutation = useMutation({
    mutationFn: scheduledJobsApi.save,
    onSuccess: async (job) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.scheduledJobs });
      setDrawerOpen(false);
      setEditingJob(null);
      form.resetFields();
      setSearchParams((previous) => {
        const next = new URLSearchParams(previous);
        next.set("selected", job.id);
        return next;
      });
      await message.success(editingJob ? "调度任务已更新" : "调度任务已创建");
    },
    onError: (error) => {
      applyFormErrors(form, error);
      void message.error(getErrorMessage(error));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: scheduledJobsApi.remove,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.scheduledJobs });
      if (selectedJobId === deleteTarget?.id) {
        setSearchParams((previous) => {
          const next = new URLSearchParams(previous);
          next.delete("selected");
          return next;
        });
      }
      setDeleteTarget(null);
      await message.success("调度任务已删除");
    },
    onError: (error) => {
      void message.error(getErrorMessage(error, "删除调度任务失败"));
    },
  });

  function openWithTemplate(templateKey: ScheduledJobTemplateKey, job?: ScheduledJob | null) {
    setEditingJob(job ?? null);
    form.resetFields();
    form.setFieldsValue(job ? buildFormValues(job) : templateMeta[templateKey].values);
    setDrawerOpen(true);
  }

  const selectedTemplate = inferTemplate(selectedJob);

  return (
    <PermissionGuard permission="scheduler.view" forbiddenPage>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <PageHeader
          title="调度任务"
          description="管理主机巡检、服务健康检查等定时任务的触发计划、目标范围与执行策略。"
          extra={
            <PermissionActionButton
              type="primary"
              permission="scheduler.manage"
              permissionReason="当前账号缺少 scheduler.manage 权限，无法新增或编辑调度任务。"
              onClick={() => openWithTemplate("blank")}
            >
              新增任务
            </PermissionActionButton>
          }
        />

        <Card className="page-card">
          <div className="metric-grid" style={{ marginBottom: 16 }}>
            <Card size="small">
              <Typography.Text type="secondary">任务总数</Typography.Text>
              <Typography.Title level={4} style={{ margin: "8px 0 0" }}>
                {jobs.length}
              </Typography.Title>
            </Card>
            <Card size="small">
              <Typography.Text type="secondary">启用中</Typography.Text>
              <Typography.Title level={4} style={{ margin: "8px 0 0" }}>
                {enabledCount}
              </Typography.Title>
            </Card>
            <Card size="small">
              <Typography.Text type="secondary">主机巡检</Typography.Text>
              <Typography.Title level={4} style={{ margin: "8px 0 0" }}>
                {hostAvailabilityCount}
              </Typography.Title>
            </Card>
            <Card size="small">
              <Typography.Text type="secondary">服务巡检</Typography.Text>
              <Typography.Title level={4} style={{ margin: "8px 0 0" }}>
                {serviceHealthCount}
              </Typography.Title>
            </Card>
          </div>

          <Alert
            type="info"
            showIcon
            message="推荐场景模板"
            description={
              <Space wrap>
                <PermissionActionButton
                  permission="scheduler.manage"
                  permissionReason="当前账号缺少 scheduler.manage 权限，无法创建调度任务。"
                  onClick={() => openWithTemplate("hostAvailability")}
                >
                  主机可用性巡检
                </PermissionActionButton>
                <PermissionActionButton
                  permission="scheduler.manage"
                  permissionReason="当前账号缺少 scheduler.manage 权限，无法创建调度任务。"
                  onClick={() => openWithTemplate("serviceHealth")}
                >
                  服务健康检查
                </PermissionActionButton>
                <Typography.Text type="secondary">
                  直接带入常用的 cron、目标范围、超时和重试策略，减少重复填写。
                </Typography.Text>
              </Space>
            }
          />
        </Card>

        <div className="resource-workbench">
          <div className="resource-list-pane">
            <Card className="page-card">
              <DataTable
                rowKey="id"
                loading={jobsQuery.isLoading}
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
                    title: "任务",
                    key: "job",
                    render: (_, job) => {
                      const meta = templateMeta[inferTemplate(job)];
                      return (
                        <Space direction="vertical" size={2}>
                          <Space wrap size={[8, 4]}>
                            <Typography.Text>{job.name}</Typography.Text>
                            <Tag color={meta.color}>{meta.tag}</Tag>
                          </Space>
                          <Typography.Text type="secondary">{job.type}</Typography.Text>
                        </Space>
                      );
                    },
                  },
                  {
                    title: "状态",
                    dataIndex: "enabled",
                    render: (value: boolean) => <StatusBadge status={value ? "ACTIVE" : "DISABLED"} />,
                  },
                  { title: "Cron", dataIndex: "cronExpr" },
                  {
                    title: "范围",
                    key: "target",
                    render: (_, job) => formatTarget(job),
                  },
                  {
                    title: "执行策略",
                    key: "policy",
                    render: (_, job) => (
                      <Typography.Text type="secondary">{formatPolicy(job)}</Typography.Text>
                    ),
                  },
                  {
                    title: "下次运行",
                    dataIndex: "nextRunAt",
                    render: (value?: string) => formatDateTime(value),
                  },
                ]}
              />
            </Card>
          </div>

          <div className="resource-detail-pane">
            <ResourceDetailPanel
              title={selectedJob?.name}
              subtitle={selectedJob?.type}
              status={selectedJob ? <StatusBadge status={selectedJob.enabled ? "ACTIVE" : "DISABLED"} /> : undefined}
              meta={
                selectedJob
                  ? [
                      {
                        label: "场景",
                        value: <Tag color={templateMeta[selectedTemplate].color}>{templateMeta[selectedTemplate].tag}</Tag>,
                      },
                      { label: "执行计划", value: selectedJob.cronExpr || "--" },
                      { label: "目标范围", value: formatTarget(selectedJob) },
                      { label: "执行策略", value: formatPolicy(selectedJob) },
                      { label: "最近运行", value: formatDateTime(selectedJob.lastRunAt) },
                      { label: "下次运行", value: formatDateTime(selectedJob.nextRunAt) },
                    ]
                  : []
              }
              actions={
                selectedJob ? (
                  <Space wrap>
                    <PermissionActionButton
                      permission="scheduler.manage"
                      permissionReason="当前账号缺少 scheduler.manage 权限，无法编辑调度任务。"
                      onClick={() => openWithTemplate(inferTemplate(selectedJob), selectedJob)}
                    >
                      编辑
                    </PermissionActionButton>
                    <PermissionActionButton
                      danger
                      permission="scheduler.manage"
                      permissionReason="当前账号缺少 scheduler.manage 权限，无法删除调度任务。"
                      onClick={() => setDeleteTarget(selectedJob)}
                    >
                      删除
                    </PermissionActionButton>
                  </Space>
                ) : undefined
              }
            >
              {selectedJob ? (
                <>
                  <div className="resource-detail-section">
                    <div className="resource-subpanel">
                      <Typography.Text strong>执行说明</Typography.Text>
                      <Typography.Text type="secondary">{templateMeta[selectedTemplate].description}</Typography.Text>
                      <Typography.Text type="secondary">
                        建议将同类高风险任务收敛到稳定的并发键，避免同一资源被重复触发。
                      </Typography.Text>
                    </div>
                  </div>
                  <div className="resource-detail-section">
                    <div className="resource-subpanel">
                      <Typography.Text strong>Payload JSON</Typography.Text>
                      <pre className="service-json-block">{prettyJson(selectedJob.payloadJson)}</pre>
                    </div>
                  </div>
                  <div className="resource-detail-section">
                    <div className="resource-subpanel">
                      <Typography.Text strong>Retry Policy JSON</Typography.Text>
                      <pre className="service-json-block">{prettyJson(selectedJob.retryPolicyJson)}</pre>
                    </div>
                  </div>
                </>
              ) : null}
            </ResourceDetailPanel>
          </div>
        </div>

        <FormDrawer
          open={drawerOpen}
          title={editingJob ? "编辑调度任务" : "新增调度任务"}
          onClose={() => {
            setDrawerOpen(false);
            setEditingJob(null);
          }}
          onSubmit={() => form.submit()}
          loading={saveMutation.isPending}
          width={640}
        >
          <Form
            form={form}
            layout="vertical"
            onFinish={(values) =>
              saveMutation.mutate({
                id: editingJob?.id,
                name: values.name,
                type: values.type,
                enabled: values.enabled,
                cronExpr: values.cronExpr,
                targetType: values.targetType?.trim() || "",
                targetId: values.targetId?.trim() || "",
                payloadJson: values.payloadJson?.trim() || "",
                retryPolicyJson: values.retryPolicyJson?.trim() || "",
                timeoutSeconds: values.timeoutSeconds,
                concurrencyKey: values.concurrencyKey?.trim() || "",
              })
            }
          >
            <Alert
              type="info"
              showIcon
              style={{ marginBottom: 16 }}
              message="填写建议"
              description="Cron 决定触发频率，targetType/targetId 决定资源范围，payload 用于补充执行参数，retry policy 用于覆盖默认重试策略。"
            />

            <Form.Item label="名称" name="name" rules={[{ required: true, message: "请输入任务名称" }]}>
              <Input />
            </Form.Item>
            <Form.Item label="类型" name="type" rules={[{ required: true, message: "请输入任务类型" }]}>
              <Input placeholder="例如 host.availability.check / service.health.check" />
            </Form.Item>
            <Form.Item label="Cron 表达式" name="cronExpr" rules={[{ required: true, message: "请输入 Cron 表达式" }]}>
              <Input placeholder="例如 */5 * * * *" />
            </Form.Item>

            <div className="two-col-grid">
              <Form.Item label="目标类型" name="targetType">
                <Select
                  allowClear
                  options={[
                    { label: "host", value: "host" },
                    { label: "service", value: "service" },
                    { label: "docker_node", value: "docker_node" },
                  ]}
                  placeholder="为空表示任务自行决定范围"
                />
              </Form.Item>
              <Form.Item label="目标 ID" name="targetId">
                <Input placeholder="例如 all / 具体资源 ID" />
              </Form.Item>
            </div>

            <Form.Item
              label="Payload JSON"
              name="payloadJson"
              rules={[{ validator: validateJsonText }]}
              extra="留空表示不附带额外 payload。对于通用巡检任务，建议只保留关键执行参数。"
            >
              <Input.TextArea rows={6} style={{ fontFamily: "monospace" }} />
            </Form.Item>
            <Form.Item
              label="Retry Policy JSON"
              name="retryPolicyJson"
              rules={[{ validator: validateJsonText }]}
              extra="例如 maxRetry、backoffSeconds 等。留空时沿用后端默认重试策略。"
            >
              <Input.TextArea rows={6} style={{ fontFamily: "monospace" }} />
            </Form.Item>

            <div className="two-col-grid">
              <Form.Item
                label="超时秒数"
                name="timeoutSeconds"
                rules={[{ required: true, message: "请输入超时秒数" }]}
                extra="建议按动作耗时保守设置，过小会更容易进入 timeout。"
              >
                <InputNumber min={1} precision={0} style={{ width: "100%" }} />
              </Form.Item>
              <Form.Item
                label="并发键"
                name="concurrencyKey"
                extra="同一并发键可用于约束同类任务的重入。创建时留空会由后端自动生成。"
              >
                <Input />
              </Form.Item>
            </div>

            <Form.Item label="启用" name="enabled" valuePropName="checked">
              <Switch checkedChildren="启用" unCheckedChildren="停用" />
            </Form.Item>
          </Form>
        </FormDrawer>

        <DangerConfirm
          open={Boolean(deleteTarget)}
          title="删除调度任务"
          description={`删除后将无法继续按计划触发 ${deleteTarget?.name || "该任务"}。`}
          confirmText={deleteTarget?.name}
          loading={deleteMutation.isPending}
          onCancel={() => setDeleteTarget(null)}
          onConfirm={() => {
            if (deleteTarget) {
              deleteMutation.mutate(deleteTarget.id);
            }
          }}
        />
      </Space>
    </PermissionGuard>
  );
}
