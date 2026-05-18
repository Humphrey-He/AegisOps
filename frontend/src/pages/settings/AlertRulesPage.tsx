import { App as AntApp, Button, Card, Form, Input, InputNumber, Select, Space, Switch, Tag, Typography } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";

import { DataTable } from "../../components/DataTable";
import { DangerConfirm } from "../../components/DangerConfirm";
import { ErrorState } from "../../components/ErrorState";
import { FormDrawer } from "../../components/FormDrawer";
import { PageHeader } from "../../components/PageHeader";
import { PermissionGuard } from "../../components/PermissionGuard";
import { ResourceDetailPanel } from "../../components/resource/ResourceDetailPanel";
import { StatusBadge } from "../../components/StatusBadge";
import { alertRulesApi, notificationsApi } from "../../lib/api";
import { applyFormErrors, getErrorMessage } from "../../lib/forms";
import { formatDateTime } from "../../lib/format";
import { queryKeys } from "../../lib/queryKeys";
import type { AlertRule, AlertRuleInput, NotificationLanguage } from "../../types/models";

type AlertRuleFormValues = {
  name: string;
  eventType: AlertRule["eventType"];
  resourceType?: string;
  resourceScope?: string;
  language?: string;
  channelIds: string[];
  enabled: boolean;
  dedupeWindowSeconds: number;
  requireAck: boolean;
};

const eventTypeOptions = [
  { label: "服务发布失败", value: "service_release_failed" },
  { label: "服务健康检查失败", value: "service_health_check_failed" },
  { label: "Nginx 重载失败", value: "nginx_reload_failed" },
  { label: "Nginx 配置发布失败", value: "nginx_publish_failed" },
  { label: "主机离线", value: "host_offline" },
  { label: "主机恢复", value: "host_recovered" },
] as const;

const eventTypeFilterOptions = [{ label: "全部事件类型", value: "" }, ...eventTypeOptions] as const;

const languageOptions = [
  { label: "跟随通知通道", value: "" },
  { label: "简体中文 · zh-CN", value: "zh-CN" },
  { label: "English · en-US", value: "en-US" },
] as const;

function formatRuleLanguageLabel(language?: NotificationLanguage) {
  if (!language) {
    return "跟随通知通道";
  }
  return language === "en-US" ? "English" : "简体中文";
}

function formatEventTypeLabel(eventType?: AlertRule["eventType"]) {
  return eventTypeOptions.find((item) => item.value === eventType)?.label ?? eventType ?? "--";
}

function buildFormValues(rule?: AlertRule | null): AlertRuleFormValues {
  return {
    name: rule?.name ?? "",
    eventType: rule?.eventType ?? "service_health_check_failed",
    resourceType: rule?.resourceType ?? "",
    resourceScope: rule?.resourceScope ?? "",
    language: rule?.language ?? "",
    channelIds: rule?.channelIds ?? [],
    enabled: rule?.enabled ?? true,
    dedupeWindowSeconds: rule?.dedupeWindowSeconds ?? 300,
    requireAck: rule?.requireAck ?? false,
  };
}

