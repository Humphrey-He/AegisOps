import { App as AntApp, Button, Card, Space, Typography } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo } from "react";
import { useSearchParams } from "react-router-dom";

import { DataTable } from "../../components/DataTable";
import { ErrorState } from "../../components/ErrorState";
import { PageHeader } from "../../components/PageHeader";
import { PermissionActionButton } from "../../components/PermissionActionButton";
import { PermissionGuard } from "../../components/PermissionGuard";
import { ResourceDetailPanel } from "../../components/resource/ResourceDetailPanel";
import { StatusBadge } from "../../components/StatusBadge";
import { backupsApi } from "../../lib/api";
import { getErrorMessage } from "../../lib/forms";
import { formatBytes, formatDateTime } from "../../lib/format";
import { queryKeys } from "../../lib/queryKeys";

export function BackupsPage() {
  const { message } = AntApp.useApp();
  const queryClient = useQueryClient();
  const [searchParams, setSearchParams] = useSearchParams();
  const selectedId = searchParams.get("selected") ?? "";

  const backupsQuery = useQuery({
    queryKey: queryKeys.backups,
    queryFn: backupsApi.list,
  });

  const selectedBackup = (backupsQuery.data ?? []).find((item) => item.id === selectedId) ?? backupsQuery.data?.[0] ?? null;

  const manifestQuery = useQuery({
    queryKey: queryKeys.backupManifest(selectedBackup?.id ?? ""),
    queryFn: () => backupsApi.manifest(selectedBackup?.id ?? ""),
    enabled: Boolean(selectedBackup?.id),
  });

  const createMutation = useMutation({
    mutationFn: () => backupsApi.create(true),
    onSuccess: async (backup) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.backups });
      setSearchParams((previous) => {
        const next = new URLSearchParams(previous);
        next.set("selected", backup.id);
        return next;
      });
      await message.success("备份任务已创建");
    },
    onError: (error) => {
      void message.error(getErrorMessage(error, "创建备份失败"));
    },
  });

  const downloadMutation = useMutation({
    mutationFn: backupsApi.download,
    onSuccess: async (result) => {
      await message.success(result.fileName ? `已开始下载 ${result.fileName}` : "已开始下载备份文件");
    },
    onError: (error) => {
      void message.error(getErrorMessage(error, "下载备份失败"));
    },
  });

  const runningCount = useMemo(
    () => (backupsQuery.data ?? []).filter((item) => item.status === "RUNNING" || item.status === "PENDING").length,
    [backupsQuery.data],
  );

  if (backupsQuery.isError) {
    return <ErrorState message={backupsQuery.error.message} onRetry={() => void backupsQuery.refetch()} />;
  }

  return (
    <PermissionGuard permission="backups.view" forbiddenPage>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <PageHeader
          title="备份中心"
          description="查看系统备份产物、校验信息和 manifest 摘要，并从这里统一触发和下载当前环境的备份包。"
          extra={
            <PermissionActionButton
              type="primary"
              permission="backups.create"
              permissionReason="当前账号缺少 backups.create 权限，无法创建备份。"
              loading={createMutation.isPending}
              onClick={() => createMutation.mutate()}
            >
              立即备份
            </PermissionActionButton>
          }
        />

        <Card className="page-card">
          <div className="metric-grid">
            <Card size="small">
              <Typography.Text type="secondary">备份总数</Typography.Text>
              <Typography.Title level={4} style={{ margin: "8px 0 0" }}>{(backupsQuery.data ?? []).length}</Typography.Title>
            </Card>
            <Card size="small">
              <Typography.Text type="secondary">处理中</Typography.Text>
              <Typography.Title level={4} style={{ margin: "8px 0 0" }}>{runningCount}</Typography.Title>
            </Card>
          </div>
        </Card>

        <div className="resource-workbench">
          <div className="resource-list-pane">
            <Card className="page-card">
              <DataTable
                rowKey="id"
                loading={backupsQuery.isLoading}
                dataSource={backupsQuery.data}
                rowClassName={(item) => (item.id === selectedBackup?.id ? "resource-row-selected" : "")}
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
                    title: "备份文件",
                    key: "file",
                    render: (_, record) => (
                      <Space direction="vertical" size={2}>
                        <Typography.Text strong>{record.fileName || record.id}</Typography.Text>
                        <Typography.Text type="secondary">{record.type}</Typography.Text>
                      </Space>
                    ),
                  },
                  { title: "状态", dataIndex: "status", render: (value: string) => <StatusBadge status={value} /> },
                  { title: "大小", dataIndex: "fileSize", render: (value: number) => formatBytes(value) },
                  { title: "创建时间", dataIndex: "createdAt", render: (value: string) => formatDateTime(value) },
                ]}
              />
            </Card>
          </div>

          <div className="resource-detail-pane">
            <ResourceDetailPanel
              title={selectedBackup?.fileName || selectedBackup?.id}
              subtitle={selectedBackup ? `类型 ${selectedBackup.type}` : undefined}
              status={selectedBackup ? <StatusBadge status={selectedBackup.status} /> : undefined}
              emptyTitle="选择一条备份记录"
              emptyDescription="左侧选中备份后，这里会展示 checksum、manifest 和下载入口。"
              highlights={
                selectedBackup
                  ? [
                      {
                        label: "备份大小",
                        value: formatBytes(selectedBackup.fileSize),
                        helper: selectedBackup.masked ? "当前为脱敏备份" : "包含未脱敏内容",
                      },
                      {
                        label: "Checksum",
                        value: selectedBackup.checksum ? `${selectedBackup.checksum.slice(0, 12)}...` : "--",
                        helper: selectedBackup.checksum || "等待备份完成后生成校验值",
                      },
                    ]
                  : []
              }
              meta={
                selectedBackup
                  ? [
                      { label: "创建人", value: selectedBackup.createdBy || "--" },
                      { label: "完成时间", value: formatDateTime(selectedBackup.finishedAt) },
                    ]
                  : []
              }
              actions={
                selectedBackup ? (
                  <Space wrap>
                    <PermissionActionButton
                      permission="backups.download"
                      permissionReason="当前账号缺少 backups.download 权限，无法下载备份。"
                      disabled={selectedBackup.status !== "SUCCESS"}
                      disabledReason="仅成功生成的备份记录才能下载。"
                      loading={downloadMutation.isPending}
                      onClick={() => downloadMutation.mutate(selectedBackup.id)}
                    >
                      下载备份
                    </PermissionActionButton>
                  </Space>
                ) : undefined
              }
            >
              {selectedBackup?.errorMessage ? (
                <Card size="small">
                  <Typography.Text strong>错误信息</Typography.Text>
                  <pre className="service-json-block">{selectedBackup.errorMessage}</pre>
                </Card>
              ) : null}
              {manifestQuery.data?.manifest ? (
                <Card size="small">
                  <Typography.Text strong>Manifest</Typography.Text>
                  <pre className="service-json-block">{manifestQuery.data.manifest}</pre>
                </Card>
              ) : null}
            </ResourceDetailPanel>
          </div>
        </div>
      </Space>
    </PermissionGuard>
  );
}
