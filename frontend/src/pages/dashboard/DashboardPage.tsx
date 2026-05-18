import {
  Alert,
  Button,
  Card,
  Col,
  Empty,
  List,
  Row,
  Space,
  Tag,
  Typography,
} from "antd";
import {
  AlertOutlined,
  AuditOutlined,
  ClockCircleOutlined,
  DashboardOutlined,
  DeploymentUnitOutlined,
  WarningOutlined,
} from "@ant-design/icons";
import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";
import { useNavigate } from "react-router-dom";
import { ErrorState } from "../../components/ErrorState";
import { PageHeader } from "../../components/PageHeader";
import { StatusBadge } from "../../components/StatusBadge";
import { TaskStatus } from "../../components/TaskStatus";
import { alertsApi, dashboardApi } from "../../lib/api";
import { formatDateTime } from "../../lib/format";
import { queryKeys } from "../../lib/queryKeys";
import {
  buildAuditsPath,
  buildResourcePath,
  formatAuditActor,
  formatAuditResourceName,
  formatAuditSummary,
  formatTaskResourceName,
} from "../../lib/resourceNavigation";
import { ApiError } from "../../types/api";
import type { AlertEvent, AuditLog, Task } from "../../types/models";

const severityRank: Record<AlertEvent["severity"], number> = {
  CRITICAL: 4,
  WARNING: 3,
  WARN: 3,
  INFO: 1,
};

const severityMeta: Record<AlertEvent["severity"], { color: string; label: string }> = {
  CRITICAL: { color: "red", label: "严重" },
  WARNING: { color: "gold", label: "警告" },
  WARN: { color: "gold", label: "警告" },
  INFO: { color: "blue", label: "信息" },
};

const taskPriorityRank: Record<Task["status"], number> = {
  FAILED: 4,
  CANCELED: 3,
  RUNNING: 2,
  PENDING: 1,
  SUCCESS: 0,
};

const taskToneMeta: Record<Task["status"], { color: string; label: string }> = {
  FAILED: { color: "red", label: "失败" },
  CANCELED: { color: "orange", label: "已取消" },
  RUNNING: { color: "blue", label: "执行中" },
  PENDING: { color: "gold", label: "待执行" },
  SUCCESS: { color: "green", label: "已完成" },
};

const auditRiskPattern = /(delete|remove|destroy|rollback|reload|disable|reset|publish|rotate|grant|revoke)/i;

type ResourceRiskItem = {
  key: string;
  title: string;
  resourceType: string;
  resourceId?: string;
  openEvents: number;
  highestSeverity: AlertEvent["severity"];
  latestTriggeredAt: string;
  summary: string;
  taskId?: string;
};

function byRecentTimeDesc(left?: string, right?: string) {
  return new Date(right ?? 0).getTime() - new Date(left ?? 0).getTime();
}

function severityScore(severity?: AlertEvent["severity"]) {
  return severity ? severityRank[severity] ?? 0 : 0;
}

function isAttentionTask(task: Task) {
  return task.status === "FAILED" || task.status === "CANCELED" || task.status === "RUNNING" || task.status === "PENDING";
}

function isHighRiskAudit(audit: AuditLog) {
  return audit.result === "FAILED" || auditRiskPattern.test(`${audit.action} ${audit.summary}`);
}

function deriveResourceRisks(events: AlertEvent[]): ResourceRiskItem[] {
  const grouped = new Map<string, ResourceRiskItem>();

  events.forEach((event) => {
    const key = `${event.resourceType}:${event.resourceId || event.resourceName}`;
    const current = grouped.get(key);
    if (!current) {
      grouped.set(key, {
        key,
        title: event.resourceName || event.resourceId || event.resourceType,
        resourceType: event.resourceType,
        resourceId: event.resourceId,
        openEvents: 1,
        highestSeverity: event.severity,
        latestTriggeredAt: event.lastTriggeredAt,
        summary: event.summary || event.detail || event.eventType,
        taskId: event.taskId,
      });
      return;
    }

    current.openEvents += 1;
    if (severityScore(event.severity) > severityScore(current.highestSeverity)) {
      current.highestSeverity = event.severity;
      current.summary = event.summary || event.detail || event.eventType;
      current.taskId = event.taskId || current.taskId;
    }
    if (new Date(event.lastTriggeredAt).getTime() > new Date(current.latestTriggeredAt).getTime()) {
      current.latestTriggeredAt = event.lastTriggeredAt;
      current.summary = event.summary || event.detail || event.eventType;
      current.taskId = event.taskId || current.taskId;
    }
  });

  return Array.from(grouped.values()).sort((left, right) => {
    const severityDiff = severityScore(right.highestSeverity) - severityScore(left.highestSeverity);
    if (severityDiff !== 0) {
      return severityDiff;
    }
    if (right.openEvents !== left.openEvents) {
      return right.openEvents - left.openEvents;
    }
    return byRecentTimeDesc(left.latestTriggeredAt, right.latestTriggeredAt);
  });
}

