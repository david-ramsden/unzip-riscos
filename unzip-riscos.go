// Extract a zip file preserving RISC OS filetypes as NFS-encoded ,xxx suffixes.
//
// Extracts using Go's archive/zip (supports Stored and Deflate) and renames
// files by appending ,xxx suffixes based on RISC OS Info-ZIP extra fields
// (signature 0x4341 'AC'). Directories are detected by trailing slash and not renamed.
//
// Usage: unzip-riscos [-v] <zipfile> [<zipfile> ...] <destdir>
//
// Build static binary:
//   CGO_ENABLED=0 go build -o unzip-riscos unzip-riscos.go

package main

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var version = "dev"

func getRISCOSFiletype(extra []byte) (int, bool) {
	for i := 0; i < len(extra)-3; {
		sig := binary.LittleEndian.Uint16(extra[i:])
		size := int(binary.LittleEndian.Uint16(extra[i+2:]))
		if sig == 0x4341 && size >= 20 && i+8 <= len(extra) && bytes.Equal(extra[i+4:i+8], []byte("ARC0")) {
			if i+12 <= len(extra) {
				load := binary.LittleEndian.Uint32(extra[i+8:])
				if (load >> 20) == 0xFFF {
					return int((load >> 8) & 0xFFF), true
				}
				return 0, false // untyped / datestamped without type
			}
		}
		i += 4 + size
	}
	return 0, false
}

func extract(zippath, destdir string, verbose bool) error {
	r, err := zip.OpenReader(zippath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		isDir := len(f.Name) > 0 && f.Name[len(f.Name)-1] == '/'

		dest := filepath.Join(destdir, f.Name)

		// Guard against zip slip
		if !filepath.IsLocal(f.Name) {
			return fmt.Errorf("unsafe path in zip: %s", f.Name)
		}

		if isDir {
			if err := os.MkdirAll(dest, 0777); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(dest), 0777); err != nil {
			return err
		}

		// Append ,xxx suffix if RISC OS filetype is present
		if filetype, ok := getRISCOSFiletype(f.Extra); ok {
			dest = fmt.Sprintf("%s,%03x", dest, filetype)
		}

		if err := writeFile(f, dest); err != nil {
			return err
		}

		if verbose {
			fmt.Println(dest)
		}
	}

	return nil
}

func writeFile(f *zip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	// Fix permissions — RISC OS attributes map badly to Unix perms,
	// and files need to be world-writable so container users can write back through HostFS
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}

func main() {
	verbose := flag.Bool("v", false, "print each extracted file")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s %s [-v] <zipfile> [<zipfile> ...] <destdir>\n", os.Args[0], version)
	}
	flag.Parse()

	if flag.NArg() < 2 {
		flag.Usage()
		os.Exit(1)
	}

	args := flag.Args()
	destdir := args[len(args)-1]
	patterns := args[:len(args)-1]

	var zips []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err == nil && len(matches) > 0 {
			zips = append(zips, matches...)
		} else {
			zips = append(zips, pattern) // let extract() fail with a clear error
		}
	}

	for _, zippath := range zips {
		if err := extract(zippath, destdir, *verbose); err != nil {
			fmt.Fprintf(os.Stderr, "error processing %s: %v\n", zippath, err)
			os.Exit(1)
		}
	}
}
