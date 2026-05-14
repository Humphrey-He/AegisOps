import {
  Alert,
  App as AntApp,
  Button,
  Card,
  Form,
  Input,
  Select,
  Space,
  Tag,
  Typography,
} from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { auditsApi, registriesApi, secretsApi } from "../../lib/api";
import { queryKeys } from "../../lib/queryKeys";
import { applyFormErrors, getErrorMessage } from "../../lib/forms";
import { DataTable } from "../../components/DataTable";
import { DangerConfirm } from "../../components/DangerConfirm";
import { EmptyState } from "../../components/EmptyState";
import { ErrorState } from "../../components/ErrorState";
import { FormDrawer } from "../../components/FormDrawer";
import { PageHeader } from "../../components/PageHeader";
import { PermissionGuard } from "../../components/PermissionGuard";
import { ResourceActivityList } from "../../components/resource/ResourceActivityList";
import { ResourceDetailPanel } from "../../components/resource/ResourceDetailPanel";
import { StatusBadge } from "../../components/StatusBadge";
import { formatDateTime } from "../../lib/format";
import type { Registry, RegistryInput, Secret } from "../../types/models";

type RegistryFormValues = {
  name: string;
  url: string;
  authType: Registry["authType"];
  secretId?: string;
  description?: string;
};

const authTypeLabelMap: Record<Registry["authType"], string> = {
  NONE: "匿名访问",
  BASIC: "Basic Auth",
  TOKEN: "Bearer Token",
};

function secretMatchesRegistry(secret: Secret, authType: Registry["authType"]) {
  if (authType === "NONE") {
    return true;
  }
  if (authType === "BASIC") {
    return secret.type === "DOCKER_TOKEN" || secret.type === "SSH_PASSWORD";
  }
  return secret.type === "DOCKER_TOKEN";
}

function classifyRegistryError(message: string) {
  const normalized = message.toLowerCase();
  if (normalized.includes("authentication")) {
    return {
      type: "error" as const,
      title: "认证失败",
      description: "请检查绑定凭证是否存在，以及 Basic 模式是否使用 username:password 格式。",
    };
  }
  if (normalized.includes("network")) {
    return {
      type: "warning" as const,
      title: "网络异常",
      description: "Registry 地址可达性异常，建议检查域名、协议和网络策略。",
    };
  }
  if (normalized.includes("not found")) {
    return {
      type: "info" as const,
      title: "资源为空或不存在",
      description: "当前仓库路径或 Tag 不存在，也可能是 Registry 中还没有推送任何镜像。",
    };
  }
  return {
    type: "info" as const,
    title: "返回提示",
    description: message,
  };
}

