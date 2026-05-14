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
import { dashboardApi } from "../../lib/api";
import { ApiError } from "../../types/api";
import { queryKeys } from "../../lib/queryKeys";
import { formatDateTime } from "../../lib/format";
import { ErrorState } from "../../components/ErrorState";
import { PageHeader } from "../../components/PageHeader";
import { StatusBadge } from "../../components/StatusBadge";
import { TaskStatus } from "../../components/TaskStatus";

export function DashboardPage() {
  const navigate = useNavigate();
  const summaryQuery = useQuery({
    queryKey: queryKeys.dashboard,
    queryFn: dashboardApi.summary,
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
  const formatAuditDescription = (actor?: string, resourceName?: string, createdAt?: string) =>
    [actor || "-", resourceName || "-", createdAt ? formatDateTime(createdAt) : "-"].join(" · ");

  return (
    <Space direction="vertical" size={16} style={{ width: "100%" }}>
      <PageHeader
        title="工作台"
        description="一期 MVP 的第一屏聚焦资产接入、动作执行和可追踪结果。"
        extra={
          <Space>
            <Button onClick={() => navigate("/assets/hosts")}>新增主机</Button>
            <Button onClick={() => navigate("/docker/nodes")}>查看 Docker 节点</Button>
            <Button type="primary" onClick={() => navigate("/tasks")}>
              进入任务中心
            </Button>
          </Space>
        }
      />

      <Row gutter={[16, 16]}>
        <Col span={6}>
          <Card className="page-card" loading={summaryQuery.isLoading}>
            <Statistic title="用户数" value={data?.userCount ?? 0} />
          </Card>
        </Col>
        <Col span={6}>
          <Card className="page-card" loading={summaryQuery.isLoading}>
            <Statistic title="主机数" value={data?.hostCount ?? 0} />
          </Card>
        </Col>
        <Col span={6}>
          <Card className="page-card" loading={summaryQuery.isLoading}>
            <Statistic title="Docker 节点数" value={data?.dockerNodeCount ?? 0} />
          </Card>
        </Col>
        <Col span={6}>
          <Card className="page-card" loading={summaryQuery.isLoading}>
            <Statistic title="容器数" value={data?.containerCount ?? 0} />
          </Card>
        </Col>
      </Row>

      {(data?.unhealthyResourceCount ?? 0) > 0 ? (
        <Alert
          type="warning"
          showIcon
          message={`当前有 ${data?.unhealthyResourceCount ?? 0} 个异常资源需要关注`}
        />
      ) : null}

      <Row gutter={[16, 16]}>
        <Col span={14}>
          <Card
            className="page-card"
            title="最近任务"
            extra={<Button type="link" onClick={() => navigate("/tasks")}>查看全部</Button>}
            loading={summaryQuery.isLoading}
          >
            <List
              dataSource={data?.recentTasks ?? []}
              renderItem={(task) => (
                <List.Item
                  actions={[
                    <Button key="detail" type="link" onClick={() => navigate(`/tasks/${task.id}`)}>
                      详情
                    </Button>,
                  ]}
                >
                  <List.Item.Meta
                    title={
                      <Space>
                        <Typography.Text strong>{task.type}</Typography.Text>
                        <Typography.Text type="secondary">{task.target}</Typography.Text>
                      </Space>
                    }
                    description={`${task.initiatedBy} · ${formatDateTime(task.createdAt)}`}
                  />
                  <TaskStatus task={task} />
                </List.Item>
              )}
            />
          </Card>
        </Col>
        <Col span={10}>
          <Card
            className="page-card"
            title="最近审计"
            extra={<Button type="link" onClick={() => navigate("/audits")}>查看全部</Button>}
            loading={summaryQuery.isLoading}
          >
            <List
              dataSource={data?.recentAudits ?? []}
              renderItem={(audit) => (
                <List.Item>
                  <List.Item.Meta
                    title={
                      <Space>
                        <Typography.Text strong>{audit.action}</Typography.Text>
                        <StatusBadge status={audit.result} />
                      </Space>
                    }
                    description={formatAuditDescription(audit.actor, audit.resourceName, audit.createdAt)}
                  />
                </List.Item>
              )}
            />
          </Card>
        </Col>
      </Row>
    </Space>
  );
}
