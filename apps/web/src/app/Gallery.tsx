import { useMemo, useState } from "react";
import type { ColumnDef } from "@tanstack/react-table";
import {
  Button,
  Card,
  DataTable,
  Drawer,
  FractionProgress,
  Input,
  Modal,
  Progress,
  Select,
  StatusBadge,
  Textarea,
  useToast,
  ALL_STATUSES,
} from "../components";
import type { SelectOption } from "../components";
import { useTheme } from "./theme";

// Gallery —— 组件库独立预览页（Day 39 DoD：可独立预览 + 亮/暗切换 + StatusBadge 覆盖所有状态枚举）。
// 非生产页面，仅演示每个组件的变体/状态。生产页面从 Day 41 起铺。

interface DemoRow {
  id: string;
  endpoint: string;
  status: string;
}

export function Gallery() {
  const [theme, toggleTheme] = useTheme();
  const [modal, setModal] = useState(false);
  const [drawer, setDrawer] = useState(false);
  const [sel, setSel] = useState<string>("");
  const [bigSel, setBigSel] = useState<string>("");
  const toast = useToast();

  const manyOptions = useMemo<SelectOption[]>(
    () => Array.from({ length: 200 }, (_, i) => ({ value: `opt-${i}`, label: `端点 #${i} /api/resource/${i}` })),
    []
  );

  const rows = useMemo<DemoRow[]>(
    () =>
      Array.from({ length: 250 }, (_, i) => ({
        id: `r-${i}`,
        endpoint: `/api/orders/${i}`,
        status: ALL_STATUSES[i % ALL_STATUSES.length],
      })),
    []
  );
  const columns = useMemo<ColumnDef<DemoRow, unknown>[]>(
    () => [
      { accessorKey: "id", header: "ID" },
      { accessorKey: "endpoint", header: "Endpoint" },
      {
        accessorKey: "status",
        header: "状态",
        cell: (c) => <StatusBadge status={c.getValue() as string} />,
      },
    ],
    []
  );

  return (
    <div style={{ maxWidth: 960, margin: "0 auto", padding: "var(--sp-6)" }}>
      <header style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: "var(--sp-5)" }}>
        <h1 style={{ fontSize: "var(--fs-2xl)", margin: 0 }}>Snag 组件库</h1>
        <Button variant="secondary" onClick={toggleTheme}>
          切换到{theme === "light" ? "暗色" : "亮色"}
        </Button>
      </header>

      <Section title="Button（D2.1）">
        <Row>
          <Button variant="primary">主操作</Button>
          <Button variant="secondary">次操作</Button>
          <Button variant="ghost">弱操作</Button>
          <Button variant="danger">危险</Button>
          <Button variant="link">链接</Button>
        </Row>
        <Row>
          <Button size="sm">sm</Button>
          <Button size="md">md</Button>
          <Button size="lg">lg</Button>
          <Button loading>加载中</Button>
          <Button disabled>禁用</Button>
        </Row>
      </Section>

      <Section title="Input / Textarea（D2.2）">
        <Input label="目标 URL" placeholder="https://example.com" hint="仅授权目标" />
        <Input label="错误示例" defaultValue="bad" error="格式不合法" />
        <Textarea label="备注" placeholder="多行输入…" />
      </Section>

      <Section title="Select（D2.3，>50 虚拟化）">
        <Row>
          <Select
            options={[
              { value: "sqli", label: "SQL 注入" },
              { value: "idor", label: "IDOR" },
              { value: "xss", label: "XSS" },
            ]}
            value={sel}
            onChange={setSel}
            aria-label="攻击类型"
            placeholder="选择攻击类型"
          />
          <Select options={manyOptions} value={bigSel} onChange={setBigSel} searchable aria-label="端点" placeholder="搜索端点（200 项虚拟化）" />
        </Row>
      </Section>

      <Section title="Progress（D2.5）">
        <Progress value={42} aria-label="上传" />
        <Progress indeterminate aria-label="处理中" />
        <Progress value={90} tone="warn" aria-label="接近上限" />
        <FractionProgress done={7} total={12} />
      </Section>

      <Section title="StatusBadge（D2.6，覆盖所有状态）">
        <Row>
          {ALL_STATUSES.map((s) => (
            <StatusBadge key={s} status={s} />
          ))}
        </Row>
      </Section>

      <Section title="Card（D2.7）">
        <Row>
          <Card>静态卡片</Card>
          <Card clickable onClick={() => toast.push({ tone: "info", message: "卡片被点击" })}>
            可点卡片（hover 抬升）
          </Card>
        </Row>
      </Section>

      <Section title="Table（D2.8，250 行虚拟化）">
        <DataTable columns={columns} data={rows} sorting={[]} onSortingChange={() => {}} />
      </Section>

      <Section title="Modal / Drawer / Toast（D2.10/D2.11）">
        <Row>
          <Button onClick={() => setModal(true)}>打开 Modal</Button>
          <Button onClick={() => setDrawer(true)}>打开 Drawer</Button>
          <Button onClick={() => toast.push({ tone: "ok", message: "操作成功" })}>弹 Toast</Button>
          <Button variant="danger" onClick={() => toast.push({ tone: "danger", message: "发生错误（已脱敏）" })}>
            错误 Toast
          </Button>
        </Row>
      </Section>

      <Modal
        open={modal}
        onClose={() => setModal(false)}
        title="停止 Run？"
        danger
        confirmText="停止"
        onConfirm={() => {
          setModal(false);
          toast.push({ tone: "warn", message: "Run 已请求停止" });
        }}
      >
        破坏性操作需二次确认。停止只影响未来动作，不篡改已执行事实。
      </Modal>

      <Drawer open={drawer} onClose={() => setDrawer(false)} title="Finding 详情">
        <p style={{ color: "var(--ink-2)" }}>右侧抽屉 = Inspector。对象详情在此展示，不阻断主内容。</p>
        <StatusBadge status="REPORTABLE" />
      </Drawer>
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section style={{ marginBottom: "var(--sp-6)" }}>
      <h2 style={{ fontSize: "var(--fs-lg)", color: "var(--ink-2)", marginBottom: "var(--sp-3)" }}>{title}</h2>
      <div style={{ display: "flex", flexDirection: "column", gap: "var(--sp-3)" }}>{children}</div>
    </section>
  );
}

function Row({ children }: { children: React.ReactNode }) {
  return <div style={{ display: "flex", gap: "var(--sp-3)", flexWrap: "wrap", alignItems: "center" }}>{children}</div>;
}
