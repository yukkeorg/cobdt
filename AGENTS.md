# AGENTS.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 概要

cobdt (Cobol Data Tool) は、YAML で記述したレコード定義から COBOL 用の固定長データファイルを
操作(Manupulation) する CLI ツール。

コードコメント・ドキュメント・エラーメッセージはすべて日本語で書く。

## ドキュメント

- `docs/design.md` は仕様、 `docs/todo.md` は作業メモ。
- `docs/CONTEXT.md`（語彙集）と `docs/adr/`（設計判断）はドメイン理解の一次資料。
  - 語彙や設計判断に関わる変更を加えるときは `grill-with-docs` スキルでこれらを更新する運用。

## コマンド

```sh
go build -o cobdt ./cmd/cobdt        # ビルド
go test ./...                    # 全テスト
go test ./internal/cobol/        # 1 パッケージのテスト
go test ./internal/cobol/ -run TestRedefineLayout   # 単一テスト
go vet ./...                     # 静的解析
```

実行（ビルド済みバイナリ `cobdt`、または `go run ./cmd/cobdt`）:

```sh
cobdt create          <config.yaml> <output.dat>   # YAML からデータファイル作成
cobdt dump            <config.yaml> <input.dat>    # データファイルを解析表示
cobdt create-copybook <config.yaml> [output.cpy]   # コピーブック生成（省略時は stdout）
```

サンプル設定は `extra/`（`example.yaml`, `redefine_example.yaml`）。
