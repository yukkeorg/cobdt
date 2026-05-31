// Package copybook は record 定義から COBOL のコピーブックを生成する。
package copybook

import (
	"fmt"
	"strings"

	"yukkeorg/internal/cobol"
)

// Generate は record ツリーから COBOL コピーブックのテキストを生成する。
// レコード名は 01 レベル、record 直下の項目は 03 レベルで始まり、
// 集団項目に入るたびにレベルを +2 する。
func Generate(recordName string, items []*cobol.Item) string {
	if strings.TrimSpace(recordName) == "" {
		recordName = "DATA-RECORD"
	}
	var b strings.Builder
	writeGroupLine(&b, 0, 1, recordName, 0, "")
	writeItems(&b, items, 1)
	return b.String()
}

// writeItems は depth の階層（レベル = 2*depth+1）に items を書き出す。
func writeItems(b *strings.Builder, items []*cobol.Item, depth int) {
	level := 2*depth + 1
	for _, it := range items {
		if it.IsGroup() {
			writeGroupLine(b, depth, level, it.Group, it.Occurs, it.Redefine)
			writeItems(b, it.Children, depth+1)
		} else {
			writeLeafLine(b, depth, level, it.Leaf, it.Occurs, it.Redefine)
		}
	}
}

// writeGroupLine は集団項目／レコード名の行（PICTURE 句なし）を書き出す。
func writeGroupLine(b *strings.Builder, depth, level int, name string, occurs int, redefine string) {
	line := fmt.Sprintf("%s%02d  %s", indent(depth), level, name)
	if redefine != "" {
		line += " REDEFINES " + redefine
	}
	if occurs > 0 {
		line += fmt.Sprintf(" OCCURS %d TIMES", occurs)
	}
	fmt.Fprintf(b, "%s.\n", line)
}

// picColumn は PICTURE 句を揃える桁位置（ネストの深さによらず一定）。
const picColumn = 40

// writeLeafLine は基本項目／FILLER の行（REDEFINES / PICTURE / USAGE / OCCURS / VALUE 句つき）を書き出す。
func writeLeafLine(b *strings.Builder, depth, level int, f *cobol.Field, occurs int, redefine string) {
	clause := "PIC " + f.Picture()
	if f.Usage == cobol.UsagePacked {
		clause += " PACKED-DECIMAL"
	}
	// OCCURS と VALUE は同時指定できないため、表項目は OCCURS のみ出力する。
	if occurs > 0 {
		clause += fmt.Sprintf(" OCCURS %d TIMES", occurs)
	} else {
		clause += " " + valueClause(f)
	}

	prefix := fmt.Sprintf("%s%02d  %s", indent(depth), level, f.DisplayName())
	if redefine != "" {
		prefix += " REDEFINES " + redefine
	}
	fmt.Fprintf(b, "%-*s %s.\n", picColumn, prefix, clause)
}

// valueClause は VALUE 句を返す。未指定なら数値項目は ZERO、それ以外は SPACE。
func valueClause(f *cobol.Field) string {
	if !f.HasValue {
		if f.Type == cobol.TypeNumeric {
			return "VALUE ZERO"
		}
		return "VALUE SPACE"
	}
	if fig, ok := cobol.FigurativeConstant(f.Value); ok {
		return "VALUE " + fig // ZERO / SPACE は予約語として引用符なしで出力
	}
	if f.Type == cobol.TypeNumeric {
		return "VALUE " + f.Value // 数値リテラルは引用符なし
	}
	return fmt.Sprintf("VALUE %q", f.Value)
}

// indent は depth に応じた行頭インデントを返す（01 レベルが 8 桁目に来るように 7 桁ぶん）。
func indent(depth int) string {
	return strings.Repeat(" ", 7+depth*4)
}
