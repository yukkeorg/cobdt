// Package config は設定 YAML を読み込み、ファイル編成・項目定義・データ行を
// 取り出した Spec を構築する。
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	yaml "go.yaml.in/yaml/v4"

	"yukkeorg/internal/cobol"
	"yukkeorg/internal/datafile"
)

// rawConfig は設定 YAML 全体のマッピングを表す。
// name / organization / n-encode / record / data はいずれもトップレベルのキー。
type rawConfig struct {
	Name         string    `yaml:"name"`
	Organization string    `yaml:"organization"`
	NEncode      string    `yaml:"n-encode"`
	Record       yaml.Node `yaml:"record"` // 並び順と構造を保つため Node のまま受け取る
	Data         yaml.Node `yaml:"data"`   // 表項目は入れ子になりうるため Node のまま受け取る
}

// Spec は YAML から取り出した、create / dump / create-copybook に必要な情報一式。
type Spec struct {
	Name         string // レコード名（DATARECORD-NAME）
	Organization datafile.Organization
	NEncode      cobol.NEncoding   // N（日本語）項目の文字エンコード
	Items        []*cobol.Item     // record の木構造（集団項目・表・再定義を保持）
	Fields       []cobol.FlatField // Items をフラット化した葉項目（dump 用。再定義は重なって並ぶ）
	rows         []*yaml.Node      // データ行（各行は構造を保ったシーケンスノード）
}

// Load は設定 YAML を読み込み Spec を構築する。
func Load(path string) (*Spec, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg rawConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("YAML の解析に失敗しました: %w", err)
	}

	org, err := datafile.ParseOrganization(cfg.Organization)
	if err != nil {
		return nil, err
	}

	nenc, err := cobol.ParseNEncoding(cfg.NEncode)
	if err != nil {
		return nil, err
	}

	items, err := itemsFromRecord(&cfg.Record)
	if err != nil {
		return nil, err
	}
	if err := validateRedefines(items); err != nil {
		return nil, err
	}
	fields := cobol.FlattenItems(items)
	if len(fields) == 0 {
		return nil, fmt.Errorf("record に項目がありません")
	}

	// n-encode は record 全体に適用される設定。N 項目へ伝搬する。
	for _, ff := range fields {
		if ff.Field.Type == cobol.TypeJapanese {
			ff.Field.NEncode = nenc
		}
	}

	rows, err := parseDataRows(&cfg.Data)
	if err != nil {
		return nil, err
	}

	return &Spec{
		Name:         cfg.Name,
		Organization: org,
		NEncode:      nenc,
		Items:        items,
		Fields:       fields,
		rows:         rows,
	}, nil
}

// itemsFromRecord は record シーケンスを木構造（集団項目・表を保持）へ解析する。
func itemsFromRecord(node *yaml.Node) ([]*cobol.Item, error) {
	if node.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("record はシーケンスである必要があります")
	}
	return parseItemNodes(node)
}

// parseItemNodes はシーケンスノードの各要素を Item へ解析する。
func parseItemNodes(seq *yaml.Node) ([]*cobol.Item, error) {
	var items []*cobol.Item
	for _, n := range seq.Content {
		it, err := parseItem(n)
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, nil
}

// parseItem は record の 1 要素を解析する。
// type が GROUP なら集団項目、それ以外は基本項目／FILLER とみなす。
func parseItem(node *yaml.Node) (*cobol.Item, error) {
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("record の項目はマッピングである必要があります")
	}
	m := nodeMap(node)

	typeNode, ok := m["type"]
	if !ok {
		return nil, fmt.Errorf("record の項目には type が必要です")
	}
	typ := strings.TrimSpace(typeNode.Value)

	occurs, err := parseOccurs(m)
	if err != nil {
		return nil, err
	}

	var it *cobol.Item
	if strings.EqualFold(typ, "GROUP") {
		it, err = parseGroup(m, occurs)
		if err != nil {
			return nil, err
		}
	} else {
		f, err := parseLeaf(typ, m)
		if err != nil {
			return nil, err
		}
		if occurs > 0 && f.HasValue {
			return nil, fmt.Errorf("項目 %s: occurs と value は同時に指定できません", f.DisplayName())
		}
		it = &cobol.Item{Leaf: f, Occurs: occurs}
	}

	it.Redefine = strings.TrimSpace(valueOf(m, "redefine"))
	return it, nil
}

