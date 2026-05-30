package cobol

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseField は "9(5)V99 COMP-3" のような項目定義文字列を Field に解析する。
// def は PICTURE 文字列とそれに続く任意の USAGE 指定からなる。
func ParseField(name, def string) (*Field, error) {
	tokens := strings.Fields(def)
	if len(tokens) == 0 {
		return nil, fmt.Errorf("項目 %s: 定義が空です", name)
	}

	usage := UsageDisplay
	if len(tokens) > 1 {
		switch strings.ToUpper(strings.Join(tokens[1:], " ")) {
		case "DISPLAY":
			usage = UsageDisplay
		case "PACKED-DECIMAL", "COMP-3", "COMPUTATIONAL-3":
			usage = UsagePacked
		default:
			return nil, fmt.Errorf("項目 %s: 不明な USAGE です: %q", name, strings.Join(tokens[1:], " "))
		}
	}

	f := &Field{Name: name, Raw: tokens[0], Usage: usage}

	rest := strings.ToUpper(tokens[0])

	// FILLER（無名項目）: 型は X として扱い、USAGE は指定できない。
	if strings.HasPrefix(rest, "FILLER") {
		if usage == UsagePacked {
			return nil, fmt.Errorf("項目 %s: FILLER に USAGE は指定できません", name)
		}
		n, err := parseParenCount(rest)
		if err != nil {
			return nil, fmt.Errorf("項目 %s: %w", name, err)
		}
		f.Type = TypeAlphanumeric
		f.Filler = true
		f.Length = n
		f.Usage = UsageDisplay
		return f, nil
	}

	if strings.HasPrefix(rest, "S") {
		f.Signed = true
		rest = rest[1:]
	}
	if len(rest) == 0 {
		return nil, fmt.Errorf("項目 %s: PICTURE が不正です: %q", name, def)
	}

	switch rest[0] {
	case '9':
		f.Type = TypeNumeric
		f.IntDigits, f.DecDigits = parseNumericPic(rest)
		total := f.IntDigits + f.DecDigits
		if total == 0 {
			return nil, fmt.Errorf("項目 %s: 数値項目の桁数が 0 です: %q", name, def)
		}
		if total > 31 {
			return nil, fmt.Errorf("項目 %s: 数値項目の桁数は最大 31 桁です（指定: %d 桁）", name, total)
		}
	case 'X':
		f.Type = TypeAlphanumeric
		f.Length = parseRepeatCount(rest, 'X')
		if f.Signed {
			return nil, fmt.Errorf("項目 %s: 英数字項目に符号 S は指定できません", name)
		}
	case 'N':
		f.Type = TypeJapanese
		f.Length = parseRepeatCount(rest, 'N')
		if f.Signed {
			return nil, fmt.Errorf("項目 %s: 日本語項目に符号 S は指定できません", name)
		}
	default:
		return nil, fmt.Errorf("項目 %s: 不明な型です: %q", name, def)
	}

	if usage == UsagePacked && f.Type != TypeNumeric {
		return nil, fmt.Errorf("項目 %s: PACKED-DECIMAL は数値項目のみ指定できます", name)
	}
	return f, nil
}

// parseNumericPic は "9(5)V99" などを (整数桁, 小数桁) に分解する。
// V（仮想小数点）の前後で桁数を振り分ける。
func parseNumericPic(pic string) (intDigits, decDigits int) {
	afterV := false
	for i := 0; i < len(pic); i++ {
		switch pic[i] {
		case '9':
			n, skip := repeatAt(pic, i)
			if afterV {
				decDigits += n
			} else {
				intDigits += n
			}
			i += skip
		case 'V':
			afterV = true
		}
	}
	return
}

// parseRepeatCount は "X(20)" や "NNN" の総文字数を数える。
func parseRepeatCount(pic string, ch byte) int {
	length := 0
	for i := 0; i < len(pic); i++ {
		if pic[i] == ch {
			n, skip := repeatAt(pic, i)
			length += n
			i += skip
		}
	}
	return length
}

// parseParenCount は "FILLER(10)" のような "(n)" 表記から桁数 n を取り出す。
func parseParenCount(pic string) (int, error) {
	open := strings.IndexByte(pic, '(')
	closeIdx := strings.IndexByte(pic, ')')
	if open < 0 || closeIdx < open {
		return 0, fmt.Errorf("桁数 (n) の指定が必要です: %q", pic)
	}
	n, err := strconv.Atoi(strings.TrimSpace(pic[open+1 : closeIdx]))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("桁数が不正です: %q", pic)
	}
	return n, nil
}

// repeatAt は pic[i] の文字に続く "(n)" 反復指定を解釈し、桁数 n と
// 読み飛ばすべきバイト数 skip を返す。反復指定がなければ (1, 0)。
func repeatAt(pic string, i int) (n, skip int) {
	if i+1 < len(pic) && pic[i+1] == '(' {
		if rel := strings.IndexByte(pic[i:], ')'); rel > 2 {
			if v, err := strconv.Atoi(pic[i+2 : i+rel]); err == nil {
				return v, rel
			}
		}
	}
	return 1, 0
}
