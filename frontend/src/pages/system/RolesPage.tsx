import {
  Alert,
  App as AntApp,
  Button,
  Card,
  Checkbox,
  Col,
  Descriptions,
  Form,
  Input,
  Radio,
  Row,
  Space,
  Tag,
  Typography,
} from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { rolesApi } from "../../lib/api";
import { permissionGroups } from "../../lib/permissions";
import { queryKeys } from "../../lib/queryKeys";
import { applyFormErrors, getErrorMessage } from "../../lib/forms";
import { DataTable } from "../../components/DataTable";
import { FormDrawer } from "../../components/FormDrawer";
import { PageHeader } from "../../components/PageHeader";
import { PermissionActionButton } from "../../components/PermissionActionButton";
import { PermissionGuard } from "../../components/PermissionGuard";
import type { Role, RoleInput } from "../../types/models";

type RoleFormValues = {
  name: string;
  description: string;
  permissions: string[];
};

type ScopeType = "ALL" | "GROUP" | "OWNER" | "RESOURCE_SET";

type ScopeTypeDefinition = {
  key: ScopeType;
  label: string;
  summary: string;
};

type ResourceScopeBlueprint = {
  key: string;
  title: string;
  subtitle: string;
  resourceType: string;
  permissionPrefixes: string[];
  recommendedScope: ScopeType;
  scopeHint: string;
  scopeValueExample: string;
};

const scopeTypeDefinitions: ScopeTypeDefinition[] = [
  { key: "ALL", label: "全量资源", summary: "适合平台管理员、审计管理员等需要全局视图的角色。" },
  { key: "GROUP", label: "资源分组", summary: "适合按环境、业务线、主机组或节点组进行边界划分。" },
  { key: "OWNER", label: "责任归属", summary: "适合限制为本人创建、本人负责或本人所属团队的资源。" },
  { key: "RESOURCE_SET", label: "显式资源集", summary: "适合对少量高价值资源做白名单式绑定。" },
];

const resourceScopeBlueprints: ResourceScopeBlueprint[] = [
  {
    key: "host",
    title: "Host",
    subtitle: "主机资产、SSH 检测与可用性动作",
    resourceType: "Host",
    permissionPrefixes: ["hosts."],
    recommendedScope: "GROUP",
    scopeHint: "建议按环境或主机组约束，例如 production / staging。",
    scopeValueExample: '{"groups":["production","staging"]}',
  },
  {
    key: "service",
    title: "Service",
    subtitle: "服务定义、发布、升级与回滚",
    resourceType: "Service",
    permissionPrefixes: ["services."],
    recommendedScope: "GROUP",
    scopeHint: "建议按业务域或服务组划分，避免跨线操作。",
    scopeValueExample: '{"groups":["edge","payment"]}',
  },
  {
    key: "registry",
    title: "Registry",
    subtitle: "镜像仓库配置、同步与连接测试",
    resourceType: "Registry",
    permissionPrefixes: ["registries."],
    recommendedScope: "RESOURCE_SET",
    scopeHint: "敏感仓库建议显式绑定少量资源，而不是宽泛放开。",
    scopeValueExample: '{"resourceIds":["registry-prod","registry-shared"]}',
  },
  {
    key: "docker-node",
    title: "Docker Node",
    subtitle: "节点连通、容器操作与执行节点边界",
    resourceType: "DockerNode",
    permissionPrefixes: ["docker."],
    recommendedScope: "RESOURCE_SET",
    scopeHint: "运行节点通常更适合按明确节点集收口。",
    scopeValueExample: '{"resourceIds":["docker-prod-01","docker-prod-02"]}',
  },
  {
    key: "nginx-node",
    title: "Nginx Node",
    subtitle: "配置测试、发布、生效与回滚",
    resourceType: "NginxNode",
    permissionPrefixes: ["nginx."],
    recommendedScope: "GROUP",
    scopeHint: "建议按站点域、边缘集群或网关组统一约束。",
    scopeValueExample: '{"groups":["gateway","edge"]}',
  },
];

function matchesResourcePermission(permission: string, prefixes: string[]) {
  return prefixes.some((prefix) => permission.startsWith(prefix));
}

