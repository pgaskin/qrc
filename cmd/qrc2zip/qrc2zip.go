// Command qrc2zip extracts compiled Qt resources into a zip file.
package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pgaskin/qrc"
	"github.com/spf13/pflag"
)

type QRC2Zip struct {
	Output    string
	Force     bool
	Recursive bool
	Exclude   []string
	Verbose   bool
}

func main() {
	var q2z QRC2Zip
	var help bool

	pflag.CommandLine.SortFlags = false
	pflag.StringVarP(&q2z.Output, "output", "o", "resources.zip", "Output filename")
	pflag.BoolVarP(&q2z.Force, "force", "f", false, "Ignore errors during extraction if possible")
	pflag.BoolVarP(&q2z.Recursive, "recursive", "r", false, "Expand nested RCC files")
	pflag.StringArrayVarP(&q2z.Exclude, "exclude", "e", nil, "Exclude files matching this glob (can be specified multiple times)")
	pflag.BoolVarP(&q2z.Verbose, "verbose", "v", false, "Show information about the files being extracted")
	pflag.BoolVarP(&help, "help", "h", false, "Show this help text")
	pflag.Parse()

	if help || (pflag.NArg() != 1 && pflag.NArg() != 5) {
		fmt.Fprintf(os.Stderr, ""+
			"Usage: %s [options] rcc_file\n"+
			"       %s [options] executable format_version tree_offset data_offset names_offset\n"+
			"\nOptions:\n"+
			"%s"+
			"\nExecutable offsets:\n"+
			"  To find executable offsets and format version, look for calls to qRegisterResourceData. These\n"+
			"  are usually within entry points or qInitResource* functions. qRegisterResourceData takes four\n"+
			"  arguments: format, tree, names, data.\n"+
			"\nQt support:\n"+
			"  Format versions 1-3 are supported, along with locale/country codes from Qt 5.13. Resources\n"+
			"  can be compressed with zlib or zstd.\n"+
			"\nOutput:\n"+
			"  The extracted resources are written to a zip file. The directory structure is preserved and\n"+
			"  separated with forward slashes on all platforms. If the file has language/country constraints,\n"+
			"  they are added to the filename before the extension in the format '[language!LanguageName]'\n"+
			"  and [country!CountryName]. If the Qt resource format is >= 2, the modification time is also\n"+
			"  written for each file.\n"+
			"\ngithub.com/pgaskin/qrc\n",
			os.Args[0], os.Args[0], pflag.CommandLine.FlagUsages(),
		)
		if help {
			os.Exit(0)
		} else {
			os.Exit(2)
		}
		return
	}

	var err error
	switch pflag.NArg() {
	case 1:
		err = q2z.DoRCC(pflag.Args()[0])
	case 5:
		var formatVersion int
		var treeOffset, dataOffset, namesOffset int64
		formatVersion, err = strconv.Atoi(pflag.Args()[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: parse format version %q: %v.\n", pflag.Args()[1], err)
			os.Exit(2)
			return
		}
		treeOffset, err = strconv.ParseInt(pflag.Args()[2], 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: parse tree offset %q: %v.\n", pflag.Args()[2], err)
			os.Exit(2)
			return
		}
		dataOffset, err = strconv.ParseInt(pflag.Args()[3], 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: parse data offset %q: %v.\n", pflag.Args()[3], err)
			os.Exit(2)
			return
		}
		namesOffset, err = strconv.ParseInt(pflag.Args()[4], 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: parse names offset %q: %v.\n", pflag.Args()[4], err)
			os.Exit(2)
			return
		}
		err = q2z.DoRaw(pflag.Args()[0], formatVersion, treeOffset, dataOffset, namesOffset)
	default:
		panic("unexpected number of arguments")
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v.\n", err)
		os.Exit(1)
		return
	}

	os.Exit(0)
	return
}

func (q2z QRC2Zip) DoRCC(rcc string) error {
	f, err := os.Open(rcc)
	if err != nil {
		return fmt.Errorf("open rcc file: %w", err)
	}
	defer f.Close()

	r, err := qrc.NewReaderFromRCC(f)
	if err != nil {
		return fmt.Errorf("parse rcc file %q: %w", rcc, err)
	}

	return q2z.doReader(r)
}

func (q2z QRC2Zip) DoRaw(file string, formatVersion int, treeOffset, dataOffset, namesOffset int64) error {
	f, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	r, err := qrc.NewReader(f, formatVersion, treeOffset, dataOffset, namesOffset)
	if err != nil {
		return fmt.Errorf("parse rcc file %q: %w", file, err)
	}

	return q2z.doReader(r)
}

func (q2z QRC2Zip) doReader(r *qrc.Reader) error {
	fon := "." + q2z.Output + ".tmp"
	defer os.Remove(fon)

	fo, err := os.OpenFile(fon, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("create output temp file: %w", err)
	}
	defer fo.Close()

	zw := zip.NewWriter(fo)
	if err := q2z.generate(zw, r); err != nil {
		return fmt.Errorf("generate zip: %w", err)
	}
	if err = zw.Close(); err != nil {
		return fmt.Errorf("generate zip: %w", err)
	}

	if err := fo.Close(); err != nil {
		return fmt.Errorf("close zip: %w", err)
	}

	if err := os.Rename(fon, q2z.Output); err != nil {
		return fmt.Errorf("rename temp file %q to output file %q: %w", fon, q2z.Output, err)
	}

	return nil
}

func (q2z QRC2Zip) generate(w *zip.Writer, r *qrc.Reader) error {
	return r.Walk(func(rpath string, entry *qrc.ReaderEntry, err error) error {
		for _, p := range q2z.Exclude {
			if m, err := path.Match(p, rpath); err != nil {
				return fmt.Errorf("check for match against skip pattern %q: %w", p, err)
			} else if m {
				if q2z.Verbose {
					fmt.Printf("SKIP    %q (matches %q)\n", rpath, p)
				}
				return filepath.SkipDir
			}
		}
		var offset, size int64
		if err == nil && entry != nil {
			offset = entry.Offset()
			size, err = entry.Size()
		}
		if err != nil {
			if q2z.Verbose {
				fmt.Printf("ERROR  %q (%v)\n", rpath, err)
			}
			if q2z.Force {
				fmt.Fprintf(os.Stderr, "Warning: ignoring error: walk %q: %v\n", rpath, err)
				return nil
			}
			return err
		}
		if entry.IsDir() {
			if q2z.Verbose {
				fmt.Printf("DIR     %q (0x%X + %d)\n", rpath, offset, size)
			}
			return nil
		}

		var c string
		x, y := entry.Constraints()
		if x != qrc.CountryAnyCountry {
			c += "[country!" + x.String() + "]"
		}
		if y != qrc.LanguageAnyLanguage && y != qrc.LanguageC {
			c += "[language!" + y.String() + "]"
		}

		var f string
		if c != "" {
			f = strings.TrimSuffix(rpath, filepath.Ext(rpath)) + c + filepath.Ext(rpath)
		} else {
			f = rpath
		}

		if q2z.Verbose {
			if rpath != f {
				fmt.Printf("FILE    %q => %q (0x%X + %d)\n", rpath, f, offset, size)
			} else {
				fmt.Printf("FILE    %q (0x%X + %d)\n", rpath, offset, size)
			}
		}

		d, err := entry.Open()
		if err != nil {
			return fmt.Errorf("open resource %q %s: %w", rpath, c, err)
		}
		defer d.Close()

		z, err := w.CreateHeader(&zip.FileHeader{
			Name:     f, // will already be separated with slashes
			Modified: entry.ModTime(),
		})
		if err != nil {
			return fmt.Errorf("create zip header for %q: %w", f, err)
		}

		if _, err := io.Copy(z, d); err != nil {
			return fmt.Errorf("write contents of %q: %w", f, err)
		}

		return nil
	}, q2z.Recursive)
}
