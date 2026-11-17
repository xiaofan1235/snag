package evidence

import "testing"

func allPass() DetChecks {
	return DetChecks{
		EvidenceHashOK: true, ActionExecuted: true, SameActionReqResp: true, IdentityMatch: true,
		RolesComplete: true, PolicyValid: true, CitationsComplete: true, NoContraryEvidence: true,
	}
}

// 确定性验证器：全过→pass；任一不过→拒并列出原因（§13.4）。
func TestDeterministicVerify(t *testing.T) {
	if ok, _ := DeterministicVerify(allPass()); !ok {
		t.Fatal("全项通过应 pass")
	}
	c := allPass()
	c.IdentityMatch = false
	ok, reasons := DeterministicVerify(c)
	if ok || len(reasons) != 1 || reasons[0] != "identity_match" {
		t.Fatalf("身份不匹配应拒并列出原因，实得 ok=%v reasons=%v", ok, reasons)
	}
	// 存在相反证据也必须拒。
	c2 := allPass()
	c2.NoContraryEvidence = false
	if ok, _ := DeterministicVerify(c2); ok {
		t.Fatal("有相反证据应拒")
	}
}

type fakeSem struct{ v SemanticVerdict }

func (f fakeSem) Verify(detPassed bool, _ []byte) (SemanticVerdict, error) { return f.v, nil }

func passVerdict() SemanticVerdict {
	return SemanticVerdict{
		DataSensitive: true, TrulyCrossIdentity: true, DiffSupportsClaim: true,
		NoAltExplanation: true, PreconditionHonest: true, SeverityNotInflated: true,
	}
}

// DoD：确定性+语义两层都过→CONFIRMED；语义层**不能绕过**确定性层。
func TestVerifyCombined(t *testing.T) {
	// 确定性不过：即便语义全过也不 Confirmed（语义不能救）。
	detFail := allPass()
	detFail.IdentityMatch = false
	if res, _ := Verify(detFail, fakeSem{v: passVerdict()}, nil); res.Confirmed {
		t.Fatal("确定性不过时语义层不应绕过")
	}
	// 两层都过 → Confirmed。
	if res, _ := Verify(allPass(), fakeSem{v: passVerdict()}, nil); !res.Confirmed {
		t.Fatal("两层都过应 Confirmed")
	}
	// 确定性过但语义不过（如定级过高）→ 不 Confirmed。
	bad := passVerdict()
	bad.SeverityNotInflated = false
	if res, _ := Verify(allPass(), fakeSem{v: bad}, nil); res.Confirmed {
		t.Fatal("语义层不过不应 Confirmed")
	}
}
