package providers

import (
	"errors"
	"sync"
)

// registry.go —— Provider 注册表 + 角色 Model Profile（§7.4 profiles）。
//
// 角色（strategist/executor/verifier）→ Profile（provider+model+effort）→ 具体 Provider。
// 「按角色 Model Profile」借鉴 Pentagi（§49）：不同角色可绑不同模型/档位。

// ErrNoProvider / ErrNoProfile 为解析失败。
var (
	ErrNoProvider = errors.New("providers: provider 未注册")
	ErrNoProfile  = errors.New("providers: 角色 profile 未配置")
)

// Profile 是一个角色的模型画像（§7.4）。
type Profile struct {
	Provider string // 注册表中的 Provider 名
	Model    string // 具体模型
	Effort   string // 推理档位 low/medium/high
}

// Registry 持有已注册 Provider 与角色 Profile。线程安全。
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
	profiles  map[string]Profile // role -> profile
}

// NewRegistry 构造空注册表。
func NewRegistry() *Registry {
	return &Registry{providers: map[string]Provider{}, profiles: map[string]Profile{}}
}

// Register 以名字注册一个 Provider（重名覆盖）。
func (r *Registry) Register(name string, p Provider) {
	r.mu.Lock()
	r.providers[name] = p
	r.mu.Unlock()
}

// Get 取一个已注册 Provider。
func (r *Registry) Get(name string) (Provider, bool) {
	r.mu.RLock()
	p, ok := r.providers[name]
	r.mu.RUnlock()
	return p, ok
}

// SetProfile 配置某角色的 Profile。
func (r *Registry) SetProfile(role string, pr Profile) {
	r.mu.Lock()
	r.profiles[role] = pr
	r.mu.Unlock()
}

// ForRole 解析角色 → (Provider, Profile)。角色未配置或其 Provider 未注册即报错。
func (r *Registry) ForRole(role string) (Provider, Profile, error) {
	r.mu.RLock()
	pr, ok := r.profiles[role]
	p, pok := r.providers[pr.Provider]
	r.mu.RUnlock()
	if !ok {
		return nil, Profile{}, ErrNoProfile
	}
	if !pok {
		return nil, Profile{}, ErrNoProvider
	}
	return p, pr, nil
}
