package findings

import (
	"context"
	"testing"
)

// §31 V7：CVSS v3.1 基础分确定性计算。
func TestCVSSDeterministic(t *testing.T) {
	// 经典 Critical 9.8：AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H。
	crit := CVSSv31{AV: "N", AC: "L", PR: "N", UI: "N", S: "U", C: "H", I: "H", A: "H"}
	if s := crit.BaseScore(); s != 9.8 {
		t.Fatalf("Critical 向量基础分应 9.8，实得 %v", s)
	}
	if crit.Severity() != "Critical" {
		t.Fatalf("应 Critical，实得 %s", crit.Severity())
	}
	if crit.Vector() != "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H" {
		t.Fatalf("向量串不符：%s", crit.Vector())
	}
	// Medium 6.5：C:L/I:L/A:N。
	med := CVSSv31{AV: "N", AC: "L", PR: "N", UI: "N", S: "U", C: "L", I: "L", A: "N"}
	if s := med.BaseScore(); s != 6.5 {
		t.Fatalf("Medium 向量基础分应 6.5，实得 %v", s)
	}
	if med.Severity() != "Medium" {
		t.Fatalf("应 Medium，实得 %s", med.Severity())
	}
	// 无影响 → None 0.0。
	none := CVSSv31{AV: "N", AC: "L", PR: "N", UI: "N", S: "U", C: "N", I: "N", A: "N"}
	if none.BaseScore() != 0 || none.Severity() != "None" {
		t.Fatalf("无影响应 None 0，实得 %v/%s", none.BaseScore(), none.Severity())
	}
	// 确定性：同向量两次一致。
	if crit.BaseScore() != crit.BaseScore() {
		t.Fatal("CVSS 计算应确定性")
	}
}

func reportableFinding(t *testing.T, s *Store, runID string) string {
	t.Helper()
	fid, err := s.CreateCandidate(context.Background(), runID, "", "IDOR", "idor", t0)
	if err != nil {
		t.Fatal(err)
	}
	for _, to := range []string{FindConfirmed, FindTriaged, FindReportable} {
		if err := s.TransitionFinding(context.Background(), fid, to, ActorVerifier, t0); err != nil {
			t.Fatal(err)
		}
	}
	return fid
}

// DoD：REPORTABLE 发现被新反证 → 自动降级（§13.4）。
func TestReplayCounterEvidenceDowngrade(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx)
	s := NewStore(w)

	// 弱反证 → NEEDS_EVIDENCE。
	f1 := reportableFinding(t, s, "run-1")
	target, err := s.ApplyDowngrade(ctx, f1, "run-1", DowngradeCounterEvidence, false, nil, t0)
	if err != nil || target != FindNeedsEvidence {
		t.Fatalf("弱反证应降到 NEEDS_EVIDENCE，实得 %s err=%v", target, err)
	}
	if st, _ := s.FindingState(ctx, f1); st != FindNeedsEvidence {
		t.Fatalf("Finding 应 NEEDS_EVIDENCE，实得 %s", st)
	}
	if _, v, _ := s.ActiveVerdict(ctx, f1); v != VerdictNeedsEvidence {
		t.Fatalf("活跃 Verdict 应 NEEDS_EVIDENCE，实得 %s", v)
	}

	// 强反证 → REJECTED。
	f2 := reportableFinding(t, s, "run-1")
	target2, _ := s.ApplyDowngrade(ctx, f2, "run-1", DowngradeCounterEvidence, true, nil, t0)
	if target2 != FindRejected {
		t.Fatalf("强反证应降到 REJECTED，实得 %s", target2)
	}
}

// DoD：关联 Evidence 哈希校验失败 → 降级 + Evidence 进 Quarantine（§13.4）。
func TestHashFailDowngradeQuarantine(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx)
	s := NewStore(w)
	seedEvidence(t, w, ctx, "ev-1", "run-1")
	fid := reportableFinding(t, s, "run-1")

	target, err := s.ApplyDowngrade(ctx, fid, "run-1", DowngradeHashFail, false, []string{"ev-1"}, t0)
	if err != nil || target != FindNeedsEvidence {
		t.Fatalf("哈希失败应降级，实得 %s err=%v", target, err)
	}
	if q, _ := s.EvidenceQuarantined(ctx, "ev-1"); !q {
		t.Fatal("哈希失败的 Evidence 应进 Quarantine")
	}
}

// Verdict 带 revision，新结论 supersede 旧的（§13.4）。
func TestVerdictRevisionSupersede(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx)
	s := NewStore(w)
	fid, _ := s.CreateCandidate(ctx, "run-1", "", "x", "idor", t0)

	r1, _ := s.WriteVerdict(ctx, fid, "run-1", VerdictConfirmed, "replay1", 1, "", t0)
	r2, _ := s.WriteVerdict(ctx, fid, "run-1", VerdictNeedsEvidence, "replay2_counter", 1, "", t0)
	if r1 != 1 || r2 != 2 {
		t.Fatalf("revision 应递增 1,2，实得 %d,%d", r1, r2)
	}
	rev, v, _ := s.ActiveVerdict(ctx, fid)
	if rev != 2 || v != VerdictNeedsEvidence {
		t.Fatalf("活跃 Verdict 应为 rev2 NEEDS_EVIDENCE，实得 rev%d %s", rev, v)
	}
}
