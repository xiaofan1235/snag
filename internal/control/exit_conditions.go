package control

// exit_conditions.go —— 退出门禁默认条件集（§14.2 清单）。每条绑定到**已存在的权威表**做确定性只读检查。
//
// 诚实声明（与开发方「文档化偏离」约定一致）：§14.2 清单里有三项的后端子系统本批次尚未建表——
// 未消费 Permit、测试数据清理核对、Residual Risk 记录。这三项以 unwired 占位条件给出（默认通过，
// detail 标注 unwired），并登记进 PROGRESS「集成待接线清单」；相应子系统落地后用 NewExitGate 的 extra
// 同名覆盖成真实实现即可，无需改门禁骨架。其余 11 项均为真实绑定。

import (
	"context"
	"database/sql"
	"fmt"
)

// defaultConditions 返回 §14.2 的 14 条门禁条件（11 真实 + 3 unwired 占位）。
func defaultConditions(cfg GateConfig) []GateCondition {
	return []GateCondition{
		{"no_nonterminal_job", condNoNonterminalJob},
		{"no_outcome_unknown", condNoOutcomeUnknown},
		{"no_valid_unconsumed_permit", condUnwired("permits 子系统未接线")},
		{"no_highvalue_open_discovery", condNoHighValueOpenDiscovery},
		{"no_open_hypothesis", condNoOpenHypothesis},
		{"coverage_threshold", condCoverageThreshold(cfg.CoverageThresholdPct)},
		{"na_quality", condNAQuality},
		{"findings_have_verdict", condFindingsHaveVerdict},
		{"no_pending_quarantine", condNoPendingQuarantine},
		{"no_pending_approval", condNoPendingApproval},
		{"no_stale_cells", condNoStaleCells},
		{"test_data_cleanup", condUnwired("清理核对账本未接线")},
		{"residual_risk_recorded", condUnwired("residual_risk 表未接线")},
		{"report_manifest_buildable", condReportManifestBuildable},
	}
}

// condUnwired 构造一个「未接线」占位条件：默认通过，detail 标注，便于后续 extra 覆盖。
func condUnwired(why string) CheckFunc {
	return func(ctx context.Context, tx *sql.Tx, runID string) (bool, string, error) {
		return true, "unwired placeholder: " + why, nil
	}
}

func scalarInt(ctx context.Context, tx *sql.Tx, q string, args ...any) (int, error) {
	var n int
	e := tx.QueryRowContext(ctx, q, args...).Scan(&n)
	return n, e
}

// 1) 没有非终态 Job（DONE/CANCELLED/FAILED 之外都算未终结）。
func condNoNonterminalJob(ctx context.Context, tx *sql.Tx, runID string) (bool, string, error) {
	n, e := scalarInt(ctx, tx,
		`SELECT COUNT(*) FROM jobs WHERE run_id=? AND state NOT IN ('DONE','CANCELLED','FAILED')`, runID)
	if e != nil {
		return false, "", e
	}
	return n == 0, fmt.Sprintf("%d 个非终态 Job", n), nil
}

// 2) 没有未处理 OUTCOME_UNKNOWN（Job 进 RECONCILING 态表示有未决 OUTCOME_UNKNOWN，§5.2）。
func condNoOutcomeUnknown(ctx context.Context, tx *sql.Tx, runID string) (bool, string, error) {
	n, e := scalarInt(ctx, tx,
		`SELECT COUNT(*) FROM jobs WHERE run_id=? AND state='RECONCILING'`, runID)
	if e != nil {
		return false, "", e
	}
	return n == 0, fmt.Sprintf("%d 个 Job 处于 RECONCILING（未决 OUTCOME_UNKNOWN）", n), nil
}

// 4) 没有高价值 OPEN/FOLLOWING Discovery（§12.4）。
func condNoHighValueOpenDiscovery(ctx context.Context, tx *sql.Tx, runID string) (bool, string, error) {
	n, e := scalarInt(ctx, tx,
		`SELECT COUNT(*) FROM discoveries WHERE run_id=? AND high_value=1 AND state IN ('OPEN','FOLLOWING')`, runID)
	if e != nil {
		return false, "", e
	}
	return n == 0, fmt.Sprintf("%d 个高价值未处置 Discovery", n), nil
}

//  5. 没有未处理 Hypothesis（OPEN 态）。注：schema 无优先级列，此处保守地把所有 OPEN 视为需处置
//     （§14.2「未处理的高优先级 Hypothesis」的保守上界），避免带着开放假设就宣布完成。
func condNoOpenHypothesis(ctx context.Context, tx *sql.Tx, runID string) (bool, string, error) {
	n, e := scalarInt(ctx, tx,
		`SELECT COUNT(*) FROM hypotheses WHERE run_id=? AND state='OPEN'`, runID)
	if e != nil {
		return false, "", e
	}
	return n == 0, fmt.Sprintf("%d 个 OPEN Hypothesis 未处置", n), nil
}

