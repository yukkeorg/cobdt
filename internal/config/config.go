// Package config は設定 YAML を読み込み、ファイル編成・項目定義・データ行を
// 取り出した Spec を構築する。
package config

import (
	"fmt"
	"os"
	"strings"

	yaml "go.yaml.in/yaml/v4"

	"yukkeorg/internal/cobol"
	"yukkeorg/internal/datafile"
)

// rawConfig は設定 YAML 全体のマッピングを表す。
// name / organization / n-encode / record / data はいずれもトップレベルのキー。
type rawConfig struct {
	Name         string     `yaml:"name"`
	Organization string     `yaml:"organization"`
	NEncode      string     `yaml:"n-encode"`
	Record       yaml.Node  `yaml:"record"` // 並び順と構造を保つため Node のまま受け取る
	Data         [][]string `yaml:"data"`   // 各レコードは値のシーケンス（ブロック形式 / フロー形式）
}

// Spec は YAML から取り出した、create / dump / create-copybook に必要な情報一式。
type Spec struct {
	Name         string          // レコード名（DATARECORD-NAME）
	Organization datafile.Organization
	NEncode      cobol.NEncoding // N（日本語）項目の文字エンコード
	Items        []*cobol.Item   // record の木構造（集団項目を保持）
	Fields       []*cobol.Field  // Items をフラット化した葉項目（create / dump 用）
	Rows         [][]string      // データ行（各行は項目値のリスト）
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
	for _, f := range fields {
		if f.Type == cobol.TypeJapanese {
			f.NEncode = nenc
		}
	}

	return &Spec{
		Name:         cfg.Name,
		Organization: org,
		NEncode:      nenc,
		Items:        items,
		Fields:       fields,
		Rows:         cfg.Data,
	}, nil
}

// itemsFromRecord は record シーケンスを木構造（集団項目を保持）へ解析する。
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

// parseItem は record の 1 要素を解析する。type キーを持てば基本項目／FILLER、
// 持たなければ「集団項目名: 子シーケンス」の集団項目とみなす。
func parseItem(node *yaml.Node) (*cobol.Item, error) {
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("record の項目はマッピングである必要があります")
	}
	m := nodeMap(node)

	if typeNode, ok := m["type"]; ok {
		f, err := parseLeaf(typeNode, m)
		if err != nil {
			return nil, err
		}
		return &cobol.Item{Leaf: f}, nil
	}

	// 集団項目: 単一キー（集団項目名）の値が子シーケンス。
	if len(node.Content) != 2 {
		return nil, fmt.Errorf("集団項目は「名前: 子項目のリスト」の形式である必要があります")
	}
	groupName := node.Content[0].Value
	children := node.Content[1]
	if children.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("集団項目 %s の子項目はシーケンスである必要があります", groupName)
	}
	childItems, err := parseItemNodes(children)
	if err != nil {
		return nil, err
	}
	return &cobol.Item{Group: groupName, Children: childItems}, nil
}

// parseLeaf は基本項目／FILLER のマッピングから Field を構築する。
func parseLeaf(typeNode *yaml.Node, m map[string]*yaml.Node) (*cobol.Field, error) {
	typ := strings.TrimSpace(typeNode.Value)
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
