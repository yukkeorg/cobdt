# TODO

作業メモ。設計の経緯は `docs/adr/`、仕様は `docs/design.md` を参照。

## 完了

- [x] OCCURS 句への対応
- [x] REDEFINES 句への対応
- [x] コピーブック取り込みモード（import-copybook）
- [x] 表意定数 LOW-VALUE / HIGH-VALUE への対応
- [x] data の raw セル（数値項目への生バイト書き込み）
- [x] コピーブック断片（01 を持たないコピーブック）の入出力対応
      （create-copybook `--start-level N`、import-copybook `--fragment` / `--name`）。
      `docs/adr/0003-copybook-fragment-explicit-flag.md`・`docs/CONTEXT.md`「コピーブック断片」参照。
- [x] create モードの別ファイル YAML データ入力（`create --data-yaml <data.yaml>`）。`data` 部だけを
      切り出した YAML を読み、inline `data` を無視する。フォーマットは inline `data` と同一（入れ子・
      raw・表意定数フル対応）。`data:` 以外のキー・0 件・キー欠如はエラー。
      `docs/design.md`「別ファイルの YAML データ入力」参照。

## 未着手

- [ ] create モードの CSV データ入力（`create --data-csv <data.csv>`）。ヘッダ名で項目に対応づけ、
      FILLER 非対応・OCCURS/再定義を含む record はエラー。`docs/design.md`「CSV からのデータ入力」・
      `docs/adr/0004-csv-input-header-mapping.md` 参照。
- [ ] EBCDIC（日本語 EBCDIC / DBCS）の N 項目エンコード・デコード実装
      （現状は `n-encode: ebcdic` を値として受け付けるが、エンコード・デコード時にエラー。
      Go 標準ライブラリ群に CP930/939 などの DBCS 変換がないため保留。`docs/design.md` 参照）

## スコープ外（意図的な非対応）

実装漏れではなく、設計判断・COBOL 仕様により対応しないもの。

- import-copybook が受け付けない構文（COMP/BINARY、編集用 PICTURE、SYNC、OCCURS DEPENDING ON、
  66/77 レベルなど）。`docs/adr/0002-import-copybook-strict-subset.md` 参照。
- 同一エントリでの OCCURS と REDEFINES の併用。COBOL 仕様で適合しないためエラーとする
  （再定義グループの従属項目に OCCURS を付けるのは合法で、対応済み）。
  `docs/adr/0001-redefines-overlapping-area.md` 参照。