// itemName は項目の名前を返す。集団項目は集団名、基本項目は項目名、
// FILLER は名前を持たないため空文字列を返す。
func itemName(it *cobol.Item) string {
	if it.IsGroup() {
		return it.Group
	}
	if it.Leaf.Filler {
		return ""
	}
	return it.Leaf.Name
}

// validateRedefines は同じ階層に並ぶ項目群の再定義を検証し、集団項目の従属項目を
// 再帰的に検証する。再定義項目（redefine 指定あり）は、直前の再定義領域に属する
// 原定義または別の再定義項目を対象として参照し、原定義と同一バイト数でなければ
// ならない。FILLER への redefine、occurs との併用、occurs 項目の対象化は禁止する。
func validateRedefines(items []*cobol.Item) error {
	i := 0
	for i < len(items) {
		orig := items[i]
		if orig.Redefine != "" {
			return fmt.Errorf("再定義項目 %s: 対象 %s が同じ階層の直前に見つかりません", itemName(orig), orig.Redefine)
		}

		area := map[string]*cobol.Item{strings.ToUpper(itemName(orig)): orig}
		origSize := cobol.ItemSize(orig)

		j := i + 1
		for j < len(items) && items[j].Redefine != "" {
			rd := items[j]
			if itemName(rd) == "" {
				return fmt.Errorf("FILLER 項目には redefine を指定できません")
			}
			if rd.Occurs > 0 {
				return fmt.Errorf("再定義項目 %s に occurs は指定できません", itemName(rd))
			}
			tgt, ok := area[strings.ToUpper(strings.TrimSpace(rd.Redefine))]
			if !ok {
				return fmt.Errorf("再定義項目 %s: 対象 %s が同じ階層の直前に見つかりません", itemName(rd), rd.Redefine)
			}
			if tgt.Occurs > 0 {
				return fmt.Errorf("occurs を持つ項目 %s は再定義の対象にできません", itemName(tgt))
			}
			if sz := cobol.ItemSize(rd); sz != origSize {
				return fmt.Errorf("再定義項目 %s のサイズ %d バイトが原定義 %s の %d バイトと一致しません（FILLER で揃えてください）",
					itemName(rd), sz, itemName(orig), origSize)
			}
			area[strings.ToUpper(itemName(rd))] = rd
			j++
		}

		// 再定義領域の各メンバー（集団項目）の従属項目を検証する。
		for k := i; k < j; k++ {
			if items[k].IsGroup() {
				if err := validateRedefines(items[k].Children); err != nil {
					return err
				}
			}
		}
		i = j
	}
	return nil
}

// parseGroup は集団項目（type: GROUP）を解析する。name と subs は必須。
func parseGroup(m map[string]*yaml.Node, occurs int) (*cobol.Item, error) {
	name := strings.TrimSpace(valueOf(m, "name"))
	if name == "" {
		return nil, fmt.Errorf("集団項目(GROUP)には name が必要です")
	}
	if _, ok := m["usage"]; ok {
		return nil, fmt.Errorf("集団項目 %s に usage は指定できません", name)
	}
	if _, ok := m["value"]; ok {
		return nil, fmt.Errorf("集団項目 %s に value は指定できません", name)
	}
	subsNode, ok := m["subs"]
	if !ok || subsNode.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("集団項目 %s には subs（従属項目のリスト）が必要です", name)
	}
	children, err := parseItemNodes(subsNode)
	if err != nil {
		return nil, err
	}
	if len(children) == 0 {
		return nil, fmt.Errorf("集団項目 %s には従属項目が必要です", name)
	}
	return &cobol.Item{Group: name, Children: children, Occurs: occurs}, nil
}

