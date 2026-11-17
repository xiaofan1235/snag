package sandbox_test

import (
	"testing"

	"pentagent/internal/sandbox"
)

// sandbox_test.go —— Day 19：沙箱硬化默认 + attestation 分级门禁（§11.1/§31 P5x）。

func TestHardenedDefault(t *testing.T) {
	p := sandbox.HardenedDefault()
	if !p.NonRoot || !p.CapDropAll || !p.ReadOnlyRootfs || !p.NoDockerSocket || !p.NoHostNetwork || !p.NoDefaultRoute {
		t.Errorf("硬化默认应全开（§11.1），得 %+v", p)
	}
}

func TestAttestationGating(t *testing.T) {
	cases := []struct {
		att   sandbox.Attestation
		allow bool
	}{
		{sandbox.AttestFull, true},
		{sandbox.AttestPartial, true},
		{sandbox.AttestNone, false},
		{sandbox.AttestUnknown, false},
	}
	for _, c := range cases {
		m := sandbox.NewManager(fixedSandbox{c.att})
		if m.AllowActiveTesting() != c.allow {
			t.Errorf("attestation=%s 应 allow=%v", c.att, c.allow)
		}
	}
}

type fixedSandbox struct{ a sandbox.Attestation }

func (f fixedSandbox) Attestation() sandbox.Attestation { return f.a }
func (f fixedSandbox) Profile() sandbox.Profile         { return sandbox.Profile{} }