function isHighRiskPermission(permission: string) {
  return /\.(manage|operate|release|rollback|publish|reload|test|rotate|download|create|cancel|retry|dispatch)$/.test(
    permission,
  );
}

export function RolesPage() {
  const { message } = AntApp.useApp();
  const [form] = Form.useForm<RoleFormValues>();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editingRole, setEditingRole] = useState<Role | null>(null);
  const queryClient = useQueryClient();
  const selectedPermissions = Form.useWatch("permissions", form) ?? [];

  const rolesQuery = useQuery({
    queryKey: queryKeys.roles,
    queryFn: rolesApi.list,
  });

  const resourceScopeCards = useMemo(
    () =>
      resourceScopeBlueprints.map((item) => {
        const permissions = selectedPermissions.filter((permission) =>
          matchesResourcePermission(permission, item.permissionPrefixes),
        );
        return {
          ...item,
          permissions,
        };
      }),
    [selectedPermissions],
  );

  const resourceScopeSummary = useMemo(() => {
    const scopedResourceCount = resourceScopeCards.filter((item) => item.permissions.length > 0).length;
    const highRiskCount = selectedPermissions.filter(isHighRiskPermission).length;
    return {
      permissionCount: selectedPermissions.length,
      scopedResourceCount,
      highRiskCount,
    };
  }, [resourceScopeCards, selectedPermissions]);

  const saveMutation = useMutation({
    mutationFn: rolesApi.save,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.roles });
      await queryClient.invalidateQueries({ queryKey: queryKeys.audits });
      await message.success(editingRole ? "角色已更新" : "角色已创建");
      setDrawerOpen(false);
      setEditingRole(null);
      form.resetFields();
    },
    onError: (error) => {
      applyFormErrors(form, error);
      void message.error(getErrorMessage(error));
    },
  });

  return (
    <PermissionGuard permission="roles.view" forbiddenPage>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <PageHeader
          title="角色管理"
          description="角色决定用户进入系统后能看到的菜单和可执行的操作。当前列表只展示你有权查看的角色。"
          extra={
            <PermissionActionButton
              type="primary"
              permission="roles.manage"
              permissionReason="当前账号缺少 roles.manage 权限，无法新增或编辑角色。"
              onClick={() => {
                setEditingRole(null);
                form.resetFields();
                form.setFieldsValue({ permissions: [] });
                setDrawerOpen(true);
              }}
            >
              新增角色
            </PermissionActionButton>
          }
        />

        <Alert
          type="info"
          showIcon
          message="角色同时承载功能权限与资源边界设计"
          description="功能权限已经接入实际授权链路；资源范围策略也已纳入角色抽屉，便于管理员在配置权限时同步审阅未来的资源边界口径。"
        />

        <Card className="page-card">
          <DataTable
            rowKey="id"
            loading={rolesQuery.isLoading}
            dataSource={rolesQuery.data}
            columns={[
              {
                title: "角色",
                dataIndex: "name",
                render: (_, role) => (
                  <Space>
                    <span>{role.name}</span>
                    {role.builtIn ? <Tag color="blue">内置</Tag> : null}
                  </Space>
                ),
              },
              { title: "描述", dataIndex: "description" },
              {
                title: "权限数",
                dataIndex: "permissions",
                render: (permissions: string[]) => permissions.length,
              },
              {
                title: "操作",
                key: "actions",
                render: (_, role) => (
                  <PermissionActionButton
                    size="small"
                    permission="roles.manage"
                    permissionReason="当前账号缺少 roles.manage 权限，无法变更角色权限。"
                    onClick={() => {
                      setEditingRole(role);
                      form.setFieldsValue({
                        name: role.name,
                        description: role.description,
                        permissions: role.permissions,
                      });
                      setDrawerOpen(true);
                    }}
                  >
                    查看/编辑权限
                  </PermissionActionButton>
                ),
              },
            ]}
          />
        </Card>

        <FormDrawer
          open={drawerOpen}
          title={editingRole ? "编辑角色" : "新增角色"}
          width={760}
          onClose={() => {
            setDrawerOpen(false);
            setEditingRole(null);
          }}
          onSubmit={() => form.submit()}
          loading={saveMutation.isPending}
        >
          <Form
            form={form}
            layout="vertical"
            onFinish={(values) =>
              saveMutation.mutate({
                id: editingRole?.id,
                name: values.name,
                description: values.description,
                permissions: values.permissions ?? [],
              } satisfies RoleInput)
            }
          >
            <Form.Item label="角色名称" name="name" rules={[{ required: true, message: "请输入角色名称" }]}>
              <Input />
            </Form.Item>
            <Form.Item label="描述" name="description">
              <Input.TextArea rows={3} />
            </Form.Item>
            <Form.Item label="权限" name="permissions" extra="勾选后，绑定该角色的用户会获得对应菜单和操作权限。">
              <Checkbox.Group style={{ width: "100%" }}>
                <Space direction="vertical" size={16} style={{ width: "100%" }}>
                  {Object.entries(permissionGroups).map(([group, items]) => (
                    <Card key={group} size="small">
                      <Space direction="vertical" size={12} style={{ width: "100%" }}>
                        <Typography.Text strong>{group}</Typography.Text>
                        <Space direction="vertical" size={8} style={{ width: "100%" }}>
                          {items.map((item) => (
                            <Checkbox key={item.key} value={item.key}>
                              {item.label}
                              <Typography.Text type="secondary"> {item.description}</Typography.Text>
                            </Checkbox>
                          ))}
                        </Space>
                      </Space>
                    </Card>
                  ))}
                </Space>
              </Checkbox.Group>
            </Form.Item>

            <Form.Item
              label="资源范围策略"
              extra="这一区域用于同步设计角色的资源边界。当前实时授权仍以后端已接入的功能权限校验为准；这里展示的是后续 scope 接入时将沿用的字段形态与策略口径。"
            >
              <Space direction="vertical" size={16} style={{ width: "100%" }}>
                <Alert
                  type="info"
                  showIcon
                  message="策略结构与角色权限联动展示"
                  description="当角色拥有 Host、Service、Registry、Docker Node、Nginx Node 等资源权限时，可以在这里同步审阅建议的范围类型、范围值形态与对应资源类别。"
                />

                <Row gutter={[12, 12]}>
                  <Col xs={24} md={8}>
                    <Card size="small">
                      <Space direction="vertical" size={4}>
                        <Typography.Text type="secondary">已勾选功能权限</Typography.Text>
                        <Typography.Title level={4} style={{ margin: 0 }}>
                          {resourceScopeSummary.permissionCount}
                        </Typography.Title>
                        <Typography.Text type="secondary">会直接提交到当前角色配置</Typography.Text>
                      </Space>
                    </Card>
                  </Col>
                  <Col xs={24} md={8}>
                    <Card size="small">
                      <Space direction="vertical" size={4}>
                        <Typography.Text type="secondary">涉及资源类别</Typography.Text>
                        <Typography.Title level={4} style={{ margin: 0 }}>
                          {resourceScopeSummary.scopedResourceCount}
                        </Typography.Title>
                        <Typography.Text type="secondary">用于判断哪些资源需要范围约束</Typography.Text>
                      </Space>
                    </Card>
                  </Col>
                  <Col xs={24} md={8}>
                    <Card size="small">
                      <Space direction="vertical" size={4}>
                        <Typography.Text type="secondary">高风险动作权限</Typography.Text>
                        <Typography.Title level={4} style={{ margin: 0 }}>
                          {resourceScopeSummary.highRiskCount}
                        </Typography.Title>
                        <Typography.Text type="secondary">建议优先配置更严格的资源边界</Typography.Text>
                      </Space>
                    </Card>
                  </Col>
                </Row>

                <Row gutter={[16, 16]}>
                  <Col xs={24} xl={10}>
                    <Card size="small" title="字段形态">
                      <Descriptions size="small" column={1} colon={false}>
                        <Descriptions.Item label="resource_type">
                          <Typography.Text code>Host</Typography.Text>
                          <Typography.Text code style={{ marginLeft: 8 }}>
                            Service
                          </Typography.Text>
                          <Typography.Text code style={{ marginLeft: 8 }}>
                            Registry
                          </Typography.Text>
                          <Typography.Text code style={{ marginLeft: 8 }}>
                            DockerNode
                          </Typography.Text>
                          <Typography.Text code style={{ marginLeft: 8 }}>
                            NginxNode
                          </Typography.Text>
                        </Descriptions.Item>
                        <Descriptions.Item label="scope_type">
                          <Typography.Text code>ALL</Typography.Text>
                          <Typography.Text code style={{ marginLeft: 8 }}>
                            GROUP
                          </Typography.Text>
                          <Typography.Text code style={{ marginLeft: 8 }}>
                            OWNER
                          </Typography.Text>
                          <Typography.Text code style={{ marginLeft: 8 }}>
                            RESOURCE_SET
                          </Typography.Text>
                        </Descriptions.Item>
                        <Descriptions.Item label="scope_value_json">
                          用于存放分组、责任归属或资源 ID 集合，便于后续直接落到角色权限范围模型。
                        </Descriptions.Item>
                      </Descriptions>
                    </Card>
                  </Col>
                  <Col xs={24} xl={14}>
                    <Card size="small" title="范围类型口径">
                      <Space direction="vertical" size={12} style={{ width: "100%" }}>
                        {scopeTypeDefinitions.map((item) => (
                          <Card key={item.key} size="small">
                            <Space direction="vertical" size={4} style={{ width: "100%" }}>
                              <Space size={8}>
                                <Tag color="blue">{item.key}</Tag>
                                <Typography.Text strong>{item.label}</Typography.Text>
                              </Space>
                              <Typography.Text type="secondary">{item.summary}</Typography.Text>
                            </Space>
                          </Card>
                        ))}
                      </Space>
                    </Card>
                  </Col>
                </Row>

                <Card size="small" title="资源类别策略">
                  <Row gutter={[16, 16]}>
                    {resourceScopeCards.map((item) => (
                      <Col xs={24} xl={12} key={item.key}>
                        <Card
                          size="small"
                          title={
                            <Space size={8}>
                              <Typography.Text strong>{item.title}</Typography.Text>
                              <Tag>{item.resourceType}</Tag>
                            </Space>
                          }
                          extra={
                            item.permissions.length ? (
                              <Tag color="blue">{item.permissions.length} 项相关权限</Tag>
                            ) : (
                              <Tag>未勾选相关权限</Tag>
                            )
                          }
                        >
                          <Space direction="vertical" size={12} style={{ width: "100%" }}>
                            <Typography.Text type="secondary">{item.subtitle}</Typography.Text>

                            <div>
                              <Typography.Text strong>适用权限</Typography.Text>
                              <div style={{ marginTop: 8 }}>
                                <Space size={[8, 8]} wrap>
                                  {item.permissions.length ? (
                                    item.permissions.map((permission) => <Tag color="blue" key={permission}>{permission}</Tag>)
                                  ) : (
                                    <Tag>当前角色未选择该类资源权限</Tag>
                                  )}
                                </Space>
                              </div>
                            </div>

                            <div>
                              <Typography.Text strong>建议范围类型</Typography.Text>
                              <div style={{ marginTop: 8 }}>
                                <Radio.Group value={item.recommendedScope} disabled optionType="button" buttonStyle="solid">
                                  {scopeTypeDefinitions.map((scope) => (
                                    <Radio.Button key={scope.key} value={scope.key}>
                                      {scope.key}
                                    </Radio.Button>
                                  ))}
                                </Radio.Group>
                              </div>
                            </div>

                            <div>
                              <Typography.Text strong>范围值示例</Typography.Text>
                              <Input.TextArea value={item.scopeValueExample} disabled autoSize={{ minRows: 2, maxRows: 3 }} />
                              <Typography.Text type="secondary">{item.scopeHint}</Typography.Text>
                            </div>
                          </Space>
                        </Card>
                      </Col>
                    ))}
                  </Row>
                </Card>
              </Space>
            </Form.Item>
          </Form>
        </FormDrawer>
      </Space>
    </PermissionGuard>
  );
}
