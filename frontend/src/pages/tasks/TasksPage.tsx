import { Button, Card, Input, Space } from "antd";
import { useQuery } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { tasksApi } from "../../lib/api";
import { queryKeys } from "../../lib/queryKeys";
import { formatDateTime } from "../../lib/format";
import { DataTable } from "../../components/DataTable";
import { PageHeader } from "../../components/PageHeader";
import { PermissionGuard } from "../../components/PermissionGuard";
import { TaskStatus } from "../../components/TaskStatus";

export function TasksPage() {
  const [keyword, setKeyword] = useState("");
  const navigate = useNavigate();
  const tasksQuery = useQuery({
    queryKey: queryKeys.tasks,
    queryFn: tasksApi.list,
    refetchInterval: 3000,
  });

  const filteredData = useMemo(() => {
    const items = tasksQuery.data ?? [];
    if (!keyword.trim()) {
      return items;
    }
    const normalized = keyword.toLowerCase();
    return items.filter((task) => `${task.type} ${task.target} ${task.initiatedBy}`.toLowerCase().includes(normalized));
  }, [tasksQuery.data, keyword]);

  return (
    <PermissionGuard permission="tasks.view" forbiddenPage>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <PageHeader title="任务中心" description="所有长耗时或高风险动作都在这里汇总追踪。" />

        <Card className="page-card">
          <Input.Search allowClear placeholder="搜索任务类型、目标或发起人" style={{ width: 320 }} onSearch={setKeyword} />
        </Card>

        <Card className="page-card">
          <DataTable
            rowKey="id"
            loading={tasksQuery.isLoading}
            dataSource={filteredData}
            columns={[
              {
                title: "任务",
                dataIndex: "type",
                render: (_, task) => (
                  <Space direction="vertical" size={2}>
                    <span>{task.type}</span>
                    <span style={{ color: "#64748b" }}>{task.summary ?? task.target}</span>
                  </Space>
                ),
              },
              { title: "目标", dataIndex: "target" },
              { title: "发起人", dataIndex: "initiatedBy" },
              { title: "创建时间", dataIndex: "createdAt", render: (value) => formatDateTime(value) },
              { title: "状态", key: "status", render: (_, task) => <TaskStatus task={task} /> },
              {
                title: "操作",
                key: "actions",
                render: (_, task) => (
                  <Button type="link" onClick={() => navigate(`/tasks/${task.id}`)}>
                    查看详情
                  </Button>
                ),
              },
            ]}
          />
        </Card>
      </Space>
    </PermissionGuard>
  );
}
