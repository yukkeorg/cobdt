# TODO

作業メモ。設計の経緯は `docs/adr/`、仕様は `docs/design.md` を参照。

## 完了

- [x] OCCURS 句への対応
- [x] REDEFINES 句への対応
- [x] コピーブック取り込みモード（import-copybook）
- [x] 表意定数 LOW-VALUE / HIGH-VALUE への対応

## 未着手

- [ ] EBCDIC（日本語 EBCDIC / DBCS）の N 項目エンコード・デコード実装
      （現状は `n-encode: ebcdic` を値として受け付けるが、エンコード・デコード時にエラー。
      Go 標準ライブラリ群に CP930/939 などの DBCS 変換がないため保留。`docs/design.md` 参照）

## スコープ外（意図的な非対応）

実装漏れではなく、設計判断・COBOL 仕様により対応しないもの。

- import-copybook が受け付けない構文（COMP/BINARY、編集用 PICTURE、SYNC、OCCURS DEPENDING ON、
  66/77 レベルなど）。`docs/adr/0002-import-copybook-strict-subset.md` 参照。
- 同一エントリでの OCCURS と REDEFINES の併用。COBOL 仕様で違法のためエラーとする
  （再定義グループの従属項目に OCCURS を付けるのは合法で、対応済み）。
  `docs/adr/0001-redefines-overlapping-area.md` 参照。
