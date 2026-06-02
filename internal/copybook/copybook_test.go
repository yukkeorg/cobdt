package copybook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yukkeorg/internal/cobol"
	"yukkeorg/internal/config"
)

func TestParseBasic(t *testing.T) {
	src := `       01  CUST-REC.
           03  CUST-ID        PIC 9(6).
           03  CUST-NAME      PIC X(30).
           03  CUST-BAL       PIC S9(7)V99 USAGE COMP-3 VALUE ZERO.
           03  RATES          PIC S9(3)V99 OCCURS 4 TIMES.
           03  FILLER         PIC X(2).
`
	name, items, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if name != "CUST-REC" {
		t.Errorf("name = %q, want CUST-REC", name)
	}
	if len(items) != 5 {
		t.Fatalf("len(items) = %d, want 5", len(items))
	}

	// CUST-ID: 数値 9(6)
	if f := items[0].Leaf; f == nil || f.Type != cobol.TypeNumeric || f.IntDigits != 6 {
		t.Errorf("items[0] = %+v, want 数値 9(6)", items[0])
	}
	// CUST-BAL: パック10進数 + 符号 + 小数
	if f := items[2].Leaf; f == nil || f.Usage != cobol.UsagePacked || !f.Signed || f.DecDigits != 2 {
		t.Errorf("items[2] = %+v, want S9(7)V99 PACKED-DECIMAL", items[2])
	}
	// CUST-BAL: VALUE ZERO は既定値なので HasValue でも YAML では省略される
	if !items[2].Leaf.HasValue {
		t.Errorf("items[2].HasValue = false, want true")
	}
	// RATES: OCCURS 4
	if items[3].Occurs != 4 {
		t.Errorf("items[3].Occurs = %d, want 4", items[3].Occurs)
	}
	// FILLER: 型 X、長さ 2
	if f := items[4].Leaf; f == nil || !f.Filler || f.Length != 2 {
		t.Errorf("items[4] = %+v, want FILLER(2)", items[4])
	}
}

func TestParseGroupNesting(t *testing.T) {
	src := `       01  REC.
           03  HEAD.
               05  A  PIC 9(2).
               05  B  PIC X(4).
           03  TAIL   PIC X(3).
`
	_, items, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if !items[0].IsGroup() || items[0].Group != "HEAD" {
		t.Fatalf("items[0] is not group HEAD: %+v", items[0])
	}
	if len(items[0].Children) != 2 {
		t.Errorf("HEAD children = %d, want 2", len(items[0].Children))
	}
	if items[1].IsGroup() {
		t.Errorf("items[1] should be leaf TAIL")
	}
}

func TestParseRedefines(t *testing.T) {
	src := `       01  REC.
           03  REC-TYPE     PIC 9(2).
           03  BODY-PERSON.
               05  PERSON-NAME  PIC X(10).
               05  PERSON-AGE   PIC 9(3).
           03  BODY-COMPANY REDEFINES BODY-PERSON.
               05  COMP-ID      PIC 9(7).
               05  COMP-CODE    PIC X(6).
`
	_, items, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("len(items) = %d, want 3", len(items))
	}
	if items[2].Redefine != "BODY-PERSON" {
		t.Errorf("items[2].Redefine = %q, want BODY-PERSON", items[2].Redefine)
	}
}

func TestParseSkipsCommentsAndConditionNames(t *testing.T) {
	src := `000100* これはコメント
000200 01  REC.
000300     03  S   PIC X.
000400         88  YES   VALUE 'Y'.
000500         88  NO    VALUE 'N'.
000600     03  T   PIC 9(3).
`
	_, items, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2 (88 レベルは読み飛ばす)", len(items))
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]string{
		"COMP バイナリ":  "       01  R.\n           03  A PIC S9(4) COMP.\n",
		"編集 PIC Z":   "       01  R.\n           03  A PIC ZZ9.\n",
		"編集 PIC 小数点": "       01  R.\n           03  A PIC 99.99.\n",
		"英字 A 型":     "       01  R.\n           03  A PIC A(5).\n",
		"ODO":        "       01  R.\n           03  N PIC 9.\n           03  T PIC X OCCURS 1 TO 9 DEPENDING ON N.\n",
		"複数 01":      "       01  R.\n           03  A PIC X.\n       01  S.\n           03  B PIC X.\n",
		"66 RENAMES": "       01  R.\n           03  A PIC X.\n           66  AA RENAMES A.\n",
		"77 独立項目":    "       77  W PIC 9(3).\n",
		"SYNC":       "       01  R.\n           03  A PIC S9(4) SYNC.\n",
		"01 が基本項目":   "       01  R PIC X(10).\n",
		"集団に従属項目なし":  "       01  R.\n           03  G.\n",
		"01 で始まらない":  "       03  A PIC X.\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if _, _, err := Parse([]byte(src)); err == nil {
				t.Errorf("Parse(%s) はエラーになるべきです", name)
			}
		})
	}
}

