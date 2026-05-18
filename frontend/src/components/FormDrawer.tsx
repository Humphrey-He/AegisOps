import { Button, Drawer, Space } from "antd";
import type { PropsWithChildren } from "react";

type FormDrawerProps = PropsWithChildren<{
  open: boolean;
  title: string;
  loading?: boolean;
  width?: number;
  submitText?: string;
  cancelText?: string;
  onClose: () => void;
  onSubmit: () => void;
}>;

function resolveDrawerWidth(width: number) {
  if (width <= 560) {
    return 480;
  }
  if (width <= 700) {
    return 640;
  }
  return 760;
}

export function FormDrawer({
  open,
  title,
  loading,
  width = 640,
  submitText = "保存",
  cancelText = "取消",
  onClose,
  onSubmit,
  children,
}: FormDrawerProps) {
  return (
    <Drawer
      open={open}
      title={title}
      width={resolveDrawerWidth(width)}
      onClose={onClose}
      closable={!loading}
      destroyOnClose
      maskClosable={!loading}
      keyboard={!loading}
      footer={
        <div style={{ display: "flex", justifyContent: "flex-end" }}>
          <Space>
            <Button onClick={onClose} disabled={loading}>
              {cancelText}
            </Button>
            <Button type="primary" loading={loading} onClick={onSubmit}>
              {submitText}
            </Button>
          </Space>
        </div>
      }
      styles={{
        header: { paddingInline: 24, paddingBlock: 18 },
        body: { padding: 24, paddingBottom: 8 },
        footer: { paddingInline: 24, paddingBlock: 16, borderTop: "1px solid var(--aegis-border-soft)" },
      }}
    >
      {children}
    </Drawer>
  );
}
