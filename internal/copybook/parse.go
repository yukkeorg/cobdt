package copybook

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	yaml "go.yaml.in/yaml/v4"

	"yukkeorg/internal/cobol"
)

// Parse は COBOL コピーブックのテキストを解析し、レコード名（01 レベル名）と
// record の木構造を返す。cdm のモデルで表現できない構文に遭遇した場合は
// best-effort 変換やスキップをせず、行と項目を示してエラーを返す
// （docs/adr/0002-import-copybook-strict-subset.md 参照）。
func Parse(src []byte) (name string, items []*cobol.Item, err error) {
	stmts, err := scanStatements(string(src))
	if err != nil {
		return "", nil, err
	}

	var entries []*entry
	for _, st := range stmts {
		e, err := parseEntry(st)
		if err != nil {
			return "", nil, err
		}
		if e != nil { // nil は 88 レベルなど読み飛ばす項目
			entries = append(entries, e)
		}
	}
	if len(entries) == 0 {
		return "", nil, fmt.Errorf("コピーブックに項目がありません")
	}

	first := entries[0]
	if first.level != 1 {
		return "", nil, fmt.Errorf("%d 行目: コピーブックは 01 レベルで始まる必要があります", first.line)
	}
	if first.pic != "" {
		return "", nil, fmt.Errorf("%d 行目: 01 レベルが基本項目です。従属項目を持つ集団項目を指定してください", first.line)
	}
	if first.name == "" {
		return "", nil, fmt.Errorf("%d 行目: 01 レベルに名前がありません", first.line)
	}

	pos := 1
	items, err = buildSiblings(entries, &pos)
	if err != nil {
		return "", nil, err
	}
	if len(items) == 0 {
		return "", nil, fmt.Errorf("%d 行目: 01 レベル %s に従属項目がありません", first.line, first.name)
	}
	if pos < len(entries) {
		rest := entries[pos]
		if rest.level == 1 {
			return "", nil, fmt.Errorf("%d 行目: 01 レベルが複数あります（cdm は単一レコードのみ対応します）", rest.line)
		}
		return "", nil, fmt.Errorf("%d 行目: レベル番号の階層が不正です", rest.line)
	}

	return first.name, items, nil
}

// entry は解析済みの 1 つのデータ記述項目（レベル番号と各句）。
type entry struct {
	level       int
	name        string // FILLER および名前のない項目は空文字列
	isFiller    bool
	pic         string // 空なら集団項目
	usagePacked bool
	occurs      int
	redefine    string
	value       string
	hasValue    bool
	line        int
}

// buildSiblings は entries[*pos] から始まる同一レベルの項目群を木構造に組み立てる。
// 集団項目（PICTURE を持たない項目）の従属項目は、続くより大きいレベル番号の項目群を
// 再帰的に取り込む。
func buildSiblings(entries []*entry, pos *int) ([]*cobol.Item, error) {
	if *pos >= len(entries) {
		return nil, nil
	}
	level := entries[*pos].level
	var items []*cobol.Item
	for *pos < len(entries) && entries[*pos].level == level {
		e := entries[*pos]
		*pos++
		hasChildren := *pos < len(entries) && entries[*pos].level > level

		if e.pic == "" {
			// 集団項目
			if e.name == "" {
				return nil, fmt.Errorf("%d 行目: 名前のない集団項目（FILLER グループ）は未対応です", e.line)
			}
			if e.usagePacked {
				return nil, fmt.Errorf("%d 行目: 集団項目 %s への USAGE は未対応です", e.line, e.name)
			}
			if !hasChildren {
				return nil, fmt.Errorf("%d 行目: 集団項目 %s に従属項目がありません", e.line, e.name)
			}
			children, err := buildSiblings(entries, pos)
			if err != nil {
				return nil, err
			}
			items = append(items, &cobol.Item{
				Group:    e.name,
				Children: children,
				Occurs:   e.occurs,
				Redefine: e.redefine,
			})
			continue
		}

		// 基本項目／FILLER
		if hasChildren {
			label := e.name
			if label == "" {
				label = "FILLER"
			}
			return nil, fmt.Errorf("%d 行目: 基本項目 %s に従属項目があります", e.line, label)
		}
		leaf, err := buildLeaf(e)
		if err != nil {
			return nil, err
		}
		items = append(items, &cobol.Item{Leaf: leaf, Occurs: e.occurs, Redefine: e.redefine})
	}
	return items, nil
}

