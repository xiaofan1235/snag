import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ColumnDef } from "@tanstack/react-table";
import { DataTable } from "./Table";

interface R {
  id: string;
  n: number;
}
const columns: ColumnDef<R, unknown>[] = [
  { accessorKey: "id", header: "ID" },
  { accessorKey: "n", header: "N" },
];

describe("DataTable", () => {
  it("loading 态渲染骨架（无数据行）", () => {
    render(<DataTable columns={columns} data={[]} loading />);
    expect(document.querySelectorAll(".sn-skel").length).toBeGreaterThan(0);
  });

  it("empty 态渲染空提示", () => {
    render(<DataTable columns={columns} data={[]} empty="还没有数据" />);
    expect(screen.getByText("还没有数据")).toBeInTheDocument();
  });

  it("error 态 role=alert", () => {
    render(<DataTable columns={columns} data={[]} error="加载失败" />);
    expect(screen.getByRole("alert")).toHaveTextContent("加载失败");
  });

  it("小数据集渲染全部行", () => {
    const data = [
      { id: "a", n: 1 },
      { id: "b", n: 2 },
    ];
    render(<DataTable columns={columns} data={data} />);
    expect(screen.getByText("a")).toBeInTheDocument();
    expect(screen.getByText("b")).toBeInTheDocument();
  });

  it("可排序表头带 aria-sort，点击切换", async () => {
    const data = [
      { id: "a", n: 2 },
      { id: "b", n: 1 },
    ];
    let sorting: unknown[] = [];
    const { rerender } = render(
      <DataTable
        columns={columns}
        data={data}
        sorting={[]}
        onSortingChange={(s) => {
          sorting = s;
        }}
      />
    );
    const th = screen.getByText("N").closest("th")!;
    expect(th).toHaveAttribute("aria-sort", "none");
    await userEvent.click(th);
    expect(sorting.length).toBe(1);
    rerender(<DataTable columns={columns} data={data} sorting={sorting as never} onSortingChange={() => {}} />);
    expect(screen.getByText("N").closest("th")).toHaveAttribute("aria-sort", "ascending");
  });

  it("大数据集（>100 行）虚拟化：DOM 行数远小于数据量", () => {
    const data = Array.from({ length: 500 }, (_, i) => ({ id: `r${i}`, n: i }));
    render(<DataTable columns={columns} data={data} rowHeight={36} />);
    const bodyRows = document.querySelectorAll(".sn-table tbody tr");
    // 含 2 个 spacer 行；真实渲染行应远少于 500。
    expect(bodyRows.length).toBeLessThan(80);
  });
});
