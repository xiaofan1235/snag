import { useRef } from "react";
import {
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  useReactTable,
  type ColumnDef,
  type SortingState,
} from "@tanstack/react-table";
import { useVirtualizer } from "@tanstack/react-virtual";
import { cx } from "./cx";

export interface DataTableProps<T> {
  columns: ColumnDef<T, unknown>[];
  data: T[];
  loading?: boolean;
  error?: React.ReactNode;
  empty?: React.ReactNode;
  sorting?: SortingState;
  onSortingChange?: (s: SortingState) => void;
  /** 超过此行数启用行虚拟化（D2.8：>100 必虚拟化）。 */
  virtualizeThreshold?: number;
  rowHeight?: number;
  className?: string;
}

// DataTable（D2.8）：排序（aria-sort）、粘性表头、loading(Skeleton)/empty(EmptyState)/error 态；
// 行数 > 阈值用 TanStack Virtual 虚拟化，不一次渲染数千行。
export function DataTable<T>({
  columns,
  data,
  loading = false,
  error,
  empty = "暂无数据",
  sorting,
  onSortingChange,
  virtualizeThreshold = 100,
  rowHeight = 36,
  className,
}: DataTableProps<T>) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const table = useReactTable({
    data,
    columns,
    sortDescFirst: false, // 首次点击升序（可预期；TanStack 对数字列默认降序优先，这里统一为升序先）
    state: sorting ? { sorting } : undefined,
    onSortingChange: onSortingChange
      ? (updater) => {
          const next = typeof updater === "function" ? updater(sorting ?? []) : updater;
          onSortingChange(next);
        }
      : undefined,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
  });

  const rows = table.getRowModel().rows;
  const virtualize = rows.length > virtualizeThreshold;
  const rowVirtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => rowHeight,
    overscan: 12,
    enabled: virtualize,
  });

  const colCount = table.getAllLeafColumns().length;

  function ariaSort(dir: false | "asc" | "desc"): "ascending" | "descending" | "none" {
    if (dir === "asc") return "ascending";
    if (dir === "desc") return "descending";
    return "none";
  }

  return (
    <div className={cx("sn-table-wrap", className)} ref={scrollRef} style={{ maxHeight: 440 }}>
      <table className="sn-table">
        <thead>
          {table.getHeaderGroups().map((hg) => (
            <tr key={hg.id}>
              {hg.headers.map((h) => {
                const sortable = h.column.getCanSort();
                return (
                  <th
                    key={h.id}
                    aria-sort={sortable ? ariaSort(h.column.getIsSorted()) : undefined}
                    onClick={sortable ? h.column.getToggleSortingHandler() : undefined}
                  >
                    {flexRender(h.column.columnDef.header, h.getContext())}
                    {sortable && (
                      <span aria-hidden="true" style={{ color: "var(--ink-3)" }}>
                        {h.column.getIsSorted() === "asc" ? " ▲" : h.column.getIsSorted() === "desc" ? " ▼" : ""}
                      </span>
                    )}
                  </th>
                );
              })}
            </tr>
          ))}
        </thead>
        <tbody>
          {loading ? (
            Array.from({ length: 5 }).map((_, i) => (
              <tr key={`skel-${i}`}>
                {Array.from({ length: colCount }).map((__, j) => (
                  <td key={j}>
                    <div className="sn-skel" />
                  </td>
                ))}
              </tr>
            ))
          ) : error ? (
            <tr>
              <td colSpan={colCount}>
                <div className="sn-empty" style={{ color: "var(--danger)" }} role="alert">
                  {error}
                </div>
              </td>
            </tr>
          ) : rows.length === 0 ? (
            <tr>
              <td colSpan={colCount}>
                <div className="sn-empty">{empty}</div>
              </td>
            </tr>
          ) : virtualize ? (
            <>
              <tr style={{ height: rowVirtualizer.getVirtualItems()[0]?.start ?? 0 }} aria-hidden="true" />
              {rowVirtualizer.getVirtualItems().map((vi) => {
                const row = rows[vi.index];
                return (
                  <tr key={row.id} style={{ height: vi.size }}>
                    {row.getVisibleCells().map((cell) => (
                      <td key={cell.id}>{flexRender(cell.column.columnDef.cell, cell.getContext())}</td>
                    ))}
                  </tr>
                );
              })}
              <tr
                style={{
                  height:
                    rowVirtualizer.getTotalSize() -
                    (rowVirtualizer.getVirtualItems().at(-1)?.end ?? 0),
                }}
                aria-hidden="true"
              />
            </>
          ) : (
            rows.map((row) => (
              <tr key={row.id}>
                {row.getVisibleCells().map((cell) => (
                  <td key={cell.id}>{flexRender(cell.column.columnDef.cell, cell.getContext())}</td>
                ))}
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  );
}
