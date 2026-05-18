import { Button, Card, Input, Select, Space, Tag } from "antd";
import { useQuery } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { AuditDrawer } from "../../components/AuditDrawer";
import { DataTable } from "../../components/DataTable";
import { PageHeader } from "../../components/PageHeader";
import { PermissionGuard } from "../../components/PermissionGuard";
import { StatusBadge } from "../../components/StatusBadge";
import { auditsApi } from "../../lib/api";
import { formatDateTime } from "../../lib/format";
import {
  buildResourcePath,
  formatAuditActor,
  formatAuditResourceName,
  formatAuditSummary,
  getResourceTypeLabel,
  normalizeResourceType,
} from "../../lib/resourceNavigation";
import type { AuditLog } from "../../types/models";

const resultOptions: Array<{ label: string; value: AuditLog["result"] }> = [
  { label: "成功", value: "SUCCESS" },
  { label: "失败", value: "FAILED" },
];

export function AuditsPage() {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const [selectedAudit, setSelectedAudit] = useState<AuditLog | null>(null);
  const keyword = searchParams.get("keyword") ?? "";
  const resultFilter = searchParams.get("result") ?? "";
  const resourceTypeFilter = searchParams.get("resourceType") ?? "";
  const resourceIdFilter = searchParams.get("resourceId") ?? "";

  const auditsQuery = useQuery({
    queryKey: ["audits", resultFilter, resourceTypeFilter, resourceIdFilter],
    queryFn: () =>
      auditsApi.list({
        result: resultFilter || undefined,
        resourceType: resourceTypeFilter || undefined,
        resourceId: resourceIdFilter || undefined,
      }),
  });

  const resourceTypeOptions = useMemo(() => {
    const items = new Map<string, string>();
    for (const audit of auditsQuery.data ?? []) {
      const normalized = normalizeResourceType(audit.resourceType);
      if (!normalized) {
        continue;
      }
      items.set(normalized, audit.resourceType);
    }
    return Array.from(items.entries()).map(([value, original]) => ({
      label: getResourceTypeLabel(original),
      value,
    }));
  }, [auditsQuery.data]);

  const filteredData = useMemo(() => {
    let items = auditsQuery.data ?? [];

    if (resultFilter) {
      items = items.filter((audit) => audit.result === resultFilter);
    }
    if (resourceTypeFilter) {
      items = items.filter(
        (audit) => normalizeResourceType(audit.resourceType) === normalizeResourceType(resourceTypeFilter),
      );
    }
    if (resourceIdFilter) {
      const normalizedResourceId = resourceIdFilter.toLowerCase();
      items = items.filter((audit) =>
        `${audit.resourceId ?? ""} ${audit.resourceName} ${audit.summary}`.toLowerCase().includes(normalizedResourceId),
      );
    }
    if (!keyword.trim()) {
      return items;
    }
    const normalizedKeyword = keyword.trim().toLowerCase();
    return items.filter((audit) =>
      `${audit.actor} ${audit.action} ${audit.resourceName} ${audit.summary} ${audit.traceId}`
        .toLowerCase()
        .includes(normalizedKeyword),
    );
  }, [auditsQuery.data, keyword, resourceIdFilter, resourceTypeFilter, resultFilter]);

  const resourcePath = buildResourcePath(resourceTypeFilter, resourceIdFilter);

  function setFilter(key: string, value?: string) {
    setSearchParams((previous) => {
      const next = new URLSearchParams(previous);
      const normalized = value?.trim() ?? "";
      if (normalized) {
        next.set(key, normalized);
      } else {
        next.delete(key);
      }
      return next;
    });
  }

  function clearFilters() {
    setSearchParams((previous) => {
      const next = new URLSearchParams(previous);
      next.delete("keyword");
      next.delete("result");
      next.delete("resourceType");
      next.delete("resourceId");
      return next;
    });
  }

  return (
    <PermissionGuard permission="audits.view" forbiddenPage>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <PageHeader
          title="审计日志"
          description="按结果、资源维度和关键字稳定排查操作记录。"
          extra={
            <Space wrap>
              {resourcePath ? <Button onClick={() => navigate(resourcePath)}>回到资源</Button> : null}
              <Button onClick={clearFilters}>清空筛选</Button>
            </Space>
          }
        />

        <Card className="page-card">
          <div className="page-toolbar">
            <div className="page-toolbar-start">
              <Input.Search
                allowClear
                placeholder="搜索操作人、动作、资源、摘要或 Trace ID"
                style={{ width: 360 }}
                value={keyword}
                onChange={(event) => setFilter("keyword", event.target.value)}
                onSearch={(value) => setFilter("keyword", value)}
              />
              <Select
                allowClear
                placeholder="结果"
                style={{ width: 140 }}
                value={resultFilter || undefined}
                options={resultOptions}
                onChange={(value) => setFilter("result", value)}
              />
              <Select
                allowClear
                placeholder="资源类型"
                style={{ width: 160 }}
                value={resourceTypeFilter || undefined}
                options={resourceTypeOptions}
                onChange={(value) => setFilter("resourceType", value)}
              />
              <Input
                allowClear
                placeholder="资源 ID"
                style={{ width: 220 }}
                value={resourceIdFilter}
                onChange={(event) => setFilter("resourceId", event.target.value)}
              />
            </div>
          </div>
        </Card>

        <Card className="page-card">
          <DataTable
            rowKey="id"
            loading={auditsQuery.isLoading}
            dataSource={filteredData}
            columns={[
              {
                title: "操作人",
                dataIndex: "actor",
                render: (value: string) => formatAuditActor(value),
              },
              { title: "动作", dataIndex: "action" },
              {
                title: "资源",
                key: "resource",
                render: (_, audit) => (
                  <Space direction="vertical" size={4}>
                    <span>{formatAuditResourceName(audit)}</span>
                    {audit.resourceType ? <Tag>{getResourceTypeLabel(audit.resourceType)}</Tag> : null}
                  </Space>
                ),
              },
              {
                title: "摘要",
                dataIndex: "summary",
                render: (_: string, audit) => formatAuditSummary(audit.summary, audit.action),
              },
              {
                title: "结果",
                dataIndex: "result",
                render: (value: string) => <StatusBadge status={value} />,
              },
              {
                title: "时间",
                dataIndex: "createdAt",
                render: (value: string) => formatDateTime(value),
              },
              {
                title: "操作",
                key: "actions",
                render: (_, audit) => {
                  const targetPath = buildResourcePath(audit.resourceType, audit.resourceId);
                  return (
                    <Space size={0}>
                      <Button type="link" onClick={() => setSelectedAudit(audit)}>
                        详情
                      </Button>
                      {targetPath ? (
                        <Button type="link" onClick={() => navigate(targetPath)}>
                          资源
                        </Button>
                      ) : null}
                    </Space>
                  );
                },
              },
            ]}
          />
        </Card>

        <AuditDrawer open={Boolean(selectedAudit)} audit={selectedAudit} onClose={() => setSelectedAudit(null)} />
      </Space>
    </PermissionGuard>
  );
}
