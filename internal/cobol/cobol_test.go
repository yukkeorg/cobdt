package cobol

import (
	"bytes"
	"testing"
)

func TestParseField(t *testing.T) {
	tests := []struct {
		name string
		def  string
		want Field
	}{
		{"EMP-ID", "9(5)", Field{Type: TypeNumeric, IntDigits: 5, Usage: UsageDisplay}},
		{"SALARY", "S9(7)V99 COMP-3", Field{Type: TypeNumeric, Signed: true, IntDigits: 7, DecDigits: 2, Usage: UsagePacked}},
		{"RATE", "S9(1)V99", Field{Type: TypeNumeric, Signed: true, IntDigits: 1, DecDigits: 2, Usage: UsageDisplay}},
		{"NAME", "X(20)", Field{Type: TypeAlphanumeric, Length: 20, Usage: UsageDisplay}},
		{"KANJI", "N(8)", Field{Type: TypeJapanese, Length: 8, Usage: UsageDisplay}},
		{"DIGITS", "999", Field{Type: TypeNumeric, IntDigits: 3, Usage: UsageDisplay}},
		{"MIX", "9(3)V9", Field{Type: TypeNumeric, IntDigits: 3, DecDigits: 1, Usage: UsageDisplay}},
		{"", "FILLER(3)", Field{Type: TypeAlphanumeric, Length: 3, Usage: UsageDisplay, Filler: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := ParseField(tt.name, tt.def)
			if err != nil {
				t.Fatalf("ParseField(%q) error: %v", tt.def, err)
			}
			if f.Type != tt.want.Type || f.Signed != tt.want.Signed ||
				f.IntDigits != tt.want.IntDigits || f.DecDigits != tt.want.DecDigits ||
				f.Length != tt.want.Length || f.Usage != tt.want.Usage ||
				f.Filler != tt.want.Filler {
				t.Errorf("ParseField(%q) = %+v, want fields %+v", tt.def, f, tt.want)
			}
		})
	}
}

func TestParseFieldErrors(t *testing.T) {
	bad := []string{"", "Z(3)", "SX(4)", "SN(2)", "9(3) BOGUS", "X(4) COMP-3", "9(32)", "9(20)V9(12)"}
	for _, def := range bad {
		if _, err := ParseField("F", def); err == nil {
			t.Errorf("ParseField(%q) expected error, got nil", def)
		}
	}
}

