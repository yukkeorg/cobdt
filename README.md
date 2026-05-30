# cdm - Cobol Data Manipurator

YAML 定義から COBOL 言語で利用するデータファイルを作成・解析するコマンドラインツールです。

- YAML に記述したレコード定義（PICTURE / USAGE / 編成）からデータファイルを生成する
- 既存のデータファイルを定義に従って解析し、項目名・型・値を表示する
- レコード定義から COBOL のコピーブックを生成する

## 特徴

- ファイル編成: 順編成（sequential）／行順編成（line sequential）
- データ型: 数値項目（`9`、符号 `S`・仮想小数点 `V`）、英数字項目（`X`）、日本語項目（`N`）、無名項目（`FILLER`）
- 内部格納形式: ゾーン10進数（DISPLAY）／パック10進数（PACKED-DECIMAL・COMP-3）
- 集団項目（グループ）のネストに対応
- 日本語項目の文字エンコード指定（`sjis`。`ebcdic` は将来対応）

## 必要環境

- Go 1.26 以降

依存ライブラリ:

- 文字コードの取り扱い: `golang.org/x/text`
- YAML ファイル操作: `go.yaml.in/yaml/v4`

## ビルド

```sh
go build -o cdm ./cmd/cdm
```

テストの実行:

```sh
go test ./...
```

## 使い方

```text
cdm create          <config.yaml> <output.dat>   YAML の内容からデータファイルを作成
cdm dump            <config.yaml> <input.dat>    データファイルを解析してコンソールへ表示
cdm create-copybook <config.yaml> [output.cpy]   record 定義から COBOL コピーブックを生成（省略時は標準出力）
```

### データ作成（create）

`record` を参照し、`organization` で指定された編成で、`data` の内容を持つデータファイルを作成します。

```sh
cdm create example.yaml output.dat
```

### データダンプ（dump）

データファイルを `organization` で指定された編成で読み込み、`record` で解析して、項目名・型名・値を標準出力へ表示します。

```sh
cdm dump example.yaml output.dat
```

### コピーブック作成（create-copybook）

`record` の定義から COBOL コピーブックを生成します。出力先を省略すると標準出力へ出力します。

```sh
cdm create-copybook example.yaml record.cpy
```

レベル番号はレコード名（`name`）を 01 とし、`record` 直下の項目を 03 から始め、集団項目に入るたびに +2 します。

出力例:

```cobol
       01  EMP-RECORD.
           03  EMP.
               05  EMP-ID                PIC 9(5) VALUE 0.
               05  EMP-NAME              PIC X(20) VALUE SPACE.
               05  EMP-NAME-KANJI        PIC N(8) VALUE SPACE.
           03  SALARY                    PIC S9(7)V99 PACKED-DECIMAL VALUE ZERO.
           03  BONUS-RATE                PIC S9(1)V99 VALUE ZERO.
           03  FILLER                    PIC X(3) VALUE SPACE.
```

## YAML ファイルフォーマット

```yaml
name: DATARECORD-NAME
organization: ORGANIZATION
n-encode: N-ENCODE
record:
    - GROUPNAME:
        - type: TYPE(DIGIT)
          name: DATANAME
          usage: USAGE
          value: INITIAL-VALUE
        - type: FILLER(DIGIT)
          value: INITIAL-VALUE
    - type: TYPE(DIGIT)
      name: DATANAMEn
      usage: USAGE
      value: INITIAL-VALUE
    - type: FILLER(DIGIT)
      value: INITIAL-VALUE
data:
    - [DATANAME-VALUE, FILLER-VALUE, ..., DATANAMEn-VALUE]
    - [ ... ]
```

`type`・`organization`・`n-encode` に指定する値は大文字小文字を区別しません。

### トップレベル

| キー | 説明 |
| --- | --- |
| `name` | レコード名。未指定なら `DATA-RECORD`。コピーブックでは 01 レベル名として使う |
| `organization` | ファイル編成。`sequential`（順編成）／`line sequential`（行順編成） |
| `n-encode` | N 項目の文字エンコード。`sjis`（shift-jis, cp932。既定）／`ebcdic` |
| `record` | レコードの項目定義（集団項目・基本項目・FILLER のシーケンス） |
| `data` | データファイルへ書き込む値。`record` をフラット化した順に並べる |

### 項目（record）

- **集団項目（GROUPNAME）**: `type`・`usage` は持たず、子として集団項目・基本項目・FILLER を持てる。
- **基本項目（DATANAME）**: `type`（必須）・`name`（必須）・`usage`・`value` を指定する。
- **FILLER**: 無名項目。`type: FILLER(桁数)` で指定し、型は X として扱う。

#### データ型（type）

| 型 | 説明 |
| --- | --- |
| `9` | 数値項目。右詰め、空き桁は 0 埋め。小数部含めて最大 31 桁。`S`=符号、`V`=仮想小数点 |
| `X` | 英数字項目。左詰め、空き桁は半角空白埋め |
| `N` | 日本語項目。文字コードは `n-encode`。左詰め、空き桁は全角空白埋め |
| `FILLER` | 無名項目。桁数を指定でき、型は X |

`DIGIT` は桁数を表します。`9(3)` は `999` とも書けます。

#### 内部格納形式（usage）

数値項目（`9`）のみ指定でき、未指定なら `DISPLAY` です。

| usage | 説明 |
| --- | --- |
| `DISPLAY` | ゾーン10進数 |
| `PACKED-DECIMAL` | パック10進数 |
| `COMP-3` | パック10進数 |

#### 初期値（value）

`value` は項目の初期値を表します。未指定の場合、数値項目は `ZERO`、それ以外は `SPACE` です。
`value` はコピーブック作成モードの VALUE 句として出力され、データ作成モードでは `data` の値を使用します。

`ZERO`（または `ZEROS`・`ZEROES`）・`SPACE`（または `SPACES`）が単独で指定された場合は、COBOL の表意定数として扱います（コピーブックでは引用符なしの予約語として出力）。

### データ（data）

`record` で指定した項目に対応する値を指定します。`type` が FILLER の項目の値も指定する必要があります。
値は `record` の構造をフラット化した順に並べます（集団項目自体の値は指定しません）。

値に `ZERO`（または `ZEROS`・`ZEROES`）・`SPACE`（または `SPACES`）が単独で指定された場合は、COBOL の表意定数として扱い、項目全体をゼロ／空白で埋めます。

### 設定例

```yaml
name: EMP-RECORD
organization: sequential
n-encode: sjis
record:
    - EMP:
        - type: 9(5)
          name: EMP-ID
          value: 0
        - type: X(20)
          name: EMP-NAME
        - type: N(8)
          name: EMP-NAME-KANJI
    - type: S9(7)V99
      name: SALARY
      usage: COMP-3
    - type: S9(1)V99
      name: BONUS-RATE
    - type: FILLER(3)
data:
    - [123, John Smith, 山田太郎, 1234567.89, -1.25, "***"]
    - [7, Alice, 鈴木花子, -42.5, 0.5, "---"]
```

## ライセンス

本ソフトウェアは MIT ライセンスの下で配布されます。
