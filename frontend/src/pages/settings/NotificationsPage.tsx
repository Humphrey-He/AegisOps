import { App as AntApp, Button, Card, Form, Input, Radio, Select, Space, Switch, Tag, Typography } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";

import { DangerConfirm } from "../../components/DangerConfirm";
import { DataTable } from "../../components/DataTable";
import { ErrorState } from "../../components/ErrorState";
import { FormDrawer } from "../../components/FormDrawer";
import { PageHeader } from "../../components/PageHeader";
import { PermissionActionButton } from "../../components/PermissionActionButton";
import { PermissionGuard } from "../../components/PermissionGuard";
import { ResourceDetailPanel } from "../../components/resource/ResourceDetailPanel";
import { StatusBadge } from "../../components/StatusBadge";
import { notificationsApi, secretsApi } from "../../lib/api";
import { applyFormErrors, getErrorMessage } from "../../lib/forms";
import { formatDateTime } from "../../lib/format";
import { queryKeys } from "../../lib/queryKeys";
import { useSessionStore } from "../../store/sessionStore";
import type { NotificationChannel, NotificationChannelInput, Secret } from "../../types/models";

type SecretBindingMode = "existing" | "create";

type NotificationFormValues = {
  name: string;
  type: NotificationChannel["type"];
  enabled: boolean;
  language: NotificationChannel["language"];
  target: string;
  publicConfig?: string;
  secretBindingMode: SecretBindingMode;
  configSecretId?: string;
  config?: string;
};

const channelTypeOptions = [
  { label: "Telegram", value: "TELEGRAM" },
  { label: "WeCom", value: "WECOM" },
  { label: "Email", value: "EMAIL" },
] as const;

const languageOptions = [
  { label: "简体中文 · zh-CN", value: "zh-CN" },
  { label: "English · en-US", value: "en-US" },
] as const;

function formatLanguageLabel(language?: NotificationChannel["language"]) {
  if (language === "en-US") {
    return "English";
  }
  return "简体中文";
}

function resolveSecretBindingMode(channel?: NotificationChannel | null): SecretBindingMode {
  if (channel?.configSecretId) {
    return "existing";
  }
  return "create";
}

function buildFormValues(channel?: NotificationChannel | null): NotificationFormValues {
  return {
    name: channel?.name ?? "",
    type: channel?.type ?? "TELEGRAM",
    enabled: channel?.enabled ?? true,
    language: channel?.language ?? "zh-CN",
    target: channel?.target ?? "",
    publicConfig: channel?.publicConfig ?? "",
    secretBindingMode: resolveSecretBindingMode(channel),
    configSecretId: channel?.configSecretId ?? "",
    config: "",
  };
}

function buildSecretLabel(secret: Secret) {
  const segments = [secret.name, secret.type];
  if (secret.purpose) {
    segments.push(secret.purpose);
  }
  return segments.join(" · ");
}

function validateJsonString(value?: string) {
  if (!value || !value.trim()) {
    return true;
  }
  JSON.parse(value);
  return true;
}

