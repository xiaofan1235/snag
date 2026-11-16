package main

import "testing"

// gen_test.go —— Q-12：生成器类型映射遇未知类型必须报错，不静默回退 string。

func TestTypeMappersRejectUnknown(t *testing.T) {
	if _, err := goScalar("integre"); err == nil { // 拼错 integer
		t.Error("goScalar 对未知标量类型应报错")
	}
	if _, err := tsScalar("stringg"); err == nil {
		t.Error("tsScalar 对未知标量类型应报错")
	}
	if _, err := goType(prop{Type: "objekt"}); err == nil {
		t.Error("goType 对未知 Schema 类型应报错")
	}
	// 数组元素类型拼错也应报错（递归传播）。
	if _, err := goType(prop{Type: "array", Items: &prop{Type: "bogus"}}); err == nil {
		t.Error("goType 对数组元素未知类型应报错")
	}
}

func TestTypeMappersAcceptKnown(t *testing.T) {
	cases := map[string]string{"string": "string", "integer": "int64", "number": "float64", "boolean": "bool"}
	for in, want := range cases {
		got, err := goScalar(in)
		if err != nil || got != want {
			t.Errorf("goScalar(%q)=%q,%v; 期望 %q,nil", in, got, err, want)
		}
	}
	if got, err := goType(prop{Ref: "#/components/schemas/Run"}); err != nil || got != "Run" {
		t.Errorf("goType($ref)=%q,%v; 期望 Run,nil", got, err)
	}
	if got, err := goType(prop{Type: "array", Items: &prop{Type: "string"}}); err != nil || got != "[]string" {
		t.Errorf("goType([]string)=%q,%v", got, err)
	}
}
