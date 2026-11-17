package findings

import (
	"math"
	"strings"
)

// severity.go —— CVSS 向量确定性计算（§31 V7）：程序从 Finding 结构化字段算 CVSS v3.1 基础分 + 向量 +
// 严重度分档，**AI 只写描述**、不自报分数。CVSS 是其官方公式本身用浮点评分的算法（与 §31 C7「金额/Token
// 不碰 float」无关，此处浮点是 CVSS 规范的正确实现）。

// CVSSv31 是 CVSS 3.1 基础指标组（取值用官方缩写）。
type CVSSv31 struct {
	AV string // N/A/L/P  攻击途径
	AC string // L/H      攻击复杂度
	PR string // N/L/H    所需权限
	UI string // N/R      用户交互
	S  string // U/C      影响范围
	C  string // N/L/H    机密性影响
	I  string // N/L/H    完整性影响
	A  string // N/L/H    可用性影响
}

func avVal(s string) float64 {
	switch s {
	case "N":
		return 0.85
	case "A":
		return 0.62
	case "L":
		return 0.55
	case "P":
		return 0.2
	}
	return 0.85
}

func acVal(s string) float64 {
	if s == "H" {
		return 0.44
	}
	return 0.77
}

func prVal(pr, scope string) float64 {
	changed := scope == "C"
	switch pr {
	case "N":
		return 0.85
	case "L":
		if changed {
			return 0.68
		}
		return 0.62
	case "H":
		if changed {
			return 0.5
		}
		return 0.27
	}
	return 0.85
}

func uiVal(s string) float64 {
	if s == "R" {
		return 0.62
	}
	return 0.85
}

func ciaVal(s string) float64 {
	switch s {
	case "H":
		return 0.56
	case "L":
		return 0.22
	}
	return 0.0
}

// roundup 按 CVSS 3.1 定义向上取整到 1 位小数。
func roundup(x float64) float64 {
	return math.Ceil(x*10) / 10
}

// BaseScore 计算 CVSS v3.1 基础分（§31 V7，确定性）。
func (m CVSSv31) BaseScore() float64 {
	iss := 1 - (1-ciaVal(m.C))*(1-ciaVal(m.I))*(1-ciaVal(m.A))
	var impact float64
	changed := m.S == "C"
	if changed {
		impact = 7.52*(iss-0.029) - 3.25*math.Pow(iss-0.02, 15)
	} else {
		impact = 6.42 * iss
	}
	expl := 8.22 * avVal(m.AV) * acVal(m.AC) * prVal(m.PR, m.S) * uiVal(m.UI)
	if impact <= 0 {
		return 0.0
	}
	var base float64
	if changed {
		base = math.Min(1.08*(impact+expl), 10)
	} else {
		base = math.Min(impact+expl, 10)
	}
	return roundup(base)
}

// Severity 按基础分给出分档（§31 V7）。
func (m CVSSv31) Severity() string {
	s := m.BaseScore()
	switch {
	case s == 0:
		return "None"
	case s < 4.0:
		return "Low"
	case s < 7.0:
		return "Medium"
	case s < 9.0:
		return "High"
	default:
		return "Critical"
	}
}

// Vector 返回规范化的 CVSS 3.1 向量串。
func (m CVSSv31) Vector() string {
	return strings.Join([]string{
		"CVSS:3.1", "AV:" + m.AV, "AC:" + m.AC, "PR:" + m.PR, "UI:" + m.UI,
		"S:" + m.S, "C:" + m.C, "I:" + m.I, "A:" + m.A,
	}, "/")
}
