import { Alert, Button, Input, Modal, Space, Typography } from "antd";
import { useMemo, useState } from "react";

type DangerConfirmProps = {
  open: boolean;
  title: string;
  description: string;
  confirmText?: string;
  loading?: boolean;
  onCancel: () => void;
  onConfirm: () => void;
};

export function DangerConfirm({
  open,
  title,
  description,
  confirmText,
  loading,
  onCancel,
  onConfirm,
}: DangerConfirmProps) {
  const [inputValue, setInputValue] = useState("");
  const disabled = useMemo(() => {
    if (!confirmText) {
      return false;
    }
    return inputValue !== confirmText;
  }, [confirmText, inputValue]);

  return (
    <Modal
      open={open}
      title={title}
      onCancel={() => {
        setInputValue("");
        onCancel();
      }}
      onOk={() => {
        onConfirm();
        setInputValue("");
      }}
      okText="确认执行"
      okButtonProps={{ danger: true, disabled, loading }}
      cancelText="取消"
    >
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <Alert
          type="warning"
          showIcon
          message="这是高风险操作"
          description="一期 MVP 里，所有容器启停类动作都建议保留明确确认。"
        />
        <Typography.Paragraph style={{ marginBottom: 0 }}>{description}</Typography.Paragraph>
        {confirmText ? (
          <Space direction="vertical" size={8} style={{ width: "100%" }}>
            <Typography.Text type="secondary">请输入 {confirmText} 以确认操作。</Typography.Text>
            <Input value={inputValue} onChange={(event) => setInputValue(event.target.value)} />
          </Space>
        ) : null}
      </Space>
    </Modal>
  );
}
