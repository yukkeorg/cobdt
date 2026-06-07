// Package app は create / dump / create-copybook 各モードの処理本体を提供する。
//
// 各関数は出力先（ファイルか標準出力か）の解決を行わない純粋な処理本体である。
// データの書き出し先は out（io.Writer）、確認・進捗などの診断メッセージは diag
// （標準エラー出力を想定）へ書く。out と diag の解決・open/close は cmd 層が担う。
package app

import (
	"fmt"
	"io"
	"os"

	"yukkeorg/cobdt/internal/cobol"
	"yukkeorg/cobdt/internal/config"
	"yukkeorg/cobdt/internal/copybook"
	"yukkeorg/cobdt/internal/datafile"
)

// report は出力完了メッセージを診断用 Writer へ書く。
// dest が空のときは標準出力への出力とみなす。
func report(diag io.Writer, dest, detail string) {
	target := dest
	if target == "" {
		target = "標準出力"
	}
	if detail == "" {
		fmt.Fprintf(diag, "作成しました: %s\n", target)
		return
	}
	fmt.Fprintf(diag, "作成しました: %s （%s）\n", target, detail)
}

// Create は設定 YAML の内容を編成に従ってデータファイルへ書き出す。
// dataYAMLPath が空でないときは、書き込む値を inline の data ではなくその別 YAML
// ファイルから取り込む（docs/design.md「別ファイルの YAML データ入力」参照）。
// 生成データは out へ、確認メッセージは diag へ書く。dest は確認メッセージに出す
// 出力先ラベル（ファイルパス。標準出力のときは空文字）。
func Create(configPath, dataYAMLPath, dest string, out, diag io.Writer) error {
	spec, err := config.Load(configPath)
	if err != nil {
		return err
	}

	if dataYAMLPath != "" {
		if err := spec.LoadDataYAML(dataYAMLPath); err != nil {
			return err
		}
	}

	records, err := spec.BuildRecords()
	if err != nil {
		return err
	}

	if err := datafile.WriteRecords(out, spec.Organization, records); err != nil {
		return err
	}

	report(diag, dest, fmt.Sprintf("%d レコード, レコード長 %d バイト",
		len(records), cobol.RecordLength(spec.Fields)))
	return nil
}

// Dump はデータファイルを編成に従って読み込み、各レコードの項目名・型名・値を表示する。
func Dump(configPath, inputPath string, out io.Writer) error {
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
		fmt.Fprintln(out, "レコードがありません。")
		return nil
	}

	for idx, rec := range records {
		fmt.Fprintf(out, "===== レコード %d =====\n", idx+1)
		// Fields は再定義項目の葉を原定義と同じオフセットに重ねて並べる。各葉を
		// 自分のオフセットで読み出すため、再定義領域は全解釈が並列に表示される。
		for _, ff := range spec.Fields {
			size := ff.Field.Size()
			if ff.Offset+size > len(rec) {
				fmt.Fprintf(out, "  %-20s %-24s <レコード長不足>\n", ff.Name, ff.Field.TypeName())
				continue
			}
			val, err := cobol.Decode(ff.Field, rec[ff.Offset:ff.Offset+size])
			if err != nil {
				return fmt.Errorf("レコード %d %s: %w", idx+1, ff.Name, err)
			}
			fmt.Fprintf(out, "  %-20s %-24s %s\n", ff.Name, ff.Field.TypeName(), val)
		}
	}
	return nil
}

// CreateCopybook は record 定義から COBOL コピーブックを生成する。
// startLevel が 0 のときは 01 レコード行つきの完全なコピーブックを、1 以上のときは
// 01 を持たないコピーブック断片を startLevel から生成する（docs/CONTEXT.md
// 「コピーブック断片」参照）。生成内容は out へ、確認メッセージは diag へ書く。
// dest は確認メッセージに出す出力先ラベル（標準出力のときは空文字）。
func CreateCopybook(configPath string, startLevel int, dest string, out, diag io.Writer) error {
	spec, err := config.Load(configPath)
	if err != nil {
		return err
	}

	var text string
	if startLevel > 0 {
		text, err = copybook.GenerateFragment(spec.Items, startLevel)
	} else {
		text, err = copybook.Generate(spec.Name, spec.Items)
	}
	if err != nil {
		return err
	}

	if _, err := io.WriteString(out, text); err != nil {
		return err
	}
	report(diag, dest, "")
	return nil
}

// ImportCopybook は COBOL コピーブックを解析し、cobdt の設定 YAML を生成する。
// fragment が true のときは 01 レベルを持たないコピーブック断片として取り込み、
// 生成 YAML の name には引数 name を用いる（docs/CONTEXT.md「コピーブック断片」参照）。
// 生成内容は out へ、確認メッセージは diag へ書く。dest は確認メッセージに出す
// 出力先ラベル（標準出力のときは空文字）。
func ImportCopybook(inputPath string, fragment bool, name, dest string, out, diag io.Writer) error {
	src, err := os.ReadFile(inputPath)
	if err != nil {
		return err
	}

	var items []*cobol.Item
	recordName := name
	if fragment {
		items, err = copybook.ParseFragment(src)
	} else {
		recordName, items, err = copybook.Parse(src)
	}
	if err != nil {
		return err
	}

	yamlOut, err := copybook.RenderYAML(recordName, items)
	if err != nil {
		return err
	}

	if _, err := out.Write(yamlOut); err != nil {
		return err
	}
	report(diag, dest, "")
	return nil
}
