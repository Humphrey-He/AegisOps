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
import { auditsApi, registriesApi, secretsApi } from "../../lib/api";
import { applyFormErrors, getErrorMessage } from "../../lib/forms";
import { formatDateTime } from "../../lib/format";
import { queryKeys } from "../../lib/queryKeys";
import { auditMatchesResource, buildAuditsPath } from "../../lib/resourceNavigation";
import type { Registry, RegistryInput, Secret } from "../../types/models";

type RegistryFormValues = {
  name: string;
  url: string;
  authType: Registry["authType"];
  secretId?: string;
  description?: string;
};

const authTypeLabelMap: Record<Registry["authType"], string> = {
  NONE: "Anonymous",
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
      description: "请检查绑定凭证是否存在，并确认 Basic 模式是否使用 username:password 格式。",
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
      description: "当前仓库路径或 Tag 不存在，也可能是 Registry 里暂时还没有推送任何镜像。",
    };
  }
  return {
    type: "info" as const,
    title: "返回提示",
    description: message,
  };
}

function truncateDigest(value?: string) {
  if (!value) {
    return "--";
  }
  if (value.length <= 26) {
    return value;
  }
  return `${value.slice(0, 18)}...${value.slice(-8)}`;
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
    queryFn: () => auditsApi.list(),
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
    onSuccess: async (result) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["registries"] }),
        selectedRegistryId
          ? queryClient.invalidateQueries({ queryKey: queryKeys.registry(selectedRegistryId) })
          : Promise.resolve(),
        queryClient.invalidateQueries({ queryKey: queryKeys.audits }),
      ]);
      setPanelError(null);
      setLatestActionText(
        result.taskId
          ? `已完成一次 Registry 连通性检测，任务 ${result.taskId} 可在任务中心继续跟踪。`
          : "已完成一次 Registry 连通性检测。",
      );
      await message.success("Registry 连接测试成功");
    },
    onError: async (error) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["registries"] }),
        selectedRegistryId
          ? queryClient.invalidateQueries({ queryKey: queryKeys.registry(selectedRegistryId) })
          : Promise.resolve(),
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
        label: `${item.name} / ${item.type}`,
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
      .filter((audit) =>
        auditMatchesResource(audit, "registry", selectedRegistry.id, [selectedRegistry.name, selectedRegistry.url]),
      )
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
  const totalRegistries = registriesQuery.data?.length ?? 0;
  const onlineRegistries = (registriesQuery.data ?? []).filter((item) => item.status === "ONLINE").length;
  const authRegistries = (registriesQuery.data ?? []).filter((item) => item.authType !== "NONE").length;
  const testedRegistries = (registriesQuery.data ?? []).filter((item) => Boolean(item.lastTestAt)).length;
  const repositoryCount = repositoriesQuery.data?.repositories.length ?? 0;
  const selectedTagCount = tagsQuery.data?.tags.length ?? 0;
  const manifestDigest = manifestQuery.data?.digest || "--";
  const primaryAction = selectedRegistry?.status === "ONLINE" ? "browse" : "test";

  const summaryItems = [
    {
      label: "Registry 总数",
      value: totalRegistries,
      helper: keyword ? `当前按关键词“${keyword}”过滤` : "当前纳管的镜像仓库数量",
    },
    {
      label: "在线 Registry",
      value: onlineRegistries,
      helper: "最近一次连接测试成功，可继续浏览仓库目录与 Tag",
    },
    {
      label: "需认证仓库",
      value: authRegistries,
      helper: "包含 Basic Auth 和 Bearer Token 两类接入方式",
    },
    {
      label: "已测试仓库",
      value: testedRegistries,
      helper: "至少保留过一条连通性测试记录",
    },
  ];

  if (registriesQuery.isError) {
    return <ErrorState message={registriesQuery.error.message} onRetry={() => void registriesQuery.refetch()} />;
  }

  return (
    <PermissionGuard permission="registries.view" forbiddenPage>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <PageHeader
          title="Registry"
          description="统一管理镜像仓库接入、连通性检测、仓库目录与 Tag 浏览。"
          eyebrow="交付来源 / Registry 工作台"
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
          <div className="workbench-summary-grid">
            {summaryItems.map((item) => (
              <div key={item.label} className="workbench-summary-card">
                <Typography.Text className="workbench-summary-label">{item.label}</Typography.Text>
                <div className="workbench-summary-value">{item.value}</div>
                <div className="workbench-summary-helper">{item.helper}</div>
              </div>
            ))}
          </div>
        </Card>

        <Card className="page-card">
          <div className="page-toolbar">
            <div className="page-toolbar-start">
              <Input.Search
                allowClear
                placeholder="搜索 Registry 名称、地址或说明"
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
                      description="接入镜像仓库后，可以为服务选择镜像版本并查看镜像摘要。"
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
              kicker={selectedRegistry ? "镜像来源上下文" : undefined}
              subtitle={selectedRegistry?.url}
              status={selectedRegistry ? <StatusBadge status={selectedRegistry.status} /> : undefined}
              helper={
                selectedRegistry
                  ? "围绕当前 Registry 集中查看认证方式、连接状态、仓库目录、Tag 与 Manifest 摘要，减少在资源列表、发布动作和审计之间来回切换。"
                  : undefined
              }
              highlights={
                selectedRegistry
                  ? [
                      {
                        label: "目录数量",
                        value: repositoryCount,
                        helper: selectedRegistry.status === "ONLINE" ? "可直接浏览当前可用的 Repositories" : "建议先完成连接测试再继续浏览",
                      },
                      {
                        label: "当前 Tag 数量",
                        value: selectedRepository ? selectedTagCount : "--",
                        helper: selectedRepository ? `当前仓库：${selectedRepository}` : "选择一个 Repository 后显示 Tag 规模",
                      },
                      {
                        label: "Manifest Digest",
                        value: manifestDigest === "--" ? "--" : truncateDigest(manifestDigest),
                        helper: manifestDigest === "--" ? "选中 Tag 后即可查看镜像摘要" : "可作为后续发布引用的版本指纹",
                      },
                    ]
                  : []
              }
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
                        value: repositoryCount,
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
                  <>
                    <div className="resource-action-group">
                      <PermissionGuard permission="registries.test">
                        <Button
                          type={primaryAction === "test" ? "primary" : "default"}
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
                      <Button
                        type={primaryAction === "browse" ? "primary" : "default"}
                        onClick={() => {
                          const firstRepository = repositoriesQuery.data?.repositories?.[0];
                          if (firstRepository) {
                            setSelectedRepository(firstRepository);
                            setSelectedReference("");
                          }
                        }}
                      >
                        浏览仓库
                      </Button>
                    </div>
                    <div className="resource-action-group">
                      <Button
                        onClick={() =>
                          navigate(
                            buildAuditsPath({
                              resourceType: "registry",
                              resourceId: selectedRegistry.id,
                            }),
                          )
                        }
                      >
                        查看审计
                      </Button>
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
                    </div>
                  </>
                ) : undefined
              }
            >
              {latestActionText ? (
                <div className="resource-detail-section resource-callout">
                  <Typography.Text type="secondary">{latestActionText}</Typography.Text>
                </div>
              ) : null}

              {selectedRegistry && !currentErrorMeta && selectedRegistry.status !== "ONLINE" ? (
                <div className="resource-detail-section">
                  <Alert
                    type="warning"
                    showIcon
                    message="当前 Registry 连接状态不稳定"
                    description="建议先完成连接测试，确认 URL、认证方式和网络策略正常，再继续浏览仓库目录与 Tag。"
                  />
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
                  <Space direction="vertical" size={2}>
                    <Typography.Text strong>仓库目录</Typography.Text>
                    <Typography.Text type="secondary">
                      先选定 Repository，再继续下探 Tag 与 Manifest 摘要，方便后续发布直接引用。
                    </Typography.Text>
                  </Space>
                  <Typography.Text type="secondary">
                    {repositoriesQuery.isLoading ? "正在加载目录..." : `${repositoryCount} 个仓库`}
                  </Typography.Text>
                </div>
                <div className="two-col-grid registry-browser-grid" style={{ marginTop: 12 }}>
                  <div className="resource-subpanel registry-browser-card">
                    <Space direction="vertical" size={2}>
                      <Typography.Text strong>Repositories</Typography.Text>
                      <Typography.Text type="secondary">选择一个仓库后，可继续查看当前可用的 Tag 版本。</Typography.Text>
                    </Space>
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
                    <Space direction="vertical" size={2}>
                      <Typography.Text strong>{selectedRepository ? `Tags / ${selectedRepository}` : "Tags"}</Typography.Text>
                      <Typography.Text type="secondary">
                        {selectedRepository ? "选择一个 Tag 后可以进一步查看 Manifest 摘要。" : "先从左侧选择一个 Repository"}
                      </Typography.Text>
                    </Space>
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
                            description="这通常意味着镜像尚未推送，或者 Registry 返回了空目录。"
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
                  <Space direction="vertical" size={2}>
                    <Typography.Text strong>镜像摘要</Typography.Text>
                    <Typography.Text type="secondary">
                      最后一步是核对 Digest、Content-Type 和 Manifest 结构，为发布和回滚做版本确认。
                    </Typography.Text>
                  </Space>
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
                helper="审计记录可以帮助你确认连接测试、认证调整和仓库绑定是否刚刚发生过变更。"
                actionLabel={selectedRegistry ? "查看全部审计" : undefined}
                onActionClick={
                  selectedRegistry
                    ? () =>
                        navigate(
                          buildAuditsPath({
                            resourceType: "registry",
                            resourceId: selectedRegistry.id,
                          }),
                        )
                    : undefined
                }
                items={relatedAudits.map((audit) => ({
                  key: audit.id,
                  title: audit.action,
                  description: audit.summary,
                  meta: `${audit.actor} / ${formatDateTime(audit.createdAt)}`,
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
                  { label: "Anonymous", value: "NONE" },
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
          description={`删除后将移除 ${deleteTarget?.name ?? ""} 的仓库浏览入口。若仍有服务引用该 Registry，删除请求可能被拒绝。`}
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
