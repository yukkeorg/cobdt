// cobdt (Cobol Data Tool) は YAML 定義から COBOL が読めるデータファイルを
// 作成（create）し、既存のデータファイルを定義に従って解析・表示（dump）するツールです。
//
// 使い方:
//
//	cobdt create          [--data-yaml <data.yaml>] <config.yaml> <output.dat>  YAML の内容からデータファイルを作成
//	cobdt dump            <config.yaml> <input.dat>    データファイルを解析してコンソールへ表示
//	cobdt create-copybook <config.yaml> [output.cpy]   record 定義から COBOL コピーブックを生成
//	cobdt import-copybook <input.cpy>   [output.yaml]  COBOL コピーブックから設定 YAML を生成
package main

import (
	"flag"
	"fmt"
	"os"

	"yukkeorg/cobdt/internal/app"
)

func usage() {
	fmt.Fprint(os.Stderr, `cobdt - Cobol Data Tool
YAML 定義から COBOL 用データファイルを作成／解析するツール

使い方:
  cobdt create          [--data-yaml <data.yaml>] <config.yaml> <output.dat>
                                                   YAML の内容からデータファイルを作成
                                                   --data-yaml <data.yaml>: data 部を切り出した別 YAML から値を取り込む
  cobdt dump            <config.yaml> <input.dat>    データファイルを解析してコンソールへ表示
  cobdt create-copybook [--start-level N] <config.yaml> [output.cpy]
                                                   record 定義から COBOL コピーブックを生成（省略時は標準出力）
                                                   --start-level N: 01 行を出さず N（2〜49）始まりの断片を生成
  cobdt import-copybook [--fragment] [--name NAME] <input.cpy> [output.yaml]
                                                   COBOL コピーブックから設定 YAML を生成（省略時は標準出力）
                                                   --fragment: 01 を持たない断片として取り込む
                                                   --name NAME: 断片モードで付けるレコード名（既定 DATA-RECORD）
`)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "create":
		fs := flag.NewFlagSet("create", flag.ExitOnError)
		fs.Usage = usage
		dataYAML := fs.String("data-yaml", "", "data 部だけを切り出した別 YAML ファイルから値を取り込む（inline の data を無視する）")
		_ = fs.Parse(os.Args[2:])
		args := fs.Args()
		if len(args) != 2 {
			usage()
			os.Exit(2)
		}
		err = app.Create(args[0], args[1], *dataYAML, os.Stdout)
	case "dump":
		if len(os.Args) != 4 {
			usage()
			os.Exit(2)
		}
		err = app.Dump(os.Args[2], os.Args[3], os.Stdout)
	case "create-copybook":
		fs := flag.NewFlagSet("create-copybook", flag.ExitOnError)
		fs.Usage = usage
		startLevel := fs.Int("start-level", 0, "断片モードの開始レベル番号（2〜49）。指定すると 01 行を出さないコピーブック断片を生成する")
		_ = fs.Parse(os.Args[2:])
		args := fs.Args()
		if len(args) < 1 || len(args) > 2 {
			usage()
			os.Exit(2)
		}
		output := ""
		if len(args) == 2 {
			output = args[1]
		}
		err = app.CreateCopybook(args[0], output, *startLevel, os.Stdout)
	case "import-copybook":
		fs := flag.NewFlagSet("import-copybook", flag.ExitOnError)
		fs.Usage = usage
		fragment := fs.Bool("fragment", false, "01 レベルを持たないコピーブック断片として取り込む")
		name := fs.String("name", "DATA-RECORD", "断片モードで生成 YAML に付けるレコード名")
		_ = fs.Parse(os.Args[2:])
		args := fs.Args()
		if len(args) < 1 || len(args) > 2 {
			usage()
			os.Exit(2)
		}
		output := ""
		if len(args) == 2 {
			output = args[1]
		}
		err = app.ImportCopybook(args[0], output, *fragment, *name, os.Stdout)
	case "-h", "--help", "help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "不明なモードです: %s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "エラー: %v\n", err)
		os.Exit(1)
	}
}
