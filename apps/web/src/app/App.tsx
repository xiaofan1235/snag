// App —— 顶层路由分发（UI 文档 D4）。按 hash 路径选页，包进 AppShell 三区布局。
// 路由优先级：静态段先于 :param（/runs/new 先于 /runs/:id）。

import { AppShell } from "./AppShell";
import { usePath } from "./router";
import { matchRoute } from "./router";
import { DashboardPage, RunsPage, NewRunPage, InboxPage, PlaceholderPage, PageHead, KnowledgeGlobalPage, SkillsGlobalPage, ToolsGlobalPage, OperationsGlobalPage, SettingsGlobalPage } from "./pages";
import { RunWorkbench } from "./RunWorkbench";
import { Gallery } from "./Gallery";

function resolve(path: string): React.ReactNode {
  if (matchRoute("/", path)) return <DashboardPage />;
  if (matchRoute("/runs/new", path)) return <NewRunPage />;
  if (matchRoute("/runs", path)) return <RunsPage />;
  const runSeg = matchRoute("/runs/:id/:seg", path);
  if (runSeg) return <RunWorkbench runId={runSeg.id} seg={runSeg.seg} />;
  const runOnly = matchRoute("/runs/:id", path);
  if (runOnly) return <RunWorkbench runId={runOnly.id} seg="command" />;
  if (matchRoute("/inbox", path)) return <InboxPage />;
  if (matchRoute("/knowledge", path)) return <KnowledgeGlobalPage />;
  if (matchRoute("/skills", path)) return <SkillsGlobalPage />;
  if (matchRoute("/tools", path)) return <ToolsGlobalPage />;
  if (matchRoute("/operations", path)) return <OperationsGlobalPage />;
  if (matchRoute("/settings", path)) return <SettingsGlobalPage />;
  if (matchRoute("/dev/components", path)) {
    return (
      <>
        <PageHead title="组件库（开发参考）" />
        <Gallery />
      </>
    );
  }
  return <PlaceholderPage title="页面不存在" note="该路径无对应页面，请从左侧导航进入。" />;
}

export function App() {
  const path = usePath();
  return <AppShell>{resolve(path)}</AppShell>;
}
