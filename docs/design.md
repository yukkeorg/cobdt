# cobdt - Cobol Data Tool

## 概要

本アプリケーションは、COBOL 言語で使用されるデータファイルを取り扱うコマンドラインツールである。
YAML で記述したレコード定義をもとに、次の 3 つの操作を行う。

- COBOL が読み取れるデータファイルを作成する（データ作成モード）
- 既存のデータファイルを定義に従って解析し、項目名・型・値を画面へ出力する（データダンプモード）
- レコード定義から COBOL のコピーブックを生成する（コピーブック作成モード）

## プログラムの構成やルール

- Go 言語を使用する。
- コマンド作成用のソースコードは `cmd/cobdt/main.go` に記述する。
- 可能な限りモジュール化を行いパッケージ化する。
- パッケージ化したモジュールは `internal/` に配置する。
- 外部ライブラリは次を利用する。
  - 文字コードの取り扱い: `golang.org/x/text`
  - YAML ファイル操作: `go.yaml.in/yaml/v4`

## YAML ファイルフォーマット

設定 YAML のフォーマットは次のとおり。

```yaml
name: DATARECORD-NAME
organization: ORGANIZATION
n-encode: N-ENCODE
record:
    - type: GROUP
      name: GROUP-NAME
      subs:
        - type: TYPE(DIGIT)
          name: DATANAME
          usage: USAGE
          value: INITIAL-VALUE
        - type: TYPE(DIGIT)
          name: TABLE-DATANAME
          occurs: OCCURS
        - type: FILLER(DIGIT)
          value: INITIAL-VALUE
    - type: TYPE(DIGIT)
      name: DATANAMEn
      usage: USAGE
      value: INITIAL-VALUE
    - type: FILLER(DIGIT)
      value: INITIAL-VALUE
data:
    - [DATANAME-VALUE, [TABLE-VALUE-1, ...], FILLER-VALUE, ..., DATANAMEn-VALUE, FILLER-VALUE]
    - [ ... ]
```

`type`・`organization`・`n-encode` に指定する値は、ケースインセンシティブ（大文字小文字を区別しない）である。

### トップレベルのキー

| キー | 説明 |
| --- | --- |
| `name` | レコード名（DATARECORD-NAME）。未指定なら `DATA-RECORD`。コピーブックでは 01 レベルのレコード名として使う |
| `organization` | ファイル編成。`sequential`／`line sequential` |
| `n-encode` | N（日本語）項目の文字エンコード。未指定なら `sjis` |
| `record` | レコードの項目定義（集団項目・基本項目・FILLER のシーケンス） |
| `data` | データ作成モードでファイルへ書き込む値。`record` をフラット化した順に並べる |

## ファイル編成（organization）

`organization` は次に対応する。いずれも固定長レコードである。

| 編成 | 説明 |
| --- | --- |
| `sequential` | 順編成。区切り文字なし。 |
| `line sequential` | 行順編成。区切り文字は改行コード。 |

## N データ項目の文字エンコード（n-encode）

`n-encode` には N 項目の文字エンコードを以下から指定できる。未指定の場合は `sjis` とする。

| エンコード | 別名 |
| --- | --- |
| `sjis` | `shift-jis`, `shiftjis`, `shift_jis`, `cp932` |
| `ebcdic` | |

`sjis` のみ実装済みである。`ebcdic`（日本語 EBCDIC / DBCS）は値としては受け付けるが、
これを利用する N 項目のエンコード・デコード時にエラーを返す。
Go の標準的なライブラリ群に日本語 EBCDIC（CP930/939 など）の DBCS 変換が存在しないためである。

## レコードの項目（record）

`record` は項目のシーケンスである。各項目は必ず `type` を持つ。
`type: GROUP` であれば集団項目、それ以外は基本項目または FILLER として扱う。