function buildOverallTone(openAlertCount: number, failedTaskCount: number, unhealthyResourceCount: number) {
  if (openAlertCount > 0 || failedTaskCount > 0 || unhealthyResourceCount > 0) {
    return {
      label: "需要立即处理",
      type: "warning" as const,
      helper: "首页已将待处理告警、失败任务和异常资源提前到首屏。",
    };
  }

  return {
    label: "总体稳定",
    type: "success" as const,
    helper: "当前未发现待处理风险，适合继续跟进执行中的变更与审计轨迹。",
  };
}

function renderSectionHeader(title: string, helper: string, extra?: React.ReactNode) {
  return (
    <div className="page-toolbar">
      <Space direction="vertical" size={2}>
        <Typography.Text strong>{title}</Typography.Text>
        <Typography.Text type="secondary">{helper}</Typography.Text>
      </Space>
      {extra}
    </div>
  );
}

function renderEmpty(title: string, description: string) {
  return (
    <Empty
      image={Empty.PRESENTED_IMAGE_SIMPLE}
      description={
        <Space direction="vertical" size={4}>
          <Typography.Text strong>{title}</Typography.Text>
          <Typography.Text type="secondary">{description}</Typography.Text>
        </Space>
      }
    />
  );
}

export function DashboardPage() {
  const navigate = useNavigate();
  const summaryQuery = useQuery({
    queryKey: queryKeys.dashboard,
    queryFn: dashboardApi.summary,
  });
  const alertEventsQuery = useQuery({
    queryKey: [...queryKeys.alertEvents, "dashboard-open"],
    queryFn: () => alertsApi.listEvents({ status: "OPEN" }),
  });

  if (summaryQuery.isError) {
    return (
      <ErrorState
        message={summaryQuery.error.message}
        traceId={summaryQuery.error instanceof ApiError ? summaryQuery.error.traceId : undefined}
        onRetry={() => void summaryQuery.refetch()}
      />
    );
  }

  const data = summaryQuery.data;
  const recentTasks = data?.recentTasks ?? [];
  const recentAudits = data?.recentAudits ?? [];

  const prioritizedAlertEvents = useMemo(() => {
    return [...(alertEventsQuery.data ?? [])].sort((left, right) => {
      const severityDiff = severityScore(right.severity) - severityScore(left.severity);
      if (severityDiff !== 0) {
        return severityDiff;
      }
      return byRecentTimeDesc(left.lastTriggeredAt, right.lastTriggeredAt);
    });
  }, [alertEventsQuery.data]);

  const recentAlertFeed = useMemo(() => {
    return [...(alertEventsQuery.data ?? [])]
      .sort((left, right) => byRecentTimeDesc(left.lastTriggeredAt, right.lastTriggeredAt))
      .slice(0, 5);
  }, [alertEventsQuery.data]);

  const attentionTasks = useMemo(() => {
    return [...recentTasks]
      .filter(isAttentionTask)
      .sort((left, right) => {
        const statusDiff = taskPriorityRank[right.status] - taskPriorityRank[left.status];
        if (statusDiff !== 0) {
          return statusDiff;
        }
        return byRecentTimeDesc(left.createdAt, right.createdAt);
      });
  }, [recentTasks]);

  const abnormalResources = useMemo(() => deriveResourceRisks(prioritizedAlertEvents), [prioritizedAlertEvents]);

  const auditFeed = useMemo(() => {
    return [...recentAudits].sort((left, right) => {
      const riskDiff = Number(isHighRiskAudit(right)) - Number(isHighRiskAudit(left));
      if (riskDiff !== 0) {
        return riskDiff;
      }
      return byRecentTimeDesc(left.createdAt, right.createdAt);
    });
  }, [recentAudits]);

  const openAlertCount = data?.openAlertCount ?? prioritizedAlertEvents.length;
  const unhealthyResourceCount = data?.unhealthyResourceCount ?? abnormalResources.length;
  const failedTaskCount = attentionTasks.filter((task) => task.status === "FAILED" || task.status === "CANCELED").length;
  const executingTaskCount = attentionTasks.filter((task) => task.status === "RUNNING" || task.status === "PENDING").length;
  const highRiskAuditCount = auditFeed.filter(isHighRiskAudit).length;
  const highestSeverity = prioritizedAlertEvents[0]?.severity;
  const overallTone = buildOverallTone(openAlertCount, failedTaskCount, unhealthyResourceCount);

  const topAttentionMessage =
    openAlertCount > 0
      ? `当前有 ${openAlertCount} 条待处理告警${highestSeverity ? `，最高等级为${severityMeta[highestSeverity].label}` : ""}。`
      : failedTaskCount > 0
        ? `最近任务中有 ${failedTaskCount} 条失败或已取消，建议优先核查执行结果与关联资源。`
        : executingTaskCount > 0
          ? `当前仍有 ${executingTaskCount} 条任务正在执行或排队，可在任务中心持续跟进。`
          : "当前首页未发现待处理风险，适合回看最新审计与变更轨迹。";

  const situationTiles = [
    {
      key: "overall",
      title: "总体态势",
      value: overallTone.label,
      helper: overallTone.helper,
      accent: overallTone.type === "success" ? "#166534" : "#9a3412",
      background: overallTone.type === "success" ? "#f0fdf4" : "#fff7ed",
      onClick: () => navigate("/alerts/events"),
    },
    {
      key: "risk",
      title: "优先风险",
      value: `${openAlertCount} 条`,
      helper: highestSeverity
        ? `待处理告警已按严重度与触发时间排序，最高为${severityMeta[highestSeverity].label}。`
        : "当前没有待处理告警，可继续关注最新变更与执行轨迹。",
      accent: "#b91c1c",
      background: "#fef2f2",
      onClick: () => navigate("/alerts/events"),
    },
    {
      key: "tasks",
      title: "执行焦点",
      value: `${failedTaskCount} 失败 / ${executingTaskCount} 执行中`,
      helper:
        failedTaskCount > 0
          ? "失败任务已排到前列，方便先定位阻塞项，再回看执行中的任务。"
          : "当前没有失败任务，主要关注执行中与待执行链路。",
      accent: failedTaskCount > 0 ? "#b91c1c" : "#1d4ed8",
      background: failedTaskCount > 0 ? "#fef2f2" : "#eff6ff",
      onClick: () => navigate("/tasks"),
    },
    {
      key: "resources",
      title: "异常资源",
      value: `${unhealthyResourceCount} 个`,
      helper:
        abnormalResources[0]?.title
          ? `当前最需要关注 ${abnormalResources[0].title}，首页已给出对应告警与入口。`
          : "暂无从告警中识别出的异常资源，资源列表保持稳定。",
      accent: unhealthyResourceCount > 0 ? "#b91c1c" : "#166534",
      background: unhealthyResourceCount > 0 ? "#fef2f2" : "#f0fdf4",
      onClick: () => navigate("/assets/hosts"),
    },
    {
      key: "audits",
      title: "高风险审计",
      value: `${highRiskAuditCount} 条`,
      helper:
        highRiskAuditCount > 0
          ? "失败审计和高风险动作被提前排序，方便快速核查变更影响。"
          : "最近审计以常规轨迹为主，没有突出的失败或高风险动作。",
      accent: highRiskAuditCount > 0 ? "#7c2d12" : "#334155",
      background: highRiskAuditCount > 0 ? "#fff7ed" : "#f8fafc",
      onClick: () => navigate("/audits"),
    },
  ];

  return (
    <Space direction="vertical" size={16} style={{ width: "100%" }}>
      <PageHeader
        title="工作台"
        description="首页按异常优先重排，先看风险和阻塞，再回看资源状态、执行链路与审计轨迹。"
        extra={
          <Space wrap>
            <Button type="primary" onClick={() => navigate("/alerts/events")}>
              告警中心
            </Button>
            <Button onClick={() => navigate("/tasks")}>任务中心</Button>
            <Button onClick={() => navigate("/assets/hosts")}>主机</Button>
            <Button onClick={() => navigate("/audits")}>审计中心</Button>
          </Space>
        }
      />

      <Alert
        type={overallTone.type}
        showIcon
        icon={overallTone.type === "success" ? <DashboardOutlined /> : <WarningOutlined />}
        message={topAttentionMessage}
        description="当前首页将优先风险、失败/执行中任务、异常资源与最近审计拆成独立工作区，减少跨页跳转后的上下文丢失。"
      />

      <Card className="page-card" loading={summaryQuery.isLoading || alertEventsQuery.isLoading}>
        <Space direction="vertical" size={16} style={{ width: "100%" }}>
          {renderSectionHeader(
            "总体态势",
            "先判断系统是否稳定，再决定是优先处理风险、跟进执行链路，还是回看最近变更。",
          )}

          <Row gutter={[12, 12]}>
            {situationTiles.map((tile) => (
              <Col key={tile.key} flex="1 1 220px">
                <button
                  type="button"
                  onClick={tile.onClick}
                  style={{
                    width: "100%",
                    textAlign: "left",
                    border: "1px solid #e5e7eb",
                    borderRadius: 8,
                    background: tile.background,
                    padding: 16,
                    cursor: "pointer",
                  }}
                >
                  <Space direction="vertical" size={10} style={{ width: "100%" }}>
                    <Typography.Text type="secondary">{tile.title}</Typography.Text>
                    <Typography.Title level={4} style={{ margin: 0, color: tile.accent }}>
                      {tile.value}
                    </Typography.Title>
                    <Typography.Text>{tile.helper}</Typography.Text>
                  </Space>
                </button>
              </Col>
            ))}
          </Row>
        </Space>
      </Card>

      <Row gutter={[16, 16]} align="top">
        <Col span={16}>
          <Space direction="vertical" size={16} style={{ width: "100%" }}>
            <Card className="page-card" loading={alertEventsQuery.isLoading}>
              <Space direction="vertical" size={16} style={{ width: "100%" }}>
                {renderSectionHeader(
                  "优先风险",
                  "将待处理告警按严重度和最近触发时间排序，优先暴露最需要人来处理的事件。",
                  <Button type="link" onClick={() => navigate("/alerts/events")}>
                    查看全部
                  </Button>,
                )}

                {alertEventsQuery.isError
                  ? (
                    <Alert
                      type="warning"
                      showIcon
                      message="告警数据暂时不可用"
                      description={alertEventsQuery.error.message}
                    />
                    )
                  : (
                    <List
                      dataSource={prioritizedAlertEvents.slice(0, 4)}
                      locale={{
                        emptyText: renderEmpty("暂无待处理告警", "当前没有需要人工接手的告警事件。"),
                      }}
                      renderItem={(event) => {
                        const resourcePath = buildResourcePath(event.resourceType, event.resourceId);
                        const severity = severityMeta[event.severity];
                        return (
                          <List.Item
                            actions={[
                              event.taskId ? (
                                <Button key="task" type="link" onClick={() => navigate(`/tasks/${event.taskId}`)}>
                                  任务
                                </Button>
                              ) : null,
                              resourcePath ? (
                                <Button key="resource" type="link" onClick={() => navigate(resourcePath)}>
                                  资源
                                </Button>
                              ) : null,
                              <Button key="alerts" type="link" onClick={() => navigate("/alerts/events")}>
                                处理
                              </Button>,
                            ].filter(Boolean)}
                          >
                            <List.Item.Meta
                              title={
                                <Space size={[8, 8]} wrap>
                                  <Typography.Text strong>{event.summary || event.eventType}</Typography.Text>
                                  <Tag color={severity.color}>{severity.label}</Tag>
                                  <StatusBadge status={event.status} />
                                </Space>
                              }
                              description={
                                <Space direction="vertical" size={4}>
                                  <Typography.Text type="secondary">
                                    {`${event.resourceName || event.resourceId || "--"} · 最近触发 ${formatDateTime(event.lastTriggeredAt)}`}
                                  </Typography.Text>
                                  <Typography.Text>{event.detail || "请优先核查关联资源、任务结果和最近审计轨迹。"}</Typography.Text>
                                  <Typography.Text type="secondary">
                                    {`首次触发 ${formatDateTime(event.firstTriggeredAt)}`}
                                  </Typography.Text>
                                </Space>
                              }
                            />
                          </List.Item>
                        );
                      }}
                    />
                    )}
              </Space>
            </Card>

            <Card className="page-card" loading={summaryQuery.isLoading}>
              <Space direction="vertical" size={16} style={{ width: "100%" }}>
                {renderSectionHeader(
                  "失败 / 执行中任务",
                  "先暴露失败、已取消和正在执行的链路，帮助你判断哪里卡住、哪里仍在推进。",
                  <Button type="link" onClick={() => navigate("/tasks")}>
                    查看全部
                  </Button>,
                )}

                <List
                  dataSource={attentionTasks.slice(0, 6)}
                  locale={{
                    emptyText: renderEmpty("近期任务平稳", "最近任务里没有失败、取消或仍在执行的链路。"),
                  }}
                  renderItem={(task) => {
                    const resourcePath = buildResourcePath(task.resourceType, task.resourceId);
                    const tone = taskToneMeta[task.status];
                    return (
                      <List.Item
                        actions={[
                          <Button key="detail" type="link" onClick={() => navigate(`/tasks/${task.id}`)}>
                            详情
                          </Button>,
                          resourcePath ? (
                            <Button key="resource" type="link" onClick={() => navigate(resourcePath)}>
                              资源
                            </Button>
                          ) : null,
                        ].filter(Boolean)}
                      >
                        <List.Item.Meta
                          title={
                            <Space align="start" size={12} wrap>
                              <Typography.Text strong>{task.type}</Typography.Text>
                              <Tag color={tone.color}>{tone.label}</Tag>
                              <TaskStatus task={task} />
                            </Space>
                          }
                          description={
                            <Space direction="vertical" size={4}>
                              <Typography.Text>{formatTaskResourceName(task)}</Typography.Text>
                              <Typography.Text type="secondary">
                                {`${task.initiatedBy || "--"} · 创建于 ${formatDateTime(task.createdAt)}`}
                              </Typography.Text>
                              <Typography.Text>{task.summary || "进入任务详情可查看步骤、日志和执行结果。"}</Typography.Text>
                            </Space>
                          }
                        />
                      </List.Item>
                    );
                  }}
                />
              </Space>
            </Card>

            <Card className="page-card" loading={alertEventsQuery.isLoading}>
              <Space direction="vertical" size={16} style={{ width: "100%" }}>
                {renderSectionHeader(
                  "异常资源",
                  "基于待处理告警提取受影响对象，让排查从资源入口开始，而不是只停留在事件列表。",
                  <Button type="link" onClick={() => navigate("/assets/hosts")}>
                    查看资源
                  </Button>,
                )}

                {alertEventsQuery.isError ? (
                  <Alert
                    type="warning"
                    showIcon
                    message="异常资源视图暂时不可用"
                    description="待处理告警读取失败，暂时无法从首页提取受影响资源。"
                  />
                ) : abnormalResources.length ? (
                  <List
                    dataSource={abnormalResources.slice(0, 5)}
                    renderItem={(resource) => {
                      const resourcePath = buildResourcePath(resource.resourceType, resource.resourceId);
                      const severity = severityMeta[resource.highestSeverity];
                      return (
                        <List.Item
                          actions={[
                            resource.taskId ? (
                              <Button key="task" type="link" onClick={() => navigate(`/tasks/${resource.taskId}`)}>
                                相关任务
                              </Button>
                            ) : null,
                            resourcePath ? (
                              <Button key="resource" type="link" onClick={() => navigate(resourcePath)}>
                                进入资源
                              </Button>
                            ) : null,
                            <Button key="alerts" type="link" onClick={() => navigate("/alerts/events")}>
                              告警
                            </Button>,
                          ].filter(Boolean)}
                        >
                          <List.Item.Meta
                            title={
                              <Space size={[8, 8]} wrap>
                                <Typography.Text strong>{resource.title}</Typography.Text>
                                <Tag color={severity.color}>{severity.label}</Tag>
                                <Typography.Text type="secondary">{`${resource.openEvents} 条待处理事件`}</Typography.Text>
                              </Space>
                            }
                            description={
                              <Space direction="vertical" size={4}>
                                <Typography.Text>{resource.summary}</Typography.Text>
                                <Typography.Text type="secondary">
                                  {`最近触发 ${formatDateTime(resource.latestTriggeredAt)}`}
                                </Typography.Text>
                              </Space>
                            }
                          />
                        </List.Item>
                      );
                    }}
                  />
                ) : (
                  renderEmpty(
                    "暂无异常资源",
                    unhealthyResourceCount > 0
                      ? `统计显示有 ${unhealthyResourceCount} 个异常资源，但当前告警列表未返回具体对象。`
                      : "当前没有从待处理告警中识别出的异常资源对象。",
                  )
                )}
              </Space>
            </Card>
          </Space>
        </Col>

        <Col span={8}>
          <Space direction="vertical" size={16} style={{ width: "100%" }}>
            <Card className="page-card" loading={alertEventsQuery.isLoading}>
              <Space direction="vertical" size={16} style={{ width: "100%" }}>
                {renderSectionHeader(
                  "最近告警",
                  "按最近触发时间回看告警节奏，避免只看最高严重度而忽略新近波动。",
                  <Button type="link" onClick={() => navigate("/alerts/events")}>
                    告警中心
                  </Button>,
                )}

                {alertEventsQuery.isError ? (
                  <Alert type="warning" showIcon message="最近告警读取失败" description={alertEventsQuery.error.message} />
                ) : (
                  <List
                    size="small"
                    dataSource={recentAlertFeed}
                    locale={{
                      emptyText: renderEmpty("告警流暂时安静", "最近没有新的待处理告警进入工作台。"),
                    }}
                    renderItem={(event) => {
                      const severity = severityMeta[event.severity];
                      return (
                        <List.Item
                          actions={[
                            <Button key="view" type="link" onClick={() => navigate("/alerts/events")}>
                              查看
                            </Button>,
                          ]}
                        >
                          <List.Item.Meta
                            title={
                              <Space size={[8, 8]} wrap>
                                <Typography.Text strong>{event.summary || event.eventType}</Typography.Text>
                                <Tag color={severity.color}>{severity.label}</Tag>
                              </Space>
                            }
                            description={
                              <Space direction="vertical" size={2}>
                                <Typography.Text type="secondary">{event.resourceName || event.resourceId || "--"}</Typography.Text>
                                <Typography.Text type="secondary">{formatDateTime(event.lastTriggeredAt)}</Typography.Text>
                              </Space>
                            }
                          />
                        </List.Item>
                      );
                    }}
                  />
                )}
              </Space>
            </Card>

            <Card className="page-card" loading={summaryQuery.isLoading}>
              <Space direction="vertical" size={16} style={{ width: "100%" }}>
                {renderSectionHeader(
                  "最近审计",
                  "失败动作和高风险变更会提前排序，帮助你判断当前风险是否来自刚发生的操作。",
                  <Button type="link" onClick={() => navigate("/audits")}>
                    审计中心
                  </Button>,
                )}

                <List
                  size="small"
                  dataSource={auditFeed.slice(0, 6)}
                  locale={{
                    emptyText: renderEmpty("暂无最近审计", "新的操作和系统动作会在这里形成可追溯记录。"),
                  }}
                  renderItem={(audit) => {
                    const auditPath = buildAuditsPath({
                      resourceType: audit.resourceType,
                      resourceId: audit.resourceId,
                    });
                    const resourcePath = buildResourcePath(audit.resourceType, audit.resourceId);
                    const risky = isHighRiskAudit(audit);
                    return (
                      <List.Item
                        actions={[
                          <Button key="audit" type="link" onClick={() => navigate(auditPath)}>
                            审计
                          </Button>,
                          resourcePath ? (
                            <Button key="resource" type="link" onClick={() => navigate(resourcePath)}>
                              资源
                            </Button>
                          ) : null,
                        ].filter(Boolean)}
                      >
                        <List.Item.Meta
                          title={
                            <Space size={[8, 8]} wrap>
                              <Typography.Text strong>{audit.action}</Typography.Text>
                              {risky ? <Tag color="volcano">高风险</Tag> : null}
                              <StatusBadge status={audit.result} />
                            </Space>
                          }
                          description={
                            <Space direction="vertical" size={2}>
                              <Typography.Text type="secondary">
                                {`${formatAuditActor(audit.actor)} · ${formatAuditResourceName(audit)}`}
                              </Typography.Text>
                              <Typography.Text>{formatAuditSummary(audit.summary, audit.action)}</Typography.Text>
                              <Typography.Text type="secondary">{formatDateTime(audit.createdAt)}</Typography.Text>
                            </Space>
                          }
                        />
                      </List.Item>
                    );
                  }}
                />
              </Space>
            </Card>
          </Space>
        </Col>
      </Row>
    </Space>
  );
}
