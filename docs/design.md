# cdm - Cobol Data Manipurator

## 概要

本アプリケーションは、COBOL言語で使用されるデータを作成します。
YAMLファイルからCOBOLが読み取ることができるデータファイルを作成したり、読み込んで値を画面へ出力する。

## プログラムの構成やルール

- Go言語を使用する
- コマンド作成用のソースコードは、cmd/cdm/main.go に記述する。
- 可能な限りモジュール化を行いパッケージ化を行う。
- パッケージ化したモジュールは internal/ に配置する。
- 外部ライブラリは次を利用する。
  - 文字コードの取り扱い: `golang.org/x/text`
  - YAMLファイル操作: `go.yaml.in/yaml/v4`

## YAMLファイルフォーマット

YAMLファイルのフォーマットは次のようにする。

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
        ...
    ...
    - type: TYPE(DIGIT)
      name: DATANAMEn
      usage: USAGE
      value: INITIAL-VALUE
    - type: FILLER(DIGIT)
      value: INITIAL-VALUE
    ... snip ...
data:
    - [DATANAME1-VALUE, FILLER-VALUE,  ... snip ..., DATANAMEn-VALUE, FILLER-VALUE, ... snip ...]
    - [ ... ]
    ...

TYPE、ORGANAIZATION、N-ENCODEに指定される値は、ケースインセンシティブ(大文字小文字不問)である。

### データ編成

DATARECORD-NAME は、 record の名前を指定する。指定されない場合は `DATA-RECORD` とする。
コピーブック作成モードでは 01 レベルのレコード名として使用する。

ORGANIZATION は次に対応する。

- sequential: 順編成。固定長レコード。区切り文字なし。
- line sequential: 行順編成。固定長レコード。区切り文字は改行コード

### Nデータ項目の文字エンコード

N-ENCODEには、N項目の文字エンコードを以下から指定できる。
未指定の場合は`sjis`が設定される。

- sjis(shift-jis, cp932)
- ebcdic

### データ型

GROUPNAME は集団項目を表す。TYPEやUSAGEは指定できない。子の情報として、集団項目、基本項目、または FILLER 項目を持つことができる。

DATANAME は基本項目を表し、TYPEやUSAGEが指定され、具体的な値が入る。

TYPE は基本項目の型を指定し、次に対応する。

- 9: 数値項目。右詰め。空いた桁に0で埋める。小数部含めて最大31桁。
  - S: 符号
  - V: 仮想小数点
- X: 英数字項目。左詰め。空いた桁は半角空白で埋める。
- N: 日本語項目。文字コードはN-ENCODEとする。左詰め。空いた桁は全角空白で埋める。
- FILLER: 無名項目を表し、桁数を指定できる。型はXとする。

USAGE は数値項目(9)の内部格納形式を表し、次に対応する。指定がない場合はDISPLAYとする。数値項目以外のデータ項目には指定できない。

- DISPLAY: ゾーン10進数
- PACKED-DECIMAL: パック10進数
- COMP-3: パック10進数

DIGIT は その項目の桁数を表す。9(3)は、999とも表現できる。

INITIAL-VALUE は 初期値を表す。指定されていない場合、数値項目はZERO、それ以外の項目はSPACEとする。
ZERO(または、ZEROS, ZEROES)、SPACE(または、SPACES) が単独で出現した場合、COBOL言語における予約語と同様の扱いをおこなう。
INITIAL-VALUEはコピーブック作成モードの VALUE 句として出力する。データ作成モードでは data の値を使用するため、VALUE は反映しない。

## データ

recordで指定した項目に対応するデータファイルを作成するときの値として利用する。
typeがFILLERの項目の値も指定しなければならない。
値は、record項目の構造をフラット化した状態で表現する。したがって、集団項目の値は指定できない。
ZERO(または、ZEROS, ZEROES)、SPACE(または、SPACES) が単独で出現した場合、COBOL言語における予約語と同様の扱いをおこなう。

## モード

本コマンドは次のモードを持つ。

- データ作成モード
- データダンプモード
- コピーブック作成モード

モードはサブコマンドとして指定する。

### データ作成モード(create)

recordを参照し、organizationで指定された編成で、dataの内容を持つデータファイルを作成する。

### データダンプモード(dump)

データファイルをorganizationで指定された編成でレコードを読み込み、recordで分析して、標準出力にデータ項目名、型名、値を表示する。

### コピーブック作成モード(create-copybook)

recordを参照し、レコードの内容を記述されたコピーブックを作成する。

出力先を指定しない場合は標準出力へ出力する。

レベル番号はレコード名(name)を01とし、record直下の項目を03から始め、集団項目に入るたびにレベルを+2する。

各項目は次の規則で出力する。

- 基本項目・FILLER は PICTURE 句を持つ。FILLER は `PIC X(DIGIT)` として出力する。
- USAGE が PACKED-DECIMAL / COMP-3 の項目は `PACKED-DECIMAL` を出力する。DISPLAY は句を省略する。
- VALUE 句は全ての基本項目・FILLER に出力する。VALUE が未指定のものは数値項目を `VALUE ZERO`、それ以外を `VALUE SPACE` とする。
- 集団項目は PICTURE 句・VALUE 句を持たない。
