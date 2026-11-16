package schemas_test

import (
	"bytes"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"pentagent/api/schemas"
)

// schema_test.go —— 每个契约 Schema 自身必须是合法、可编译的 JSON Schema（Day 04）。
// 命令覆盖：go test ./api/...

func TestAllSchemasCompile(t *testing.T) {
	for _, name := range schemas.All {
		b, err := schemas.FS.ReadFile(name)
		if err != nil {
			t.Errorf("读取 %s: %v", name, err)
			continue
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
		if err != nil {
			t.Errorf("%s 不是合法 JSON: %v", name, err)
			continue
		}
		c := jsonschema.NewCompiler()
		if err := c.AddResource("mem://"+name, doc); err != nil {
			t.Errorf("%s AddResource: %v", name, err)
			continue
		}
		if _, err := c.Compile("mem://" + name); err != nil {
			t.Errorf("%s 不是合法 JSON Schema: %v", name, err)
		}
	}
	if len(schemas.All) != 5 {
		t.Errorf("契约 Schema 应为 5 个, 得到 %d", len(schemas.All))
	}
}
