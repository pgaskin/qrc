package qrc

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"path/filepath"
	"strings"
	"time"
)

// Reader is a high-level reader for compiled Qt resources. It is thread-safe if
// the underlying reader is.
type Reader struct {
	format      int
	reader      io.ReaderAt
	treeOffset  int64
	dataOffset  int64
	namesOffset int64
	root        *Node
}

// ReaderEntry is an entry read by a Reader.
type ReaderEntry struct {
	v string
	n *Node
	r *Reader
}

// WalkFunc is the same as filepath.WalkFunc. The path is always separated with
// forward slashes and does not have a leading dot. In addition,
// filepath.SkipDir will also prevent recursion into embedded RCC files if
// present.
type WalkFunc func(path string, entry *ReaderEntry, err error) error

// NewReader initializes a reader with the provided version and offsets.
func NewReader(r io.ReaderAt, formatVersion int, treeOffset, dataOffset, namesOffset int64) (*Reader, error) {
	rd := &Reader{
		format:      formatVersion,
		reader:      r,
		treeOffset:  treeOffset,
		dataOffset:  dataOffset,
		namesOffset: namesOffset,
	}

	n, err := ParseNode(rd.tree(), rd.format)
	if err != nil {
		return nil, fmt.Errorf("parse root node: %w", err)
	}
	rd.root = n

	return rd, nil
}

// NewReaderFromRCC initializes a reader for the provided RCC file.
func NewReaderFromRCC(r io.ReaderAt) (*Reader, error) {
	h, err := ParseRCCHeader(io.NewSectionReader(r, 0, 1<<63-1))
	if err != nil {
		return nil, fmt.Errorf("parse rcc header: %w", err)
	}
	return NewReader(r, int(h.FormatVersion), int64(h.TreeOffset), int64(h.DataOffset), int64(h.NamesOffset))
}

// TODO: func NewReaderFromELF(f *elf.File) ([]*Reader, error); either find calls to qRegisterResourceData or use a heuristic

// Children returns the top-level files in the resource root.
func (r *Reader) Children() ([]*ReaderEntry, error) {
	return (&ReaderEntry{
		n: r.root,
		r: r,
	}).Children()
}

// Walk calls the provided WalkFunc for each entry in the tree, similarly to
// filepath.Walk (including filepath.SkipDir). If rccRecurse is true, nested RCC
// files are opened and treated as a directory.
func (r *Reader) Walk(fn WalkFunc, rccRecurse bool) error {
	return walk(fn, rccRecurse, "", &ReaderEntry{
		n: r.root,
		r: r,
	})
}

// walk is a recursive depth-first helper for Walk.
func walk(fn WalkFunc, rccRecurse bool, path string, entry *ReaderEntry) error {
	if entry.IsDir() {
		// attempt to read the dir's children
		c, err := entry.Children()
		if err != nil {
			// call fn for the dir itself with the error
			if path == "" {
				// return the error itself for the root directory
				return err
			}
			if err := fn(path, entry, fmt.Errorf("walk: get children for dir %q: %w", path, err)); err != nil {
				if err == filepath.SkipDir {
					return nil
				}
				return fmt.Errorf("walk %q: %w", path, err)
			}
			return nil
		}

		// call fn for the dir itself if it's not the root
		if path != "" {
			if err := fn(path, entry, nil); err != nil {
				if err == filepath.SkipDir {
					return nil
				}
				return fmt.Errorf("walk %q: %w", path, err)
			}
		}

		// call fn for the dir's contents
		for _, v := range c {
			if err := walk(fn, rccRecurse, strings.TrimLeft(path+"/"+v.Name(), "/"), v); err != nil {
				if err == filepath.SkipDir {
					panic("filepath.SkipDir shouldn't have been returned from walk")
				}
				if path == "" {
					// return the error itself for the root directory
					return err
				}
				return fmt.Errorf("walk %q: %w", path, err)
			}
		}

		return nil
	}

	// check whether to treat the file as a nested rcc dir
	if rccRecurse && filepath.Ext(path) == ".rcc" {
		// attempt to open the rcc
		d, err := entry.Open()

		if err != nil {
			// call fn for the rcc itself with the error
			if err := fn(path, entry, fmt.Errorf("walk: open nested rcc %q: %w", path, err)); err != nil {
				if err == filepath.SkipDir {
					return nil
				}
				return fmt.Errorf("walk %q: %w", path, err)
			}
			return nil
		}
		defer d.Close()

		// attempt to read the rcc
		buf, err := ioutil.ReadAll(d)

		if err != nil {
			// call fn for the rcc itself with the error
			if err := fn(path, entry, fmt.Errorf("walk: read nested rcc %q into memory: %w", path, err)); err != nil {
				if err == filepath.SkipDir {
					return nil
				}
				return fmt.Errorf("walk %q: %w", path, err)
			}
			return nil
		}

		// attempt to parse the rcc
		r, err := NewReaderFromRCC(bytes.NewReader(buf))

		if err != nil {
			// call fn for the rcc itself with the error
			if err := fn(path, entry, fmt.Errorf("walk: parse nested rcc %q: %w", path, err)); err != nil {
				if err == filepath.SkipDir {
					return nil
				}
				return fmt.Errorf("walk %q: %w", path, err)
			}
			return nil
		}

		// re-walk the opened rcc as a dir
		if err := walk(fn, rccRecurse, path, &ReaderEntry{
			v: entry.v,
			n: r.root,
			r: r,
		}); err != nil {
			if err == filepath.SkipDir {
				panic("filepath.SkipDir shouldn't have been returned from walk")
			}
			return fmt.Errorf("walk nested rcc %q: %w", path, err)
		}

		return nil
	}

	if err := fn(path, entry, nil); err != nil {
		if err == filepath.SkipDir {
			return nil
		}
		return fmt.Errorf("walk %q: %w", path, err)
	}

	return nil
}

