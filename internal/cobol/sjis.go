package cobol

import (
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// utf8ToSJIS は UTF-8 文字列を Shift-JIS バイト列へ変換する。
func utf8ToSJIS(s string) ([]byte, error) {
	out, _, err := transform.Bytes(japanese.ShiftJIS.NewEncoder(), []byte(s))
	return out, err
}

// sjisToUTF8 は Shift-JIS バイト列を UTF-8 文字列へ変換する。
func sjisToUTF8(b []byte) (string, error) {
	out, _, err := transform.Bytes(japanese.ShiftJIS.NewDecoder(), b)
	return string(out), err
}