// 6) 覆盖率达到阈值：(TESTED+NA)/total ≥ 阈值%。无单元格时视为达标（无面可测）。
func condCoverageThreshold(thresholdPct int) CheckFunc {
	return func(ctx context.Context, tx *sql.Tx, runID string) (bool, string, error) {
		total, e := scalarInt(ctx, tx, `SELECT COUNT(*) FROM coverage_cells WHERE run_id=?`, runID)
		if e != nil {
			return false, "", e
		}
		if total == 0 {
			return true, "无覆盖单元格", nil
		}
		covered, e := scalarInt(ctx, tx,
			`SELECT COUNT(*) FROM coverage_cells WHERE run_id=? AND state IN ('TESTED','NA')`, runID)
		if e != nil {
			return false, "", e
		}
		pct := covered * 100 / total
		return pct >= thresholdPct, fmt.Sprintf("覆盖率 %d%%（%d/%d），阈值 %d%%", pct, covered, total, thresholdPct), nil
	}
}

// 7) N/A 质量：每个 NA 单元格必须有 rationale（排除原因），否则质量不合格。
func condNAQuality(ctx context.Context, tx *sql.Tx, runID string) (bool, string, error) {
	n, e := scalarInt(ctx, tx,
		`SELECT COUNT(*) FROM coverage_cells WHERE run_id=? AND state='NA' AND (rationale IS NULL OR rationale='')`, runID)
	if e != nil {
		return false, "", e
	}
	return n == 0, fmt.Sprintf("%d 个 NA 单元格缺排除依据", n), nil
}

//  8. Finding 全部有验证结论：已被声明为 Finding（CONFIRMED/TRIAGED/REPORTABLE/NEEDS_EVIDENCE）却无 active
//     verdict 的记为未验证。HYPOTHESIS/CANDIDATE（尚在推进）与 REJECTED（已处置）不计。
func condFindingsHaveVerdict(ctx context.Context, tx *sql.Tx, runID string) (bool, string, error) {
	n, e := scalarInt(ctx, tx, `
		SELECT COUNT(*) FROM findings f
		WHERE f.run_id=? AND f.state IN ('CONFIRMED','TRIAGED','REPORTABLE','NEEDS_EVIDENCE')
		  AND NOT EXISTS (SELECT 1 FROM verification_verdicts v WHERE v.finding_id=f.id AND v.superseded=0)`, runID)
	if e != nil {
		return false, "", e
	}
	return n == 0, fmt.Sprintf("%d 个 Finding 缺验证结论", n), nil
}

// 9) 没有待处理 Quarantine。
func condNoPendingQuarantine(ctx context.Context, tx *sql.Tx, runID string) (bool, string, error) {
	n, e := scalarInt(ctx, tx,
		`SELECT COUNT(*) FROM evidence WHERE run_id=? AND COALESCE(quarantined,0)=1`, runID)
	if e != nil {
		return false, "", e
	}
	return n == 0, fmt.Sprintf("%d 份证据待隔离处理", n), nil
}

// 10) 没有待处理审批（approvals state=PENDING）。
func condNoPendingApproval(ctx context.Context, tx *sql.Tx, runID string) (bool, string, error) {
	n, e := scalarInt(ctx, tx,
		`SELECT COUNT(*) FROM approvals WHERE run_id=? AND state='PENDING'`, runID)
	if e != nil {
		return false, "", e
	}
	return n == 0, fmt.Sprintf("%d 个待处理审批", n), nil
}

// 11) 必要补测 Epoch 已执行：没有残留 STALE 单元格（SurfaceRevision 变化后的补测已做）。
func condNoStaleCells(ctx context.Context, tx *sql.Tx, runID string) (bool, string, error) {
	n, e := scalarInt(ctx, tx,
		`SELECT COUNT(*) FROM coverage_cells WHERE run_id=? AND state='STALE'`, runID)
	if e != nil {
		return false, "", e
	}
	return n == 0, fmt.Sprintf("%d 个 STALE 单元格待补测", n), nil
}

// 14) 报告 Manifest 可生成：所有 REPORTABLE Finding 都有 active verdict 且 ≥1 条逐字引用（事实有来源）。
func condReportManifestBuildable(ctx context.Context, tx *sql.Tx, runID string) (bool, string, error) {
	noVerdict, e := scalarInt(ctx, tx, `
		SELECT COUNT(*) FROM findings f
		WHERE f.run_id=? AND f.state='REPORTABLE'
		  AND NOT EXISTS (SELECT 1 FROM verification_verdicts v WHERE v.finding_id=f.id AND v.superseded=0)`, runID)
	if e != nil {
		return false, "", e
	}
	noCite, e := scalarInt(ctx, tx, `
		SELECT COUNT(*) FROM findings f
		WHERE f.run_id=? AND f.state='REPORTABLE'
		  AND NOT EXISTS (SELECT 1 FROM evidence_citations c WHERE c.finding_id=f.id)`, runID)
	if e != nil {
		return false, "", e
	}
	ok := noVerdict == 0 && noCite == 0
	return ok, fmt.Sprintf("%d 个 REPORTABLE 缺结论、%d 个缺引用", noVerdict, noCite), nil
}