// buildLeaf は entry から基本項目／FILLER の Field を構築する。FILLER は cdm の
// モデルに合わせて、同一バイト数の X 項目（FILLER(n)）として表現する。
func buildLeaf(e *entry) (*cobol.Field, error) {
	if err := validatePic(e.pic, e.line); err != nil {
		return nil, err
	}
	def := e.pic
	if e.usagePacked {
		def += " PACKED-DECIMAL"
	}
	f, err := cobol.ParseField(e.name, def)
	if err != nil {
		return nil, fmt.Errorf("%d 行目: %w", e.line, err)
	}
	if e.isFiller {
		// FILLER は型 X 固定。元の PICTURE のバイト数を保ったまま FILLER(n) にする。
		f = &cobol.Field{
			Filler: true,
			Type:   cobol.TypeAlphanumeric,
			Length: f.Size(),
			Usage:  cobol.UsageDisplay,
		}
	}
	if e.hasValue {
		f.Value = e.value
		f.HasValue = true
	}
	return f, nil
}

// validatePic は cdm が表現できる PICTURE 文字集合 {9 X N S V ( ) 数字} のみで
// 構成されているかを検証する。Z・,・.・$・A などの編集用記号を弾く。
func validatePic(pic string, line int) error {
	for _, r := range strings.ToUpper(pic) {
		switch {
		case r >= '0' && r <= '9':
		case r == 'X' || r == 'N' || r == 'S' || r == 'V' || r == '(' || r == ')':
		default:
			return fmt.Errorf("%d 行目: 未対応の PICTURE です（編集用記号や英字型など）: %q", line, pic)
		}
	}
	return nil
}

// parseEntry は 1 つの文（ステートメント）を entry に解析する。
// 88 レベルなど読み飛ばす項目は (nil, nil) を返す。
func parseEntry(st stmt) (*entry, error) {
	toks := tokenize(st.text)
	if len(toks) == 0 {
		return nil, nil
	}

	level, err := strconv.Atoi(toks[0])
	if err != nil {
		return nil, fmt.Errorf("%d 行目: レベル番号として解釈できません: %q", st.line, toks[0])
	}
	switch {
	case level == 88:
		return nil, nil // 条件名はレイアウトを持たないため読み飛ばす
	case level == 66:
		return nil, fmt.Errorf("%d 行目: 66 レベル（RENAMES）は未対応です", st.line)
	case level == 77:
		return nil, fmt.Errorf("%d 行目: 77 レベル（独立項目）は未対応です", st.line)
	case level < 1 || level > 49:
		return nil, fmt.Errorf("%d 行目: 不正なレベル番号です: %d", st.line, level)
	}

	e := &entry{level: level, line: st.line}

	idx := 1
	if idx >= len(toks) {
		return nil, fmt.Errorf("%d 行目: 項目に名前がありません", st.line)
	}
	switch {
	case strings.EqualFold(toks[idx], "FILLER"):
		e.isFiller = true
		idx++
	case isClauseKeyword(strings.ToUpper(toks[idx])):
		// 名前が省略された無名項目は FILLER として扱う。
		e.isFiller = true
	default:
		e.name = toks[idx]
		idx++
	}

	for i := idx; i < len(toks); i++ {
		w := strings.ToUpper(toks[i])
		switch {
		case w == "REDEFINES":
			i++
			if i >= len(toks) {
				return nil, fmt.Errorf("%d 行目: REDEFINES の対象がありません", st.line)
			}
			e.redefine = toks[i]

		case w == "PIC" || w == "PICTURE":
			i++
			if i < len(toks) && strings.EqualFold(toks[i], "IS") {
				i++
			}
			if i >= len(toks) {
				return nil, fmt.Errorf("%d 行目: PICTURE 句に内容がありません", st.line)
			}
			e.pic = toks[i]

		case w == "OCCURS":
			i++
			if i >= len(toks) {
				return nil, fmt.Errorf("%d 行目: OCCURS の要素数がありません", st.line)
			}
			n, err := strconv.Atoi(toks[i])
			if err != nil || n < 1 {
				return nil, fmt.Errorf("%d 行目: OCCURS の要素数が不正です: %q", st.line, toks[i])
			}
			e.occurs = n
			if i+1 < len(toks) && strings.EqualFold(toks[i+1], "TIMES") {
				i++
			}

		case w == "TO" || w == "DEPENDING":
			return nil, fmt.Errorf("%d 行目: OCCURS ... DEPENDING ON（可変長テーブル）は未対応です", st.line)

		case w == "INDEXED" || w == "ASCENDING" || w == "DESCENDING":
			// INDEXED BY / KEY 句の被演算子（識別子）を次の句まで読み飛ばす。
			for i+1 < len(toks) && !isClauseKeyword(strings.ToUpper(toks[i+1])) {
				i++
			}

		case w == "VALUE" || w == "VALUES":
			i++
			if i < len(toks) && strings.EqualFold(toks[i], "IS") {
				i++
			}
			if i >= len(toks) {
				return nil, fmt.Errorf("%d 行目: VALUE 句に値がありません", st.line)
			}
			if strings.EqualFold(toks[i], "ALL") {
				return nil, fmt.Errorf("%d 行目: VALUE ALL は未対応です", st.line)
			}
			val, quoted := unquote(toks[i])
			if !quoted {
				switch strings.ToUpper(val) {
				case "ZERO", "ZEROS", "ZEROES", "SPACE", "SPACES":
					// cdm が対応する表意定数
				case "HIGH-VALUE", "HIGH-VALUES", "LOW-VALUE", "LOW-VALUES",
					"QUOTE", "QUOTES", "NULL", "NULLS":
					return nil, fmt.Errorf("%d 行目: 未対応の表意定数です: %s", st.line, val)
				}
			}
			e.value = val
			e.hasValue = true

		case w == "USAGE":
			i++
			if i < len(toks) && strings.EqualFold(toks[i], "IS") {
				i++
			}
			if i >= len(toks) {
				return nil, fmt.Errorf("%d 行目: USAGE 句に内容がありません", st.line)
			}
			if err := e.applyUsage(toks[i], st.line); err != nil {
				return nil, err
			}

		case isUsageWord(w):
			if err := e.applyUsage(toks[i], st.line); err != nil {
				return nil, err
			}

		case w == "IS":
			// 句の途中に紛れる IS は読み飛ばす。

		case w == "SYNC" || w == "SYNCHRONIZED":
			return nil, fmt.Errorf("%d 行目: SYNC（SYNCHRONIZED）はレコード長を変えるため未対応です", st.line)
		case w == "JUSTIFIED" || w == "JUST":
			return nil, fmt.Errorf("%d 行目: JUSTIFIED は未対応です", st.line)
		case w == "BLANK":
			return nil, fmt.Errorf("%d 行目: BLANK WHEN ZERO は未対応です", st.line)
		case w == "SIGN":
			return nil, fmt.Errorf("%d 行目: SIGN 句は未対応です", st.line)

		default:
			return nil, fmt.Errorf("%d 行目: 未対応の句です: %q", st.line, toks[i])
		}
	}

	return e, nil
}

