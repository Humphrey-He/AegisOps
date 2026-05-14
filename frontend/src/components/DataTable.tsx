import { Empty, Table } from "antd";
import type { TableProps } from "antd";

export function DataTable<RecordType extends object>(props: TableProps<RecordType>) {
  return (
    <Table<RecordType>
      bordered={false}
      size="middle"
      pagination={{ pageSize: 10, showSizeChanger: false, ...(props.pagination ?? {}) }}
      locale={{ emptyText: <Empty description="暂无数据" /> }}
      {...props}
    />
  );
}
