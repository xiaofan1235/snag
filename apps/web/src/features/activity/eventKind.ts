// eventKind —— 把事件类型映射到 EventCard 类型（UI 文档 §16.13）。纯函数，可独立单测。
import type { EventCardKind } from "../../components/domain/EventCard";

export function eventKind(eventType: string): EventCardKind {
  if (eventType.startsWith("plan.")) return "plan";
  if (eventType.startsWith("action.")) return "action";
  if (eventType.startsWith("approval.")) return "approval";
  if (eventType.startsWith("budget.")) return "budget";
  if (eventType.startsWith("policy.") || eventType.includes("error") || eventType === "auth.required") return "error";
  if (eventType.startsWith("decision.") || eventType.startsWith("job.") || eventType.startsWith("attempt.")) return "decision";
  if (eventType.startsWith("directive.") || eventType.startsWith("external_message.")) return "user";
  if (
    eventType.startsWith("observation.") ||
    eventType.startsWith("evidence.") ||
    eventType.startsWith("exchange.") ||
    eventType.startsWith("finding.") ||
    eventType.startsWith("discovery.")
  )
    return "observation";
  return "observation";
}