// applyUsage は USAGE 指定の語を解釈する。COMP-3 系はパック10進数、DISPLAY 系は
// ゾーン10進数、それ以外（バイナリなど）は未対応としてエラーを返す。
func (e *entry) applyUsage(word string, line int) error {
	switch strings.ToUpper(word) {
	case "DISPLAY", "DISPLAY-1":
		return nil
	case "COMP-3", "COMPUTATIONAL-3", "PACKED-DECIMAL":
		e.usagePacked = true
		return nil
	default:
		return fmt.Errorf("%d 行目: USAGE %s は未対応です", line, word)
	}
}

// isUsageWord は word が USAGE 指定の語かどうかを返す。
func isUsageWord(word string) bool {
	switch strings.ToUpper(word) {
	case "DISPLAY", "DISPLAY-1",
		"COMP-3", "COMPUTATIONAL-3", "PACKED-DECIMAL",
		"COMP", "COMPUTATIONAL", "COMP-1", "COMPUTATIONAL-1",
		"COMP-2", "COMPUTATIONAL-2", "COMP-4", "COMPUTATIONAL-4",
		"COMP-5", "COMPUTATIONAL-5", "COMP-X", "BINARY", "INDEX", "POINTER":
		return true
	}
	return false
}

// isClauseKeyword はデータ記述句のキーワードかどうかを返す。無名項目（名前省略）の
// 判定と、INDEXED BY / KEY 句の被演算子読み飛ばしの境界に使う。
func isClauseKeyword(word string) bool {
	switch word {
	case "REDEFINES", "PIC", "PICTURE", "OCCURS", "VALUE", "VALUES",
		"USAGE", "SYNC", "SYNCHRONIZED", "JUSTIFIED", "JUST", "SIGN",
		"BLANK", "DEPENDING", "TO", "INDEXED", "ASCENDING", "DESCENDING":
		return true
	}
	return isUsageWord(word)
}