func TestRenderOmitsDefaultValue(t *testing.T) {
	src := `       01  REC.
           03  N   PIC 9(3) VALUE ZERO.
           03  X1  PIC X(4) VALUE SPACE.
           03  X2  PIC X(4) VALUE 'AB'.
           03  N2  PIC 9(3) VALUE 5.
`
	name, items, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := RenderYAML(name, items)
	if err != nil {
		t.Fatalf("RenderYAML: %v", err)
	}
	yamlText := string(out)

	// 既定値（数値 ZERO / 英数字 SPACE）は省略される。
	if strings.Contains(yamlText, "value: ZERO") || strings.Contains(yamlText, "value: SPACE") {
		t.Errorf("既定値 value が出力されています:\n%s", yamlText)
	}
	// 非既定値は出力される。
	if !strings.Contains(yamlText, `value: "AB"`) {
		t.Errorf("文字列 value が出力されていません:\n%s", yamlText)
	}
	if !strings.Contains(yamlText, "value: 5") {
		t.Errorf("数値 value が出力されていません:\n%s", yamlText)
	}
}

func TestParseFigurativeValue(t *testing.T) {
	src := `       01  REC.
           03  A   PIC X(4) VALUE HIGH-VALUE.
           03  B   PIC 9(3) VALUE LOW-VALUES.
`
	_, items, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if items[0].Leaf.Value != "HIGH-VALUE" {
		t.Errorf("items[0].Value = %q, want HIGH-VALUE", items[0].Leaf.Value)
	}
	// 複数形 LOW-VALUES もそのまま保持する（FigurativeConstant が正規化する）。
	if items[1].Leaf.Value != "LOW-VALUES" {
		t.Errorf("items[1].Value = %q, want LOW-VALUES", items[1].Leaf.Value)
	}

	// LOW-VALUE / HIGH-VALUE は既定値ではないため value として出力される。
	out, err := RenderYAML("REC", items)
	if err != nil {
		t.Fatalf("RenderYAML: %v", err)
	}
	if !strings.Contains(string(out), "value: HIGH-VALUE") {
		t.Errorf("value: HIGH-VALUE が出力されていません:\n%s", out)
	}
}

// TestRoundTripThroughConfig は生成 YAML が config.Load で読み込め、コピーブックの
// レイアウトを保つことを検証する（コピーブック → YAML → Spec の往復）。
func TestRoundTripThroughConfig(t *testing.T) {
	src := `       01  EMP-REC.
           03  EMP.
               05  EMP-ID    PIC 9(5).
               05  EMP-NAME  PIC X(20).
           03  SALARY        PIC S9(7)V99 USAGE COMP-3.
           03  RATES         PIC S9(3)V99 OCCURS 3 TIMES.
           03  FILLER        PIC X(3).
`
	name, items, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := RenderYAML(name, items)
	if err != nil {
		t.Fatalf("RenderYAML: %v", err)
	}

	path := filepath.Join(t.TempDir(), "out.yaml")
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
	spec, err := config.Load(path)
	if err != nil {
		t.Fatalf("生成 YAML が config.Load で読めません: %v\n%s", err, out)
	}

	// レコード長: EMP-ID(5)+EMP-NAME(20)+SALARY(packed 5桁/2+1=... 9桁→9/2+1=5)+RATES(5桁→5*3=15)+FILLER(3)
	// = 5+20+5+15+3 = 48
	if got := cobol.RecordLength(spec.Fields); got != 48 {
		t.Errorf("RecordLength = %d, want 48", got)
	}
}
