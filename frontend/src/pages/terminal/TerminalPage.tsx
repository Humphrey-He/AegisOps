import { Card, Space } from "antd";
import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router-dom";
import { terminalApi } from "../../lib/api";
import { queryKeys } from "../../lib/queryKeys";
import { ConnectionStatus } from "../../components/ConnectionStatus";
import { ErrorState } from "../../components/ErrorState";
import { PageHeader } from "../../components/PageHeader";
import { PermissionGuard } from "../../components/PermissionGuard";
import { TerminalFrame } from "../../components/TerminalFrame";

export function TerminalPage() {
  const { sessionId = "" } = useParams();
  const sessionQuery = useQuery({
    queryKey: queryKeys.terminal(sessionId),
    queryFn: () => terminalApi.detail(sessionId),
    enabled: Boolean(sessionId),
    refetchInterval: 10_000,
  });

  if (sessionQuery.isError) {
    return <ErrorState message={sessionQuery.error.message} onRetry={() => void sessionQuery.refetch()} />;
  }

  return (
    <PermissionGuard permission="terminal.open" forbiddenPage>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <PageHeader title="WebSSH 会话" description="连接后端真实 WebSocket 流，并通过 xterm.js 进行交互。" />

        <Card className="page-card" loading={sessionQuery.isLoading}>
          <ConnectionStatus
            label={sessionQuery.data?.hostName ?? "终端连接"}
            status={sessionQuery.data?.status ?? "DISCONNECTED"}
            helper={`会话 ID: ${sessionId}`}
          />
        </Card>

        {sessionQuery.data ? (
          <TerminalFrame
            title={`${sessionQuery.data.hostName} 终端`}
            lines={sessionQuery.data.welcomeLines}
            status={sessionQuery.data.status}
            wsUrl={terminalApi.wsUrl(sessionId)}
          />
        ) : null}
      </Space>
    </PermissionGuard>
  );
}
