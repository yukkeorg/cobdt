// Package cobol は COBOL のデータ項目定義（PICTURE / USAGE）の解析と、
// 値とバイト列の相互変換（ゾーン10進数・パック10進数・英数字・日本語）を提供する。
package cobol

import (
	"fmt"
	"strconv"
	"strings"
)

// FieldType は項目の種別。
type FieldType int

const (
	TypeNumeric      FieldType = iota // 9: 数値項目
	TypeAlphanumeric                  // X: 英数字項目
	TypeJapanese                      // N: 日本語項目（Shift-JIS）
)

// Usage は USAGE 指定（内部格納形式）。
type Usage int

const (
	UsageDisplay Usage = iota // DISPLAY（ゾーン10進数）
	UsagePacked               // PACKED-DECIMAL / COMP-3（パック10進数）
)

// NEncoding は N（日本語）項目の文字エンコード。
type NEncoding int

const (
	NEncodeSJIS   NEncoding = iota // sjis（Shift-JIS）。既定値。
	NEncodeEBCDIC                  // ebcdic（日本語 EBCDIC）
)

// ParseNEncoding は YAML の n-encode 文字列を NEncoding に変換する。
// 空文字列（未指定）の場合は既定の sjis を返す。
func ParseNEncoding(s string) (NEncoding, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "sjis", "shift-jis", "shiftjis", "shift_jis", "cp932":
		return NEncodeSJIS, nil
	case "ebcdic", "ebicdic": // design.md の表記揺れ（ebicdic）も許容
		return NEncodeEBCDIC, nil
	default:
		return 0, fmt.Errorf("不明な n-encode です: %q", s)
	}
}

// FigurativeConstant は値が COBOL の表意定数（ZERO / SPACE / SPACES）として
// 単独で現れているかを判定し、正規化した名称（"ZERO" または "SPACE"）と true を返す。
func FigurativeConstant(value string) (string, bool) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "ZERO", "ZEROS", "ZEROES":
		return "ZERO", true
	case "SPACE", "SPACES":
		return "SPACE", true
	}
	return "", false
}

// String は N 項目エンコードの表示名を返す。
func (e NEncoding) String() string {
	switch e {
	case NEncodeSJIS:
		return "sjis"
	case NEncodeEBCDIC:
		return "ebcdic"
	default:
		return "unknown"
	}
}

// Field は 1 つのデータ項目の定義。
type Field struct {
	Name      string
	Raw       string // 元の PICTURE 文字列（dump 表示用）
	Type      FieldType
	Signed    bool // S（符号）
	IntDigits int  // V より前の桁数（数値項目）
	DecDigits int  // V より後の桁数（数値項目）
	Length    int  // 文字数（X / N 項目）
	Usage     Usage
	NEncode   NEncoding // N（日本語）項目の文字エンコード（既定: sjis）
	Filler    bool      // FILLER（無名項目）かどうか。型は X として扱う。
	Value     string    // 初期値（VALUE）。HasValue が false のとき未指定。
	HasValue  bool      // value が明示指定されたか
}

// Item は record を構成する木構造の 1 ノード。
// 集団項目のときは Group 名と Children を持ち、基本項目／FILLER のときは Leaf を持つ。
// Occurs が 1 以上のとき、その項目は表（OCCURS）として要素数ぶん繰り返す。
type Item struct {
	Group    string  // 集団項目名（集団項目のとき非空）
	Leaf     *Field  // 基本項目／FILLER（葉のとき非 nil）
	Children []*Item // 集団項目の子項目
	Occurs   int     // OCCURS 要素数。0 なら表ではない。
}

// IsGroup は集団項目なら true を返す。
func (it *Item) IsGroup() bool { return it.Leaf == nil }

// FlatField は record をフラット化した 1 つの基本項目。
// 表項目（OCCURS）は要素ごとに展開され、Name に添字（"NAME(1)" など）が付く。
type FlatField struct {
	Field *Field
	Name  string
}

// FlattenItems は record ツリーを葉項目の並びへ展開する。
// OCCURS 指定された項目は要素数だけ繰り返し、各葉に添字付きの表示名を与える。
// 集団項目自身は値を持たないため展開結果には含まれない。
func FlattenItems(items []*Item) []FlatField {
	var out []FlatField
	flattenItems(items, nil, &out)
	return out
}

func flattenItems(items []*Item, subs []int, out *[]FlatField) {
	for _, it := range items {
		if it.Occurs <= 0 {
			flattenOne(it, subs, out)
			continue
		}
		for i := 1; i <= it.Occurs; i++ {
			flattenOne(it, appendInt(subs, i), out)
		}
	}
}

func flattenOne(it *Item, subs []int, out *[]FlatField) {
	if it.IsGroup() {
		flattenItems(it.Children, subs, out)
		return
	}
	*out = append(*out, FlatField{
		Field: it.Leaf,
		Name:  it.Leaf.DisplayName() + formatSubscripts(subs),
	})
}

// appendInt は s を変更せずに v を追加した新しいスライスを返す。
func appendInt(s []int, v int) []int {
	out := make([]int, len(s)+1)
	copy(out, s)
	out[len(s)] = v
	return out
}

// formatSubscripts は添字列を "(1)" や "(2, 3)" の形式に整形する。
func formatSubscripts(subs []int) string {
	if len(subs) == 0 {
		return ""
	}
	parts := make([]string, len(subs))
	for i, v := range subs {
		parts[i] = strconv.Itoa(v)
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// DisplayName は dump 表示用の項目名を返す。FILLER 項目は "FILLER" を返す。
func (f *Field) DisplayName() string {
	if f.Filler {
		return "FILLER"
	}
	return f.Name
}

// Size は項目がファイル上で占めるバイト数を返す。
func (f *Field) Size() int {
	switch f.Type {
	case TypeNumeric:
		total := f.IntDigits + f.DecDigits
		if f.Usage == UsagePacked {
			// パック10進数: 2 桁 / バイト + 符号ニブル
			return total/2 + 1
		}
		return total // ゾーン10進数: 1 桁 / バイト（符号は最下位桁にオーバーパンチ）
	case TypeAlphanumeric:
		return f.Length
	case TypeJapanese:
		return f.Length * 2 // Shift-JIS は 1 文字 2 バイト
	}
	return 0
}

// Picture はコピーブックの PICTURE 句に使う文字列を返す。
// FILLER は X(n) として表現し、それ以外は元の PICTURE をそのまま使う。
func (f *Field) Picture() string {
	if f.Filler {
		return fmt.Sprintf("X(%d)", f.Length)
	}
	return strings.ToUpper(f.Raw)
}

// TypeName は dump 表示用の型名文字列を返す。
func (f *Field) TypeName() string {
	usage := "DISPLAY"
	if f.Usage == UsagePacked {
		usage = "PACKED-DECIMAL"
	}
	return fmt.Sprintf("%s %s", f.Raw, usage)
}

// RecordLength はフラット化した項目群が構成する固定長レコードのバイト数を返す。
func RecordLength(fields []FlatField) int {
	total := 0
	for _, ff := range fields {
		total += ff.Field.Size()
	}
	return total
}
