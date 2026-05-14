import { Space, Typography } from "antd";
import { StatusBadge } from "./StatusBadge";

type ConnectionStatusProps = {
  label: string;
  status: string;
  helper?: string;
};

export function ConnectionStatus({ label, status, helper }: ConnectionStatusProps) {
  return (
    <Space direction="vertical" size={4}>
      <Typography.Text type="secondary">{label}</Typography.Text>
      <StatusBadge status={status} />
      {helper ? <Typography.Text className="stat-help">{helper}</Typography.Text> : null}
    </Space>
  );
}