func TestFieldSize(t *testing.T) {
	cases := []struct {
		def  string
		want int
	}{
		{"9(5)", 5},            // ゾーン10進数: 5 桁 = 5 バイト
		{"S9(7)V99 COMP-3", 5}, // パック: 9 桁 -> 9/2+1 = 5 バイト
		{"X(20)", 20},          // 英数字
		{"N(8)", 16},           // 日本語: 1 文字 2 バイト
	}
	for _, c := range cases {
		f, err := ParseField("F", c.def)
		if err != nil {
			t.Fatalf("ParseField(%q): %v", c.def, err)
		}
		if got := f.Size(); got != c.want {
			t.Errorf("Size(%q) = %d, want %d", c.def, got, c.want)
		}
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	cases := []struct {
		def      string
		value    string
		wantBack string
	}{
		{"9(5)", "123", "00123"},
		{"S9(7)V99 COMP-3", "1234567.89", "1234567.89"},
		{"S9(7)V99 COMP-3", "-42.5", "-0000042.50"},
		{"S9(1)V99", "-1.25", "-1.25"},
		{"S9(1)V99", "0.5", "0.50"},
		{"X(10)", "Alice", "Alice"},
		{"N(8)", "山田太郎", "山田太郎"},
	}
	for _, c := range cases {
		f, err := ParseField("F", c.def)
		if err != nil {
			t.Fatalf("ParseField(%q): %v", c.def, err)
		}
		enc, err := Encode(f, c.value)
		if err != nil {
			t.Fatalf("Encode(%q, %q): %v", c.def, c.value, err)
		}
		if got := f.Size(); got != len(enc) {
			t.Errorf("Encode(%q) length = %d, Size() = %d", c.def, len(enc), got)
		}
		back, err := Decode(f, enc)
		if err != nil {
			t.Fatalf("Decode(%q): %v", c.def, err)
		}
		if back != c.wantBack {
			t.Errorf("round trip %q value %q = %q, want %q", c.def, c.value, back, c.wantBack)
		}
	}
}

func TestFlattenItems(t *testing.T) {
	mk := func(name, def string) *Field {
		f, err := ParseField(name, def)
		if err != nil {
			t.Fatalf("ParseField(%q): %v", def, err)
		}
		return f
	}
	// ID, LINE OCCURS 2 { CODE, QTY OCCURS 2 }
	items := []*Item{
		{Leaf: mk("ID", "9(3)")},
		{Group: "LINE", Occurs: 2, Children: []*Item{
			{Leaf: mk("CODE", "X(2)")},
			{Leaf: mk("QTY", "9(2)"), Occurs: 2},
		}},
	}
	want := []string{"ID", "CODE(1)", "QTY(1, 1)", "QTY(1, 2)", "CODE(2)", "QTY(2, 1)", "QTY(2, 2)"}
	got := FlattenItems(items)
	if len(got) != len(want) {
		t.Fatalf("FlattenItems len = %d, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].Name != w {
			t.Errorf("field[%d].Name = %q, want %q", i, got[i].Name, w)
		}
	}
}

func TestRedefineLayout(t *testing.T) {
	mk := func(name, def string) *Field {
		f, err := ParseField(name, def)
		if err != nil {
			t.Fatalf("ParseField(%q): %v", def, err)
		}
		return f
	}
	// REC-TYPE 9(2), BODY-PERSON {X(10),9(3)}, BODY-COMPANY REDEFINES BODY-PERSON {9(7),X(6)}, TRAILER X(3)
	items := []*Item{
		{Leaf: mk("REC-TYPE", "9(2)")},
		{Group: "BODY-PERSON", Children: []*Item{
			{Leaf: mk("PERSON-NAME", "X(10)")},
			{Leaf: mk("PERSON-AGE", "9(3)")},
		}},
		{Group: "BODY-COMPANY", Redefine: "BODY-PERSON", Children: []*Item{
			{Leaf: mk("COMP-ID", "9(7)")},
			{Leaf: mk("COMP-CODE", "X(6)")},
		}},
		{Leaf: mk("TRAILER", "X(3)")},
	}

	if got, want := ItemSize(items[1]), ItemSize(items[2]); got != want {
		t.Errorf("ItemSize(BODY-PERSON)=%d, ItemSize(BODY-COMPANY)=%d, want equal", got, want)
	}

	fields := fieldsByName(FlattenItems(items))
	wantOffsets := map[string]int{
		"REC-TYPE": 0, "PERSON-NAME": 2, "PERSON-AGE": 12,
		"COMP-ID": 2, "COMP-CODE": 9, "TRAILER": 15,
	}
	for name, off := range wantOffsets {
		ff, ok := fields[name]
		if !ok {
			t.Fatalf("flat field %q not found", name)
		}
		if ff.Offset != off {
			t.Errorf("%s offset = %d, want %d", name, ff.Offset, off)
		}
	}

	if got := RecordLength(FlattenItems(items)); got != 18 {
		t.Errorf("RecordLength = %d, want 18 (REC-TYPE 2 + area 13 + TRAILER 3)", got)
	}
}

// fieldsByName は名前→FlatField の対応表を作るテスト補助。
func fieldsByName(fields []FlatField) map[string]FlatField {
	m := make(map[string]FlatField, len(fields))
	for _, ff := range fields {
		m[ff.Name] = ff
	}
	return m
}

func TestEncodeFigurative(t *testing.T) {
	fx, _ := ParseField("F", "X(3)")
	fn, _ := ParseField("F", "9(3)")
	fnat, _ := ParseField("F", "N(2)")
	fp, _ := ParseField("F", "S9(5) COMP-3") // パック10進数: 3 バイト

	cases := []struct {
		field *Field
		value string
		want  []byte
	}{
		{fx, "SPACE", []byte("   ")},
		{fx, "SPACES", []byte("   ")},
		{fx, "ZERO", []byte("000")},
		{fx, "zero", []byte("000")}, // ケースインセンシティブ
		{fn, "ZERO", []byte("000")},
		{fn, "ZEROS", []byte("000")},
		{fn, "ZEROES", []byte("000")},
		{fnat, "SPACE", []byte{0x81, 0x40, 0x81, 0x40}}, // 全角空白
		{fnat, "ZERO", []byte{0x82, 0x4f, 0x82, 0x4f}},  // 全角ゼロ
		// LOW-VALUE / HIGH-VALUE は型に関係なくバイト長ぶんを 0x00 / 0xFF で埋める。
		{fx, "LOW-VALUE", []byte{0x00, 0x00, 0x00}},
		{fx, "HIGH-VALUE", []byte{0xff, 0xff, 0xff}},
		{fx, "LOW-VALUES", []byte{0x00, 0x00, 0x00}},  // 複数形の別名
		{fx, "HIGH-VALUES", []byte{0xff, 0xff, 0xff}}, // 複数形の別名
		{fn, "LOW-VALUE", []byte{0x00, 0x00, 0x00}},
		{fnat, "HIGH-VALUE", []byte{0xff, 0xff, 0xff, 0xff}}, // N(2) = 4 バイト
		{fp, "LOW-VALUE", []byte{0x00, 0x00, 0x00}},          // パック10進数も 3 バイト
		{fp, "HIGH-VALUE", []byte{0xff, 0xff, 0xff}},
	}
	for _, c := range cases {
		got, err := Encode(c.field, c.value)
		if err != nil {
			t.Fatalf("Encode(%q): %v", c.value, err)
		}
		if !bytes.Equal(got, c.want) {
			t.Errorf("Encode(%s, %q) = % x, want % x", c.field.Raw, c.value, got, c.want)
		}
	}
}

func TestDecodeFigurative(t *testing.T) {
	fx, _ := ParseField("F", "X(3)")
	fn, _ := ParseField("F", "9(3)")
	fnat, _ := ParseField("F", "N(2)")

	// 全バイトが 0x00 / 0xFF なら型に関係なく LOW-VALUE / HIGH-VALUE と表示する。
	labelCases := []struct {
		field *Field
		bytes []byte
		want  string
	}{
		{fx, []byte{0x00, 0x00, 0x00}, "LOW-VALUE"},
		{fx, []byte{0xff, 0xff, 0xff}, "HIGH-VALUE"},
		{fn, []byte{0x00, 0x00, 0x00}, "LOW-VALUE"},
		{fnat, []byte{0xff, 0xff, 0xff, 0xff}, "HIGH-VALUE"}, // N の 0xFF は Shift-JIS 変換できないが、ラベルで回避
	}
	for _, c := range labelCases {
		got, err := Decode(c.field, c.bytes)
		if err != nil {
			t.Fatalf("Decode(% x): %v", c.bytes, err)
		}
		if got != c.want {
			t.Errorf("Decode(%s, % x) = %q, want %q", c.field.Raw, c.bytes, got, c.want)
		}
	}

	// 正規のゼロ（0x30）は LOW-VALUE と誤検出されない。
	if got, _ := Decode(fn, []byte("000")); got == "LOW-VALUE" {
		t.Errorf("正規のゼロが LOW-VALUE と誤判定されました: %q", got)
	}

	// encode → decode の往復で表意定数名が保たれる。
	for _, name := range []string{"LOW-VALUE", "HIGH-VALUE"} {
		b, err := Encode(fnat, name)
		if err != nil {
			t.Fatalf("Encode(%q): %v", name, err)
		}
		if got, _ := Decode(fnat, b); got != name {
			t.Errorf("往復 %q = %q", name, got)
		}
	}
}

func TestEncodeZonedOverpunch(t *testing.T) {
	f, _ := ParseField("F", "S9(3)")
	enc, _ := Encode(f, "-1")
	// "001" の最下位桁 '1' (0x31) が 0x70+1 = 0x71 にオーバーパンチされる
	want := []byte{'0', '0', 0x71}
	if !bytes.Equal(enc, want) {
		t.Errorf("Encode signed zoned = % x, want % x", enc, want)
	}
}

func TestEncodePackedSignNibble(t *testing.T) {
	// S9(3) COMP-3: 3 桁 -> 2 バイト。正の符号ニブルは 0x0C、負は 0x0D。
	f, _ := ParseField("F", "S9(3) COMP-3")
	pos, _ := Encode(f, "123")
	if pos[len(pos)-1]&0x0F != 0x0C {
		t.Errorf("positive sign nibble = %x, want C", pos[len(pos)-1]&0x0F)
	}
	neg, _ := Encode(f, "-123")
	if neg[len(neg)-1]&0x0F != 0x0D {
		t.Errorf("negative sign nibble = %x, want D", neg[len(neg)-1]&0x0F)
	}
	// 符号なしは 0x0F
	g, _ := ParseField("F", "9(3) COMP-3")
	uns, _ := Encode(g, "123")
	if uns[len(uns)-1]&0x0F != 0x0F {
		t.Errorf("unsigned sign nibble = %x, want F", uns[len(uns)-1]&0x0F)
	}
}
