package cobol

import (
	"fmt"
	"strings"
)

// Decode は項目定義 f に従ってバイト列 b を表示用の値文字列へ変換する。
// 全バイトが 0x00 / 0xFF のときは、バイト埋めの表意定数 LOW-VALUE / HIGH-VALUE と
// みなしてラベル表示する（型に従った復号より優先する）。正規のゼロは 0x30、パック10進数の
// ゼロは符号ニブルが非ゼロのため、全 0x00 と衝突しない。
func Decode(f *Field, b []byte) (string, error) {
	if len(b) > 0 {
		if allBytes(b, 0x00) {
			return "LOW-VALUE", nil
		}
		if allBytes(b, 0xFF) {
			return "HIGH-VALUE", nil
		}
	}
	switch f.Type {
	case TypeNumeric:
		var digits string
		var neg bool
		if f.Usage == UsagePacked {
			digits, neg = decodePacked(b)
		} else {
			digits, neg = decodeZoned(b)
		}
		if !f.Signed {
			neg = false
		}
		return formatNumeric(f, digits, neg), nil
	case TypeAlphanumeric:
		return strings.TrimRight(string(b), " "), nil
	case TypeJapanese:
		if f.NEncode != NEncodeSJIS {
			return "", fmt.Errorf("n-encode=%s（日本語 EBCDIC）は未対応です", f.NEncode)
		}
		s, err := sjisToUTF8(b)
		if err != nil {
			return "", fmt.Errorf("Shift-JIS 復元に失敗しました: %w", err)
		}
		return strings.TrimRight(s, "　 "), nil
	}
	return "", fmt.Errorf("項目 %s: 未対応の型です", f.Name)
}

// allBytes は b の全要素が v と等しいかを返す。
func allBytes(b []byte, v byte) bool {
	for _, c := range b {
		if c != v {
			return false
		}
	}
	return true
}

// decodeZoned はゾーン10進数を桁文字列と符号に分解する。
func decodeZoned(b []byte) (digits string, negative bool) {
	out := make([]byte, len(b))
	for i, c := range b {
		switch {
		case c >= '0' && c <= '9':
			out[i] = c
		case c >= zoneOverpunchNegBase && c <= zoneOverpunchNegMax: // 負のオーバーパンチ
			out[i] = '0' + (c - zoneOverpunchNegBase)
			negative = true
		case c >= zoneOverpunchPosBase && c <= zoneOverpunchPosMax: // 正のオーバーパンチ（別実装互換）
			out[i] = '0' + (c - zoneOverpunchPosBase)
		default:
			out[i] = '0'
		}
	}
	return string(out), negative
}

// decodePacked はパック10進数を桁文字列と符号に分解する。
func decodePacked(b []byte) (digits string, negative bool) {
	var sb strings.Builder
	nibbles := make([]byte, 0, len(b)*2)
	for _, by := range b {
		nibbles = append(nibbles, by>>4, by&0x0F)
	}
	if len(nibbles) == 0 {
		return "", false
	}
	sign := nibbles[len(nibbles)-1]
	for _, n := range nibbles[:len(nibbles)-1] {
		sb.WriteByte('0' + n)
	}
	negative = sign == packSignNegative || sign == packSignNegativeAlt
	return sb.String(), negative
}

// formatNumeric は桁文字列を PICTURE に従った値の文字列に整える。
func formatNumeric(f *Field, digits string, negative bool) string {
	total := f.IntDigits + f.DecDigits
	if len(digits) > total {
		digits = digits[len(digits)-total:] // 余分な上位桁を捨てる
	} else if len(digits) < total {
		digits = strings.Repeat("0", total-len(digits)) + digits
	}

	s := digits[:f.IntDigits]
	if f.DecDigits > 0 {
		s = digits[:f.IntDigits] + "." + digits[f.IntDigits:]
	}
	if negative {
		s = "-" + s
	}
	return s
}
