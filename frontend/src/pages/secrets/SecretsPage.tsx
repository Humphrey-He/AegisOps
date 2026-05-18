import { App as AntApp, Button, Card, Form, Input, Select, Space, Tag, Typography } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { DangerConfirm } from "../../components/DangerConfirm";
import { DataTable } from "../../components/DataTable";
import { EmptyState } from "../../components/EmptyState";
import { ErrorState } from "../../components/ErrorState";
import { FormDrawer } from "../../components/FormDrawer";
import { PageHeader } from "../../components/PageHeader";
import { PermissionGuard } from "../../components/PermissionGuard";
import { StatusBadge } from "../../components/StatusBadge";
import { ResourceDetailPanel } from "../../components/resource/ResourceDetailPanel";
import { secretsApi } from "../../lib/api";
import { applyFormErrors, getErrorMessage } from "../../lib/forms";
import { formatDateTime } from "../../lib/format";
import { queryKeys } from "../../lib/queryKeys";
import { SecretInput } from "../../components/SecretInput";
import type { Secret, SecretInputPayload, SecretReadAudit, SecretReference } from "../../types/models";

type SecretFormValues = {
  name: string;
  type: Secret["type"];
  description?: string;
  purpose?: string;
  status?: Secret["status"];
  secretValue: string;
};

const secretTypeOptions = [
  { label: "SSH 私钥", value: "SSH_PRIVATE_KEY" },
  { label: "SSH 密码", value: "SSH_PASSWORD" },
  { label: "Docker TLS", value: "DOCKER_TLS" },
  { label: "Docker Token", value: "DOCKER_TOKEN" },
  { label: "Webhook", value: "WEBHOOK" },
  { label: "API Token", value: "API_TOKEN" },
  { label: "SMTP", value: "SMTP" },
] as const;

function buildFormValues(secret?: Secret | null): SecretFormValues {
  return {
    name: secret?.name ?? "",
    type: secret?.type ?? "SSH_PRIVATE_KEY",
    description: secret?.description ?? "",
    purpose: secret?.purpose ?? "",
    status: secret?.status ?? "ACTIVE",
    secretValue: "",
  };
}

function renderReferenceTarget(item: SecretReference) {
  return [item.resourceType, item.resourceId].filter(Boolean).join(" · ");
}

function renderReadAuditResult(item: SecretReadAudit) {
  if (item.result === "FAILED") {
    return <StatusBadge status="FAILED" />;
  }
  return <StatusBadge status="SUCCESS" />;
}

