package cobol

// maxNumericDigits は数値項目（PIC 9）の最大桁数。
const maxNumericDigits = 31

// パック10進数（COMP-3）の最下位ニブルに格納する符号。
const (
	packSignPositive    = 0x0C // 正
	packSignNegative    = 0x0D // 負
	packSignUnsigned    = 0x0F // 符号なし
	packSignNegativeAlt = 0x0B // decode 時に負と見なす別実装の符号
)

// ゾーン10進数（DISPLAY）の符号付き最下位桁に施すオーバーパンチ。
// 桁 0..9 が base..base+9 に対応する（GnuCOBOL の ASCII 既定）。
const (
	zoneOverpunchNegBase = 0x70 // 負の基点（0x70..0x79）
	zoneOverpunchNegMax  = 0x79 // 負の上端
	zoneOverpunchPosBase = 0x40 // 正の基点（0x40..0x49、別実装互換）
	zoneOverpunchPosMax  = 0x49 // 正の上端
)

// sjisFullWidthSpace は Shift-JIS の全角空白（2 バイト）。
var sjisFullWidthSpace = []byte{0x81, 0x40}
