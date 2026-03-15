package main

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// buildRISCOSExtra constructs an Info-ZIP RISC OS extra field (signature 0x4341).
// load is the 32-bit load address; exec, attr, size can be zero for these tests.
func buildRISCOSExtra(load, exec, attr, size uint32) []byte {
	// Header: sig (2) + data size (2) + "ARC0" (4) + load (4) + exec (4) + size (4) + attr (4) + zero (4) = 28 bytes total
	dataSize := uint16(24) // bytes after the 4-byte header
	buf := make([]byte, 28)
	binary.LittleEndian.PutUint16(buf[0:], 0x4341)
	binary.LittleEndian.PutUint16(buf[2:], dataSize)
	copy(buf[4:], "ARC0")
	binary.LittleEndian.PutUint32(buf[8:], load)
	binary.LittleEndian.PutUint32(buf[12:], exec)
	binary.LittleEndian.PutUint32(buf[16:], size)
	binary.LittleEndian.PutUint32(buf[20:], attr)
	return buf
}

// buildTypedLoad returns a load address encoding a RISC OS filetype.
// Format: 0xFFF?_???? where the 12-bit filetype sits at bits 8–19.
func buildTypedLoad(filetype uint32) uint32 {
	return 0xFFF00000 | (filetype << 8) | 0x00
}

// makeZip writes a ZIP archive into a temp file and returns its path.
// entries maps entry name to (content, extra).
func makeZip(t *testing.T, entries map[string]struct {
	content []byte
	extra   []byte
}) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "test-*.zip")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	for name, e := range entries {
		fh := &zip.FileHeader{Name: name, Method: zip.Deflate}
		fh.Extra = e.extra
		fw, err := w.CreateHeader(fh)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(e.content); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

// --- getRISCOSFiletype unit tests ---

func TestGetRISCOSFiletype_Typed(t *testing.T) {
	extra := buildRISCOSExtra(buildTypedLoad(0xFFD), 0, 0, 0)
	ft, ok := getRISCOSFiletype(extra)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if ft != 0xFFD {
		t.Fatalf("expected filetype 0xFFD, got 0x%03X", ft)
	}
}

func TestGetRISCOSFiletype_Untyped(t *testing.T) {
	// Load address without 0xFFF top 12 bits — untyped file
	extra := buildRISCOSExtra(0x12345678, 0, 0, 0)
	_, ok := getRISCOSFiletype(extra)
	if ok {
		t.Fatal("expected ok=false for untyped file")
	}
}

func TestGetRISCOSFiletype_NoField(t *testing.T) {
	_, ok := getRISCOSFiletype([]byte{})
	if ok {
		t.Fatal("expected ok=false for empty extra")
	}
}

func TestGetRISCOSFiletype_WrongSignature(t *testing.T) {
	extra := buildRISCOSExtra(buildTypedLoad(0xFFD), 0, 0, 0)
	// Corrupt the signature
	extra[0] = 0x00
	_, ok := getRISCOSFiletype(extra)
	if ok {
		t.Fatal("expected ok=false for wrong signature")
	}
}

func TestGetRISCOSFiletype_WrongMagic(t *testing.T) {
	extra := buildRISCOSExtra(buildTypedLoad(0xFFD), 0, 0, 0)
	// Corrupt "ARC0"
	extra[4] = 'X'
	_, ok := getRISCOSFiletype(extra)
	if ok {
		t.Fatal("expected ok=false for wrong ARC0 magic")
	}
}

func TestGetRISCOSFiletype_Truncated(t *testing.T) {
	extra := buildRISCOSExtra(buildTypedLoad(0xFFD), 0, 0, 0)
	// Truncate before the load address
	_, ok := getRISCOSFiletype(extra[:10])
	if ok {
		t.Fatal("expected ok=false for truncated extra")
	}
}

func TestGetRISCOSFiletype_SecondField(t *testing.T) {
	// Prepend an unrelated extra field before the RISC OS one
	other := make([]byte, 8)
	binary.LittleEndian.PutUint16(other[0:], 0x0001) // some other sig
	binary.LittleEndian.PutUint16(other[2:], 4)      // 4 bytes of data
	extra := append(other, buildRISCOSExtra(buildTypedLoad(0xB60), 0, 0, 0)...)
	ft, ok := getRISCOSFiletype(extra)
	if !ok {
		t.Fatal("expected ok=true when RISC OS field is second")
	}
	if ft != 0xB60 {
		t.Fatalf("expected filetype 0xB60, got 0x%03X", ft)
	}
}

// --- extract integration tests ---

func TestExtract_WithRISCOSFiletype(t *testing.T) {
	extra := buildRISCOSExtra(buildTypedLoad(0xFFD), 0, 0, 0)
	zipPath := makeZip(t, map[string]struct {
		content []byte
		extra   []byte
	}{
		"readme": {content: []byte("hello"), extra: extra},
	})

	dest := t.TempDir()
	if err := extract(zipPath, dest, false); err != nil {
		t.Fatal(err)
	}

	expected := filepath.Join(dest, "readme,ffd")
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("expected file %q not found: %v", expected, err)
	}
}

func TestExtract_WithoutRISCOSFiletype(t *testing.T) {
	zipPath := makeZip(t, map[string]struct {
		content []byte
		extra   []byte
	}{
		"plain.txt": {content: []byte("hello"), extra: nil},
	})

	dest := t.TempDir()
	if err := extract(zipPath, dest, false); err != nil {
		t.Fatal(err)
	}

	expected := filepath.Join(dest, "plain.txt")
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("expected file %q not found: %v", expected, err)
	}
}

func TestExtract_Directory(t *testing.T) {
	zipPath := makeZip(t, map[string]struct {
		content []byte
		extra   []byte
	}{
		"subdir/": {content: nil, extra: nil},
	})

	dest := t.TempDir()
	if err := extract(zipPath, dest, false); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(filepath.Join(dest, "subdir"))
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Fatal("expected a directory")
	}
}

func TestExtract_FileContents(t *testing.T) {
	content := []byte("test content")
	zipPath := makeZip(t, map[string]struct {
		content []byte
		extra   []byte
	}{
		"file.txt": {content: content, extra: nil},
	})

	dest := t.TempDir()
	if err := extract(zipPath, dest, false); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(dest, "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("content mismatch: got %q, want %q", got, content)
	}
}

func TestExtract_ZipSlip(t *testing.T) {
	// Manually build a ZIP with a path traversal entry
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	fh := &zip.FileHeader{Name: "../escape.txt", Method: zip.Store}
	fw, err := w.CreateHeader(fh)
	if err != nil {
		t.Fatal(err)
	}
	fw.Write([]byte("evil"))
	w.Close()

	zipPath := filepath.Join(t.TempDir(), "slip.zip")
	if err := os.WriteFile(zipPath, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	dest := t.TempDir()
	if err := extract(zipPath, dest, false); err == nil {
		t.Fatal("expected error for zip-slip path, got nil")
	}
}

func TestExtract_Verbose(t *testing.T) {
	extra := buildRISCOSExtra(buildTypedLoad(0xFFD), 0, 0, 0)
	zipPath := makeZip(t, map[string]struct {
		content []byte
		extra   []byte
	}{
		"file": {content: []byte("x"), extra: extra},
	})

	// Redirect stdout to capture verbose output
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	dest := t.TempDir()
	if err := extract(zipPath, dest, true); err != nil {
		t.Fatal(err)
	}

	w.Close()
	os.Stdout = old

	var out bytes.Buffer
	out.ReadFrom(r)
	if out.Len() == 0 {
		t.Fatal("expected verbose output, got none")
	}
}