| 区分 | 必須キー | 任意キー | 指定できないキー |
| --- | --- | --- | --- |
| 集団項目 | `type: GROUP`, `name`, `subs` | `occurs`, `redefine` | `usage`, `value` |
| 基本項目 | `type`, `name` | `usage`, `value`, `occurs`, `redefine` | — |
| FILLER | `type: FILLER(DIGIT)` | `value`, `occurs` | `name`, `usage`, `redefine` |

- **集団項目**: 従属項目のリストを `subs` に持つ。従属項目には集団項目・基本項目・FILLER を入れられる（ネスト可）。
- **基本項目**: 単一のデータ項目。`type` に PICTURE を指定する。
- **FILLER**: 無名項目。型は X として扱う。

### データ型（type）

`type` は項目の型を指定し、次に対応する。

| 型 | 説明 |
| --- | --- |
| `9` | 数値項目。右詰め、空いた桁は 0 で埋める。小数部を含めて最大 31 桁。`S`=符号、`V`=仮想小数点 |
| `X` | 英数字項目。左詰め、空いた桁は半角空白で埋める |
| `N` | 日本語項目。文字コードは `n-encode`。左詰め、空いた桁は全角空白で埋める |
| `FILLER` | 無名項目。桁数を指定でき、型は X として扱う |
| `GROUP` | 集団項目。`name` と `subs`（従属項目）を持つ |

`DIGIT` はその項目の桁数を表す。`9(3)` は `999` とも表現できる。

### 内部格納形式（usage）

`usage` は数値項目（`9`）の内部格納形式を表し、次に対応する。未指定の場合は `DISPLAY`。
数値項目以外には指定できない。

| usage | 説明 |
| --- | --- |
| `DISPLAY` | ゾーン10進数 |
| `PACKED-DECIMAL` | パック10進数 |
| `COMP-3` | パック10進数 |

### 表項目（occurs）

`occurs` に要素数（1 以上の整数）を指定すると、その項目を表（配列）として要素数ぶん繰り返す。
基本項目・集団項目のどちらにも指定できる。

`occurs` と `value` は同時に指定できない。

