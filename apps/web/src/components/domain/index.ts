// 领域组件（UI 文档 D2.13）统一出口。
export { EventCard } from "./EventCard";
export type { EventCardProps, EventCardKind } from "./EventCard";
export { EvidenceViewer } from "./EvidenceViewer";
export type { EvidenceViewerProps, EvidenceTab } from "./EvidenceViewer";
export { HTTPViewer } from "./HTTPViewer";
export type { HTTPViewerProps, HttpMessage } from "./HTTPViewer";
export { DiffViewer } from "./DiffViewer";
export type { DiffViewerProps } from "./DiffViewer";
export { computeLineDiff, diffStats } from "./diff";
export type { DiffRow, DiffOp, DiffStats } from "./diff";
export { Inspector } from "./Inspector";
export type { InspectorProps, InspectorModel, ObjectRef, InspectorField } from "./Inspector";