// unquote は引用符付きリテラル（'...' または "..."）なら引用符を外した中身と true を返す。
func unquote(tok string) (string, bool) {
	if len(tok) >= 2 {
		q := tok[0]
		if (q == '\'' || q == '"') && tok[len(tok)-1] == q {
			return tok[1 : len(tok)-1], true
		}
	}
	return tok, false
}

// ---- 字句解析（コピーブックのテキスト → 文 → トークン）----

// stmt は 1 つの文（ピリオドで区切られたデータ記述項目）とその開始行番号。
type stmt struct {
	text string
	line int
}

// scanStatements は固定形式・自由形式どちらのコピーブックも受け、コメント行と
// シーケンス番号領域を取り除いたうえで、終端ピリオド（後ろに空白か行末が続くピリオド）で
// 文に分割する。引用符内のピリオドは終端と見なさない。
func scanStatements(src string) ([]stmt, error) {
	var out []stmt
	var buf strings.Builder
	startLine := 0
	var quote byte

	flush := func() {
		if strings.TrimSpace(buf.String()) != "" {
			out = append(out, stmt{text: strings.TrimSpace(buf.String()), line: startLine})
		}
		buf.Reset()
	}

	lines := strings.Split(src, "\n")
	for n, raw := range lines {
		code, isComment := stripLine(raw)
		if isComment || strings.TrimSpace(code) == "" {
			continue
		}
		if buf.Len() > 0 {
			buf.WriteByte(' ') // 行をまたぐ文は空白でつなぐ
		} else {
			startLine = n + 1
		}
		for i := 0; i < len(code); i++ {
			c := code[i]
			if quote != 0 {
				buf.WriteByte(c)
				if c == quote {
					quote = 0
				}
				continue
			}
			switch c {
			case '\'', '"':
				quote = c
				buf.WriteByte(c)
			case '.':
				if i+1 >= len(code) || code[i+1] == ' ' {
					flush()
					startLine = n + 1
				} else {
					buf.WriteByte(c)
				}
			default:
				buf.WriteByte(c)
			}
		}
	}
	flush()
	return out, nil
}

// stripLine は 1 物理行からコメント判定と固定形式のシーケンス番号領域・標識領域の
// 除去を行い、解析対象のコード部分を返す。
func stripLine(raw string) (code string, isComment bool) {
	line := expandTabs(strings.TrimRight(raw, "\r"))

	trimmed := strings.TrimLeft(line, " ")
	if strings.HasPrefix(trimmed, "*") || strings.HasPrefix(trimmed, "/") {
		return "", true
	}
	if len(line) > 6 && (line[6] == '*' || line[6] == '/') {
		return "", true
	}

	// 固定形式: 1–6 桁が空白または数字（シーケンス番号）で、7 桁目が空白か継続標識(-)なら
	// 1–7 桁目（シーケンス番号領域＋標識領域）を取り除く。自由形式（コードが 1 桁目から
	// 始まる）はそのまま使う。なお 73–80 桁目（識別領域）は除去しない。
	if len(line) >= 7 {
		area := line[:6]
		if isBlank(area) || isAllDigit(area) {
			if ind := line[6]; ind == ' ' || ind == '-' {
				return line[7:], false
			}
		}
	}
	return line, false
}

// expandTabs はタブを 8 桁ごとのタブ位置に展開する（桁位置に依存する処理のため）。
func expandTabs(s string) string {
	if !strings.ContainsRune(s, '\t') {
		return s
	}
	var b strings.Builder
	col := 0
	for _, r := range s {
		if r == '\t' {
			n := 8 - col%8
			for range n {
				b.WriteByte(' ')
			}
			col += n
		} else {
			b.WriteRune(r)
			col++
		}
	}
	return b.String()
}

func isBlank(s string) bool {
	return strings.TrimSpace(s) == ""
}

func isAllDigit(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// tokenize は 1 文を空白区切りでトークン化する。引用符付きリテラルは引用符を含めて
// 1 トークンにまとめる。
func tokenize(s string) []string {
	var toks []string
	var cur strings.Builder
	var quote byte

	flush := func() {
		if cur.Len() > 0 {
			toks = append(toks, cur.String())
			cur.Reset()
		}
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			cur.WriteByte(c)
			if c == quote {
				quote = 0
				flush()
			}
			continue
		}
		switch c {
		case '\'', '"':
			flush()
			quote = c
			cur.WriteByte(c)
		case ' ', '\t':
			flush()
		default:
			cur.WriteByte(c)
		}
	}
	flush()
	return toks
}

