package scheduler

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
)

// dedupe.go —— 动作指纹去重（§6.9），默认**软降权**，硬拒只留字节级全重复（§2.3-2 / §31 A6：别杀 AI 思考）。
//
// 语义相近的动作（同指纹）不是「没用」——AI 换个角度重试可能有价值，故只降权（惩罚随重复次数增长），
// 让它在评分里自然靠后；只有**字节级完全重复**（同指纹 + 同原始载荷）才硬拒，因为那真的没有任何新东西。

// ActionFingerprint 是 §6.9 的动作指纹五元组。
type ActionFingerprint struct {
	NormalizedEndpoint     string
	Identity               string
	AttackType             string
	Parameter              string
	NormalizedPayloadClass string
}

// Hash 返回指纹的稳定摘要。
func (a ActionFingerprint) Hash() string {
	h := sha256.Sum256([]byte(a.NormalizedEndpoint + "\x00" + a.Identity + "\x00" + a.AttackType + "\x00" + a.Parameter + "\x00" + a.NormalizedPayloadClass))
	return hex.EncodeToString(h[:])
}

// DedupeDecision 去重判定。
type DedupeDecision int

const (
	DedupeAllow          DedupeDecision = iota // 新动作，放行
	DedupeSoftDownweight                       // 同指纹重复，软降权（penalty>0）
	DedupeHardReject                           // 字节级全重复，硬拒
)

func (d DedupeDecision) String() string {
	switch d {
	case DedupeSoftDownweight:
		return "soft_downweight"
	case DedupeHardReject:
		return "hard_reject"
	default:
		return "allow"
	}
}

// Deduper 记录已见指纹与字节级重复。线程安全。
type Deduper struct {
	mu    sync.Mutex
	seen  map[string]int  // 指纹 hash -> 已见次数
	bytes map[string]bool // 指纹hash:载荷hash -> 是否见过（字节级全重复）
}

// NewDeduper 构造。
func NewDeduper() *Deduper {
	return &Deduper{seen: map[string]int{}, bytes: map[string]bool{}}
}

// Check 判定一个动作：字节级全重复→硬拒；同指纹非字节重复→软降权（penalty=已重复次数）；全新→放行。
func (d *Deduper) Check(fp ActionFingerprint, rawPayload []byte) (DedupeDecision, float64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	fph := fp.Hash()
	ph := sha256.Sum256(rawPayload)
	byteKey := fph + ":" + hex.EncodeToString(ph[:])

	if d.bytes[byteKey] {
		return DedupeHardReject, 0 // 字节级全重复：真的没新东西
	}
	d.bytes[byteKey] = true

	n := d.seen[fph]
	d.seen[fph]++
	if n == 0 {
		return DedupeAllow, 0
	}
	return DedupeSoftDownweight, float64(n) // 惩罚随同指纹重复次数增长
}