export function NotificationsPage() {
  const { message } = AntApp.useApp();
  const permissions = useSessionStore((state) => state.permissions);
  const canManageSecretBinding = permissions.includes("*") || permissions.includes("notifications.secret.manage");
  const canViewSecrets = permissions.includes("*") || permissions.includes("secrets.view");
  const queryClient = useQueryClient();
  const [form] = Form.useForm<NotificationFormValues>();
  const [searchParams, setSearchParams] = useSearchParams();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editingChannel, setEditingChannel] = useState<NotificationChannel | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<NotificationChannel | null>(null);
  const selectedChannelId = searchParams.get("selected") ?? "";
  const secretBindingMode = Form.useWatch("secretBindingMode", form) ?? "existing";

  const channelsQuery = useQuery({
    queryKey: queryKeys.notificationChannels,
    queryFn: notificationsApi.listChannels,
  });
  const recordsQuery = useQuery({
    queryKey: queryKeys.notificationRecords,
    queryFn: notificationsApi.listRecords,
  });
  const secretsQuery = useQuery({
    queryKey: queryKeys.secrets("notification-channel"),
    queryFn: () => secretsApi.list("notification"),
    enabled: canViewSecrets,
  });

  const selectedChannel =
    (channelsQuery.data ?? []).find((item) => item.id === selectedChannelId) ?? channelsQuery.data?.[0] ?? null;

  const relatedRecords = useMemo(() => {
    if (!selectedChannel) {
      return [];
    }
    return (recordsQuery.data ?? []).filter((item) => item.channelId === selectedChannel.id).slice(0, 8);
  }, [recordsQuery.data, selectedChannel]);

  const secretMap = useMemo(
    () => new Map((secretsQuery.data ?? []).map((secret) => [secret.id, secret])),
    [secretsQuery.data],
  );

  const secretOptions = useMemo(
    () =>
      (secretsQuery.data ?? []).map((secret) => ({
        label: buildSecretLabel(secret),
        value: secret.id,
      })),
    [secretsQuery.data],
  );

  const selectedSecret = selectedChannel?.configSecretId ? secretMap.get(selectedChannel.configSecretId) : undefined;

  const saveMutation = useMutation({
    mutationFn: notificationsApi.saveChannel,
    onSuccess: async (channel) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.notificationChannels }),
        queryClient.invalidateQueries({ queryKey: queryKeys.notificationRecords }),
        queryClient.invalidateQueries({ queryKey: ["secrets"] }),
      ]);
      setDrawerOpen(false);
      setEditingChannel(null);
      form.resetFields();
      setSearchParams((previous) => {
        const next = new URLSearchParams(previous);
        next.set("selected", channel.id);
        return next;
      });
      await message.success(editingChannel ? "通知通道已更新" : "通知通道已创建");
    },
    onError: (error) => {
      applyFormErrors(form, error);
      void message.error(getErrorMessage(error));
    },
  });

  const testMutation = useMutation({
    mutationFn: notificationsApi.testChannel,
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.notificationChannels }),
        queryClient.invalidateQueries({ queryKey: queryKeys.notificationRecords }),
      ]);
      await message.success("测试通知已发送");
    },
    onError: (error) => {
      void message.error(getErrorMessage(error, "发送测试通知失败"));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: notificationsApi.removeChannel,
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.notificationChannels }),
        queryClient.invalidateQueries({ queryKey: queryKeys.notificationRecords }),
      ]);
      if (selectedChannelId === deleteTarget?.id) {
        setSearchParams((previous) => {
          const next = new URLSearchParams(previous);
          next.delete("selected");
          return next;
        });
      }
      setDeleteTarget(null);
      await message.success("通知通道已删除");
    },
    onError: (error) => {
      void message.error(getErrorMessage(error, "删除通知通道失败"));
    },
  });

  if (channelsQuery.isError) {
    return <ErrorState message={channelsQuery.error.message} onRetry={() => void channelsQuery.refetch()} />;
  }

  return (
    <PermissionGuard permission="notifications.view" forbiddenPage>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <PageHeader
          title="通知通道"
          description="统一管理 Telegram、WeCom、Email 通道，并将敏感配置收敛到 Secret 绑定。"
          extra={
            <PermissionActionButton
              type="primary"
              permission="notifications.manage"
              permissionReason="当前账号缺少 notifications.manage 权限，无法维护通知通道。"
              onClick={() => {
                setEditingChannel(null);
                form.setFieldsValue(buildFormValues(null));
                setDrawerOpen(true);
              }}
            >
              新增通道
            </PermissionActionButton>
          }
        />

        <div className="resource-workbench">
          <div className="resource-list-pane">
            <Card className="page-card">
              <DataTable
                rowKey="id"
                loading={channelsQuery.isLoading}
                dataSource={channelsQuery.data}
                rowClassName={(item) => (item.id === selectedChannel?.id ? "resource-row-selected" : "")}
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
                    title: "通道",
                    dataIndex: "name",
                    render: (_, channel) => (
                      <Space direction="vertical" size={2}>
                        <Typography.Text>{channel.name}</Typography.Text>
                        <Space size={8} wrap>
                          <Typography.Text type="secondary">{channel.type}</Typography.Text>
                          {channel.configSecretId ? <Tag color="blue">Secret</Tag> : <Tag>无密钥</Tag>}
                        </Space>
                      </Space>
                    ),
                  },
                  {
                    title: "状态",
                    dataIndex: "enabled",
                    render: (value: boolean) => <StatusBadge status={value ? "ACTIVE" : "DISABLED"} />,
                  },
                  {
                    title: "语言",
                    dataIndex: "language",
                    render: (value: NotificationChannel["language"]) => formatLanguageLabel(value),
                  },
                  {
                    title: "最近结果",
                    dataIndex: "lastTestStatus",
                    render: (value?: string) => (value ? <StatusBadge status={value} /> : "--"),
                  },
                ]}
              />
            </Card>
          </div>

          <div className="resource-detail-pane">
            <ResourceDetailPanel
              title={selectedChannel?.name}
              subtitle={selectedChannel ? `${selectedChannel.type} · ${selectedChannel.target || "--"}` : undefined}
              status={
                selectedChannel ? <StatusBadge status={selectedChannel.enabled ? "ACTIVE" : "DISABLED"} /> : undefined
              }
              meta={
                selectedChannel
                  ? [
                      { label: "默认语言", value: formatLanguageLabel(selectedChannel.language) },
                      { label: "默认接收目标", value: selectedChannel.target || "--" },
                      {
                        label: "敏感配置",
                        value: selectedChannel.configSecretId ? (
                          <Space size={8} wrap>
                            <Tag color="blue">已绑定 Secret</Tag>
                            <Typography.Text code>{selectedChannel.configSecretId}</Typography.Text>
                          </Space>
                        ) : (
                          "未绑定"
                        ),
                      },
                      {
                        label: "Secret 名称",
                        value: selectedSecret ? selectedSecret.name : selectedChannel.configSecretId ? "加载中或无访问权限" : "--",
                      },
                      {
                        label: "公开配置",
                        value: selectedChannel.publicConfig ? (
                          <Typography.Text code>{selectedChannel.publicConfig}</Typography.Text>
                        ) : (
                          "--"
                        ),
                      },
                      {
                        label: "最近测试",
                        value: selectedChannel.lastTestAt ? formatDateTime(selectedChannel.lastTestAt) : "--",
                      },
                      {
                        label: "最近结果",
                        value: selectedChannel.lastTestStatus ? <StatusBadge status={selectedChannel.lastTestStatus} /> : "--",
                      },
                      { label: "失败原因", value: selectedChannel.lastFailureReason || "--" },
                    ]
                  : []
              }
              actions={
                selectedChannel ? (
                  <Space wrap>
                    <PermissionActionButton
                      type="primary"
                      permission="notifications.test"
                      permissionReason="当前账号缺少 notifications.test 权限，无法发送测试通知。"
                      loading={testMutation.isPending}
                      onClick={() => testMutation.mutate(selectedChannel.id)}
                    >
                      发送测试
                    </PermissionActionButton>
                    <PermissionActionButton
                      permission="notifications.manage"
                      permissionReason="当前账号缺少 notifications.manage 权限，无法编辑通知通道。"
                      onClick={() => {
                        setEditingChannel(selectedChannel);
                        form.setFieldsValue(buildFormValues(selectedChannel));
                        setDrawerOpen(true);
                      }}
                    >
                      编辑
                    </PermissionActionButton>
                    <PermissionActionButton
                      danger
                      permission="notifications.manage"
                      permissionReason="当前账号缺少 notifications.manage 权限，无法删除通知通道。"
                      onClick={() => setDeleteTarget(selectedChannel)}
                    >
                      删除
                    </PermissionActionButton>
                  </Space>
                ) : undefined
              }
            >
              <div className="resource-detail-section">
                <div className="page-toolbar">
                  <Typography.Text strong>配置概览</Typography.Text>
                </div>
                <div className="resource-subpanel" style={{ marginTop: 12 }}>
                  <Space direction="vertical" size={12} style={{ width: "100%" }}>
                    <Card size="small">
                      <Space direction="vertical" size={4} style={{ width: "100%" }}>
                        <Typography.Text strong>公开配置摘要</Typography.Text>
                        <Typography.Text type="secondary">
                          适合保存非敏感参数，例如默认 chatId、收件人、SMTP Host 或 Webhook 目标说明。
                        </Typography.Text>
                        <Typography.Text code>{selectedChannel?.publicConfig || "--"}</Typography.Text>
                      </Space>
                    </Card>
                    <Card size="small">
                      <Space direction="vertical" size={4} style={{ width: "100%" }}>
                        <Typography.Text strong>敏感配置绑定</Typography.Text>
                        <Typography.Text type="secondary">
                          Token、Webhook 密钥、SMTP 用户名密码等敏感内容不再在通道详情里回显，只展示 Secret 绑定状态。
                        </Typography.Text>
                        <Space size={8} wrap>
                          {selectedChannel?.configSecretId ? <Tag color="blue">已保护</Tag> : <Tag>待绑定</Tag>}
                          {selectedChannel?.configSecretId ? (
                            <Typography.Text code>{selectedChannel.configSecretId}</Typography.Text>
                          ) : null}
                        </Space>
                      </Space>
                    </Card>
                  </Space>
                </div>
              </div>

              <div className="resource-detail-section">
                <div className="page-toolbar">
                  <Typography.Text strong>最近发送记录</Typography.Text>
                  <Typography.Text type="secondary">
                    {recordsQuery.isLoading ? "正在读取..." : `${relatedRecords.length} 条`}
                  </Typography.Text>
                </div>
                <div className="resource-subpanel" style={{ marginTop: 12 }}>
                  <DataTable
                    rowKey="id"
                    pagination={false}
                    loading={recordsQuery.isLoading}
                    dataSource={relatedRecords}
                    locale={{ emptyText: "当前通道还没有发送记录" }}
                    columns={[
                      { title: "状态", dataIndex: "status", render: (value: string) => <StatusBadge status={value} /> },
                      { title: "类型", dataIndex: "channelType" },
                      {
                        title: "时间",
                        dataIndex: "createdAt",
                        render: (value: string) => formatDateTime(value),
                      },
                      {
                        title: "结果",
                        key: "message",
                        render: (_, record) => record.errorMessage || record.responseExcerpt || "--",
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
          title={editingChannel ? "编辑通知通道" : "新增通知通道"}
          onClose={() => {
            setDrawerOpen(false);
            setEditingChannel(null);
          }}
          onSubmit={() => form.submit()}
          loading={saveMutation.isPending}
        >
          <Form
            form={form}
            layout="vertical"
            onFinish={(values) => {
              if (!canManageSecretBinding && !editingChannel?.configSecretId) {
                void message.error("当前账号缺少通知敏感配置维护权限，无法为新通道配置 Secret。");
                return;
              }
              saveMutation.mutate({
                id: editingChannel?.id,
                name: values.name,
                type: values.type,
                enabled: values.enabled,
                language: values.language,
                target: values.target,
                publicConfig: values.publicConfig?.trim() || "",
                configSecretId:
                  values.secretBindingMode === "existing" ? values.configSecretId?.trim() || editingChannel?.configSecretId || "" : "",
                config: values.secretBindingMode === "create" ? values.config?.trim() || "" : "",
              } satisfies NotificationChannelInput);
            }}
          >
            <Form.Item label="通道名称" name="name" rules={[{ required: true, message: "请输入通道名称" }]}>
              <Input />
            </Form.Item>

            <Form.Item label="通道类型" name="type" rules={[{ required: true, message: "请选择通道类型" }]}>
              <Select options={channelTypeOptions as unknown as Array<{ label: string; value: string }>} />
            </Form.Item>

            <Form.Item label="消息语言" name="language" rules={[{ required: true, message: "请选择消息语言" }]}>
              <Select options={languageOptions as unknown as Array<{ label: string; value: string }>} />
            </Form.Item>

            <Form.Item label="默认接收目标" name="target" rules={[{ required: true, message: "请输入默认接收目标" }]}>
              <Input placeholder="Telegram chatId / WeCom 群 / Email 收件人" />
            </Form.Item>

            <Form.Item
              label="公开配置"
              name="publicConfig"
              extra="保存不会被视为敏感的 JSON 片段，例如默认 chatId、SMTP Host、From / To 或其他说明性参数。"
              rules={[
                {
                  validator: async (_, value) => {
                    try {
                      validateJsonString(value);
                    } catch {
                      throw new Error("公开配置必须是合法 JSON");
                    }
                  },
                },
              ]}
            >
              <Input.TextArea rows={5} style={{ fontFamily: "monospace" }} placeholder='例如 {"chatId":"7433377081"}' />
            </Form.Item>

            <Form.Item
              label="敏感配置策略"
              name="secretBindingMode"
              extra="敏感配置不会在通道详情里回显。可以绑定已有 Secret，或在本次保存时录入并由后端自动 Secret 化。"
              rules={[{ required: true, message: "请选择敏感配置策略" }]}
            >
              <Radio.Group
                optionType="button"
                buttonStyle="solid"
                disabled={!canManageSecretBinding}
                options={[
                  { label: "绑定已有 Secret", value: "existing", disabled: !canViewSecrets },
                  { label: "录入并自动 Secret 化", value: "create" },
                ]}
              />
            </Form.Item>

            {canManageSecretBinding ? (
              secretBindingMode === "existing" ? (
                canViewSecrets ? (
                  <Form.Item
                    label="绑定 Secret"
                    name="configSecretId"
                    extra="选择后，通知通道会直接引用该 Secret 作为敏感配置来源。"
                    rules={[{ required: true, message: "请选择要绑定的 Secret" }]}
                  >
                    <Select
                      showSearch
                      loading={secretsQuery.isLoading}
                      options={secretOptions}
                      placeholder="选择通知通道使用的 Secret"
                      optionFilterProp="label"
                    />
                  </Form.Item>
                ) : (
                  <Card size="small" style={{ marginBottom: 16 }}>
                    <Space direction="vertical" size={4}>
                      <Typography.Text strong>绑定已有 Secret</Typography.Text>
                      <Typography.Text type="secondary">
                        当前账号具备通知敏感配置维护能力，但缺少 `secrets.view`，无法浏览可选 Secret 列表。
                      </Typography.Text>
                    </Space>
                  </Card>
                )
              ) : (
                <Form.Item
                  label="敏感配置"
                  name="config"
                  extra="输入完整敏感 JSON。保存后后端会生成 Secret 并把通道绑定过去，不会再返回明文。"
                  rules={[
                    { required: true, message: "请输入敏感配置 JSON" },
                    {
                      validator: async (_, value) => {
                        try {
                          validateJsonString(value);
                        } catch {
                          throw new Error("敏感配置必须是合法 JSON");
                        }
                      },
                    },
                  ]}
                >
                  <Input.TextArea rows={8} style={{ fontFamily: "monospace" }} />
                </Form.Item>
              )
            ) : (
              <Card size="small">
                <Space direction="vertical" size={4}>
                  <Typography.Text strong>敏感配置绑定</Typography.Text>
                  <Typography.Text type="secondary">
                    当前账号可以查看通知通道，但没有通知敏感配置维护权限。已存在的 Secret 绑定状态会保留，敏感内容也不会在前端回显。
                  </Typography.Text>
                </Space>
              </Card>
            )}

            {editingChannel?.configSecretId ? (
              <Card size="small" style={{ marginBottom: 16 }}>
                <Space direction="vertical" size={4} style={{ width: "100%" }}>
                  <Typography.Text strong>当前绑定</Typography.Text>
                  <Space size={8} wrap>
                    <Tag color="blue">Secret</Tag>
                    <Typography.Text code>{editingChannel.configSecretId}</Typography.Text>
                    {secretMap.get(editingChannel.configSecretId) ? (
                      <Typography.Text type="secondary">{secretMap.get(editingChannel.configSecretId)?.name}</Typography.Text>
                    ) : null}
                  </Space>
                </Space>
              </Card>
            ) : null}

            <Form.Item label="启用状态" name="enabled" valuePropName="checked">
              <Switch checkedChildren="启用" unCheckedChildren="停用" />
            </Form.Item>
          </Form>
        </FormDrawer>

        <DangerConfirm
          open={Boolean(deleteTarget)}
          title="删除通知通道"
          description={`删除后将无法继续使用 ${deleteTarget?.name || "该通道"} 发送通知，请确认当前没有依赖它的告警规则。`}
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
