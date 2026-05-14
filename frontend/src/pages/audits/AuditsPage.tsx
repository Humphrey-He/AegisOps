import { Button, Card, Input, Space } from "antd";
import { useQuery } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { auditsApi } from "../../lib/api";
import { queryKeys } from "../../lib/queryKeys";
import { formatDateTime } from "../../lib/format";
import { AuditDrawer } from "../../components/AuditDrawer";
import { DataTable } from "../../components/DataTable";
import { PageHeader } from "../../components/PageHeader";
import { PermissionGuard } from "../../components/PermissionGuard";
import { StatusBadge } from "../../components/StatusBadge";
import type { AuditLog } from "../../types/models";

export function AuditsPage() {
  const [keyword, setKeyword] = useState("");
  const [selectedAudit, setSelectedAudit] = useState<AuditLog | null>(null);
  const auditsQuery = useQuery({
    queryKey: queryKeys.audits,
    queryFn: auditsApi.list,
  });

  const filteredData = useMemo(() => {
    const items = auditsQuery.data ?? [];
    if (!keyword.trim()) {
      return items;
    }
    const normalized = keyword.toLowerCase();
    return items.filter((audit) =>
      `${audit.actor} ${audit.action} ${audit.resourceName} ${audit.summary}`.toLowerCase().includes(normalized),
    );
  }, [auditsQuery.data, keyword]);

  return (
    <PermissionGuard permission="audits.view" forbiddenPage>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <PageHeader title="审计日志" description="关键登录、凭证、主机与 Docker 操作都可以从这里追踪。" />

        <Card className="page-card">
          <Input.Search allowClear placeholder="搜索操作者、动作、资源" style={{ width: 320 }} onSearch={setKeyword} />
        </Card>

        <Card className="page-card">
          <DataTable
            rowKey="id"
            loading={auditsQuery.isLoading}
            dataSource={filteredData}
            columns={[
              { title: "操作者", dataIndex: "actor" },
              { title: "动作", dataIndex: "action" },
              { title: "资源", dataIndex: "resourceName" },
              { title: "结果", dataIndex: "result", render: (value) => <StatusBadge status={value} /> },
              { title: "Trace ID", dataIndex: "traceId" },
              { title: "时间", dataIndex: "createdAt", render: (value) => formatDateTime(value) },
              {
                title: "操作",
                key: "actions",
                render: (_, audit) => (
                  <Button type="link" onClick={() => setSelectedAudit(audit)}>
                    详情
                  </Button>
                ),
              },
            ]}
          />
        </Card>

        <AuditDrawer open={Boolean(selectedAudit)} audit={selectedAudit} onClose={() => setSelectedAudit(null)} />
      </Space>
    </PermissionGuard>
  );
}
