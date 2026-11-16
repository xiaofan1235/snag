package store

import (
	"strings"
	"testing"
)

// migrate_internal_test.go —— 迁移器纯函数的白盒测试（不需 SQLite 驱动，默认模式即跑）。
// 覆盖 Q-2：尾注里的 ; 不得切坏语句；缺 -- +goose Up 标记必须报错。

func TestSplitStatementsStripsInlineComment(t *testing.T) {
	// 尾注里含 ; —— 剥注释后应切出恰好 2 条语句，而非被注释里的 ; 切坏。
	in := "CREATE TABLE t (a INT); -- 尾注里有分号; 不应被当作分隔\nINSERT INTO t VALUES (1);"
	got := splitStatements(in)
	if len(got) != 2 {
		t.Fatalf("应切出 2 条语句, 得到 %d: %#v", len(got), got)
	}
	if !strings.Contains(got[0], "CREATE TABLE t") || !strings.Contains(got[1], "INSERT INTO t") {
		t.Errorf("切分内容不对: %#v", got)
	}
	// 切出的语句里不应残留注释文本。
	for _, s := range got {
		if strings.Contains(s, "--") {
			t.Errorf("语句残留注释: %q", s)
		}
	}
}

func TestExtractUpMissingMarkerErrors(t *testing.T) {
	if _, err := extractUp("CREATE TABLE x (id TEXT);\n-- +goose Down\nDROP TABLE x;"); err == nil {
		t.Error("缺少 '-- +goose Up' 标记时 extractUp 应报错（否则会把 Down 段也执行）")
	}
}

func TestExtractUpHappyPath(t *testing.T) {
	up, err := extractUp("-- +goose Up\nCREATE TABLE x(id TEXT);\n-- +goose Down\nDROP TABLE x;")
	if err != nil {
		t.Fatalf("正常文件不应报错: %v", err)
	}
	if !strings.Contains(up, "CREATE TABLE x") {
		t.Errorf("Up 段应含 CREATE: %q", up)
	}
	if strings.Contains(up, "DROP TABLE x") {
		t.Errorf("Up 段不应含 Down 段的 DROP: %q", up)
	}
}