`data` では表項目の値を入れ子のリストで指定する（[データ](#データdata) を参照）。
ダンプ表示では各要素を `NAME(1)`, `NAME(2)` のように添字付きで表示する。
集団項目の表に含まれる従属項目は `NAME(2, 3)` のように外側からの添字を連ねて表示する。

### 再定義（redefine）

`redefine` に対象の項目名を指定すると、その項目（再定義項目）は対象の項目（原定義）と
**同じバイト領域**を別の意味で解釈する。領域は増えず、解釈だけが増える。レコード種別に
よって後続の意味が変わるレイアウトを表すために使う。基本項目・集団項目のどちらにも指定できる。

```yaml
record:
    - type: 9(2)
      name: REC-TYPE
    - type: GROUP            # 原定義（13 バイト）
      name: BODY-PERSON
      subs:
        - { type: X(10), name: PERSON-NAME }
        - { type: 9(3),  name: PERSON-AGE }
    - type: GROUP            # BODY-PERSON を再定義（13 バイト）
      name: BODY-COMPANY
      redefine: BODY-PERSON
      subs:
        - { type: 9(7), name: COMP-ID }
        - { type: X(6), name: COMP-CODE }
```

- 再定義項目は対象と**同じ階層の兄弟**で、対象の直後（または同じ対象を指す別の再定義項目の
  直後）に置く。対象名はその位置より前の同階層項目として解決する（大文字小文字は区別しない）。
- 原定義と各再定義項目は**同一バイト数**でなければならない。揃わない場合は FILLER でパディングする。
- 同じ対象を複数の再定義項目が指してよい。
- `redefine` と `occurs` は併用できない（対象側・再定義側のいずれも）。
- ダンプ表示では、同じバイトを原定義と各再定義項目の両方の解釈でラベル付きに並べて表示する
  （ツールはレコード種別から解釈を自動選択しない）。
- コピーブックでは対象と同じレベル番号で `REDEFINES 対象名` を出力する。

`data` での値の与え方は [データ](#データdata) を参照。

### 初期値（value）

`value` は項目の初期値を表す。指定されていない場合、数値項目は `ZERO`、それ以外は `SPACE` とする。
`value` はコピーブック作成モードの VALUE 句として出力する。データ作成モードでは `data` の値を使うため反映しない。

次の表意定数が単独で出現した場合は、COBOL 言語における表意定数（予約語）と同様に扱う。
コピーブックでは引用符なしの予約語として出力する。

| 表意定数 | 別名 | 種別 | 埋める内容 |
| --- | --- | --- | --- |
| `ZERO` | `ZEROS`, `ZEROES` | 文字埋め | 型に応じたゼロ（数値はゼロ、X は `0`、N は全角ゼロ） |
| `SPACE` | `SPACES` | 文字埋め | 空白（X は半角空白、N は全角空白） |
| `LOW-VALUE` | `LOW-VALUES` | バイト埋め | 型に関係なく項目のバイト長ぶんを `0x00` で埋める |
| `HIGH-VALUE` | `HIGH-VALUES` | バイト埋め | 型に関係なく項目のバイト長ぶんを `0xFF` で埋める |

## データ（data）

`data` は、データ作成モードでデータファイルを作成するときの値を持つ。各行が 1 レコードに対応する。

`record` で指定した項目に対応する値を指定する。`type` が FILLER の項目の値も指定しなければならない。
値は `record` 項目の構造をフラット化した順に並べる。したがって集団項目自体の値は指定しない。

表項目（`occurs`）の値は入れ子のリストで指定する。

- 基本項目の表: `[要素1, 要素2, ...]`
- 集団項目の表: `[[従属1, 従属2], [従属1, 従属2], ...]`（繰り返しごとにネストする）

値に表意定数（`ZERO`／`SPACE`／`LOW-VALUE`／`HIGH-VALUE` とその別名）が単独で出現した場合は、
COBOL 言語における表意定数と同様に扱い、項目全体を埋める（[初期値](#初期値value) の表を参照）。

### 生バイト書き込み（raw セル）

通常、数値項目に与えた値は数字以外を除去して整形する（`12a34` → `01234`）。これに対し、値を
`{raw: "..."}` のマップで与えると、**sanitize も表意定数解釈もせず、文字列をそのままバイト列**として
書き込む。COBOL プログラムの不正データ耐性をテストするために、数値項目へ意図的に非数値バイトを
注入する用途で使う。

- **ゾーン10進数（DISPLAY）の数値項目にのみ**指定できる。PACKED-DECIMAL・X・N・集団項目に指定すると
  エラーになる。
- `raw` の値のバイト長は、その項目のバイト長（`9(5)` なら 5）と**正確に一致**しなければならない。
  一致しない場合はエラーとする（固定長レコードを崩さないため）。バイト長は UTF-8 換算で数える。
  末尾の空白を保つには引用符で囲む（`{raw: "AB   "}`）。
- 基本項目・OCCURS の各要素・集団項目内の葉のいずれの位置でも指定できる。

```yaml
data:
    # N1 は 9(5)。"ABCDE" を 5 バイトの生バイトでそのまま格納する
    - [{raw: "ABCDE"}, 123]
```

### 再定義領域の値

再定義領域（原定義とそれを指す再定義項目群）は、`data` 行で常に **1 スロット**を占める。
レコードごとに、その領域をどの定義で埋めるかを選ぶ。

- **原定義を使う**: 値をネストリスト（単一基本項目ならスカラ）で与える。
- **再定義項目を使う**: `{target: 対象項目名, value: 値}` のマップで与える。`value` は対象が
  集団項目ならネストリスト、基本項目ならスカラ。

1 行・1 領域につき書ける解釈は 1 つだけである。

```yaml
data:
    # 種別 01: 原定義 BODY-PERSON を使う
    - [01, ["ALICE", 30], "END"]
    # 種別 02: BODY-COMPANY で再定義
    - [02, {target: BODY-COMPANY, value: [1234567, "ABCDEF"]}, "END"]
```

## モード

本コマンドは次のモードを持ち、サブコマンドとして指定する。

| サブコマンド | モード |
| --- | --- |
| `create` | データ作成モード |
| `dump` | データダンプモード |
| `create-copybook` | コピーブック作成モード |
| `import-copybook` | コピーブック取り込みモード |

コマンドラインの解析には `urfave/cli` v3 を用いる（判断は [ADR 0005](adr/0005-urfave-cli-v3.md) 参照）。

#### バージョン表示（-v / --version）

`-v` / `--version` を付けて実行すると、バージョン情報を 1 行で表示して終了する。

```
cobdt version v1.0.0 (commit abc1234, built 2026-06-07)
```

バージョン・コミット・ビルド日は、リリースビルド時に `build.sh` が ldflags
（`-X main.version=…` など）で埋め込む。`version` はリリースタグ（`release.yml` が
`RELEASE_TAG` に渡す。ローカルの `build.sh` 実行では `local`）、`commit` は短縮 SHA、
`built` はビルド日（UTC）。ldflags を渡さない素の `go build` / `go run` では既定値
（`dev` / `none` / `unknown`）のままになり、リリース版でないことが分かる。

#### 出力先の指定（共通規約）

ファイルを出力するモード（`create` / `create-copybook` / `import-copybook`）の出力先は、
`-o` / `--output <path>` オプションで指定する。**オプションを省略した場合は標準出力へ出力する**。
入力ファイル（`config.yaml` など）は引き続き位置引数で受け取る。

- **データは標準出力、診断は標準エラー出力**: 生成物（データファイル・コピーブック・YAML）は
  `-o` 省略時に標準出力へ流す。「作成しました: …」などの確認・進捗メッセージは、`-o` の有無に
  かかわらず**常に標準エラー出力**へ出す。これにより `cdm create config.yaml | …` のような
  パイプで生成物だけを正しく受け渡せる。
- **バイナリの標準出力と TTY ガード**: `create` は固定長バイナリを出力する。`-o` を省略し、かつ
  標準出力が端末（TTY）に直結している場合は、画面をバイナリで壊さないようエラーで停止する
  （`-o` で出力先を指定するか、パイプ／リダイレクトを促す）。標準出力がパイプ・リダイレクト・
  ファイルなら、そのままバイナリを流す。TTY 判定は標準ライブラリの `os.ModeCharDevice` で行い、
  外部依存は増やさない。テキストを出力する `create-copybook` / `import-copybook` は端末に出ても
  無害なため、このガードは行わない。
- `dump` は解析結果を人が読む表示であり、従来どおり常に標準出力へ表示する（`-o` は持たない）。

### データ作成モード（create）

```sh
<<<<<<< HEAD
cobdt create [--data-csv <data.csv> | --data-yaml <data.yaml>] <config.yaml> <output.dat>
||||||| parent of b419a7c (出力先指定を -o/--output に統一し標準出力を既定にする)
cdm create [--data-csv <data.csv> | --data-yaml <data.yaml>] <config.yaml> <output.dat>
=======
cdm create [--data-csv <data.csv> | --data-yaml <data.yaml>] [-o <output.dat>] <config.yaml>
>>>>>>> b419a7c (出力先指定を -o/--output に統一し標準出力を既定にする)
```

`record` を参照し、`organization` で指定された編成でデータファイルを作成する。`-o` を省略すると
固定長バイナリを標準出力へ出力するが、標準出力が端末のときはエラーで停止する（前述
「出力先の指定（共通規約）」の TTY ガード）。書き込む値は既定では
YAML の inline `data` から取るが、外部ファイルから取り込むこともできる。

- `--data-csv <data.csv>`: CSV ファイルから取り込む
  （後述「[CSV からのデータ入力](#csv-からのデータ入力create---data-csv)」）。
- `--data-yaml <data.yaml>`: `data` 部だけを切り出した別 YAML ファイルから取り込む
  （後述「[別ファイルの YAML データ入力](#別ファイルの-yaml-データ入力create---data-yaml)」）。

これら外部データ源フラグ（`--data-*`）を指定すると inline の `data` を無視する。`--data-csv` と
`--data-yaml` の同時指定はエラーとする。

### CSV からのデータ入力（create --data-csv）

create モードで `--data-csv <data.csv>` を指定すると、書き込む値を YAML の `data` ではなく CSV ファイルから
取り込む。`record`・`organization`・`n-encode` は引き続き YAML から取る。既存システムや表計算が吐く、
集団項目で構造化された素朴な固定長表データを大量投入する用途に絞る。対応づけを位置順でなくヘッダ名で行う
判断は [ADR 0004](adr/0004-csv-input-header-mapping.md) を参照。

- **対象レコード**: 集団項目（GROUP）は対応するが、**OCCURS 項目・再定義**（`redefine` とその対象）を
  含む `record` に `--data-csv` を指定すると load 時にエラーとする。これらは 1 つの CSV 列では表現できず、
  `value` フォールバックも効かない（`occurs` と `value` は排他）ため、黙って既定で埋めず止める。
- **列と項目の対応（ヘッダ名）**: CSV の 1 行目をヘッダ（列名行）とし、列名を `record` の葉項目（基本項目）
  の名前と突き合わせる（位置順ではない）。突き合わせはケースインセンシティブ。各データ行が 1 レコードに対応する。
- **修飾名**: 葉名が一意ならその名前だけでよい。同名の葉が複数あるときだけ、集団名で `GROUP.LEAF` のように
  ドットで修飾して一意化する（一意になる最小限の部分パスでよい。なお曖昧ならエラー）。COBOL の修飾参照
  （`LEAF OF GROUP`）に相当する。
- **FILLER は入力不可**: FILLER は名前を持たないため CSV からは入力できない。
- **CSV に列がない項目**: FILLER・名前つきを問わず、その項目の `value`（未指定なら数値は `ZERO`、
  それ以外は `SPACE`）で埋める。「列がある＝そのセルの値、列がない＝初期値」という対応。
- **未知・曖昧な列**: どの葉にも一致しない列名、集団名に一致する列、修飾先が集団になる列、解決先が同じ葉に
  なる重複列は、いずれも行・列名を示してエラーとする。
- **セル値の解釈**: セル文字列はそのまま `cobol.Encode` に渡す。表意定数（`ZERO`／`SPACE`／`LOW-VALUE`／
  `HIGH-VALUE` と別名）は `data` と同様に解釈する。**空セルは空文字列**として扱う（型ごとの既定の埋めになる）。
  `raw` セルは非対応で、生バイト注入が必要なら YAML の `data` を使う。
- **文字コード・書式**: CSV は UTF-8 固定（先頭の BOM は除去する）。カンマ区切り（RFC 4180、ダブルクォートで
  のクオート・改行埋め込みに対応）。セル値は前後空白を保持し、ヘッダ列名は前後空白をトリムして突き合わせる。
- **行**: ヘッダ行は必須。ヘッダのみ・空ファイルはエラーとする（最低 1 データ行を要求）。空行は読み飛ばす。

### 別ファイルの YAML データ入力（create --data-yaml）

create モードで `--data-yaml <data.yaml>` を指定すると、書き込む値を inline の `data` ではなく、`data` 部
だけを切り出した別の YAML ファイルから取り込む。1 つの定義（`record`/`organization`/`n-encode`）を複数の
データセットで使い回すとき、定義を複製せずデータだけを別ファイルで差し替えるための機能。`--data-csv` と
異なりフォーマットは inline の `data` と完全に同一なので、OCCURS・再定義・raw セル・表意定数を含む入れ子の
値もそのまま使える。

- **ファイル構造**: トップレベルに `data:` キーを持つ YAML。その値は inline の `data` と同じ行のシーケンス。
  `data:` 以外のトップレベルキー（`record`・`organization` など）があればエラーとする（定義は config 側に
  限り、二重定義を防ぐ）。
- **データ源**: `--data-yaml` 指定時は inline の `data` を無視する。`--data-csv` との同時指定はエラー。
- **空入力**: `data:` キーが無いファイル、および `data:` が 0 件のファイルはエラーとする（最低 1 行を要求。
  `--data-csv` と揃える）。
- **値の解釈**: inline の `data` と同一。OCCURS（入れ子リスト）・再定義（`{target, value}`）・raw セル
  （`{raw: ...}`）・表意定数のすべてを inline と同じく扱う。

### データダンプモード（dump）

データファイルを `organization` で指定された編成で読み込み、`record` で解析して、
標準出力にデータ項目名・型名・値を表示する。表項目の各要素は添字付き（`NAME(1)` など）で表示する。

項目の全バイトが `0x00`／`0xFF` のときは、バイト埋めの表意定数とみなして `LOW-VALUE`／`HIGH-VALUE`
とラベル表示し、型に従った復号より優先する（N 項目の `0xFF` のように復号できないバイト列も表示できる）。
正規のゼロは `0x30`、パック10進数のゼロは符号ニブルが非ゼロのため、`LOW-VALUE` とは衝突しない。

### コピーブック作成モード（create-copybook）

```sh
<<<<<<< HEAD
cobdt create-copybook [--start-level N] <config.yaml> [output.cpy]
||||||| parent of b419a7c (出力先指定を -o/--output に統一し標準出力を既定にする)
cdm create-copybook [--start-level N] <config.yaml> [output.cpy]
=======
cdm create-copybook [--start-level N] [-o <output.cpy>] <config.yaml>
>>>>>>> b419a7c (出力先指定を -o/--output に統一し標準出力を既定にする)
```

`record` を参照し、レコードの内容を記述したコピーブックを作成する。
`-o` を省略した場合は標準出力へ出力する（前述「出力先の指定（共通規約）」）。

レベル番号はレコード名（`name`）を 01 とし、`record` 直下の項目を 03 から始め、
集団項目に入るたびにレベルを +2 する。

`--start-level N` を指定すると、01 レコード行を出力しない**コピーブック断片**を生成する
（語彙は `docs/CONTEXT.md`「コピーブック断片」、判断は [ADR 0003](adr/0003-copybook-fragment-explicit-flag.md) 参照）。
このとき `record` 直下の項目を N（2〜49）から始め、集団項目ごとに +2 する。レコード全体を包む 01
集団項目はプログラム側に置き、その中身をこの断片として COPY 句で埋め込む運用を想定する。
完全モード・断片モードのいずれでも、生成されるレベル番号が COBOL の上限 49 を超える場合はエラーとする。

各項目は次の規則で出力する。

- 基本項目・FILLER は PICTURE 句を持つ。FILLER は `PIC X(DIGIT)` として出力する。
- `usage` が PACKED-DECIMAL / COMP-3 の項目は `PACKED-DECIMAL` を出力する。DISPLAY は句を省略する。
- 表項目は `OCCURS n TIMES` を出力する。`occurs` と `value` は排他のため、表項目に VALUE 句は出力しない。
- VALUE 句は（表項目を除く）すべての基本項目・FILLER に出力する。`value` が未指定のものは
  数値項目を `VALUE ZERO`、それ以外を `VALUE SPACE` とする。
- 集団項目は PICTURE 句・VALUE 句を持たない。`occurs` があれば `OCCURS n TIMES` を付ける。
- 再定義項目は対象と同じレベル番号で `REDEFINES 対象名` を出力する（基本項目なら PICTURE 句の前、
  集団項目なら集団名の後ろ）。

ダンプ表示では、再定義領域の同じバイトを原定義・各再定義項目の両方の解釈でラベル付きに並べて
表示する。ツールはレコード種別から解釈を自動選択しない。

### コピーブック取り込みモード（import-copybook）

```sh
<<<<<<< HEAD
cobdt import-copybook [--fragment] [--name NAME] <input.cpy> [output.yaml]   # コピーブックから設定 YAML を生成（省略時は標準出力）
||||||| parent of b419a7c (出力先指定を -o/--output に統一し標準出力を既定にする)
cdm import-copybook [--fragment] [--name NAME] <input.cpy> [output.yaml]   # コピーブックから設定 YAML を生成（省略時は標準出力）
=======
cdm import-copybook [--fragment] [--name NAME] [-o <output.yaml>] <input.cpy>   # コピーブックから設定 YAML を生成（-o 省略時は標準出力）
>>>>>>> b419a7c (出力先指定を -o/--output に統一し標準出力を既定にする)
```

既存の COBOL コピーブックを解析し、`create` / `dump` / `create-copybook` で使える設定 YAML を生成する
（`create-copybook` の逆変換）。`-o` を省略した場合は標準出力へ出力する（前述「出力先の指定（共通規約）」）。

`--fragment` を指定すると、01 レコード行を持たない**コピーブック断片**として取り込む
（語彙は `docs/CONTEXT.md`「コピーブック断片」、判断は [ADR 0003](adr/0003-copybook-fragment-explicit-flag.md) 参照）。
先頭エントリのレベルを最上位とみなして同一レベルの項目群を `record` とする。断片には 01 が含まれない
前提のため、01 レベルが一つでも現れたらエラーとする（指定と現物の矛盾を黙って進めない）。断片には
レコード名がないため、生成 YAML の `name` は `--name NAME`（既定 `DATA-RECORD`）で与える。`--fragment`
を指定しない通常モードでは、従来どおり 01 で始まることを要求する。

現実のコピーブックを対象とし、任意のレベル番号・列レイアウト（1–6 桁のシーケンス番号領域、
7 桁目のコメント行）・継続行に対応する。ツリーはレベル番号の相対的な大小だけで復元する
（子は親より大きいレベル番号、兄弟は同一レベル番号）。PICTURE 句を持たず従属項目を持つものを
集団項目、PICTURE 句を持つものを基本項目／FILLER とみなす。

- **01 レベル**: コピーブックには 01 レベルが 1 つだけ現れることを要求する。その名を `name`、
  従属項目を `record` とする。複数の 01 レベルがある場合はエラーとする。
- **VALUE 句**: 解釈するが、cobdt の既定値（数値項目は `ZERO`、それ以外は `SPACE`）と一致するものは
  `value` を省略し、非既定値のみ `value` として出力する。
- **無視する要素**: 88 レベル（条件名）、コメント行、シーケンス番号領域は読み飛ばす。
- **補完するキー**: コピーブックが持たない `organization`（`sequential`）・`n-encode`（`sjis`）は
  既定値として明示出力し、`data` はコメントの雛形と空リストを出力する。`create` に使うには
  ユーザが `data` を追記する。

cobdt のモデルで表現できない構文に遭遇した場合は、best-effort 変換やスキップをせず、行と項目を
示してエラーで中断する（[ADR 0002](adr/0002-import-copybook-strict-subset.md) 参照）。
具体的には次を未対応とする。

- バイナリ数値: `COMP` / `BINARY` / `COMP-1` / `COMP-2` / `COMP-4` / `COMP-5`
- 編集用 PICTURE: `Z`・`,`・`.`・`$`・`+`・`-`・`A`（英字）・`B`・`/` など、cobdt 許容文字集合
  `{9 X N S V ( ) 数字}` 以外を含む PICTURE
- `SYNC`（SYNCHRONIZED。詰めバイトでレコード長が変わる）、`JUSTIFIED`、`BLANK WHEN ZERO`
- `OCCURS n DEPENDING ON`（可変長テーブル）
- `66`（RENAMES）・`77`（独立項目）レベル
