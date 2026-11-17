package surface

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"
)

// spec.go —— 规格驱动生成（§31 E3）：发现 OpenAPI/Swagger/GraphQL 规格后，**确定性**生成端点×参数矩阵，
// 模型只挑「哪些值得测」。纯解析、无 I/O，故确定性可测。

// openAPIDoc 是 OpenAPI/Swagger 的最小解析结构（只取生成端点矩阵所需字段）。
type openAPIDoc struct {
	Paths map[string]map[string]openAPIOp `json:"paths"`
}

type openAPIOp struct {
	Parameters  []openAPIParam `json:"parameters"`
	RequestBody *struct {
		Content map[string]struct {
			Schema struct {
				Properties map[string]struct {
					Type string `json:"type"`
				} `json:"properties"`
				Required []string `json:"required"`
			} `json:"schema"`
		} `json:"content"`
	} `json:"requestBody"`
}

type openAPIParam struct {
	Name     string `json:"name"`
	In       string `json:"in"`
	Required bool   `json:"required"`
	Schema   struct {
		Type string `json:"type"`
	} `json:"schema"`
}

var httpMethods = map[string]bool{"get": true, "post": true, "put": true, "patch": true, "delete": true, "head": true, "options": true}

// FromOpenAPI 解析 OpenAPI/Swagger 文档，确定性生成端点×参数矩阵（Source=spec）。
// 端点按 (path, method) 稳定排序，保证确定性。
func FromOpenAPI(doc []byte) ([]Endpoint, error) {
	var d openAPIDoc
	if err := json.Unmarshal(doc, &d); err != nil {
		return nil, err
	}
	var eps []Endpoint
	paths := make([]string, 0, len(d.Paths))
	for p := range d.Paths {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		ops := d.Paths[p]
		methods := make([]string, 0, len(ops))
		for m := range ops {
			methods = append(methods, m)
		}
		sort.Strings(methods)
		for _, m := range methods {
			if !httpMethods[strings.ToLower(m)] {
				continue
			}
			op := ops[m]
			ep := Endpoint{Method: strings.ToUpper(m), Path: p, Source: SrcSpec}
			for _, pa := range op.Parameters {
				ep.Params = append(ep.Params, Param{
					Name: pa.Name, Location: ParamLocation(pa.In), ValueType: pa.Schema.Type, Required: pa.Required,
				})
			}
			if op.RequestBody != nil {
				for _, c := range op.RequestBody.Content {
					reqd := map[string]bool{}
					for _, r := range c.Schema.Required {
						reqd[r] = true
					}
					names := make([]string, 0, len(c.Schema.Properties))
					for n := range c.Schema.Properties {
						names = append(names, n)
					}
					sort.Strings(names)
					for _, n := range names {
						ep.Params = append(ep.Params, Param{
							Name: n, Location: InBody, ValueType: c.Schema.Properties[n].Type, Required: reqd[n],
						})
					}
					break // 取第一个 content type（确定性由上面的 sort 保证字段序）
				}
			}
			eps = append(eps, Normalize(ep))
		}
	}
	return eps, nil
}

var (
	reGQLType = regexp.MustCompile(`(?s)type\s+(Query|Mutation)\s*\{([^}]*)\}`)
	// 字段：名 + 可选 args（整段括号被吞掉，避免把 arg 当字段）+ 冒号 + 类型起始（\[? \w）。不锚行首，支持同行多字段。
	reGQLField = regexp.MustCompile(`(\w+)\s*(?:\(([^()]*)\))?\s*:\s*\[?\w`)
	reGQLArg   = regexp.MustCompile(`(\w+)\s*:\s*(\w+)(!?)`)
)

// FromGraphQL 轻量解析 GraphQL SDL 的 Query/Mutation 字段，确定性生成端点（POST /graphql，操作名为
// url_pattern 后缀，Source=spec），字段参数作为 body 参数。无需完整 GraphQL 解析器（stdlib 正则，§12.1）。
func FromGraphQL(schema string) []Endpoint {
	var eps []Endpoint
	blocks := reGQLType.FindAllStringSubmatch(schema, -1)
	for _, blk := range blocks {
		kind, body := blk[1], blk[2]
		method := "POST"
		fields := reGQLField.FindAllStringSubmatch(body, -1)
		for _, f := range fields {
			name, args := f[1], f[2]
			// 用路径段编码操作（/graphql/<kind>.<field>），经归一化保持各操作可区分、用于覆盖率矩阵。
			// 真实请求仍是 POST /graphql（body 里带操作），此为覆盖建模的合成表示。
			ep := Endpoint{Method: method, Path: "/graphql/" + strings.ToLower(kind) + "." + name, Source: SrcSpec}
			for _, a := range reGQLArg.FindAllStringSubmatch(args, -1) {
				ep.Params = append(ep.Params, Param{
					Name: a[1], Location: InBody, ValueType: a[2], Required: a[3] == "!",
				})
			}
			eps = append(eps, Normalize(ep))
		}
	}
	// 稳定排序。
	sort.Slice(eps, func(i, j int) bool { return eps[i].URLPattern < eps[j].URLPattern })
	return eps
}