// parseLeaf は基本項目／FILLER のマッピングから Field を構築する。
func parseLeaf(typ string, m map[string]*yaml.Node) (*cobol.Field, error) {
	name := strings.TrimSpace(valueOf(m, "name"))
	usage := strings.TrimSpace(valueOf(m, "usage"))

	isFiller := strings.HasPrefix(strings.ToUpper(typ), "FILLER")
	if isFiller {
		if name != "" {
			return nil, fmt.Errorf("FILLER 項目に name は指定できません")
		}
		if usage != "" {
			return nil, fmt.Errorf("FILLER 項目に usage は指定できません")
		}
	} else if name == "" {
		return nil, fmt.Errorf("基本項目 (type=%q) には name が必要です", typ)
	}

	// cobol.ParseField は PICTURE と USAGE をスペース区切りの 1 文字列で受け取る。
	def := typ
	if usage != "" {
		def = typ + " " + usage
	}
	f, err := cobol.ParseField(name, def)
	if err != nil {
		return nil, err
	}

	if valNode, ok := m["value"]; ok {
		f.Value = valNode.Value
		f.HasValue = true
	}
	return f, nil
}

// parseOccurs は occurs キーを解釈する。未指定なら 0、指定があれば 1 以上の整数。
func parseOccurs(m map[string]*yaml.Node) (int, error) {
	n, ok := m["occurs"]
	if !ok {
		return 0, nil
	}
	v, err := strconv.Atoi(strings.TrimSpace(n.Value))
	if err != nil || v < 1 {
		return 0, fmt.Errorf("occurs は 1 以上の整数で指定してください: %q", n.Value)
	}
	return v, nil
}

// parseDataRows は data シーケンスを解析する。各行は record の構造（集団項目・表・
// 再定義領域）に合わせて入れ子になりうるため、構造を保ったシーケンスノードのまま保持する。
func parseDataRows(node *yaml.Node) ([]*yaml.Node, error) {
	if node.Kind == 0 { // data 未指定
		return nil, nil
	}
	if node.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("data はシーケンスである必要があります")
	}
	var rows []*yaml.Node
	for _, rowNode := range node.Content {
		if rowNode.Kind != yaml.SequenceNode {
			return nil, fmt.Errorf("data の各行はシーケンスである必要があります")
		}
		rows = append(rows, rowNode)
	}
	return rows, nil
}

// BuildRecords は data 行を record 定義に従ってエンコードし、レコードごとのバイト列を返す。
// 各レコード内では再定義領域につき 1 つの解釈だけが書かれるため、能動的な項目は重ならず
// 順に連結すればよい。
func (s *Spec) BuildRecords() ([][]byte, error) {
	out := make([][]byte, 0, len(s.rows))
	for idx, row := range s.rows {
		r := &rowReader{content: row.Content}
		b, err := encodeItems(s.Items, r)
		if err != nil {
			return nil, fmt.Errorf("data[%d]: %w", idx, err)
		}
		if !r.done() {
			return nil, fmt.Errorf("data[%d]: 値の数が項目数より多いです", idx)
		}
		out = append(out, b)
	}
	return out, nil
}

// rowReader は data 行のシーケンス要素を先頭から順に取り出すカーソル。
type rowReader struct {
	content []*yaml.Node
	pos     int
}

func (r *rowReader) next() (*yaml.Node, error) {
	if r.pos >= len(r.content) {
		return nil, fmt.Errorf("値の数が項目数より少ないです")
	}
	n := r.content[r.pos]
	r.pos++
	return n, nil
}

func (r *rowReader) done() bool { return r.pos >= len(r.content) }

// encodeItems は同じ階層の項目群を r から値を読みながらエンコードする。
// 再定義領域は 1 スロットとして 1 要素だけ消費し、選ばれた解釈をエンコードする。
func encodeItems(items []*cobol.Item, r *rowReader) ([]byte, error) {
	var out []byte
	for i := 0; i < len(items); {
		j := i + 1
		for j < len(items) && items[j].Redefine != "" {
			j++
		}
		if j-i > 1 {
			node, err := r.next()
			if err != nil {
				return nil, err
			}
			b, err := encodeArea(items[i:j], node)
			if err != nil {
				return nil, err
			}
			out = append(out, b...)
		} else {
			b, err := encodeItem(items[i], r)
			if err != nil {
				return nil, err
			}
			out = append(out, b...)
		}
		i = j
	}
	return out, nil
}

