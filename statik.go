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
	file, err := createSourceFile(*flagSrc)
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

func createSourceFile(srcPath string) (file *os.File, err error) {
	var (
		buffer    bytes.Buffer
		zipWriter io.Writer
	)
	zipWriter = &buffer
	f, err := ioutil.TempFile("", namePackage)
	if err != nil {
		return
	}

	zipWriter = io.MultiWriter(zipWriter, f)
	defer f.Close()
	var modTime time.Time

	w := zip.NewWriter(zipWriter)
	if err = filepath.Walk(srcPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Ignore empty directories and hidden files.
		if fi.IsDir() || strings.HasPrefix(fi.Name(), ".") {
			return nil
		}
		suffix, err := filepath.Rel(srcPath, path)
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
		f, err := w.Create(filepath.ToSlash(suffix))
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
	fmt.Fprintf(&qb, "package %s\n\n", namePackage)
	// imports
	fmt.Fprint(&qb, "import (\n")
	fmt.Fprint(&qb, "\t\"time\"\n\n")
	fmt.Fprint(&qb, "\t\"github.com/rakyll/statik/fs\"\n")
	fmt.Fprint(&qb, ")\n\n")
	// func init
	fmt.Fprint(&qb, "func init() {\n")
	fmt.Fprintf(&qb, "\tmodTime := time.Unix(%d, 0)\n", modTime.Unix())
	fmt.Fprint(&qb, "\tdata := ")
	quote(&qb, buffer.Bytes())
	fmt.Fprint(&qb, "\n\tfs.Register(modTime, data)")
	fmt.Fprint(&qb, "\n}\n")

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

func quote(dest *bytes.Buffer, bs []byte) {
	dest.WriteByte('"')
	for _, b := range bs {
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
	dest.WriteByte('"')
}

func exitWithError(err error) {
	fmt.Println(err)
	os.Exit(1)
}
