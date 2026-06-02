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
