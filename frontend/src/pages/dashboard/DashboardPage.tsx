import {
  Alert,
  Button,
  Card,
  Col,
  List,
  Row,
  Space,
  Statistic,
  Typography,
} from "antd";
import { useQuery } from "@tanstack/react-query";
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
  const recentAlertEvents = (alertEventsQuery.data ?? []).slice(0, 5);
  const runningTaskCount = recentTasks.filter(
    (task) => task.status === "RUNNING" || task.status === "PENDING",
  ).length;

  const stats = [
    {
      title: "主机",
      value: data?.hostCount ?? 0,
      onClick: () => navigate("/assets/hosts"),
    },
    {
      title: "Docker 节点",
      value: data?.dockerNodeCount ?? 0,
      onClick: () => navigate("/docker/nodes"),
    },
    {
      title: "运行中任务",
      value: runningTaskCount,
      onClick: () => navigate("/tasks"),
    },
    {
      title: "未处理告警",
      value: recentAlertEvents.length,
      onClick: () => navigate("/alerts/events"),
    },
  ];

  return (
    <Space direction="vertical" size={16} style={{ width: "100%" }}>
      <PageHeader
        title="工作台"
        description="从首页直接看清资源状态、近期动作和可追踪的执行结果。"
        extra={
          <Space wrap>
            <Button onClick={() => navigate("/assets/hosts")}>主机</Button>
            <Button onClick={() => navigate("/docker/nodes")}>Docker 节点</Button>
            <Button onClick={() => navigate("/tasks")}>任务中心</Button>
            <Button type="primary" onClick={() => navigate("/audits")}>
              审计中心
            </Button>
          </Space>
        }
      />

      <Row gutter={[16, 16]}>
        {stats.map((stat) => (
          <Col key={stat.title} span={6}>
            <Card
              className="page-card"
              hoverable
              loading={summaryQuery.isLoading}
              onClick={stat.onClick}
              style={{ cursor: "pointer" }}
            >
              <Statistic title={stat.title} value={stat.value} />
            </Card>
          </Col>
        ))}
      </Row>

      {(data?.unhealthyResourceCount ?? 0) > 0 ? (
        <Alert
          type="warning"
          showIcon
          message={`当前有 ${data?.unhealthyResourceCount ?? 0} 个异常资源需要关注`}
          description="建议优先检查最近任务和审计记录，确认异常是否仍在持续。"
        />
      ) : null}

      <Row gutter={[16, 16]}>
        <Col span={14}>
          <Card
            className="page-card"
            title="最近任务"
            extra={
              <Button type="link" onClick={() => navigate("/tasks")}>
                查看全部
              </Button>
            }
            loading={summaryQuery.isLoading}
          >
            <List
              dataSource={recentTasks}
              locale={{ emptyText: "暂无最近任务" }}
              renderItem={(task) => {
                const resourcePath = buildResourcePath(task.resourceType, task.resourceId);
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
                        <Space>
                          <Typography.Text strong>{task.type}</Typography.Text>
                          <TaskStatus task={task} />
                        </Space>
                      }
                      description={
                        <Space direction="vertical" size={2}>
                          <Typography.Text type="secondary">
                            {formatTaskResourceName(task)}
                          </Typography.Text>
                          <Typography.Text type="secondary">
                            {`${task.initiatedBy || "--"} · ${formatDateTime(task.createdAt)}`}
                          </Typography.Text>
                          <Typography.Text>{task.summary || "--"}</Typography.Text>
                        </Space>
                      }
                    />
                  </List.Item>
                );
              }}
            />
          </Card>
        </Col>
        <Col span={10}>
          <Card
            className="page-card"
            title="待处理告警"
            extra={
              <Button type="link" onClick={() => navigate("/alerts/events")}>
                查看全部
              </Button>
            }
            loading={alertEventsQuery.isLoading}
            style={{ marginBottom: 16 }}
          >
            <List
              dataSource={recentAlertEvents}
              locale={{ emptyText: "暂无待处理告警" }}
              renderItem={(event) => {
                const resourcePath = buildResourcePath(event.resourceType, event.resourceId);
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
                    ].filter(Boolean)}
                  >
                    <List.Item.Meta
                      title={
                        <Space>
                          <Typography.Text strong>{event.summary || event.eventType}</Typography.Text>
                          <StatusBadge status={event.status} />
                        </Space>
                      }
                      description={
                        <Space direction="vertical" size={2}>
                          <Typography.Text type="secondary">
                            {`${event.resourceName || event.resourceId || "--"} · ${formatDateTime(event.lastTriggeredAt)}`}
                          </Typography.Text>
                          <Typography.Text>{event.detail || "--"}</Typography.Text>
                        </Space>
                      }
                    />
                  </List.Item>
                );
              }}
            />
          </Card>

          <Card
            className="page-card"
            title="最近审计"
            extra={
              <Button type="link" onClick={() => navigate("/audits")}>
                查看全部
              </Button>
            }
            loading={summaryQuery.isLoading}
          >
            <List
              dataSource={recentAudits}
              locale={{ emptyText: "暂无最近审计" }}
              renderItem={(audit) => {
                const auditPath = buildAuditsPath({
                  resourceType: audit.resourceType,
                  resourceId: audit.resourceId,
                });
                const resourcePath = buildResourcePath(audit.resourceType, audit.resourceId);
                return (
                  <List.Item
                    actions={[
                      <Button key="audit" type="link" onClick={() => navigate(auditPath)}>
                        相关审计
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
                        <Space>
                          <Typography.Text strong>{audit.action}</Typography.Text>
                          <StatusBadge status={audit.result} />
                        </Space>
                      }
                      description={
                        <Space direction="vertical" size={2}>
                          <Typography.Text type="secondary">
                            {`${formatAuditActor(audit.actor)} · ${formatAuditResourceName(audit)} · ${formatDateTime(audit.createdAt)}`}
                          </Typography.Text>
                          <Typography.Text>{formatAuditSummary(audit.summary, audit.action)}</Typography.Text>
                        </Space>
                      }
                    />
                  </List.Item>
                );
              }}
            />
          </Card>
        </Col>
      </Row>
    </Space>
  );
}
