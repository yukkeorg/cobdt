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
	Redefine string  // REDEFINES の対象項目名（再定義項目のとき非空）。原定義と同じバイト領域に重なる。
}

// IsGroup は集団項目なら true を返す。
func (it *Item) IsGroup() bool { return it.Leaf == nil }

// FlatField は record をフラット化した 1 つの基本項目。
// 表項目（OCCURS）は要素ごとに展開され、Name に添字（"NAME(1)" など）が付く。
// Offset はその葉がレコード先頭から占める位置（バイト）。再定義項目の葉は
// 原定義の葉と同じ Offset に重なる。
type FlatField struct {
	Field  *Field
	Name   string
	Offset int
}

// FlattenItems は record ツリーを葉項目の並びへ展開し、各葉にレコード先頭からの
// オフセットを与える。OCCURS 指定された項目は要素数だけ繰り返す。再定義領域では
// 原定義に続けて各再定義項目を同じオフセットに重ねて並べる（オフセットは前進しない）。
// 集団項目自身は値を持たないため展開結果には含まれない。
func FlattenItems(items []*Item) []FlatField {
	var out []FlatField
	flattenItems(items, 0, nil, &out)
	return out
}

// flattenItems は base から始まる items の葉を out に追加し、items の直後の
// オフセットを返す。再定義領域の各メンバーは同じオフセットを共有する。
func flattenItems(items []*Item, base int, subs []int, out *[]FlatField) int {
	offset := base
	for i := 0; i < len(items); {
		j := i + 1
		for j < len(items) && items[j].Redefine != "" {
			j++
		}
		if j-i > 1 {
			// 再定義領域: 全メンバーが同じオフセットから始まる
			end := offset
			for k := i; k < j; k++ {
				if e := flattenOne(items[k], offset, subs, out); e > end {
					end = e
				}
			}
			offset = end
		} else {
			offset = flattenOne(items[i], offset, subs, out)
		}
		i = j
	}
	return offset
}

// flattenOne は base から始まる 1 項目を展開し、その直後のオフセットを返す。
func flattenOne(it *Item, base int, subs []int, out *[]FlatField) int {
	if it.Occurs <= 0 {
		return flattenOneAt(it, base, subs, out)
	}
	offset := base
	for i := 1; i <= it.Occurs; i++ {
		offset = flattenOneAt(it, offset, appendInt(subs, i), out)
	}
	return offset
}

func flattenOneAt(it *Item, base int, subs []int, out *[]FlatField) int {
	if it.IsGroup() {
		return flattenItems(it.Children, base, subs, out)
	}
	*out = append(*out, FlatField{
		Field:  it.Leaf,
		Name:   it.Leaf.DisplayName() + formatSubscripts(subs),
		Offset: base,
	})
	return base + it.Leaf.Size()
}

// ItemSize は項目がファイル上で占めるバイト数を返す。集団項目は従属項目の
// 合計（再定義の重なりは一度だけ数える）、OCCURS 指定があれば要素数倍。
func ItemSize(it *Item) int {
	var s int
	if it.IsGroup() {
		s = SiblingsSize(it.Children)
	} else {
		s = it.Leaf.Size()
	}
	if it.Occurs > 0 {
		s *= it.Occurs
	}
	return s
}

// SiblingsSize は同じ階層に並ぶ項目群が占めるバイト数を返す。
// 再定義領域は原定義と全再定義項目の最大サイズを一度だけ数える。
func SiblingsSize(items []*Item) int {
	offset := 0
	for i := 0; i < len(items); {
		j := i + 1
		for j < len(items) && items[j].Redefine != "" {
			j++
		}
		if j-i > 1 {
			end := offset
			for k := i; k < j; k++ {
				if e := offset + ItemSize(items[k]); e > end {
					end = e
				}
			}
			offset = end
		} else {
			offset += ItemSize(items[i])
		}
		i = j
	}
	return offset
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
// 再定義項目の葉は原定義と同じオフセットに重なるため、各葉の終端（Offset+Size）の
// 最大値をレコード長とする（重なりを二重に数えない）。
func RecordLength(fields []FlatField) int {
	max := 0
	for _, ff := range fields {
		if end := ff.Offset + ff.Field.Size(); end > max {
			max = end
		}
	}
	return max
}
