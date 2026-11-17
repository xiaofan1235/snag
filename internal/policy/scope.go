package policy

import (
	"fmt"
	"net/netip"
	"net/url"
	"path"
	"strconv"
	"strings"
)

// scope.go —— URL/Scope 规范化器（§9.3/§9.0.4/§31 P2x）。红线的地基。
//
// §9.0.4：红线唯一真实攻击面就是这个规范化器——做不对，后面全套红线白搭。处理 §9.3 全清单：
// 大小写、末尾点、默认端口、Userinfo 欺骗、多重 URL 编码、IPv4 异常表示（八进制/十六进制/十进制/短式）、
// IPv6、协议/端口变化。硬属性：Canonicalize **幂等**；等价变形 inScope 判定一致；**精确域名绝不匹配子域**；
// 解析分歧一律 **fail-closed 到更严那侧**（§31 P2x）。
//
// 【Q-20 一致性不变量】Canonicalize 的输出经 PDP 的 net/url 再解析必须幂等（host/scheme/port 三元组不变）——
// 否则会出现「PDP 判放行、Proxy 解析到别处」的绕过。Day 18 Policy Proxy 必须复用本函数做同源解析。
//
// 【无 IDN 库的取舍】本构建无 golang.org/x/net/idna（网络受限无法引入）：非 ASCII host 一律 fail-closed
// 拒绝（要求调用方预先 punycode）。生产接入 idna 后在此补 ToASCII。

// Canonical 是规范化后的 URL 权威三元组（+ 清洗后的 path）。scope 匹配只看 scheme/host/port。
type Canonical struct {
	Scheme string
	Host   string // 小写、去末尾点、IP 规范化（含异常 IPv4）
	Port   int    // 有效端口（显式或按 scheme 推默认）
	Path   string
}

// String 返回稳定规范串：scheme://host[:非默认端口]/path。IPv6 host 加方括号。
// 用 net/url 构造以保证 path 正确转义——与再解析互逆（Q-20 幂等前提）。
func (c Canonical) String() string {
	h := c.Host
	if strings.Contains(h, ":") { // IPv6
		h = "[" + h + "]"
	}
	if !(c.Scheme == "https" && c.Port == 443) && !(c.Scheme == "http" && c.Port == 80) {
		h = h + ":" + strconv.Itoa(c.Port)
	}
	p := c.Path
	if p == "" {
		p = "/"
	}
	u := url.URL{Scheme: c.Scheme, Host: h, Path: p}
	return u.String()
}

// Canonicalize 把原始 URL 规范化（§9.3）。任何解析歧义/不支持的形态一律报错（fail-closed）。
func Canonicalize(raw string) (Canonical, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Canonical{}, fmt.Errorf("空 URL")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return Canonical{}, fmt.Errorf("URL 解析失败: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return Canonical{}, fmt.Errorf("不支持的 scheme %q（仅 http/https）", u.Scheme)
	}
	// net/url 已把 Userinfo 与端口从 host 分离：真实 host = Hostname()（userinfo 欺骗无效）。
	// 去**全部**末尾点（FQDN 根 + 多重点），小写。
	host := strings.ToLower(strings.TrimRight(u.Hostname(), "."))
	if host == "" {
		return Canonical{}, fmt.Errorf("缺 host")
	}
	if !isASCII(host) {
		return Canonical{}, fmt.Errorf("非 ASCII host 需预先 punycode（本构建无 IDN 库，fail-closed）")
	}
	// 退化点模式（前导点 / 空标签 ..）一律 fail-closed——否则与再解析/解析器分歧（§31 P2x）。
	if strings.HasPrefix(host, ".") || strings.Contains(host, "..") {
		return Canonical{}, fmt.Errorf("host 含前导点或空标签：fail-closed 拒绝")
	}
	host = normalizeHost(host)

	port, err := effectivePort2(u.Port(), scheme)
	if err != nil {
		return Canonical{}, err
	}

	// path：按 net/url 的单次转义取 unescape，再 path.Clean 消解 ./ ../（多重编码陷阱）。
	up, err := url.PathUnescape(u.EscapedPath())
	if err != nil {
		return Canonical{}, fmt.Errorf("path 解码失败: %w", err)
	}
	if hasControlChars(up) {
		// 控制符（NUL 等）是 URL 走私/解析分歧向量 → fail-closed 拒绝（§31 P2x）。
		return Canonical{}, fmt.Errorf("path 含控制字符：fail-closed 拒绝")
	}
	cp := path.Clean(up)
	if cp == "." {
		cp = "/"
	}
	if !strings.HasPrefix(cp, "/") {
		cp = "/" + cp
	}

	return Canonical{Scheme: scheme, Host: host, Port: port, Path: cp}, nil
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func hasControlChars(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] == 0x7F {
			return true
		}
	}
	return false
}

