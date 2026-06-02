package cobol

import (
	"bytes"
	"fmt"
	"strings"
)

// Encode は値文字列 value を項目定義 f に従ってバイト列へ変換する。
func Encode(f *Field, value string) ([]byte, error) {
	if fig, ok := FigurativeConstant(value); ok {
		return encodeFigurative(f, fig)
	}
	switch f.Type {
	case TypeNumeric:
		digits, neg := numericDigits(f, value)
		if f.Usage == UsagePacked {
			return encodePacked(digits, neg, f.Signed), nil
		}
		return encodeZoned(digits, neg, f.Signed), nil
	case TypeAlphanumeric:
		return encodeAlphanumeric(value, f.Length), nil
	case TypeJapanese:
		return encodeJapanese(value, f.Length, f.NEncode)
	}
	return nil, fmt.Errorf("項目 %s: 未対応の型です", f.Name)
}

// EncodeRaw は value を sanitize（非数字除去）も表意定数解釈もせず、そのままバイト列として
// 数値項目へ書き込む。COBOL プログラムの不正データ耐性テスト用に、数値項目へ意図的に
// 非数値バイトを注入する目的で使う。ゾーン10進数（DISPLAY）の数値項目のみに指定でき、
// バイト長は項目のバイト長と正確に一致しなければならない（固定長レコードを崩さないため）。
func EncodeRaw(f *Field, value string) ([]byte, error) {
	if f.Type != TypeNumeric || f.Usage != UsageDisplay {
		return nil, fmt.Errorf("項目 %s: raw は数値(DISPLAY)項目にのみ指定できます", f.DisplayName())
	}
	b := []byte(value)
	if len(b) != f.Size() {
		return nil, fmt.Errorf("項目 %s: raw のバイト長 %d が項目のバイト長 %d と一致しません",
			f.DisplayName(), len(b), f.Size())
	}
	return b, nil
}

// encodeFigurative は表意定数を項目定義に従ったバイト列へ変換する。
// ZERO は項目をゼロ、SPACE は空白で埋める（型に応じた文字埋め）。
// LOW-VALUE / HIGH-VALUE は型に関係なくバイト長ぶんを 0x00 / 0xFF で埋める（バイト埋め）。
func encodeFigurative(f *Field, fig string) ([]byte, error) {
	switch fig {
	case "LOW-VALUE":
		return bytes.Repeat([]byte{0x00}, f.Size()), nil
	case "HIGH-VALUE":
		return bytes.Repeat([]byte{0xFF}, f.Size()), nil
	case "ZERO":
		switch f.Type {
		case TypeNumeric:
			digits, neg := numericDigits(f, "0")
			if f.Usage == UsagePacked {
				return encodePacked(digits, neg, f.Signed), nil
			}
			return encodeZoned(digits, neg, f.Signed), nil
		case TypeAlphanumeric:
			return bytes.Repeat([]byte{'0'}, f.Length), nil
		case TypeJapanese:
			return encodeJapanese(strings.Repeat("０", f.Length), f.Length, f.NEncode) // 全角ゼロ
		}
	case "SPACE":
		switch f.Type {
		case TypeNumeric:
			return bytes.Repeat([]byte{' '}, f.Size()), nil
		case TypeAlphanumeric:
			return bytes.Repeat([]byte{' '}, f.Length), nil
		case TypeJapanese:
			return encodeJapanese("", f.Length, f.NEncode) // 全角空白で埋める
		}
	}
	return nil, fmt.Errorf("項目 %s: 未対応の表意定数です: %s", f.Name, fig)
}

// numericDigits は入力値を「整数桁 + 小数桁」の桁文字列へ整形する。
func numericDigits(f *Field, value string) (digits string, negative bool) {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "-") {
		negative = true
		value = value[1:]
	} else if strings.HasPrefix(value, "+") {
		value = value[1:]
	}

	intRaw, fracRaw, _ := strings.Cut(value, ".")
	intRaw = keepDigits(intRaw)
	fracRaw = keepDigits(fracRaw)

	// 整数部は右詰め（上位を 0 で埋める／あふれた上位は切り捨て）
	if len(intRaw) > f.IntDigits {
		intRaw = intRaw[len(intRaw)-f.IntDigits:]
	} else {
		intRaw = strings.Repeat("0", f.IntDigits-len(intRaw)) + intRaw
	}
	// 小数部は左詰め（下位を 0 で埋める／あふれた下位は切り捨て）
	if len(fracRaw) > f.DecDigits {
		fracRaw = fracRaw[:f.DecDigits]
	} else {
		fracRaw = fracRaw + strings.Repeat("0", f.DecDigits-len(fracRaw))
	}
	return intRaw + fracRaw, negative
}

// encodeZoned はゾーン10進数（DISPLAY）へエンコードする。
// 符号付きで負の場合、最下位桁にオーバーパンチする（GnuCOBOL の ASCII 既定: 0x70+桁）。
func encodeZoned(digits string, negative, signed bool) []byte {
	b := []byte(digits) // ASCII '0'-'9'
	if signed && negative && len(b) > 0 {
		d := b[len(b)-1] - '0'
		b[len(b)-1] = zoneOverpunchNegBase + d
	}
	return b
}

// encodePacked はパック10進数（COMP-3）へエンコードする。
func encodePacked(digits string, negative, signed bool) []byte {
	nBytes := len(digits)/2 + 1
	totalNibbles := nBytes * 2
	digitNibbles := totalNibbles - 1 // 最下位ニブルは符号

	padded := strings.Repeat("0", digitNibbles-len(digits)) + digits
	nibbles := make([]byte, totalNibbles)
	for i := range digitNibbles {
		nibbles[i] = padded[i] - '0'
	}
	switch {
	case !signed:
		nibbles[totalNibbles-1] = packSignUnsigned
	case negative:
		nibbles[totalNibbles-1] = packSignNegative
	default:
		nibbles[totalNibbles-1] = packSignPositive
	}

	buf := make([]byte, nBytes)
	for i := range nBytes {
		buf[i] = nibbles[2*i]<<4 | nibbles[2*i+1]
	}
	return buf
}

// encodeAlphanumeric は英数字項目（X）を左詰め・半角空白埋めでエンコードする。
func encodeAlphanumeric(value string, length int) []byte {
	b := []byte(value)
	if len(b) >= length {
		return b[:length]
	}
	return append(b, bytes.Repeat([]byte{' '}, length-len(b))...)
}

// encodeJapanese は日本語項目（N）を指定エンコードで左詰め・全角空白埋めでエンコードする。
func encodeJapanese(value string, length int, enc NEncoding) ([]byte, error) {
	if enc != NEncodeSJIS {
		return nil, fmt.Errorf("n-encode=%s（日本語 EBCDIC）は未対応です", enc)
	}
	sjis, err := utf8ToSJIS(value)
	if err != nil {
		return nil, fmt.Errorf("Shift-JIS 変換に失敗しました: %w", err)
	}
	target := length * 2
	if len(sjis) >= target {
		return sjis[:target], nil
	}
	for len(sjis) < target {
		sjis = append(sjis, sjisFullWidthSpace...)
	}
	return sjis[:target], nil
}

// keepDigits は文字列から 0-9 の数字だけを抜き出す。
func keepDigits(s string) string {
	var sb strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}
