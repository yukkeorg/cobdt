// Package app は create / dump / create-copybook 各モードの処理本体を提供する。
package app

import (
	"fmt"
	"io"
	"os"

	"yukkeorg/internal/cobol"
	"yukkeorg/internal/config"
	"yukkeorg/internal/copybook"
	"yukkeorg/internal/datafile"
)

// Create は設定 YAML の内容を編成に従ってデータファイルへ書き出す。
func Create(configPath, outputPath string, stdout io.Writer) error {
	spec, err := config.Load(configPath)
	if err != nil {
		return err
	}

	records := make([][]byte, 0, len(spec.Rows))
	for rowIdx, values := range spec.Rows {
		if len(values) != len(spec.Fields) {
			return fmt.Errorf("data[%d]: 値の数 %d が項目数 %d と一致しません", rowIdx, len(values), len(spec.Fields))
		}
		var rec []byte
		for i, f := range spec.Fields {
			b, err := cobol.Encode(f, values[i])
			if err != nil {
				return fmt.Errorf("data[%d] %s: %w", rowIdx, f.DisplayName(), err)
			}
			rec = append(rec, b...)
		}
		records = append(records, rec)
	}

	if err := datafile.WriteRecords(outputPath, spec.Organization, records); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "作成しました: %s （%d レコード, レコード長 %d バイト）\n",
		outputPath, len(records), cobol.RecordLength(spec.Fields))
	return nil
}

// Dump はデータファイルを編成に従って読み込み、各レコードの項目名・型名・値を表示する。
func Dump(configPath, inputPath string, stdout io.Writer) error {
	spec, err := config.Load(configPath)
	if err != nil {
		return err
	}

	recLen := cobol.RecordLength(spec.Fields)
	records, err := datafile.ReadRecords(inputPath, spec.Organization, recLen)
	if err != nil {
		return err
	}

	if len(records) == 0 {
		fmt.Fprintln(stdout, "レコードがありません。")
		return nil
	}

	for idx, rec := range records {
		fmt.Fprintf(stdout, "===== レコード %d =====\n", idx+1)
		offset := 0
		for _, f := range spec.Fields {
			size := f.Size()
			if offset+size > len(rec) {
				fmt.Fprintf(stdout, "  %-20s %-24s <レコード長不足>\n", f.DisplayName(), f.TypeName())
				break
			}
			val, err := cobol.Decode(f, rec[offset:offset+size])
			if err != nil {
				return fmt.Errorf("レコード %d %s: %w", idx+1, f.DisplayName(), err)
			}
			fmt.Fprintf(stdout, "  %-20s %-24s %s\n", f.DisplayName(), f.TypeName(), val)
			offset += size
		}
	}
	return nil
}

// CreateCopybook は record 定義から COBOL コピーブックを生成する。
// outputPath が空のときは stdout へ出力し、指定があればそのファイルへ書き出す。
func CreateCopybook(configPath, outputPath string, stdout io.Writer) error {
	spec, err := config.Load(configPath)
	if err != nil {
		return err
	}

	text := copybook.Generate(spec.Name, spec.Items)

	if outputPath == "" {
		fmt.Fprint(stdout, text)
		return nil
	}
	if err := os.WriteFile(outputPath, []byte(text), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "作成しました: %s\n", outputPath)
	return nil
}