// ---- YAML 生成 ----

// RenderYAML は record の木構造を cdm の設定 YAML に変換する。コピーブックが持たない
// organization / n-encode は既定値を明示出力し、data はコメント付きの空リストを出力する。
func RenderYAML(name string, items []*cobol.Item) ([]byte, error) {
	if strings.TrimSpace(name) == "" {
		name = "DATA-RECORD"
	}

	root := &yaml.Node{Kind: yaml.MappingNode}

	nameKey := scalar("name")
	nameKey.HeadComment = "cdm import-copybook が生成しました。organization・n-encode・data は必要に応じて編集してください。"
	addPair(root, nameKey, scalar(name))
	addPair(root, scalar("organization"), scalar("sequential"))
	addPair(root, scalar("n-encode"), scalar("sjis"))

	recSeq := &yaml.Node{Kind: yaml.SequenceNode}
	for _, it := range items {
		recSeq.Content = append(recSeq.Content, itemNode(it))
	}
	addPair(root, scalar("record"), recSeq)

	dataKey := scalar("data")
	dataKey.HeadComment = "ここに create モード用のデータ行を記述してください（1 行 = 1 レコード）。"
	addPair(root, dataKey, &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle})

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(4)
	if err := enc.Encode(root); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// itemNode は 1 項目を YAML マッピングノードに変換する。キー順は
// type, name, usage, value, occurs, redefine, subs に揃える。
func itemNode(it *cobol.Item) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode}

	if it.IsGroup() {
		addPair(m, scalar("type"), scalar("GROUP"))
		addPair(m, scalar("name"), scalar(it.Group))
		if it.Redefine != "" {
			addPair(m, scalar("redefine"), scalar(it.Redefine))
		}
		if it.Occurs > 0 {
			addPair(m, scalar("occurs"), scalar(strconv.Itoa(it.Occurs)))
		}
		subs := &yaml.Node{Kind: yaml.SequenceNode}
		for _, c := range it.Children {
			subs.Content = append(subs.Content, itemNode(c))
		}
		addPair(m, scalar("subs"), subs)
		return m
	}

	f := it.Leaf
	if f.Filler {
		addPair(m, scalar("type"), scalar(fmt.Sprintf("FILLER(%d)", f.Length)))
	} else {
		addPair(m, scalar("type"), scalar(f.Picture()))
		addPair(m, scalar("name"), scalar(f.Name))
	}
	if f.Usage == cobol.UsagePacked {
		addPair(m, scalar("usage"), scalar("PACKED-DECIMAL"))
	}
	// occurs と value は同時指定できないため、表項目には value を出力しない。
	if it.Occurs == 0 {
		if vn, ok := valueNode(f); ok {
			addPair(m, scalar("value"), vn)
		}
	}
	if it.Occurs > 0 {
		addPair(m, scalar("occurs"), scalar(strconv.Itoa(it.Occurs)))
	}
	if it.Redefine != "" {
		addPair(m, scalar("redefine"), scalar(it.Redefine))
	}
	return m
}

// valueNode は value キーに出力すべきノードを返す。cdm の既定値（数値=ZERO/他=SPACE）と
// 一致する場合や未指定の場合は (nil, false) を返して省略する。
func valueNode(f *cobol.Field) (*yaml.Node, bool) {
	if !f.HasValue {
		return nil, false
	}
	if fig, ok := cobol.FigurativeConstant(f.Value); ok {
		// 型ごとの既定値と一致するなら省略する。
		if (fig == "ZERO" && f.Type == cobol.TypeNumeric) ||
			(fig == "SPACE" && f.Type != cobol.TypeNumeric) {
			return nil, false
		}
		return scalar(fig), true
	}
	if f.Type == cobol.TypeNumeric {
		return scalar(f.Value), true // 数値リテラルは引用符なし
	}
	n := scalar(f.Value)
	n.Style = yaml.DoubleQuotedStyle
	return n, true
}

func scalar(v string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Value: v}
}

func addPair(m *yaml.Node, key, val *yaml.Node) {
	m.Content = append(m.Content, key, val)
}
