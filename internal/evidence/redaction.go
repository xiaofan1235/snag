package evidence

import (
	"bytes"
	"regexp"
)

// redaction.go —— 脱敏投影（redacted_blob_path）：给模型/普通 UI，绝不含秘密（§13）。

var (
	bearerRe = regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9\-._~+/]+=*`)
	pwRe     = regexp.MustCompile(`(?i)("?(?:password|passwd|secret|api[_-]?key|token)"?\s*[:=]\s*")[^"]+(")`)
	setCook  = regexp.MustCompile(`(?i)(set-cookie:\s*[^=]+=)[^;\r\n]+`)
	cookieRe = regexp.MustCompile(`(?i)(cookie:\s*)[^\r\n]+`)
	authRe   = regexp.MustCompile(`(?i)(authorization:\s*)[^\r\n]+`)
)

// Redact 产出 CEBS 的脱敏投影：先替换调用方提供的字面秘密（最可靠——就是本动作用到的凭据），
// 再套通用模式（Bearer / password / token / cookie / authorization）。
func Redact(cebs []byte, secrets []string) []byte {
	out := make([]byte, len(cebs))
	copy(out, cebs)
	for _, s := range secrets {
		if s != "" {
			out = bytes.ReplaceAll(out, []byte(s), []byte("[REDACTED]"))
		}
	}
	out = bearerRe.ReplaceAll(out, []byte("${1}[REDACTED]"))
	out = pwRe.ReplaceAll(out, []byte("${1}[REDACTED]${2}"))
	out = setCook.ReplaceAll(out, []byte("${1}[REDACTED]"))
	out = cookieRe.ReplaceAll(out, []byte("${1}[REDACTED]"))
	out = authRe.ReplaceAll(out, []byte("${1}[REDACTED]"))
	return out
}
