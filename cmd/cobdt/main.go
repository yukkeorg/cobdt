// cobdt (Cobol Data Tool) は YAML 定義から COBOL が読めるデータファイルを
// 作成（create）し、既存のデータファイルを定義に従って解析・表示（dump）するツールです。
//
// 使い方:
//
//	cdm create          [--data-yaml <data.yaml>] [-o <output.dat>] <config.yaml>  YAML の内容からデータファイルを作成
//	cdm dump            <config.yaml> <input.dat>    データファイルを解析してコンソールへ表示
//	cdm create-copybook [-o <output.cpy>] <config.yaml>   record 定義から COBOL コピーブックを生成
//	cdm import-copybook [-o <output.yaml>] <input.cpy>  COBOL コピーブックから設定 YAML を生成
//
// ファイルを出力するモード（create / create-copybook / import-copybook）の出力先は
// -o / --output で指定する。省略時は標準出力へ出力し、確認メッセージは標準エラー出力へ出す
// （docs/design.md「出力先の指定（共通規約）」、コマンドラインパーサの採用は
// docs/adr/0005-urfave-cli-v3.md 参照）。
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/urfave/cli/v3"

	"yukkeorg/cobdt/internal/app"
)

func main() {
	cmd := &cli.Command{
		Name:                  "cdm",
		Usage:                 "YAML 定義から COBOL 用データファイルを作成／解析するツール",
		HideHelpCommand:       true,
		EnableShellCompletion: true,
		// 既定のエラーハンドラを差し替え、メッセージを「エラー: …」で統一する。
		ExitErrHandler: func(_ context.Context, _ *cli.Command, err error) {
			if err == nil {
				return
			}
			fmt.Fprintf(os.Stderr, "エラー: %v\n", err)
			code := 1
			var ec cli.ExitCoder
			if errors.As(err, &ec) {
				code = ec.ExitCode()
			}
			os.Exit(code)
		},
		Commands: []*cli.Command{
			createCommand(),
			dumpCommand(),
			createCopybookCommand(),
			importCopybookCommand(),
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		// ExitErrHandler が os.Exit するため通常ここには来ないが、保険として処理する。
		fmt.Fprintf(os.Stderr, "エラー: %v\n", err)
		os.Exit(1)
	}
}

func createCommand() *cli.Command {
	return &cli.Command{
		Name:      "create",
		Usage:     "YAML の内容からデータファイルを作成する",
		ArgsUsage: "<config.yaml>",
		Flags: []cli.Flag{
			outputFlag("作成したデータファイルの出力先。省略時は標準出力（端末のときはエラー）"),
			&cli.StringFlag{
				Name:  "data-yaml",
				Usage: "data 部だけを切り出した別 YAML ファイルから値を取り込む（inline の data を無視する）",
			},
		},
		Action: func(_ context.Context, c *cli.Command) error {
			configPath, err := singleArg(c, "<config.yaml>")
			if err != nil {
				return err
			}

			output := c.String("output")
			// create はバイナリを出力するため、標準出力が端末のときは画面を壊さないよう止める。
			if output == "" && isTerminal(os.Stdout) {
				return cli.Exit("出力先がありません。-o でファイルを指定するか、パイプ／リダイレクトしてください", 2)
			}

			w, dest, closeFn, err := resolveOutput(output)
			if err != nil {
				return err
			}
			if err := app.Create(configPath, c.String("data-yaml"), dest, w, os.Stderr); err != nil {
				closeFn()
				return err
			}
			return closeFn()
		},
	}
}

func dumpCommand() *cli.Command {
	return &cli.Command{
		Name:      "dump",
		Usage:     "データファイルを解析してコンソールへ表示する",
		ArgsUsage: "<config.yaml> <input.dat>",
		Action: func(_ context.Context, c *cli.Command) error {
			if c.Args().Len() != 2 {
				return cli.Exit("引数は <config.yaml> <input.dat> の2つです", 2)
			}
			return app.Dump(c.Args().Get(0), c.Args().Get(1), os.Stdout)
		},
	}
}

func createCopybookCommand() *cli.Command {
	return &cli.Command{
		Name:      "create-copybook",
		Usage:     "record 定義から COBOL コピーブックを生成する",
		ArgsUsage: "<config.yaml>",
		Flags: []cli.Flag{
			outputFlag("生成したコピーブックの出力先。省略時は標準出力"),
			&cli.IntFlag{
				Name:  "start-level",
				Usage: "断片モードの開始レベル番号（2〜49）。指定すると 01 行を出さないコピーブック断片を生成する",
			},
		},
		Action: func(_ context.Context, c *cli.Command) error {
			configPath, err := singleArg(c, "<config.yaml>")
			if err != nil {
				return err
			}

			w, dest, closeFn, err := resolveOutput(c.String("output"))
			if err != nil {
				return err
			}
			if err := app.CreateCopybook(configPath, c.Int("start-level"), dest, w, os.Stderr); err != nil {
				closeFn()
				return err
			}
			return closeFn()
		},
	}
}

func importCopybookCommand() *cli.Command {
	return &cli.Command{
		Name:      "import-copybook",
		Usage:     "COBOL コピーブックから設定 YAML を生成する",
		ArgsUsage: "<input.cpy>",
		Flags: []cli.Flag{
			outputFlag("生成した設定 YAML の出力先。省略時は標準出力"),
			&cli.BoolFlag{
				Name:  "fragment",
				Usage: "01 レベルを持たないコピーブック断片として取り込む",
			},
			&cli.StringFlag{
				Name:  "name",
				Value: "DATA-RECORD",
				Usage: "断片モードで生成 YAML に付けるレコード名",
			},
		},
		Action: func(_ context.Context, c *cli.Command) error {
			inputPath, err := singleArg(c, "<input.cpy>")
			if err != nil {
				return err
			}

			w, dest, closeFn, err := resolveOutput(c.String("output"))
			if err != nil {
				return err
			}
			if err := app.ImportCopybook(inputPath, c.Bool("fragment"), c.String("name"), dest, w, os.Stderr); err != nil {
				closeFn()
				return err
			}
			return closeFn()
		},
	}
}

// outputFlag は -o / --output の出力先フラグを生成する。
func outputFlag(usage string) cli.Flag {
	return &cli.StringFlag{
		Name:      "output",
		Aliases:   []string{"o"},
		Usage:     usage,
		TakesFile: true,
	}
}

// singleArg は位置引数がちょうど 1 つであることを確認し、その値を返す。
func singleArg(c *cli.Command, name string) (string, error) {
	if c.Args().Len() != 1 {
		return "", cli.Exit(fmt.Sprintf("引数は %s の1つです", name), 2)
	}
	return c.Args().Get(0), nil
}

// resolveOutput は -o の値からデータ用 Writer を解決する。空のときは標準出力を返す。
// dest は確認メッセージに使う出力先ラベル（標準出力のときは空文字）。closeFn は
// ファイルを開いた場合の後始末で、標準出力のときは何もしない。
func resolveOutput(path string) (w io.Writer, dest string, closeFn func() error, err error) {
	if path == "" {
		return os.Stdout, "", func() error { return nil }, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, "", nil, err
	}
	return f, path, f.Close, nil
}

// isTerminal は f が端末（TTY）に直結しているかを標準ライブラリだけで判定する。
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
