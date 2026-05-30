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
	Name         string          // レコード名（DATARECORD-NAME）
	Organization datafile.Organization
	NEncode      cobol.NEncoding  // N（日本語）項目の文字エンコード
	Items        []*cobol.Item    // record の木構造（集団項目・表を保持）
	Fields       []cobol.FlatField // Items をフラット化した葉項目（create / dump 用）
	Rows         [][]string        // データ行（各行はフラット化した項目値のリスト）
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
		Rows:         rows,
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

	if strings.EqualFold(typ, "GROUP") {
		return parseGroup(m, occurs)
	}

	f, err := parseLeaf(typ, m)
	if err != nil {
		return nil, err
	}
	if occurs > 0 && f.HasValue {
		return nil, fmt.Errorf("項目 %s: occurs と value は同時に指定できません", f.DisplayName())
	}
	return &cobol.Item{Leaf: f, Occurs: occurs}, nil
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

// parseDataRows は data シーケンスを解析する。各行は record の構造に合わせて
// 入れ子になりうるため、再帰的にフラット化したスカラ値の並びへ変換する。
func parseDataRows(node *yaml.Node) ([][]string, error) {
	if node.Kind == 0 { // data 未指定
		return nil, nil
	}
	if node.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("data はシーケンスである必要があります")
	}
	var rows [][]string
	for _, rowNode := range node.Content {
		if rowNode.Kind != yaml.SequenceNode {
			return nil, fmt.Errorf("data の各行はシーケンスである必要があります")
		}
		var vals []string
		flattenValueNode(rowNode, &vals)
		rows = append(rows, vals)
	}
	return rows, nil
}

// flattenValueNode はデータ行の入れ子シーケンスを平坦なスカラ値の並びへ展開する。
func flattenValueNode(node *yaml.Node, out *[]string) {
	switch node.Kind {
	case yaml.SequenceNode:
		for _, c := range node.Content {
			flattenValueNode(c, out)
		}
	case yaml.ScalarNode:
		*out = append(*out, node.Value)
	}
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
