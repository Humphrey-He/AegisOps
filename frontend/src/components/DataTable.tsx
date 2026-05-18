import { Table } from "antd";
import type { TableProps } from "antd";
import { EmptyState } from "./EmptyState";

function joinClassNames(...classNames: Array<string | undefined>) {
  return classNames.filter(Boolean).join(" ");
}

export function DataTable<RecordType extends object>({
  className,
  locale,
  pagination,
  ...props
}: TableProps<RecordType>) {
  const resolvedPagination =
    pagination === false
      ? false
      : {
          pageSize: 10,
          showSizeChanger: false,
          showTotal: (total: number, range: [number, number]) =>
            total > range[1] ? `${range[0]}-${range[1]} / 共 ${total} 条` : `共 ${total} 条`,
          ...(pagination ?? {}),
        };

  const resolvedLocale = {
    ...locale,
    emptyText: locale?.emptyText ?? <EmptyState title="暂无数据" description="调整筛选条件或刷新后重试。" />,
  };

  return (
    <Table<RecordType>
      {...props}
      className={joinClassNames("data-table", className)}
      bordered={false}
      size="middle"
      pagination={resolvedPagination}
      locale={resolvedLocale}
    />
  );
}
