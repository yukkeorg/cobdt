package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// loadBuild は YAML を一時ファイルへ書き出して Load し、BuildRecords の結果を返す。
func loadBuild(t *testing.T, yaml string) ([][]byte, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "c.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	spec, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return spec.BuildRecords()
}

func TestBuildRecordsRawCell(t *testing.T) {
	// {raw: "..."} は sanitize せず、そのままバイト列として数値 DISPLAY 項目へ書き込む。
	recs, err := loadBuild(t, `name: R
organization: sequential
record:
    - type: 9(5)
      name: N1
    - type: 9(3)
      name: N2
data:
    - [{raw: "ABCDE"}, 123]
`)
	if err != nil {
		t.Fatalf("BuildRecords: %v", err)
	}
	want := []byte("ABCDE123")
	if !bytes.Equal(recs[0], want) {
		t.Errorf("recs[0] = % x, want % x", recs[0], want)
	}
}

func TestBuildRecordsRawCellErrors(t *testing.T) {
	cases := map[string]string{
		"長さ不一致": `name: R
organization: sequential
record:
    - type: 9(5)
      name: N1
data:
    - [{raw: "AB"}]
`,
		"X 項目に raw": `name: R
organization: sequential
record:
    - type: X(5)
      name: A
data:
    - [{raw: "ABCDE"}]
`,
		"raw 以外のキー": `name: R
organization: sequential
record:
    - type: 9(5)
      name: N1
data:
    - [{raw: "ABCDE", foo: 1}]
`,
	}
	for name, yaml := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := loadBuild(t, yaml); err == nil {
				t.Errorf("%s はエラーになるべきです", name)
			}
		})
	}
}

// loadWithDataYAML は config YAML と別データ YAML を一時ファイルへ書き出し、Load 後に
// LoadDataYAML で data を差し替えて Spec を返す。
func loadWithDataYAML(t *testing.T, configYAML, dataYAML string) (*Spec, error) {
	t.Helper()
	dir := t.TempDir()
	cpath := filepath.Join(dir, "c.yaml")
	dpath := filepath.Join(dir, "d.yaml")
	if err := os.WriteFile(cpath, []byte(configYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dpath, []byte(dataYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	spec, err := Load(cpath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return spec, spec.LoadDataYAML(dpath)
}

const dataYAMLConfig = `name: R
organization: sequential
record:
    - type: X(3)
      name: A
    - type: 9(2)
      name: B
`

func TestLoadDataYAML(t *testing.T) {
	// 別ファイルの data 行が読み込まれ、レコードへエンコードされる。
	spec, err := loadWithDataYAML(t, dataYAMLConfig, `data:
    - [ABC, 12]
    - [XY, 7]
`)
	if err != nil {
		t.Fatalf("LoadDataYAML: %v", err)
	}
	recs, err := spec.BuildRecords()
	if err != nil {
		t.Fatalf("BuildRecords: %v", err)
	}
	want := [][]byte{[]byte("ABC12"), []byte("XY 07")}
	if len(recs) != len(want) {
		t.Fatalf("レコード数 = %d, want %d", len(recs), len(want))
	}
	for i := range want {
		if !bytes.Equal(recs[i], want[i]) {
			t.Errorf("recs[%d] = %q, want %q", i, recs[i], want[i])
		}
	}
}

func TestLoadDataYAMLIgnoresInline(t *testing.T) {
	// inline の data があっても --data-yaml の内容で上書きされる。
	configWithInline := dataYAMLConfig + `data:
    - [ZZZ, 99]
`
	spec, err := loadWithDataYAML(t, configWithInline, `data:
    - [ABC, 12]
`)
	if err != nil {
		t.Fatalf("LoadDataYAML: %v", err)
	}
	recs, err := spec.BuildRecords()
	if err != nil {
		t.Fatalf("BuildRecords: %v", err)
	}
	if len(recs) != 1 || !bytes.Equal(recs[0], []byte("ABC12")) {
		t.Errorf("recs = %q, want [ABC12]", recs)
	}
}

func TestLoadDataYAMLErrors(t *testing.T) {
	cases := map[string]string{
		"data 以外のキー": `data:
    - [ABC, 12]
organization: sequential
`,
		"data キー欠如": `rows:
    - [ABC, 12]
`,
		"data が 0 件": `data: []
`,
		"空ファイル": ``,
		"トップレベルがシーケンス": `- [ABC, 12]
`,
	}
	for name, dataYAML := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := loadWithDataYAML(t, dataYAMLConfig, dataYAML); err == nil {
				t.Errorf("%s はエラーになるべきです", name)
			}
		})
	}
}
