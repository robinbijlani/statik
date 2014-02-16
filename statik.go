package main

import (
	"archive/zip"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	namePackage    = "statik"
	nameSourceFile = "statik.go"
)

var (
	flagSrc  = flag.String("src", ".", "The path of the source directory.")
	flagDest = flag.String("dest", ".", "The destination path of the generated package.")
)

func main() {
	flag.Parse()
	file, err := generateSource(*flagSrc)
	if err != nil {
		exitWithError(err)
	}

	destDir := path.Join(*flagDest, namePackage)
	err = os.MkdirAll(destDir, 0755)
	if err != nil {
		exitWithError(err)
	}

	err = os.Rename(file.Name(), path.Join(destDir, nameSourceFile))
	if err != nil {
		exitWithError(err)
	}
}

// Walks on the source path and generates source code
// that contains source directory's contents as zip contents.
// Generates source registers generated zip contents data to
// be read by the statik/fs HTTP file system.
func generateSource(srcPath string) (file *os.File, err error) {
	var (
		buffer    bytes.Buffer
		zipWriter io.Writer
		modTime   time.Time
	)

	zipWriter = &buffer
	f, err := ioutil.TempFile("", namePackage)
	if err != nil {
		return
	}

	zipWriter = io.MultiWriter(zipWriter, f)
	defer f.Close()

	w := zip.NewWriter(zipWriter)
	if err = filepath.Walk(srcPath, func(path string, fi os.FileInfo, err error) error {
		// Ignore directories and hidden files.
		// No entry is needed for directories in a zip file.
		// Each file is represented with a path, no directory
		// entities are required to build the hierarchy.
		if fi.IsDir() || strings.HasPrefix(fi.Name(), ".") {
			return nil
		}
		relPath, err := filepath.Rel(srcPath, path)
		if err != nil {
			return err
		}
		if mt := fi.ModTime(); mt.After(modTime) {
			modTime = mt
		}
		b, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		f, err := w.Create(filepath.ToSlash(relPath))
		if err != nil {
			return err
		}
		_, err = f.Write(b)
		return err
	}); err != nil {
		return
	}
	if err = w.Close(); err != nil {
		return
	}

	// then embed it as a quoted string
	var qb bytes.Buffer
	fmt.Fprintf(&qb, `package %s

import (
		"github.com/rakyll/statik/fs"
)

func init() {
	data := "`, namePackage)
	FprintZipData(&qb, buffer.Bytes())
	fmt.Fprint(&qb, `"
	fs.Register(data)
}
`)

	// Create a temp file to output the generated code
	sourceFile, err := ioutil.TempFile("", nameSourceFile)
	if err != nil {
		return
	}
	if err = ioutil.WriteFile(sourceFile.Name(), qb.Bytes(), 0644); err != nil {
		return
	}
	return sourceFile, nil
}

// Converts zip binary contents to a string literal.
func FprintZipData(dest *bytes.Buffer, zipData []byte) {
	for _, b := range zipData {
		if b == '\n' {
			dest.WriteString(`\n`)
			continue
		}
		if b == '\\' {
			dest.WriteString(`\\`)
			continue
		}
		if b == '"' {
			dest.WriteString(`\"`)
			continue
		}
		if (b >= 32 && b <= 126) || b == '\t' {
			dest.WriteByte(b)
			continue
		}
		fmt.Fprintf(dest, "\\x%02x", b)
	}
}

// Prints out the error message and exists with a non-success signal.
func exitWithError(err error) {
	fmt.Println(err)
	os.Exit(1)
}
