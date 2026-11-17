import { StrictMode, useState } from "react";
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import "./styles/tokens.css";
import "./components/components.css";
import { Gallery } from "./app/Gallery";
import { PagesDemo } from "./app/PagesDemo";
import { ToastProvider, Button } from "./components";

const queryClient = new QueryClient();

function Root() {
  const [view, setView] = useState<"components" | "pages">("components");
  return (
    <div style={{ maxWidth: 1000, margin: "0 auto", padding: "var(--sp-5)" }}>
      <nav style={{ display: "flex", gap: "var(--sp-2)", marginBottom: "var(--sp-4)" }}>
        <Button variant={view === "components" ? "primary" : "ghost"} onClick={() => setView("components")}>
          组件库
        </Button>
        <Button variant={view === "pages" ? "primary" : "ghost"} onClick={() => setView("pages")}>
          P0 页面
        </Button>
      </nav>
      {view === "components" ? <Gallery /> : <PagesDemo />}
    </div>
  );
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <ToastProvider>
        <Root />
      </ToastProvider>
    </QueryClientProvider>
  </StrictMode>
);