export function SecretsPage() {
  const { message } = AntApp.useApp();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<SecretFormValues>();
  const [keyword, setKeyword] = useState("");
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editingSecret, setEditingSecret] = useState<Secret | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Secret | null>(null);
  const [searchParams, setSearchParams] = useSearchParams();
  const selectedSecretId = searchParams.get("selected") ?? "";

  const secretsQuery = useQuery({
    queryKey: queryKeys.secrets(keyword),
    queryFn: () => secretsApi.list(keyword),
  });
  const secretDetailQuery = useQuery({
    queryKey: queryKeys.secret(selectedSecretId),
    queryFn: () => secretsApi.detail(selectedSecretId),
    enabled: Boolean(selectedSecretId),
  });
  const secretReferencesQuery = useQuery({
    queryKey: queryKeys.secretReferences(selectedSecretId),
    queryFn: () => secretsApi.references(selectedSecretId),
    enabled: Boolean(selectedSecretId),
  });
  const secretReadAuditsQuery = useQuery({
    queryKey: queryKeys.secretReadAudits(selectedSecretId),
    queryFn: () => secretsApi.readAudits(selectedSecretId),
    enabled: Boolean(selectedSecretId),
  });

  const selectedSecret =
    secretDetailQuery.data ?? (secretsQuery.data ?? []).find((item) => item.id === selectedSecretId) ?? null;
  const references = secretReferencesQuery.data ?? [];
  const readAudits = secretReadAuditsQuery.data ?? [];

  const saveMutation = useMutation({
    mutationFn: secretsApi.save,
    onSuccess: async (secret) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["secrets"] }),
        queryClient.invalidateQueries({ queryKey: queryKeys.audits }),
        queryClient.invalidateQueries({ queryKey: queryKeys.secret(secret.id) }),
      ]);
      setDrawerOpen(false);
      setEditingSecret(null);
      form.resetFields();
      setSearchParams((previous) => {
        const next = new URLSearchParams(previous);
        next.set("selected", secret.id);
        return next;
      });
      await message.success(editingSecret ? "凭证已更新" : "凭证已创建");
    },
    onError: (error) => {
      applyFormErrors(form, error);
      void message.error(getErrorMessage(error));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (secretId: string) => secretsApi.remove(secretId),
    onSuccess: async () => {
      const removedId = deleteTarget?.id ?? "";
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["secrets"] }),
        queryClient.invalidateQueries({ queryKey: queryKeys.audits }),
      ]);
      if (removedId === selectedSecretId) {
        setSearchParams((previous) => {
          const next = new URLSearchParams(previous);
          next.delete("selected");
          return next;
        });
      }
      setDeleteTarget(null);
      await message.success("凭证已删除");
    },
    onError: async (error) => {
      void message.error(getErrorMessage(error, "删除凭证失败"));
    },
  });

  const usageTag = useMemo(() => {
    if (!references.length) {
      return <Tag>未绑定</Tag>;
    }
    return <Tag color="blue">{references.length} 个引用</Tag>;
  }, [references.length]);

  if (secretsQuery.isError) {
    return <ErrorState message={secretsQuery.error.message} onRetry={() => void secretsQuery.refetch()} />;
  }

  return (
    <PermissionGuard permission="secrets.view" forbiddenPage>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <PageHeader
          title="凭证"
          description="统一管理敏感凭证、引用关系与读取审计，避免在资源页面散落明文配置。"
          extra={
            <PermissionGuard permission="secrets.manage">
              <Button
                type="primary"
                onClick={() => {
                  setEditingSecret(null);
                  form.setFieldsValue(buildFormValues(null));
                  setDrawerOpen(true);
                }}
              >
                新增凭证
              </Button>
            </PermissionGuard>
          }
        />

        <Card className="page-card">
          <Input.Search
            allowClear
            placeholder="搜索凭证名称、类型或用途"
            style={{ width: 320 }}
            onSearch={setKeyword}
          />
        </Card>

        <div className="resource-workbench">
          <div className="resource-list-pane">
            <Card className="page-card">
              <DataTable
                rowKey="id"
                loading={secretsQuery.isLoading}
                dataSource={secretsQuery.data}
                rowClassName={(secret) => (secret.id === selectedSecretId ? "resource-row-selected" : "")}
                onRow={(secret) => ({
                  onClick: () => {
                    setSearchParams((previous) => {
                      const next = new URLSearchParams(previous);
                      next.set("selected", secret.id);
                      return next;
                    });
                  },
                })}
                locale={{
                  emptyText: (
                    <EmptyState
                      title="还没有凭证"
                      description="创建凭证后，可在主机、Docker、Registry 等页面复用绑定。"
                      action={
                        <Button type="primary" onClick={() => setDrawerOpen(true)}>
                          新增第一个凭证
                        </Button>
                      }
                    />
                  ),
                }}
                columns={[
                  {
                    title: "名称",
                    dataIndex: "name",
                    render: (_, secret) => (
                      <Space direction="vertical" size={2}>
                        <span>{secret.name}</span>
                        <Typography.Text type="secondary">{secret.purpose || secret.type}</Typography.Text>
                      </Space>
                    ),
                  },
                  { title: "类型", dataIndex: "type" },
                  {
                    title: "状态",
                    key: "status",
                    render: (_, secret) => <StatusBadge status={secret.status || "ACTIVE"} />,
                  },
                  {
                    title: "最近更新",
                    dataIndex: "updatedAt",
                    render: (value: string) => formatDateTime(value),
                  },
                ]}
              />
            </Card>
          </div>

          <div className="resource-detail-pane">
            <ResourceDetailPanel
              title={selectedSecret?.name}
              subtitle={selectedSecret?.type}
              status={selectedSecret ? <StatusBadge status={selectedSecret.status || "ACTIVE"} /> : undefined}
              meta={
                selectedSecret
                  ? [
                      { label: "用途", value: selectedSecret.purpose || "--" },
                      { label: "脱敏值", value: <Typography.Text code>{selectedSecret.valueMasked}</Typography.Text> },
                      { label: "轮换版本", value: String(selectedSecret.keyVersion ?? 1) },
                      { label: "最近轮换", value: formatDateTime(selectedSecret.lastRotatedAt) },
                      { label: "到期时间", value: formatDateTime(selectedSecret.expiresAt) },
                      { label: "引用情况", value: usageTag },
                      { label: "更新时间", value: formatDateTime(selectedSecret.updatedAt) },
                      { label: "说明", value: selectedSecret.description || "--" },
                    ]
                  : []
              }
              actions={
                selectedSecret ? (
                  <Space wrap>
                    <PermissionGuard permission="secrets.manage">
                      <Button
                        onClick={() => {
                          setEditingSecret(selectedSecret);
                          form.setFieldsValue(buildFormValues(selectedSecret));
                          setDrawerOpen(true);
                        }}
                      >
                        编辑
                      </Button>
                    </PermissionGuard>
                    <PermissionGuard permission="secrets.manage">
                      <Button danger onClick={() => setDeleteTarget(selectedSecret)}>
                        删除
                      </Button>
                    </PermissionGuard>
                  </Space>
                ) : undefined
              }
            >
              <div className="resource-detail-section">
                <div className="page-toolbar">
                  <Typography.Text strong>引用关系</Typography.Text>
                  <Typography.Text type="secondary">
                    {secretReferencesQuery.isLoading ? "正在读取..." : `${references.length} 条`}
                  </Typography.Text>
                </div>
                <div className="resource-subpanel" style={{ marginTop: 12 }}>
                  <DataTable
                    rowKey="id"
                    pagination={false}
                    loading={secretReferencesQuery.isLoading}
                    dataSource={references}
                    locale={{ emptyText: "当前凭证还没有绑定到任何资源" }}
                    columns={[
                      { title: "资源", key: "resource", render: (_, item) => renderReferenceTarget(item) },
                      { title: "字段", dataIndex: "fieldName" },
                      { title: "绑定人", dataIndex: "createdBy", render: (value?: string) => value || "--" },
                      {
                        title: "绑定时间",
                        dataIndex: "createdAt",
                        render: (value: string) => formatDateTime(value),
                      },
                    ]}
                  />
                </div>
              </div>

              <div className="resource-detail-section">
                <div className="page-toolbar">
                  <Typography.Text strong>读取审计</Typography.Text>
                  <Typography.Text type="secondary">
                    {secretReadAuditsQuery.isLoading ? "正在读取..." : `${readAudits.length} 条`}
                  </Typography.Text>
                </div>
                <div className="resource-subpanel" style={{ marginTop: 12 }}>
                  <DataTable
                    rowKey="id"
                    pagination={false}
                    loading={secretReadAuditsQuery.isLoading}
                    dataSource={readAudits}
                    locale={{ emptyText: "当前凭证还没有读取审计记录" }}
                    columns={[
                      {
                        title: "结果",
                        key: "result",
                        render: (_, item) => renderReadAuditResult(item),
                      },
                      { title: "动作", dataIndex: "action" },
                      {
                        title: "来源",
                        key: "source",
                        render: (_, item) => [item.resourceType, item.resourceId].filter(Boolean).join(" · ") || "--",
                      },
                      { title: "任务 ID", dataIndex: "taskId", render: (value?: string) => value || "--" },
                      {
                        title: "时间",
                        dataIndex: "createdAt",
                        render: (value: string) => formatDateTime(value),
                      },
                    ]}
                  />
                </div>
              </div>
            </ResourceDetailPanel>
          </div>
        </div>

        <FormDrawer
          open={drawerOpen}
          title={editingSecret ? "编辑凭证" : "新增凭证"}
          onClose={() => {
            setDrawerOpen(false);
            setEditingSecret(null);
          }}
          onSubmit={() => form.submit()}
          loading={saveMutation.isPending}
        >
          <Form
            form={form}
            layout="vertical"
            onFinish={(values) =>
              saveMutation.mutate({
                id: editingSecret?.id,
                name: values.name,
                type: values.type,
                description: values.description,
                purpose: values.purpose,
                status: values.status,
                secretValue: values.secretValue,
              } satisfies SecretInputPayload)
            }
          >
            <Form.Item label="名称" name="name" rules={[{ required: true, message: "请输入凭证名称" }]}>
              <Input />
            </Form.Item>
            <Form.Item label="类型" name="type" rules={[{ required: true, message: "请选择凭证类型" }]}>
              <Select options={secretTypeOptions as unknown as Array<{ label: string; value: string }>} />
            </Form.Item>
            <Form.Item label="用途" name="purpose">
              <Input placeholder="例如 registry auth / telegram bot / smtp credential" />
            </Form.Item>
            <Form.Item label="状态" name="status">
              <Select
                options={[
                  { label: "启用", value: "ACTIVE" },
                  { label: "停用", value: "DISABLED" },
                ]}
              />
            </Form.Item>
            <Form.Item
              label={editingSecret ? "新的凭证内容" : "凭证内容"}
              name="secretValue"
              rules={[{ required: true, message: "请输入凭证内容" }]}
            >
              <SecretInput />
            </Form.Item>
            <Form.Item label="说明" name="description">
              <Input.TextArea rows={4} />
            </Form.Item>
          </Form>
        </FormDrawer>

        <DangerConfirm
          open={Boolean(deleteTarget)}
          title="删除凭证"
          description={`删除后将移除 ${deleteTarget?.name ?? ""} 这条凭证记录。若仍有资源引用，后端会阻止删除。`}
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
