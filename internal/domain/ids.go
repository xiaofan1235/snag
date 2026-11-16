package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

// ids.go —— 领域标识符（§2.1 ID 用 UUIDv7；Day 01 §4）。
//
// 为什么自实现而不引 github.com/google/uuid：
//   - domain 层要保持零外部依赖（§3「先稳定 Domain」），且本项目可能在无网络环境构建；
//   - UUIDv7（RFC 9562）本身很小，自实现即可，还能顺带保证「同一毫秒内单调」。
//
// ID 为何用 UUIDv7 而非随机 UUIDv4：
//   - 前 48 位是毫秒时间戳，天然「时间有序」，做主键时写入局部性好、便于按时间范围扫描；
//   - 字典序 ≈ 时间序，事件/证据按 id 排序即近似按发生顺序排序。

// ID 是所有实体主键与外键的类型。底层是 UUID 字符串（规范 36 字符 8-4-4-4-12）。
type ID string

func (id ID) String() string { return string(id) }
func (id ID) IsZero() bool   { return id == "" }

var (
	idMu     sync.Mutex
	idLastMs uint64
	idSeq    uint16 // 12 位序号，放入 rand_a，保证同毫秒单调
)

// NewID 返回一个新的、时间有序且单调递增的 UUIDv7。
//
// 单调性保证：同一毫秒内连续调用，序号自增；序号用尽（4096 个/ms）时把时间戳 +1ms 借位，
// 因此在同一进程内「后生成的 id 字典序严格大于先生成的」恒成立——满足「Run/事件时间有序」不变量。
func NewID() ID {
	idMu.Lock()
	ms := uint64(time.Now().UnixMilli())
	if ms > idLastMs {
		idLastMs = ms
		idSeq = 0
	} else {
		// 时钟回拨或同毫秒：沿用 last，序号自增
		idSeq++
		if idSeq > 0x0FFF {
			idLastMs++ // 借位到下一毫秒
			idSeq = 0
		}
		ms = idLastMs
	}
	seq := idSeq
	idMu.Unlock()

	var b [16]byte
	// 48 位毫秒时间戳（大端）
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)
	// 填充随机到 b[6..15]
	_, _ = rand.Read(b[6:])
	// rand_a（12 位）放单调序号：b[6] 低 4 位 + b[7] 全 8 位
	b[6] = byte(seq >> 8 & 0x0F)
	b[7] = byte(seq)
	// 版本 7 写入 b[6] 高 4 位
	b[6] = (b[6] & 0x0F) | 0x70
	// 变体 10 写入 b[8] 高 2 位
	b[8] = (b[8] & 0x3F) | 0x80
	return ID(formatUUID(b))
}

// Key 对相同输入返回相同 id（确定性），用于幂等：同一逻辑动作多次提议得到同一 Action id，
// 避免重复副作用（§4.1 actions idempotency 约束）。
//
// 实现：sha256(parts) 取前 16 字节，打上 UUID 版本 8（自定义/确定性）与变体位。
// 分隔符用 0x1F（单元分隔符），正常文本不含，避免 ["a","bc"] 与 ["ab","c"] 碰撞。
func Key(parts ...string) ID {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x1f")))
	var b [16]byte
	copy(b[:], sum[:16])
	b[6] = (b[6] & 0x0F) | 0x80 // 版本 8：name-based / 自定义
	b[8] = (b[8] & 0x3F) | 0x80 // 变体 10
	return ID(formatUUID(b))
}

// formatUUID 把 16 字节格式化为规范 UUID 字符串 8-4-4-4-12。
func formatUUID(b [16]byte) string {
	var h [32]byte
	hex.Encode(h[:], b[:])
	var out [36]byte
	copy(out[0:8], h[0:8])
	out[8] = '-'
	copy(out[9:13], h[8:12])
	out[13] = '-'
	copy(out[14:18], h[12:16])
	out[18] = '-'
	copy(out[19:23], h[16:20])
	out[23] = '-'
	copy(out[24:36], h[20:32])
	return string(out[:])
}