export function RegistriesPage() {
  const { message } = AntApp.useApp();
  const [form] = Form.useForm<RegistryFormValues>();
  const [keyword, setKeyword] = useState("");
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editingRegistry, setEditingRegistry] = useState<Registry | null>(null);
  const [selectedRepository, setSelectedRepository] = useState("");
  const [selectedReference, setSelectedReference] = useState("");
  const [latestActionText, setLatestActionText] = useState<string | null>(null);
  const [panelError, setPanelError] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Registry | null>(null);
  const [searchParams, setSearchParams] = useSearchParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const selectedRegistryId = searchParams.get("selected") ?? "";
  const watchedAuthType = Form.useWatch("authType", form) ?? editingRegistry?.authType ?? "BASIC";

  const registriesQuery = useQuery({
    queryKey: queryKeys.registries(keyword),
    queryFn: () => registriesApi.list(keyword),
  });
  const secretsQuery = useQuery({
    queryKey: queryKeys.secrets(""),
    queryFn: () => secretsApi.list(""),
  });
  const auditsQuery = useQuery({
    queryKey: queryKeys.audits,
    queryFn: auditsApi.list,
  });
  const registryDetailQuery = useQuery({
    queryKey: queryKeys.registry(selectedRegistryId),
    queryFn: () => registriesApi.detail(selectedRegistryId),
    enabled: Boolean(selectedRegistryId),
  });
  const repositoriesQuery = useQuery({
    queryKey: queryKeys.registryRepositories(selectedRegistryId),
    queryFn: () => registriesApi.repositories(selectedRegistryId),
    enabled: Boolean(selectedRegistryId),
  });
  const tagsQuery = useQuery({
    queryKey: queryKeys.registryTags(selectedRegistryId, selectedRepository),
    queryFn: () => registriesApi.tags(selectedRegistryId, selectedRepository),
    enabled: Boolean(selectedRegistryId && selectedRepository),
    retry: false,
  });
  const manifestQuery = useQuery({
    queryKey: queryKeys.registryManifest(selectedRegistryId, selectedRepository, selectedReference),
    queryFn: () => registriesApi.manifest(selectedRegistryId, selectedRepository, selectedReference),
    enabled: Boolean(selectedRegistryId && selectedRepository && selectedReference),
    retry: false,
  });

  const selectedRegistry =
    registryDetailQuery.data ?? (registriesQuery.data ?? []).find((item) => item.id === selectedRegistryId) ?? null;

  useEffect(() => {
    const repositories = repositoriesQuery.data?.repositories ?? [];
    if (!repositories.length) {
      if (selectedRepository) {
        setSelectedRepository("");
      }
      return;
    }
    if (!selectedRepository || !repositories.includes(selectedRepository)) {
      setSelectedRepository(repositories[0]);
    }
  }, [repositoriesQuery.data?.repositories, selectedRepository]);

  useEffect(() => {
    const tags = tagsQuery.data?.tags ?? [];
    if (!tags.length) {
      if (selectedReference) {
        setSelectedReference("");
      }
      return;
    }
    if (!selectedReference || !tags.includes(selectedReference)) {
      setSelectedReference(tags[0]);
    }
  }, [tagsQuery.data?.tags, selectedReference]);

  useEffect(() => {
    if (!selectedRegistryId) {
      setPanelError(null);
      return;
    }
    if (repositoriesQuery.error) {
      setPanelError(repositoriesQuery.error.message);
      return;
    }
    if (tagsQuery.error) {
      setPanelError(tagsQuery.error.message);
      return;
    }
    if (manifestQuery.error) {
      setPanelError(manifestQuery.error.message);
      return;
    }
    setPanelError(null);
  }, [manifestQuery.error, repositoriesQuery.error, selectedRegistryId, tagsQuery.error]);

  const saveMutation = useMutation({
    mutationFn: registriesApi.save,
    onSuccess: async (registry) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["registries"] }),
        queryClient.invalidateQueries({ queryKey: queryKeys.audits }),
        queryClient.invalidateQueries({ queryKey: queryKeys.secrets("") }),
      ]);
      setDrawerOpen(false);
      setEditingRegistry(null);
      form.resetFields();
      setSearchParams((previous) => {
        const next = new URLSearchParams(previous);
        next.set("selected", registry.id);
        return next;
      });
      await message.success(editingRegistry ? "Registry 已更新" : "Registry 已创建");
    },
    onError: (error) => {
      applyFormErrors(form, error);
      void message.error(getErrorMessage(error));
    },
  });

  const testMutation = useMutation({
    mutationFn: registriesApi.test,
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["registries"] }),
        selectedRegistryId ? queryClient.invalidateQueries({ queryKey: queryKeys.registry(selectedRegistryId) }) : Promise.resolve(),
        queryClient.invalidateQueries({ queryKey: queryKeys.audits }),
      ]);
      setPanelError(null);
      setLatestActionText("已完成一次 Registry 连通性检测。");
      await message.success("Registry 连接测试成功");
    },
    onError: async (error) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["registries"] }),
        selectedRegistryId ? queryClient.invalidateQueries({ queryKey: queryKeys.registry(selectedRegistryId) }) : Promise.resolve(),
        queryClient.invalidateQueries({ queryKey: queryKeys.audits }),
      ]);
      setLatestActionText("本次连接测试返回失败，请根据下方错误提示继续排查。");
      setPanelError(getErrorMessage(error));
      void message.error(getErrorMessage(error, "Registry 测试失败"));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: registriesApi.remove,
    onSuccess: async () => {
      const removingId = deleteTarget?.id ?? "";
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["registries"] }),
        queryClient.invalidateQueries({ queryKey: queryKeys.audits }),
        queryClient.invalidateQueries({ queryKey: queryKeys.secrets("") }),
      ]);
      setDeleteTarget(null);
      if (selectedRegistryId === removingId) {
        setSearchParams((previous) => {
          const next = new URLSearchParams(previous);
          next.delete("selected");
          return next;
        });
        setSelectedRepository("");
        setSelectedReference("");
      }
      await message.success("Registry 已删除");
    },
    onError: (error) => {
      void message.error(getErrorMessage(error, "删除 Registry 失败"));
    },
  });

  const secretOptions = useMemo(() => {
    return (secretsQuery.data ?? [])
      .filter((item) => secretMatchesRegistry(item, watchedAuthType))
      .map((item) => ({
        label: `${item.name} · ${item.type}`,
        value: item.id,
      }));
  }, [secretsQuery.data, watchedAuthType]);

  const selectedSecret = useMemo(() => {
    if (!selectedRegistry?.secretId) {
      return null;
    }
    return (secretsQuery.data ?? []).find((item) => item.id === selectedRegistry.secretId) ?? null;
  }, [secretsQuery.data, selectedRegistry?.secretId]);

  const relatedAudits = useMemo(() => {
    if (!selectedRegistry) {
      return [];
    }
    return (auditsQuery.data ?? [])
      .filter((audit) => {
        const haystack = `${audit.resourceType} ${audit.resourceName} ${audit.summary}`.toLowerCase();
        return haystack.includes(selectedRegistry.id.toLowerCase()) || haystack.includes(selectedRegistry.name.toLowerCase());
      })
      .slice(0, 6);
  }, [auditsQuery.data, selectedRegistry]);

  const repositoryRows = useMemo(
    () =>
      (repositoriesQuery.data?.repositories ?? []).map((repository) => ({
        key: repository,
        repository,
        tagCount: tagsQuery.data?.name === repository ? tagsQuery.data.tags.length : undefined,
      })),
    [repositoriesQuery.data?.repositories, tagsQuery.data],
  );

  const currentErrorMeta = panelError ? classifyRegistryError(panelError) : null;

  if (registriesQuery.isError) {
    return <ErrorState message={registriesQuery.error.message} onRetry={() => void registriesQuery.refetch()} />;
  }

  return (
    <PermissionGuard permission="registries.view" forbiddenPage>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <PageHeader
          title="Registry"
          description="二期先把镜像来源管理做实：仓库接入、连通性测试、仓库目录与 Tag 浏览都在同一工作台完成。"
          extra={
            <PermissionGuard permission="registries.manage">
              <Button
                type="primary"
                onClick={() => {
                  setEditingRegistry(null);
                  form.resetFields();
                  form.setFieldsValue({ authType: "BASIC" });
                  setDrawerOpen(true);
                }}
              >
                新增 Registry
              </Button>
            </PermissionGuard>
          }
        />

        <Card className="page-card">
          <div className="page-toolbar">
            <div className="page-toolbar-start">
              <Input.Search
                allowClear
                placeholder="搜索 Registry 名称、地址或描述"
                style={{ width: 360 }}
                onSearch={(value) => {
                  setKeyword(value);
                  setLatestActionText(null);
                }}
              />
            </div>
          </div>
        </Card>

        <div className="resource-workbench">
          <div className="resource-list-pane">
            <Card className="page-card">
              <DataTable
                rowKey="id"
                loading={registriesQuery.isLoading}
                dataSource={registriesQuery.data}
                rowClassName={(registry) => (registry.id === selectedRegistryId ? "resource-row-selected" : "")}
                onRow={(registry) => ({
                  onClick: () => {
                    setSearchParams((previous) => {
                      const next = new URLSearchParams(previous);
                      next.set("selected", registry.id);
                      return next;
                    });
                    setSelectedRepository("");
                    setSelectedReference("");
                    setLatestActionText(null);
                    setPanelError(null);
                  },
                })}
                locale={{
                  emptyText: (
                    <EmptyState
                      title="还没有接入任何 Registry"
                      description="先把镜像仓库接进来，后续服务定义和发布流程才能选择镜像版本。"
                      action={
                        <Button type="primary" onClick={() => setDrawerOpen(true)}>
                          创建第一个 Registry
                        </Button>
                      }
                    />
                  ),
                }}
                columns={[
                  {
                    title: "Registry",
                    dataIndex: "name",
                    render: (_, registry) => (
                      <Space direction="vertical" size={2}>
                        <span>{registry.name}</span>
                        <span style={{ color: "#64748b" }}>{registry.url}</span>
                      </Space>
                    ),
                  },
                  {
                    title: "认证",
                    dataIndex: "authType",
                    render: (value: Registry["authType"]) => <Tag color="blue">{authTypeLabelMap[value]}</Tag>,
                  },
                  {
                    title: "状态",
                    dataIndex: "status",
                    render: (value: Registry["status"]) => <StatusBadge status={value} />,
                  },
                  {
                    title: "最近检测",
                    dataIndex: "lastTestAt",
                    render: (value?: string) => (value ? formatDateTime(value) : "--"),
                  },
                ]}
              />
            </Card>
          </div>

          <div className="resource-detail-pane">
            <ResourceDetailPanel
              title={selectedRegistry?.name}
              subtitle={selectedRegistry?.url}
              status={selectedRegistry ? <StatusBadge status={selectedRegistry.status} /> : undefined}
              meta={
                selectedRegistry
                  ? [
                      {
                        label: "认证方式",
                        value: <Tag color="blue">{authTypeLabelMap[selectedRegistry.authType]}</Tag>,
                      },
                      {
                        label: "绑定凭证",
                        value:
                          selectedRegistry.authType === "NONE"
                            ? "--"
                            : (selectedSecret?.name ?? selectedRegistry.secretId ?? "--"),
                      },
                      {
                        label: "最近检测",
                        value: selectedRegistry.lastTestAt ? formatDateTime(selectedRegistry.lastTestAt) : "--",
                      },
                      {
                        label: "仓库数量",
                        value: repositoriesQuery.data?.repositories.length ?? 0,
                      },
                      {
                        label: "说明",
                        value: selectedRegistry.description || "--",
                      },
                      {
                        label: "更新时间",
                        value: formatDateTime(selectedRegistry.updatedAt),
                      },
                    ]
                  : []
              }
              actions={
                selectedRegistry ? (
                  <Space wrap>
                    <PermissionGuard permission="registries.manage">
                      <Button
                        type={selectedRegistry.status !== "ONLINE" ? "primary" : "default"}
                        loading={testMutation.isPending}
                        onClick={() => {
                          setLatestActionText("正在执行 Registry 连通性测试...");
                          setPanelError(null);
                          testMutation.mutate(selectedRegistry.id);
                        }}
                      >
                        测试连接
                      </Button>
                    </PermissionGuard>
                    <PermissionGuard permission="registries.manage">
                      <Button
                        onClick={() => {
                          setEditingRegistry(selectedRegistry);
                          form.setFieldsValue({
                            name: selectedRegistry.name,
                            url: selectedRegistry.url,
                            authType: selectedRegistry.authType,
                            secretId: selectedRegistry.secretId || undefined,
                            description: selectedRegistry.description,
                          });
                          setDrawerOpen(true);
                        }}
                      >
                        编辑
                      </Button>
                    </PermissionGuard>
                    <PermissionGuard permission="registries.manage">
                      <Button danger onClick={() => setDeleteTarget(selectedRegistry)}>
                        删除
                      </Button>
                    </PermissionGuard>
                  </Space>
                ) : undefined
              }
            >
              {latestActionText ? (
                <div className="resource-detail-section">
                  <Typography.Text type="secondary">{latestActionText}</Typography.Text>
                </div>
              ) : null}

              {currentErrorMeta ? (
                <div className="resource-detail-section">
                  <Alert
                    type={currentErrorMeta.type}
                    showIcon
                    message={currentErrorMeta.title}
                    description={currentErrorMeta.description}
                  />
                </div>
              ) : null}

              <div className="resource-detail-section">
                <div className="page-toolbar">
                  <Typography.Text strong>仓库目录</Typography.Text>
                  <Typography.Text type="secondary">
                    {repositoriesQuery.isLoading
                      ? "正在加载目录..."
                      : `${repositoriesQuery.data?.repositories.length ?? 0} 个仓库`}
                  </Typography.Text>
                </div>
                <div className="two-col-grid registry-browser-grid" style={{ marginTop: 12 }}>
                  <div className="resource-subpanel registry-browser-card">
                    <Typography.Text strong>Repositories</Typography.Text>
                    <DataTable
                      rowKey="repository"
                      pagination={false}
                      loading={repositoriesQuery.isLoading}
                      dataSource={repositoryRows}
                      rowClassName={(row) => (row.repository === selectedRepository ? "resource-row-selected" : "")}
                      onRow={(row) => ({
                        onClick: () => {
                          setSelectedRepository(row.repository);
                          setSelectedReference("");
                          setPanelError(null);
                        },
                      })}
                      locale={{
                        emptyText: selectedRegistry ? (
                          <EmptyState
                            title="Registry 暂无仓库"
                            description="连接成功，但当前目录下还没有可浏览的镜像仓库。"
                          />
                        ) : (
                          "暂无数据"
                        ),
                      }}
                      columns={[
                        {
                          title: "仓库名",
                          dataIndex: "repository",
                        },
                        {
                          title: "Tags",
                          dataIndex: "tagCount",
                          width: 80,
                          render: (value: number | undefined) => value ?? "--",
                        },
                      ]}
                    />
                  </div>

                  <div className="resource-subpanel registry-browser-card">
                    <Typography.Text strong>{selectedRepository ? `Tags · ${selectedRepository}` : "Tags"}</Typography.Text>
                    <DataTable
                      rowKey="tag"
                      pagination={false}
                      loading={tagsQuery.isLoading}
                      dataSource={(tagsQuery.data?.tags ?? []).map((tag) => ({ tag }))}
                      rowClassName={(row) => (row.tag === selectedReference ? "resource-row-selected" : "")}
                      onRow={(row) => ({
                        onClick: () => {
                          setSelectedReference(row.tag);
                          setPanelError(null);
                        },
                      })}
                      locale={{
                        emptyText: selectedRepository ? (
                          <EmptyState
                            title="当前仓库暂无 Tag"
                            description="这通常意味着镜像尚未推送，或 Registry 返回了空目录。"
                          />
                        ) : (
                          <EmptyState title="先选择一个仓库" description="左侧选中仓库后，这里会展示可用的镜像 Tag。" />
                        ),
                      }}
                      columns={[
                        {
                          title: "Tag",
                          dataIndex: "tag",
                        },
                      ]}
                    />
                  </div>
                </div>
              </div>

              <div className="resource-detail-section">
                <div className="page-toolbar">
                  <Typography.Text strong>镜像摘要</Typography.Text>
                  {selectedReference ? (
                    <Typography.Text type="secondary">
                      {selectedRepository}:{selectedReference}
                    </Typography.Text>
                  ) : null}
                </div>
                <div className="resource-subpanel" style={{ marginTop: 12 }}>
                  {manifestQuery.isLoading ? (
                    <Typography.Text type="secondary">正在读取 Manifest...</Typography.Text>
                  ) : manifestQuery.data ? (
                    <Space direction="vertical" size={12} style={{ width: "100%" }}>
                      <div className="resource-detail-metadata">
                        <div className="resource-detail-metadata-item">
                          <Typography.Text type="secondary" className="resource-detail-metadata-label">
                            Digest
                          </Typography.Text>
                          <Typography.Text code>{manifestQuery.data.digest || "--"}</Typography.Text>
                        </div>
                        <div className="resource-detail-metadata-item">
                          <Typography.Text type="secondary" className="resource-detail-metadata-label">
                            Content-Type
                          </Typography.Text>
                          <Typography.Text>{manifestQuery.data.contentType || "--"}</Typography.Text>
                        </div>
                      </div>
                      <pre className="log-viewer" style={{ height: 240, margin: 0 }}>
                        {JSON.stringify(manifestQuery.data.manifest, null, 2)}
                      </pre>
                    </Space>
                  ) : (
                    <EmptyState
                      title="还没有选中镜像版本"
                      description="选择仓库和 Tag 后，这里会展示 Manifest 摘要，便于后续发布流程直接引用。"
                    />
                  )}
                </div>
              </div>

              <ResourceActivityList
                title="最近审计"
                actionLabel={selectedRegistry ? "查看全部审计" : undefined}
                onActionClick={selectedRegistry ? () => navigate("/audits") : undefined}
                items={relatedAudits.map((audit) => ({
                  key: audit.id,
                  title: audit.action,
                  description: audit.summary,
                  meta: `${audit.actor} · ${formatDateTime(audit.createdAt)}`,
                  extra: <StatusBadge status={audit.result} />,
                }))}
                emptyText="当前 Registry 还没有关联审计记录。"
              />
            </ResourceDetailPanel>
          </div>
        </div>

        <FormDrawer
          open={drawerOpen}
          title={editingRegistry ? "编辑 Registry" : "新增 Registry"}
          onClose={() => {
            setDrawerOpen(false);
            setEditingRegistry(null);
          }}
          onSubmit={() => form.submit()}
          loading={saveMutation.isPending}
        >
          <Form
            layout="vertical"
            form={form}
            onValuesChange={(changedValues) => {
              if ("authType" in changedValues && changedValues.authType === "NONE") {
                form.setFieldValue("secretId", undefined);
              }
            }}
            onFinish={(values) =>
              saveMutation.mutate({
                id: editingRegistry?.id,
                name: values.name,
                url: values.url,
                authType: values.authType,
                secretId: values.authType === "NONE" ? "" : values.secretId,
                description: values.description,
              } satisfies RegistryInput)
            }
          >
            <Form.Item label="Registry 名称" name="name" rules={[{ required: true, message: "请输入 Registry 名称" }]}>
              <Input />
            </Form.Item>
            <Form.Item label="Registry 地址" name="url" rules={[{ required: true, message: "请输入 Registry 地址" }]}>
              <Input placeholder="https://harbor.example.com" />
            </Form.Item>
            <Form.Item label="认证方式" name="authType" rules={[{ required: true, message: "请选择认证方式" }]}>
              <Select
                options={[
                  { label: "匿名访问", value: "NONE" },
                  { label: "Basic Auth", value: "BASIC" },
                  { label: "Bearer Token", value: "TOKEN" },
                ]}
              />
            </Form.Item>
            <Form.Item noStyle dependencies={["authType"]}>
              {() => {
                const authType = form.getFieldValue("authType") as Registry["authType"];
                if (authType === "NONE") {
                  return (
                    <Alert
                      style={{ marginBottom: 16 }}
                      type="info"
                      showIcon
                      message="匿名访问模式"
                      description="适用于公开镜像仓库，不会发送任何认证头。"
                    />
                  );
                }
                return (
                  <Form.Item
                    label="绑定凭证"
                    name="secretId"
                    rules={[{ required: true, message: "请选择 Registry 凭证" }]}
                    extra={
                      authType === "BASIC"
                        ? "Basic 模式下，凭证内容需要保存为 username:password。"
                        : "Token 模式下，凭证内容会作为 Bearer Token 发送。"
                    }
                  >
                    <Select
                      options={secretOptions}
                      placeholder="选择可用于 Registry 认证的凭证"
                      notFoundContent="没有可用凭证，先去凭证页创建。"
                    />
                  </Form.Item>
                );
              }}
            </Form.Item>
            <Form.Item label="说明" name="description">
              <Input.TextArea rows={4} />
            </Form.Item>
          </Form>
        </FormDrawer>

        <DangerConfirm
          open={Boolean(deleteTarget)}
          title="删除 Registry"
          description={`删除后将移除 ${deleteTarget?.name ?? ""} 的仓库浏览入口。若后续服务定义已引用该 Registry，后端联调阶段还需要补引用检查。`}
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
