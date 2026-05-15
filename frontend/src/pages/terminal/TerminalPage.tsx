import { Alert, Button, Card, Space } from "antd";
import { useQuery } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { hostsApi, terminalApi } from "../../lib/api";
import { queryKeys } from "../../lib/queryKeys";
import { ConnectionStatus } from "../../components/ConnectionStatus";
import { ErrorState } from "../../components/ErrorState";
import { PageHeader } from "../../components/PageHeader";
import { PermissionGuard } from "../../components/PermissionGuard";
import { TerminalFrame } from "../../components/TerminalFrame";
import { formatDateTime } from "../../lib/format";

export function TerminalPage() {
  const navigate = useNavigate();
  const { sessionId = "" } = useParams();
  const [frameStatus, setFrameStatus] = useState<"CONNECTING" | "CONNECTED" | "DISCONNECTED">("CONNECTING");
  const sessionQuery = useQuery({
    queryKey: queryKeys.terminal(sessionId),
    queryFn: () => terminalApi.detail(sessionId),
    enabled: Boolean(sessionId),
    refetchInterval: 10_000,
  });
  const hostQuery = useQuery({
    queryKey: queryKeys.host(sessionQuery.data?.hostId ?? ""),
    queryFn: () => hostsApi.detail(sessionQuery.data!.hostId),
    enabled: Boolean(sessionQuery.data?.hostId),
  });

  const connectionHint = useMemo(() => {
    if (!sessionQuery.data) {
      return `会话 ID: ${sessionId}`;
    }
    const parts = [`会话 ID: ${sessionId}`];
    if (frameStatus === "DISCONNECTED") {
      parts.push("终端链路已断开，可尝试重连。");
    } else if (frameStatus === "CONNECTING") {
      parts.push("正在建立 WebSocket 通道。");
    }
    return parts.join(" · ");
  }, [frameStatus, sessionId, sessionQuery.data]);

  if (sessionQuery.isError) {
    return (
      <ErrorState
        title="终端会话不可用"
        message={sessionQuery.error.message}
        onRetry={() => void sessionQuery.refetch()}
      />
    );
  }

  return (
    <PermissionGuard permission="terminal.open" forbiddenPage>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <PageHeader
          title="WebSSH 会话"
          description="连接后端真实 WebSocket 流，并通过 xterm.js 进行交互。"
          extra={
            <Space wrap>
              {sessionQuery.data?.hostId ? (
                <Button onClick={() => navigate(`/assets/hosts?selected=${encodeURIComponent(sessionQuery.data.hostId)}`)}>
                  回到主机
                </Button>
              ) : null}
              <Button onClick={() => void sessionQuery.refetch()}>刷新会话</Button>
            </Space>
          }
        />

        {sessionQuery.data?.status === "DISCONNECTED" ? (
          <Alert
            type="warning"
            showIcon
            message="终端会话已断开或已失效"
            description="如果主机仍可用，可以先刷新会话状态，再尝试重新连接。"
          />
        ) : null}

        <div className="two-col-grid">
          <Card className="page-card" loading={sessionQuery.isLoading}>
            <ConnectionStatus
              label={sessionQuery.data?.hostName ?? "终端连接"}
              status={frameStatus === "CONNECTED" ? "CONNECTED" : sessionQuery.data?.status ?? "DISCONNECTED"}
              helper={connectionHint}
            />
          </Card>
          <Card className="page-card" loading={sessionQuery.isLoading || hostQuery.isLoading}>
            <div className="resource-detail-metadata">
              <div className="resource-detail-metadata-item">
                <div className="resource-detail-metadata-label">主机地址</div>
                <div className="resource-detail-metadata-value">
                  {hostQuery.data ? `${hostQuery.data.address}:${hostQuery.data.port}` : "--"}
                </div>
              </div>
              <div className="resource-detail-metadata-item">
                <div className="resource-detail-metadata-label">凭证</div>
                <div className="resource-detail-metadata-value">{hostQuery.data?.secretId || "--"}</div>
              </div>
              <div className="resource-detail-metadata-item">
                <div className="resource-detail-metadata-label">会话创建时间</div>
                <div className="resource-detail-metadata-value">{formatDateTime(sessionQuery.data?.createdAt)}</div>
              </div>
              <div className="resource-detail-metadata-item">
                <div className="resource-detail-metadata-label">主机 ID</div>
                <div className="resource-detail-metadata-value">{sessionQuery.data?.hostId || "--"}</div>
              </div>
            </div>
          </Card>
        </div>

        {sessionQuery.data ? (
          <TerminalFrame
            title={`${sessionQuery.data.hostName} 终端`}
            lines={sessionQuery.data.welcomeLines}
            status={sessionQuery.data.status}
            wsUrl={terminalApi.wsUrl(sessionId)}
            allowReconnect
            onReconnect={() => void sessionQuery.refetch()}
            onStatusChange={setFrameStatus}
          />
        ) : null}
      </Space>
    </PermissionGuard>
  );
}
