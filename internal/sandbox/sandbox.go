// Package sandbox 实现分级沙箱与 attestation（§11 沙箱 / §31 P5x 分级沙箱）。
//
// 隔离执行面：有容器用容器（seccomp+Landlock/AppContainer/Seatbelt，§11.1），没有就降级；
// 以 attestation 分级（FULL/PARTIAL/NONE/UNKNOWN）作门禁——NONE/UNKNOWN 一律 fail-closed 禁主动测试
// （§31 P5x）。容器默认 non-root / cap_drop ALL / read_only_rootfs / no_docker_socket /
// no_host_network / 无默认路由（唯一出口 Policy Proxy，§11.1）。
//
// 【有据偏离】dev-plan 的 Runner↔Core 走 gRPC+Protobuf，但本环境网络受限无法引入 grpc/protobuf 依赖；
// 改为进程内 Go 接口（见 runners 包），接口按 gRPC 形状设计，日后可平滑包一层 gRPC 传输。
// 本环境亦无法真正拉起容器，故 Day 19 落「降级进程沙箱 + attestation 分级 + 门禁」骨架（§4「先支持
// 降级进程沙箱即可跑起来」）；真容器与各平台隔离原语随部署环境接入。
package sandbox

// Attestation 是沙箱隔离强度证明（§31 P5x）。
type Attestation string

const (
	AttestFull    Attestation = "FULL"    // 完整容器隔离（seccomp+LSM+无默认路由）
	AttestPartial Attestation = "PARTIAL" // 降级：部分隔离（如进程级 + 代理唯一出口，无内核级强隔离）
	AttestNone    Attestation = "NONE"    // 无隔离
	AttestUnknown Attestation = "UNKNOWN" // 无法确定
)

// Profile 是沙箱安全配置（§11.1 容器默认）。
type Profile struct {
	NonRoot        bool
	CapDropAll     bool
	ReadOnlyRootfs bool
	NoDockerSocket bool
	NoHostNetwork  bool
	NoDefaultRoute bool // 无默认路由，唯一出口 Policy Proxy
}

// HardenedDefault 返回 §11.1 的硬化默认配置。
func HardenedDefault() Profile {
	return Profile{
		NonRoot: true, CapDropAll: true, ReadOnlyRootfs: true,
		NoDockerSocket: true, NoHostNetwork: true, NoDefaultRoute: true,
	}
}

// Sandbox 是一个隔离执行环境。
type Sandbox interface {
	Attestation() Attestation
	Profile() Profile
}

// DegradedSandbox 是降级进程沙箱（无内核级强隔离，但强制代理唯一出口）。attestation=PARTIAL。
type DegradedSandbox struct {
	profile Profile
}

// NewDegradedSandbox 构造降级沙箱（应用硬化默认中可在进程级落实的部分）。
func NewDegradedSandbox() *DegradedSandbox {
	return &DegradedSandbox{profile: Profile{NoDefaultRoute: true, NonRoot: true}}
}

func (s *DegradedSandbox) Attestation() Attestation { return AttestPartial }
func (s *DegradedSandbox) Profile() Profile         { return s.profile }

// Manager 选择沙箱并据 attestation 做主动测试门禁。
type Manager struct {
	sb Sandbox
}

// NewManager 用给定沙箱构造（生产按环境探测容器能力；本环境注入降级沙箱）。
func NewManager(sb Sandbox) *Manager { return &Manager{sb: sb} }

// Attestation 透出当前沙箱证明。
func (m *Manager) Attestation() Attestation { return m.sb.Attestation() }

// Sandbox 返回底层沙箱。
func (m *Manager) Sandbox() Sandbox { return m.sb }

// AllowActiveTesting 报告当前 attestation 是否允许主动测试（§31 P5x：NONE/UNKNOWN fail-closed）。
func (m *Manager) AllowActiveTesting() bool {
	switch m.sb.Attestation() {
	case AttestFull, AttestPartial:
		return true
	default: // NONE / UNKNOWN / 其它
		return false
	}
}
