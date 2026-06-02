# import-copybook は表現できない構文を best-effort 変換せずエラーにする

`import-copybook` は現実のコピーブック（任意レベル番号・列レイアウト・コメント・継続行）を受け入れる堅牢パーサにする一方、cdm のモデル（型 9/X/N/FILLER、usage DISPLAY/PACKED-DECIMAL、固定要素数 OCCURS、REDEFINES）で表現できない構文 — COMP/BINARY/COMP-1/COMP-2 のバイナリ数値、Z・,・.・$ などの編集用 PICTURE、A 英字型、SYNC（詰めバイトでレコード長が変わる）、JUSTIFIED/BLANK WHEN ZERO、OCCURS DEPENDING ON、66/77 レベル — に遭遇したら、行と項目を示して**エラーで中断**すると決めた。レイアウトに影響しない要素（88 レベル、7 桁目コメント行、1–6 桁シーケンス番号領域）だけは黙って読み飛ばす。

best-effort 変換（未対応項目を同一バイト数の FILLER に置換する／無視してスキップする）も検討したが、退けた。cdm の最終目的は固定長データファイルの create/dump であり、変換が一項目でもバイト数を取り違えると後続のオフセットが全てずれて静かに壊れたデータを生む。沈黙して不正確な YAML を出すより、表現できないと明示して止めるほうが安全だからである。

## Consequences

- 実装は `cobol.ParseField` を再利用するため COMP・A 型・多くの編集 PIC は自動的にエラーになるが、`PIC 99.99` のように `.` や `Z` を含む編集 PIC は `9(4)` と誤読されうる。cdm 許容文字集合 `{9 X N S V ( ) 数字}` 外を含む PIC を明示的に弾く検証を別途設ける。
- ツリー復元はレベル番号の相対大小のみで行う（子は親より大きいレベル番号、兄弟は同一レベル番号）。PIC を持たず従属項目を持つものを集団項目、PIC を持つものを基本項目/FILLER とみなす。
- コピーブックが持たない情報は補完する。`organization: sequential`・`n-encode: sjis` を既定値として明示出力し、`data:` はコメント雛形＋空リストを出力する（create に使うにはユーザが data を追記する）。
- VALUE 句は解釈するが、cdm 既定値（数値=ZERO/他=SPACE）と一致するものは省略し、非既定値のみ `value:` として出力する。
