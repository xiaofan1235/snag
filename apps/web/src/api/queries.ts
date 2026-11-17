// queries.ts —— TanStack Query 封装（UI 文档 §16.33：快照用 Query，SSE 失效后重取）。
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "./client";
import { qk } from "../events/router";
import type { CreateRunRequest, PauseRunRequest, StartRunRequest } from "./schema.gen";

export function useRuns() {
  return useQuery({ queryKey: ["runs"], queryFn: () => api.listRuns() });
}

export function useRun(runId: string) {
  return useQuery({ queryKey: qk.run(runId), queryFn: () => api.getRun(runId), enabled: !!runId });
}

export function useRunStatus(runId: string) {
  return useQuery({ queryKey: qk.runStatus(runId), queryFn: () => api.getRunStatus(runId), enabled: !!runId });
}

export function useRunJobs(runId: string) {
  return useQuery({ queryKey: qk.jobs(runId), queryFn: () => api.listRunJobs(runId), enabled: !!runId });
}

export function useRunEvidence(runId: string) {
  return useQuery({ queryKey: qk.evidence(runId), queryFn: () => api.listRunEvidence(runId), enabled: !!runId });
}

export function useRunReports(runId: string) {
  return useQuery({ queryKey: qk.reports(runId), queryFn: () => api.listRunReports(runId), enabled: !!runId });
}

// ---- Mutations（命令）----
// §16.33：命令返回 202+operation_id 只表示受理；最终状态等事件/查询，不 optimistic 改状态。

export function useCreateRun() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (b: CreateRunRequest) => api.createRun(b),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["runs"] }),
  });
}

export function useStartRun(runId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (b: StartRunRequest) => api.startRun(runId, b),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.run(runId) }),
  });
}

export function usePauseRun(runId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (b: PauseRunRequest) => api.pauseRun(runId, b),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.runStatus(runId) }),
  });
}

export function useStopRun(runId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.stopRun(runId),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.run(runId) }),
  });
}