// normalizeHost 规范化 host：标准 IP（netip 压缩/小写）、异常 IPv4（inet_aton 式）→ dotted-decimal；
// 否则按域名原样返回（已小写去末尾点）。
func normalizeHost(host string) string {
	h := host
	if strings.HasPrefix(h, "[") && strings.HasSuffix(h, "]") {
		h = h[1 : len(h)-1]
	}
	if addr, err := netip.ParseAddr(h); err == nil {
		return addr.String()
	}
	if ip, ok := parseInetAton(host); ok {
		return ip
	}
	return host
}

// parseInetAton 按 inet_aton 规则解析异常 IPv4（八进制 0177 / 十六进制 0x7f / 十进制 2130706433 /
// 短式 127.1）。成功返回规范 dotted-decimal。普通域名返回 ok=false。
func parseInetAton(host string) (string, bool) {
	parts := strings.Split(host, ".")
	if len(parts) == 0 || len(parts) > 4 {
		return "", false
	}
	vals := make([]uint64, len(parts))
	for i, p := range parts {
		v, ok := parseIPNum(p)
		if !ok {
			return "", false
		}
		vals[i] = v
	}
	var n uint64
	switch len(parts) {
	case 1:
		n = vals[0]
	case 2:
		if vals[0] > 0xFF || vals[1] > 0xFFFFFF {
			return "", false
		}
		n = vals[0]<<24 | vals[1]
	case 3:
		if vals[0] > 0xFF || vals[1] > 0xFF || vals[2] > 0xFFFF {
			return "", false
		}
		n = vals[0]<<24 | vals[1]<<16 | vals[2]
	case 4:
		if vals[0] > 0xFF || vals[1] > 0xFF || vals[2] > 0xFF || vals[3] > 0xFF {
			return "", false
		}
		n = vals[0]<<24 | vals[1]<<16 | vals[2]<<8 | vals[3]
	}
	if n > 0xFFFFFFFF {
		return "", false
	}
	return fmt.Sprintf("%d.%d.%d.%d", (n>>24)&0xFF, (n>>16)&0xFF, (n>>8)&0xFF, n&0xFF), true
}

// parseIPNum 解析一个 IPv4 段：0x/0X 前缀=十六进制，0 前缀=八进制，否则十进制。
func parseIPNum(s string) (uint64, bool) {
	if s == "" {
		return 0, false
	}
	base := 10
	switch {
	case len(s) >= 2 && (s[0:2] == "0x" || s[0:2] == "0X"):
		base, s = 16, s[2:]
		if s == "" {
			return 0, false
		}
	case len(s) >= 2 && s[0] == '0':
		base, s = 8, s[1:]
	}
	v, err := strconv.ParseUint(s, base, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func effectivePort2(portStr, scheme string) (int, error) {
	if portStr == "" {
		if scheme == "https" {
			return 443, nil
		}
		return 80, nil
	}
	n, err := strconv.Atoi(portStr)
	if err != nil || n < 1 || n > 65535 {
		return 0, fmt.Errorf("端口非法: %q", portStr)
	}
	return n, nil
}

// HostInTarget 判断规范化 host/scheme/port 是否落在授权 Target 内（精确/泛域名分别匹配，§9.3）。
// 精确域名**绝不**自动匹配子域（除非 allow_subdomains）；泛域名 *.base 匹配 base 及其任意子域。
func HostInTarget(c Canonical, t Target) bool {
	// scheme
	if len(t.Schemes) > 0 {
		if !contains(t.Schemes, c.Scheme) {
			return false
		}
	} else if c.Scheme != "http" && c.Scheme != "https" {
		return false
	}
	// port
	if len(t.Ports) > 0 && !containsInt(t.Ports, c.Port) {
		return false
	}
	// host
	for _, rule := range t.Hosts {
		if hostMatches(c.Host, rule, t.AllowSubdomains) {
			return true
		}
	}
	return false
}

// InScope：规范化原始 URL 后判定是否在 Target 内。解析失败 → (false, err)（fail-closed）。
func InScope(raw string, t Target) (bool, Canonical, error) {
	c, err := Canonicalize(raw)
	if err != nil {
		return false, Canonical{}, err
	}
	return HostInTarget(c, t), c, nil
}
