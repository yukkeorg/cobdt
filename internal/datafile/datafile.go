// Package datafile は COBOL のファイル編成（organization）に応じた
// 固定長レコードの読み書きを提供する。
package datafile

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
)

// Organization はファイル編成の種類。
type Organization int

const (
	OrgSequential     Organization = iota // 順編成（固定長レコードを区切りなしで連結）
	OrgLineSequential                     // 行順編成（固定長レコードを改行で区切る）
)

// ParseOrganization は YAML の organization 文字列を Organization に変換する。
func ParseOrganization(s string) (Organization, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "sequential", "sequencial", "seq":
		return OrgSequential, nil
	case "line sequential", "line sequentiall", "line-sequential", "line_sequential", "lineseq":
		return OrgLineSequential, nil
	default:
		return 0, fmt.Errorf("不明な organization です: %q", s)
	}
}

// String は編成の表示名を返す。
func (o Organization) String() string {
	switch o {
	case OrgSequential:
		return "sequential"
	case OrgLineSequential:
		return "line sequential"
	default:
		return "unknown"
	}
}

// WriteRecords はレコード群を編成に応じて w へ書き出す。
// 行順編成では各レコードの末尾に改行（LF）を付加する。
// 出力先（ファイルか標準出力か）の解決と open/close は呼び出し側の責務とする。
func WriteRecords(w io.Writer, org Organization, records [][]byte) error {
	for _, rec := range records {
		if _, err := w.Write(rec); err != nil {
			return err
		}
		if org == OrgLineSequential {
			if _, err := w.Write([]byte{'\n'}); err != nil {
				return err
			}
		}
	}
	return nil
}

// ReadRecords は編成に応じてファイルからレコード群を読み込む。
// recLen は順編成での固定レコード長（バイト）。
func ReadRecords(path string, org Organization, recLen int) ([][]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var records [][]byte
	switch org {
	case OrgLineSequential:
		for line := range bytes.SplitSeq(data, []byte{'\n'}) {
			line = bytes.TrimSuffix(line, []byte{'\r'})
			if len(line) == 0 {
				continue
			}
			records = append(records, line)
		}
	case OrgSequential:
		if recLen <= 0 {
			return nil, fmt.Errorf("レコード長が 0 です")
		}
		r := bytes.NewReader(data)
		for {
			buf := make([]byte, recLen)
			n, err := io.ReadFull(r, buf)
			if n == recLen {
				records = append(records, buf)
				continue
			}
			if err == io.EOF {
				break
			}
			if err == io.ErrUnexpectedEOF {
				return nil, fmt.Errorf("末尾のレコードが不完全です（%d バイト, 期待値 %d バイト）", n, recLen)
			}
			return nil, err
		}
	}
	return records, nil
}
