package datafile

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestParseOrganization(t *testing.T) {
	cases := map[string]Organization{
		"sequential":      OrgSequential,
		"seq":             OrgSequential,
		"line sequential": OrgLineSequential,
		"lineseq":         OrgLineSequential,
	}
	for s, want := range cases {
		got, err := ParseOrganization(s)
		if err != nil {
			t.Fatalf("ParseOrganization(%q): %v", s, err)
		}
		if got != want {
			t.Errorf("ParseOrganization(%q) = %v, want %v", s, got, want)
		}
	}
	if _, err := ParseOrganization("bogus"); err == nil {
		t.Error("ParseOrganization(bogus) expected error")
	}
}

func TestSequentialRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "seq.dat")
	recs := [][]byte{[]byte("ABC"), []byte("DEF")}

	if err := WriteRecords(path, OrgSequential, recs); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if !bytes.Equal(raw, []byte("ABCDEF")) {
		t.Errorf("sequential bytes = %q, want ABCDEF", raw)
	}

	got, err := ReadRecords(path, OrgSequential, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !bytes.Equal(got[0], recs[0]) || !bytes.Equal(got[1], recs[1]) {
		t.Errorf("ReadRecords sequential = %q, want %q", got, recs)
	}
}

func TestLineSequentialRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "line.dat")
	recs := [][]byte{[]byte("ABC"), []byte("DEFGH")}

	if err := WriteRecords(path, OrgLineSequential, recs); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if !bytes.Equal(raw, []byte("ABC\nDEFGH\n")) {
		t.Errorf("line sequential bytes = %q, want ABC\\nDEFGH\\n", raw)
	}

	got, err := ReadRecords(path, OrgLineSequential, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !bytes.Equal(got[0], recs[0]) || !bytes.Equal(got[1], recs[1]) {
		t.Errorf("ReadRecords line sequential = %q, want %q", got, recs)
	}
}

func TestSequentialIncompleteRecord(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.dat")
	os.WriteFile(path, []byte("ABCD"), 0o644) // 4 バイト, レコード長 3 では割り切れない

	if _, err := ReadRecords(path, OrgSequential, 3); err == nil {
		t.Error("expected error for incomplete trailing record")
	}
}