export function AlertRulesPage() {
  const { message } = AntApp.useApp();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<AlertRuleFormValues>();
  const [searchParams, setSearchParams] = useSearchParams();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editingRule, setEditingRule] = useState<AlertRule | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<AlertRule | null>(null);
  const eventTypeFilter = searchParams.get("eventType") ?? "";
  const selectedRuleId = searchParams.get("selected") ?? "";

  const rulesQuery = useQuery({
    queryKey: queryKeys.alertRules,
    queryFn: alertRulesApi.list,
  });
  const channelsQuery = useQuery({
    queryKey: queryKeys.notificationChannels,
    queryFn: notificationsApi.listChannels,
  });

  const filteredRules = useMemo(() => {
    const items = rulesQuery.data ?? [];
    if (!eventTypeFilter) {
      return items;
    }
    return items.filter((item) => item.eventType === eventTypeFilter);
  }, [eventTypeFilter, rulesQuery.data]);

  const selectedRule = useMemo(() => {
    if (!filteredRules.length) {
      return null;
    }
    return filteredRules.find((item) => item.id === selectedRuleId) ?? filteredRules[0];
  }, [filteredRules, selectedRuleId]);

  useEffect(() => {
    if (!selectedRuleId) {
      return;
    }
    if (filteredRules.some((item) => item.id === selectedRuleId)) {
      return;
    }
    setSearchParams((previous) => {
      const next = new URLSearchParams(previous);
      if (filteredRules[0]?.id) {
        next.set("selected", filteredRules[0].id);
      } else {
        next.delete("selected");
      }
      return next;
    });
  }, [filteredRules, selectedRuleId, setSearchParams]);

  const channelOptions = useMemo(
    () =>
      (channelsQuery.data ?? []).map((item) => ({
        label: `${item.name} · ${item.type} · ${item.language}`,
        value: item.id,
      })),
    [channelsQuery.data],
  );

  const channelNameMap = useMemo(
    () => new Map((channelsQuery.data ?? []).map((item) => [item.id, item.name])),
    [channelsQuery.data],
  );

  const saveMutation = useMutation({
    mutationFn: alertRulesApi.save,
    onSuccess: async (rule) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.alertRules });
      setDrawerOpen(false);
      setEditingRule(null);
      form.resetFields();
      setSearchParams((previous) => {
        const next = new URLSearchParams(previous);
        if (eventTypeFilter && rule.eventType !== eventTypeFilter) {
          next.set("eventType", rule.eventType);
        }
        next.set("selected", rule.id);
        return next;
      });
      await message.success(editingRule ? "告警规则已更新" : "告警规则已创建");
    },
    onError: (error) => {
      applyFormErrors(form, error);
      void message.error(getErrorMessage(error));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: alertRulesApi.remove,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.alertRules });
      if (selectedRuleId === deleteTarget?.id) {
        setSearchParams((previous) => {
          const next = new URLSearchParams(previous);
          next.delete("selected");
          return next;
        });
      }
      setDeleteTarget(null);
      await message.success("告警规则已删除");
    },
    onError: (error) => {
      void message.error(getErrorMessage(error, "删除告警规则失败"));
    },
  });

  if (rulesQuery.isError) {
    return <ErrorState message={rulesQuery.error.message} onRetry={() => void rulesQuery.refetch()} />;
  }

  return (
    <PermissionGuard permission="alerts.view" forbiddenPage>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <PageHeader
          title="告警规则"
          description="定义事件类型、资源范围、通知目标，以及规则级语言覆盖。"
          extra={
            <PermissionGuard permission="alerts.manage">
              <Button
                type="primary"
                onClick={() => {
                  setEditingRule(null);
                  form.setFieldsValue(buildFormValues(null));
                  setDrawerOpen(true);
                }}
              >
                新增规则
              </Button>
            </PermissionGuard>
          }
        />

        <div className="resource-workbench">
          <div className="resource-list-pane">
            <Card className="page-card">
              <div className="page-toolbar">
                <div className="page-toolbar-start">
                  <Select
                    style={{ width: 220 }}
                    options={eventTypeFilterOptions as unknown as Array<{ label: string; value: string }>}
                    value={eventTypeFilter}
                    onChange={(value) =>
                      setSearchParams((previous) => {
                        const next = new URLSearchParams(previous);
                        if (value) {
                          next.set("eventType", value);
                        } else {
                          next.delete("eventType");
                        }
                        next.delete("selected");
                        return next;
                      })
                    }
                  />
                </div>
                <Typography.Text type="secondary">
                  当前显示 {filteredRules.length} / {(rulesQuery.data ?? []).length} 条规则
                </Typography.Text>
              </div>
              <DataTable
                rowKey="id"
                loading={rulesQuery.isLoading}
                dataSource={filteredRules}
                rowClassName={(item) => (item.id === selectedRule?.id ? "resource-row-selected" : "")}
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
                    title: "规则",
                    dataIndex: "name",
                    render: (_, rule) => (
                      <Space direction="vertical" size={2}>
                        <Typography.Text>{rule.name}</Typography.Text>
                        <Typography.Text type="secondary">{formatEventTypeLabel(rule.eventType)}</Typography.Text>
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
                    render: (value?: NotificationLanguage) => formatRuleLanguageLabel(value),
                  },
                  {
                    title: "确认",
                    dataIndex: "requireAck",
                    render: (value: boolean) => (value ? "需要" : "自动"),
                  },
                ]}
              />
            </Card>
          </div>

          <div className="resource-detail-pane">
            <ResourceDetailPanel
              title={selectedRule?.name}
              subtitle={formatEventTypeLabel(selectedRule?.eventType)}
              status={selectedRule ? <StatusBadge status={selectedRule.enabled ? "ACTIVE" : "DISABLED"} /> : undefined}
              meta={
                selectedRule
                  ? [
                      { label: "资源类型", value: selectedRule.resourceType || "--" },
                      { label: "资源范围", value: selectedRule.resourceScope || "--" },
                      { label: "消息语言", value: formatRuleLanguageLabel(selectedRule.language) },
                      { label: "去重窗口", value: `${selectedRule.dedupeWindowSeconds} 秒` },
                      { label: "人工确认", value: selectedRule.requireAck ? "需要" : "不需要" },
                      {
                        label: "通知目标",
                        value: selectedRule.channelIds.length ? (
                          <Space wrap>
                            {selectedRule.channelIds.map((id) => (
                              <Tag key={id}>{channelNameMap.get(id) ?? id}</Tag>
                            ))}
                          </Space>
                        ) : (
                          "--"
                        ),
                      },
                    ]
                  : []
              }
              actions={
                selectedRule ? (
                  <PermissionGuard permission="alerts.manage">
                    <Button
                      onClick={() => {
                        setEditingRule(selectedRule);
                        form.setFieldsValue(buildFormValues(selectedRule));
                        setDrawerOpen(true);
                      }}
                    >
                      编辑
                    </Button>
                    <Button danger onClick={() => setDeleteTarget(selectedRule)}>
                      删除
                    </Button>
                  </PermissionGuard>
                ) : undefined
              }
            >
              <div className="resource-detail-section">
                <div className="resource-subpanel">
                  <Typography.Text strong>规则说明</Typography.Text>
                  <Typography.Text type="secondary">
                    规则可按资源范围与事件类型精确匹配，并可覆盖通知通道的默认消息语言。
                  </Typography.Text>
                  <Typography.Text type="secondary">
                    最近更新：{selectedRule?.updatedAt ? formatDateTime(selectedRule.updatedAt) : "--"}
                  </Typography.Text>
                </div>
              </div>
            </ResourceDetailPanel>
          </div>
        </div>

        <FormDrawer
          open={drawerOpen}
          title={editingRule ? "编辑告警规则" : "新增告警规则"}
          onClose={() => {
            setDrawerOpen(false);
            setEditingRule(null);
          }}
          onSubmit={() => form.submit()}
          loading={saveMutation.isPending}
        >
          <Form
            form={form}
            layout="vertical"
            onFinish={(values) =>
              saveMutation.mutate({
                id: editingRule?.id,
                name: values.name,
                eventType: values.eventType,
                resourceType: values.resourceType?.trim() || "",
                resourceScope: values.resourceScope?.trim() || "",
                language: ((values.language?.trim() || undefined) as NotificationLanguage | undefined),
                channelIds: values.channelIds ?? [],
                enabled: values.enabled,
                dedupeWindowSeconds: Number(values.dedupeWindowSeconds) || 300,
                requireAck: values.requireAck,
                suppressDuplicates: true,
              } satisfies AlertRuleInput)
            }
          >
            <Form.Item label="规则名称" name="name" rules={[{ required: true, message: "请输入规则名称" }]}>
              <Input />
            </Form.Item>
            <Form.Item label="事件类型" name="eventType" rules={[{ required: true, message: "请选择事件类型" }]}>
              <Select options={eventTypeOptions as unknown as Array<{ label: string; value: string }>} />
            </Form.Item>
            <Form.Item label="资源类型" name="resourceType">
              <Input placeholder="例如 service / host / nginx_node" />
            </Form.Item>
            <Form.Item label="资源范围" name="resourceScope">
              <Input placeholder="例如 production / gateway / critical" />
            </Form.Item>
            <Form.Item label="消息语言" name="language">
              <Select options={languageOptions as unknown as Array<{ label: string; value: string }>} />
            </Form.Item>
            <Form.Item label="通知目标" name="channelIds" rules={[{ required: true, message: "请至少选择一个通知通道" }]}>
              <Select mode="multiple" options={channelOptions} />
            </Form.Item>
            <Form.Item label="去重窗口（秒）" name="dedupeWindowSeconds" rules={[{ required: true, message: "请输入去重窗口" }]}>
              <InputNumber min={30} max={86400} style={{ width: "100%" }} />
            </Form.Item>
            <Form.Item label="启用规则" name="enabled" valuePropName="checked">
              <Switch checkedChildren="启用" unCheckedChildren="停用" />
            </Form.Item>
            <Form.Item label="需要人工确认" name="requireAck" valuePropName="checked">
              <Switch checkedChildren="需要确认" unCheckedChildren="自动关闭" />
            </Form.Item>
          </Form>
        </FormDrawer>

        <DangerConfirm
          open={Boolean(deleteTarget)}
          title="删除告警规则"
          description={`删除后，${deleteTarget?.name || "该规则"} 将不再参与异常匹配与通知分发。`}
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
