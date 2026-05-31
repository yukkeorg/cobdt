// cdm (Cobol Data Manipurator) は YAML 定義から COBOL が読めるデータファイルを
// 作成（create）し、既存のデータファイルを定義に従って解析・表示（dump）するツールです。
//
// 使い方:
//
//	cdm create          <config.yaml> <output.dat>   YAML の内容からデータファイルを作成
//	cdm dump            <config.yaml> <input.dat>    データファイルを解析してコンソールへ表示
//	cdm create-copybook <config.yaml> [output.cpy]   record 定義から COBOL コピーブックを生成
package main

import (
	"fmt"
	"os"

	"yukkeorg/internal/app"
)

func usage() {
	fmt.Fprint(os.Stderr, `cdm - Cobol Data Manipurator
YAML 定義から COBOL 用データファイルを作成／解析するツール

使い方:
  cdm create          <config.yaml> <output.dat>   YAML の内容からデータファイルを作成
  cdm dump            <config.yaml> <input.dat>    データファイルを解析してコンソールへ表示
  cdm create-copybook <config.yaml> [output.cpy]   record 定義から COBOL コピーブックを生成（省略時は標準出力）
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
		if len(os.Args) != 4 {
			usage()
			os.Exit(2)
		}
		err = app.Create(os.Args[2], os.Args[3], os.Stdout)
	case "dump":
		if len(os.Args) != 4 {
			usage()
			os.Exit(2)
		}
		err = app.Dump(os.Args[2], os.Args[3], os.Stdout)
	case "create-copybook":
		if len(os.Args) < 3 || len(os.Args) > 4 {
			usage()
			os.Exit(2)
		}
		output := ""
		if len(os.Args) == 4 {
			output = os.Args[3]
		}
		err = app.CreateCopybook(os.Args[2], output, os.Stdout)
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