func (r Reader) tree() *io.SectionReader {
	return io.NewSectionReader(r.reader, r.treeOffset, math.MaxInt32)
}

func (r Reader) data() *io.SectionReader {
	return io.NewSectionReader(r.reader, r.dataOffset, math.MaxInt32)
}

func (r Reader) names() *io.SectionReader {
	return io.NewSectionReader(r.reader, r.namesOffset, math.MaxInt32)
}

func newReaderEntry(r *Reader, n *Node) (*ReaderEntry, error) {
	var e ReaderEntry
	e.r = r
	e.n = n
	if v, err := n.Name(r.names()); err != nil {
		return nil, err
	} else {
		e.v = v
	}
	return &e, nil
}

// Name returns the name of the entry.
func (e ReaderEntry) Name() string {
	return e.v
}

// Constraints returns the country/language constraints for the file. A
// directory can contain multiple files with the same name, but different constraints.
func (e ReaderEntry) Constraints() (Country, Language) {
	return e.n.Country, e.n.Language
}

// ModTime returns the modification time of the entry. On format versions < 2,
// a zero time is always returned.
func (e ReaderEntry) ModTime() time.Time {
	return e.n.ModTime()
}

// IsDir returns true if the entry represents a directory.
func (e ReaderEntry) IsDir() bool {
	return e.n.IsDir()
}

// Flags returns the flags set on the underlying node.
func (e ReaderEntry) Flags() NodeFlag {
	return e.n.Flags
}

// Children reads and returns the child entries. If the entry is not a
// directory, an error is returned.
func (e ReaderEntry) Children() ([]*ReaderEntry, error) {
	n, err := e.n.Children(e.r.tree())
	if err != nil {
		return nil, err
	}

	x := make([]*ReaderEntry, len(n))
	for i := range n {
		v, err := n[i].Name(e.r.names())
		if err != nil {
			return nil, fmt.Errorf("parse child %d: read name %w", i, err)
		}
		x[i] = &ReaderEntry{
			v: v,
			n: n[i],
			r: e.r,
		}
	}

	return x, nil
}

// Open opens a reader for the contents of the entry. If the entry is a
// directory, an error is returned.
func (e ReaderEntry) Open() (io.ReadCloser, error) {
	rc, _, _, err := e.n.Data(e.r.data())
	return rc, err
}

// Offset returns the real offset of the entry's contents relative to the base
// io.ReaderAt used when creating the Reader. If the entry is a directory, the
// offset points to the first child's tree node. If the entry is a file, the
// offset points to the first byte of data (immediately after the uint32 size
// header, plus the 4-byte qCompress zlib header if the node has the
// NodeFlagCompressed flag).
func (e ReaderEntry) Offset() int64 {
	if e.IsDir() {
		return e.r.treeOffset + e.n.dirTreeOffset()
	}
	offset := e.r.dataOffset + e.n.fileDataOffset()
	if e.n.Flags.Has(NodeFlagCompressed) {
		offset += 4 // qCompress zlib header
	}
	return offset
}

// Size returns the real (i.e. as-is, possibly compressed) size of the
// underlying data relative to the base io.ReaderAt used when creating the
// Reader. If the entry is a directory, the size is the total of all child tree
// nodes (i.e. Offset() + Size() = end of last child). If the entry is a file, the
// size is the size of the underlying data in the file. To get the uncompressed
// size, Open() the entry and count the number of bytes read.
func (e ReaderEntry) Size() (int64, error) {
	if e.IsDir() {
		return e.n.dirSize(), nil
	}
	return e.n.fileSize(e.r.data())
}
