import { Button, Drawer, Space } from "antd";
import type { PropsWithChildren } from "react";

type FormDrawerProps = PropsWithChildren<{
  open: boolean;
  title: string;
  loading?: boolean;
  width?: number;
  onClose: () => void;
  onSubmit: () => void;
}>;

export function FormDrawer({
  open,
  title,
  loading,
  width = 520,
  onClose,
  onSubmit,
  children,
}: FormDrawerProps) {
  return (
    <Drawer
      open={open}
      title={title}
      width={width}
      onClose={onClose}
      destroyOnClose
      extra={
        <Space>
          <Button onClick={onClose}>取消</Button>
          <Button type="primary" loading={loading} onClick={onSubmit}>
            保存
          </Button>
        </Space>
      }
    >
      {children}
    </Drawer>
  );
}
