package repository

import (
	"strings"
	"testing"

	"xorm.io/builder"
)

func TestToBuilderCondAndOrGroup(t *testing.T) {
	// status = 1 AND (role = admin OR role = owner)
	nodes := []*exprNode{
		{
			join: joinAnd,
			cond: &Condition{Field: "status", Op: OpEq, Value: 1},
		},
		{
			join: joinAnd,
			children: []*exprNode{
				{join: joinAnd, cond: &Condition{Field: "role", Op: OpEq, Value: "admin"}},
				{join: joinOr, cond: &Condition{Field: "role", Op: OpEq, Value: "owner"}},
			},
		},
	}

	cond := toBuilderCond(nodes)
	sql, args, err := builder.ToSQL(cond)
	if err != nil {
		t.Fatalf("ToSQL failed: %v", err)
	}

	normalized := strings.ToLower(strings.ReplaceAll(sql, " ", ""))
	// 列名经 quoteIdent 后为 `status`=?
	if !strings.Contains(normalized, "`status`=?") && !strings.Contains(normalized, "status=?") {
		t.Fatalf("expected status predicate, got SQL=%s", sql)
	}
	if !strings.Contains(normalized, "or") {
		t.Fatalf("expected OR in SQL, got %s", sql)
	}
	if len(args) != 3 {
		t.Fatalf("expected 3 args, got %v", args)
	}
}

func TestQuoteIdentReservedWord(t *testing.T) {
	if got := quoteIdent("key"); got != "`key`" {
		t.Fatalf("quoteIdent(key)=%q", got)
	}
	if got := quoteIdent("`key`"); got != "`key`" {
		t.Fatalf("already quoted: %q", got)
	}
	if got := quoteIdent("t.key"); got != "`t`.`key`" {
		t.Fatalf("table.column: %q", got)
	}
}

func TestQuoteOrderByReservedWord(t *testing.T) {
	if got := quoteOrderBy("key ASC"); got != "`key` ASC" {
		t.Fatalf("got %q", got)
	}
	if got := quoteOrderBy("key ASC, id DESC"); got != "`key` ASC, `id` DESC" {
		t.Fatalf("got %q", got)
	}
}

func TestQueryBuilderOrChainBuildsNodes(t *testing.T) {
	qb := &QueryBuilder[struct{}]{nextJoin: joinAnd}
	qb.Eq("a", 1).Or().Eq("b", 2).And().Eq("c", 3)

	if len(qb.nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(qb.nodes))
	}
	if qb.nodes[1].join != joinOr {
		t.Fatalf("second node should be OR-joined")
	}
	if qb.nodes[2].join != joinAnd {
		t.Fatalf("third node should be AND-joined")
	}
}
