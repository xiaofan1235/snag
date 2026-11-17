// CommandButton —— 受控命令按钮（UI 文档 §16.33/§16.34）。幂等键 + 可选二次确认 + 结果跳转。
// 命令提交后 UI 进 pending，等 operation 终态才反映结果（不 optimistic 改状态）。

import { useRef, useState } from "react";
import { Button, type ButtonVariant } from "../../components/Button";
import { Modal } from "../../components/Modal";
import { CommandRunner, type CommandDeps, type CommandState } from "./idempotency";

export interface CommandButtonProps {
  label: string;
  deps: CommandDeps;
  variant?: ButtonVariant;
  /** 破坏性命令需二次确认。 */
  confirm?: { title: string; body: React.ReactNode; danger?: boolean };
  onSettled?: (state: CommandState, resultRef: string | null) => void;
}

export function CommandButton({ label, deps, variant = "primary", confirm, onSettled }: CommandButtonProps) {
  const [state, setState] = useState<CommandState>("IDLE");
  const [confirming, setConfirming] = useState(false);
  // 同一按钮实例复用同一 runner（故重试复用同一 Idempotency-Key，不生成第二份）。
  const runnerRef = useRef<CommandRunner | null>(null);

  async function exec() {
    setConfirming(false);
    if (!runnerRef.current) runnerRef.current = new CommandRunner(deps);
    setState("SUBMITTING");
    const final = await runnerRef.current.run();
    setState(final);
    onSettled?.(final, runnerRef.current.resultRef);
  }

  const busy = state === "SUBMITTING" || state === "PENDING";

  return (
    <>
      <Button
        variant={state === "FAILED" ? "danger" : variant}
        loading={busy}
        onClick={() => (confirm ? setConfirming(true) : exec())}
      >
        {state === "FAILED" ? `${label}（重试）` : state === "PENDING" ? "受理中…" : label}
      </Button>
      {confirm && (
        <Modal
          open={confirming}
          onClose={() => setConfirming(false)}
          title={confirm.title}
          danger={confirm.danger}
          confirmText="确认"
          onConfirm={exec}
        >
          {confirm.body}
        </Modal>
      )}
    </>
  );
}