// encodeItem は再定義に関与しない 1 項目をエンコードする。集団項目（非 OCCURS）は
// 透過的に同じカーソルから従属項目を読み、OCCURS 項目は 1 つのネストリストを消費する。
func encodeItem(it *cobol.Item, r *rowReader) ([]byte, error) {
	if it.Occurs > 0 {
		node, err := r.next()
		if err != nil {
			return nil, fmt.Errorf("項目 %s: %w", displayName(it), err)
		}
		if node.Kind != yaml.SequenceNode {
			return nil, fmt.Errorf("項目 %s: 表項目の値はリストで指定してください", displayName(it))
		}
		if len(node.Content) != it.Occurs {
			return nil, fmt.Errorf("項目 %s: 表の要素数 %d が occurs %d と一致しません", displayName(it), len(node.Content), it.Occurs)
		}
		var out []byte
		for _, elem := range node.Content {
			b, err := encodeOnce(it, elem)
			if err != nil {
				return nil, err
			}
			out = append(out, b...)
		}
		return out, nil
	}

	if it.IsGroup() {
		// 透過的な集団項目: 従属項目を同じカーソルから続けて読む。
		return encodeItems(it.Children, r)
	}

	node, err := r.next()
	if err != nil {
		return nil, fmt.Errorf("項目 %s: %w", displayName(it), err)
	}
	return encodeLeaf(it.Leaf, node)
}

// encodeArea は再定義領域を 1 要素 node からエンコードする。node が target/value の
// マップなら対象の再定義項目を、そうでなければ原定義を選ぶ。
func encodeArea(area []*cobol.Item, node *yaml.Node) ([]byte, error) {
	member := area[0] // 既定は原定義
	valueNode := node

	if node.Kind == yaml.MappingNode {
		m := nodeMap(node)
		tn, ok := m["target"]
		vn, ok2 := m["value"]
		if !ok || !ok2 {
			return nil, fmt.Errorf("再定義の指定には target と value が必要です")
		}
		target := strings.TrimSpace(tn.Value)
		member = nil
		for _, it := range area {
			if strings.EqualFold(itemName(it), target) {
				member = it
				break
			}
		}
		if member == nil {
			return nil, fmt.Errorf("target %q は再定義領域に見つかりません", target)
		}
		valueNode = vn
	}

	return encodeOnce(member, valueNode)
}

// encodeOnce は OCCURS 1 要素ぶん（または再定義領域 1 スロットぶん）の項目 it を、
// ラップされた値ノード node からエンコードする。集団項目なら node はその従属項目を
// 並べたシーケンス、基本項目なら node はスカラ。
func encodeOnce(it *cobol.Item, node *yaml.Node) ([]byte, error) {
	if it.IsGroup() {
		if node.Kind != yaml.SequenceNode {
			return nil, fmt.Errorf("項目 %s: 集団項目の値はリストで指定してください", displayName(it))
		}
		sub := &rowReader{content: node.Content}
		b, err := encodeItems(it.Children, sub)
		if err != nil {
			return nil, err
		}
		if !sub.done() {
			return nil, fmt.Errorf("項目 %s: 値の数が従属項目より多いです", displayName(it))
		}
		return b, nil
	}
	return encodeLeaf(it.Leaf, node)
}

// encodeLeaf は基本項目／FILLER をスカラ値ノードからエンコードする。
func encodeLeaf(f *cobol.Field, node *yaml.Node) ([]byte, error) {
	if node.Kind != yaml.ScalarNode {
		return nil, fmt.Errorf("項目 %s: 値はスカラで指定してください", f.DisplayName())
	}
	return cobol.Encode(f, node.Value)
}

// displayName はエラーメッセージ用の項目名を返す。
func displayName(it *cobol.Item) string {
	if it.IsGroup() {
		return it.Group
	}
	return it.Leaf.DisplayName()
}

// nodeMap はマッピングノードを key->value ノードの対応表に変換する。
func nodeMap(node *yaml.Node) map[string]*yaml.Node {
	m := make(map[string]*yaml.Node, len(node.Content)/2)
	for i := 0; i+1 < len(node.Content); i += 2 {
		m[node.Content[i].Value] = node.Content[i+1]
	}
	return m
}

// valueOf は対応表からスカラ値を取り出す。無ければ空文字列。
func valueOf(m map[string]*yaml.Node, key string) string {
	if n, ok := m[key]; ok {
		return n.Value
	}
	return ""
}
