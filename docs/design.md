# cdm - Cobol Data Manipurator

## 概要

本アプリケーションは、COBOL 言語で使用されるデータファイルを取り扱うコマンドラインツールである。
YAML で記述したレコード定義をもとに、次の 3 つの操作を行う。

- COBOL が読み取れるデータファイルを作成する（データ作成モード）
- 既存のデータファイルを定義に従って解析し、項目名・型・値を画面へ出力する（データダンプモード）
- レコード定義から COBOL のコピーブックを生成する（コピーブック作成モード）

## プログラムの構成やルール

- Go 言語を使用する。
- コマンド作成用のソースコードは `cmd/cdm/main.go` に記述する。
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
| 集団項目 | `type: GROUP`, `name`, `subs` | `occurs` | `usage`, `value` |
| 基本項目 | `type`, `name` | `usage`, `value`, `occurs` | — |
| FILLER | `type: FILLER(DIGIT)` | `value`, `occurs` | `name`, `usage` |

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

### 初期値（value）

`value` は項目の初期値を表す。指定されていない場合、数値項目は `ZERO`、それ以外は `SPACE` とする。
`value` はコピーブック作成モードの VALUE 句として出力する。データ作成モードでは `data` の値を使うため反映しない。

`ZERO`（または `ZEROS`, `ZEROES`）、`SPACE`（または `SPACES`）が単独で出現した場合は、
COBOL 言語における表意定数（予約語）と同様に扱う。コピーブックでは引用符なしの予約語として出力する。

## データ（data）

`data` は、データ作成モードでデータファイルを作成するときの値を持つ。各行が 1 レコードに対応する。

`record` で指定した項目に対応する値を指定する。`type` が FILLER の項目の値も指定しなければならない。
値は `record` 項目の構造をフラット化した順に並べる。したがって集団項目自体の値は指定しない。

表項目（`occurs`）の値は入れ子のリストで指定する。

- 基本項目の表: `[要素1, 要素2, ...]`
- 集団項目の表: `[[従属1, 従属2], [従属1, 従属2], ...]`（繰り返しごとにネストする）

値に `ZERO`（または `ZEROS`, `ZEROES`）、`SPACE`（または `SPACES`）が単独で出現した場合は、
COBOL 言語における表意定数と同様に扱い、項目全体をゼロ／空白で埋める。

## モード

本コマンドは次のモードを持ち、サブコマンドとして指定する。

| サブコマンド | モード |
| --- | --- |
| `create` | データ作成モード |
| `dump` | データダンプモード |
| `create-copybook` | コピーブック作成モード |

### データ作成モード（create）

`record` を参照し、`organization` で指定された編成で、`data` の内容を持つデータファイルを作成する。

### データダンプモード（dump）

データファイルを `organization` で指定された編成で読み込み、`record` で解析して、
標準出力にデータ項目名・型名・値を表示する。表項目の各要素は添字付き（`NAME(1)` など）で表示する。

### コピーブック作成モード（create-copybook）

`record` を参照し、レコードの内容を記述したコピーブックを作成する。
出力先を指定しない場合は標準出力へ出力する。

レベル番号はレコード名（`name`）を 01 とし、`record` 直下の項目を 03 から始め、
集団項目に入るたびにレベルを +2 する。

各項目は次の規則で出力する。

- 基本項目・FILLER は PICTURE 句を持つ。FILLER は `PIC X(DIGIT)` として出力する。
- `usage` が PACKED-DECIMAL / COMP-3 の項目は `PACKED-DECIMAL` を出力する。DISPLAY は句を省略する。
- 表項目は `OCCURS n TIMES` を出力する。`occurs` と `value` は排他のため、表項目に VALUE 句は出力しない。
- VALUE 句は（表項目を除く）すべての基本項目・FILLER に出力する。`value` が未指定のものは
  数値項目を `VALUE ZERO`、それ以外を `VALUE SPACE` とする。
- 集団項目は PICTURE 句・VALUE 句を持たない。`occurs` があれば `OCCURS n TIMES` を付ける。
