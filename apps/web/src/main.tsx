import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import "./styles/tokens.css";
import "./components/components.css";
import "./app/shell.css";
import { App } from "./app/App";
import { RouterProvider } from "./app/router";
import { InspectorProvider } from "./app/inspector";
import { ToastProvider } from "./components";

const queryClient = new QueryClient();

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <ToastProvider>
        <RouterProvider>
          <InspectorProvider>
            <App />
          </InspectorProvider>
        </RouterProvider>
      </ToastProvider>
    </QueryClientProvider>
  </StrictMode>
);
