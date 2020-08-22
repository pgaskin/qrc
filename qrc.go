package qrc

import (
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/klauspost/compress/zstd"
)

//go:generate go run locale_generate.go

// NodeFlag is a flag for a Node. Multiple flags can be ORd together.
type NodeFlag uint16

const (
	NodeFlagNone           NodeFlag = 0
	NodeFlagCompressed     NodeFlag = 1
	NodeFlagDirectory      NodeFlag = 2
	NodeFlagCompressedZstd NodeFlag = 4
)

// Node represents a Qt resource tree node.
type Node struct {
	NameOffset uint32
	Flags      NodeFlag

	// if dir
	ChildCount  uint32
	ChildOffset uint32

	// if not dir
	Country    Country
	Language   Language
	DataOffset uint32

	// format >= 2
	Modified uint64

	Format int // not an actual field, but used while parsing
}

func nodeSize(format int) int64 {
	s := 14
	if format >= 2 {
		s += 8
	}
	return int64(s)
}

// ParseNode reads a Qt resource tree node from the provided reader. If an error
// occurs, any number of bytes may have been read from the reader.
func ParseNode(r io.Reader, format int) (*Node, error) {
	var n Node
	n.Format = format

	if format > 3 {
		return nil, fmt.Errorf("unsupported qrc version %d", format)
	}

	if x := binary.Size(uint16(0)); binary.Size(n.Language) != x || binary.Size(n.Country) != x || binary.Size(n.Flags) != x {
		panic("wrong enum type size")
	}

	lr := io.LimitReader(r, nodeSize(n.Format)).(*io.LimitedReader)

	if err := binary.Read(lr, binary.BigEndian, &n.NameOffset); err != nil {
		return nil, fmt.Errorf("read name offset: %w", err)
	}

	if err := binary.Read(lr, binary.BigEndian, &n.Flags); err != nil {
		return nil, fmt.Errorf("read flags: %w", err)
	}
	if err := n.Flags.Valid(); err != nil {
		return nil, fmt.Errorf("read flags: %w", err)
	}

	if n.IsDir() {
		if err := binary.Read(lr, binary.BigEndian, &n.ChildCount); err != nil {
			return nil, fmt.Errorf("read dir child count: %w", err)
		}
		if err := binary.Read(lr, binary.BigEndian, &n.ChildOffset); err != nil {
			return nil, fmt.Errorf("read dir child offset: %w", err)
		}
	} else {
		if err := binary.Read(lr, binary.BigEndian, &n.Country); err != nil {
			return nil, fmt.Errorf("read file country qualifier: %w", err)
		}
		if err := binary.Read(lr, binary.BigEndian, &n.Language); err != nil {
			return nil, fmt.Errorf("read file country qualifier: %w", err)
		}
		if err := binary.Read(lr, binary.BigEndian, &n.DataOffset); err != nil {
			return nil, fmt.Errorf("read file data offset: %w", err)
		}
	}

	if n.Format >= 2 {
		if err := binary.Read(lr, binary.BigEndian, &n.Modified); err != nil {
			return nil, fmt.Errorf("read mod time: %w", err)
		}
	}

	if lr.N != 0 {
		return nil, fmt.Errorf("bug: should have read %d bytes more", lr.N)
	}

	return &n, nil
}

// IsDir returns true if the tree node represents a directory.
func (n Node) IsDir() bool {
	return n.Flags.Has(NodeFlagDirectory)
}

// ModTime returns the file modification time for format version >= 2. On older
// versions, a zero time is returned.
func (n Node) ModTime() time.Time {
	if n.Format < 2 || n.Modified == 0 {
		return time.Time{}
	}
	return time.Unix(int64(n.Modified/1000), 0)
}

// Name reads the name of the file.
func (n Node) Name(names io.ReaderAt) (string, error) {
	var length uint16
	if err := binary.Read(io.NewSectionReader(names, int64(n.NameOffset), 2), binary.BigEndian, &length); err != nil {
		var extra string
		if err == io.EOF {
			extra = " (maybe your offsets are incorrect?)"
		}
		return "", fmt.Errorf("read length from names at %#x%s: %w", n.NameOffset, extra, err)
	}

	var hash uint32
	if err := binary.Read(io.NewSectionReader(names, int64(n.NameOffset+2), 4), binary.BigEndian, &hash); err != nil {
		return "", fmt.Errorf("read hash from names at %#x: %w", n.NameOffset+2, err)
	}

	var name string
	buf := make([]uint16, length)
	if err := binary.Read(io.NewSectionReader(names, int64(n.NameOffset+2+4), int64(length*2)), binary.BigEndian, &buf); err != nil {
		return "", fmt.Errorf("read utf16 data from names at %#x (len=%d): %w", n.NameOffset+2+4, length*2, err)
	}
	name = string(utf16.Decode(buf))
	if !utf8.ValidString(name) {
		return "", fmt.Errorf("name is likely incorrect, is invalid utf8 (%q)", name) // note: may be too strict
	}
	return name, nil
}

// Children gets the children of the tree node. If it is not a directory, an
// error is returned.
func (n Node) Children(tree io.ReaderAt) ([]*Node, error) {
	if !n.IsDir() {
		return nil, fmt.Errorf("is a file, not a directory")
	}

	c := make([]*Node, int(n.ChildCount))
	r := io.NewSectionReader(tree, int64(n.ChildOffset)*nodeSize(n.Format), int64(n.ChildCount)*nodeSize(n.Format))
	for i := range c {
		v, err := ParseNode(r, n.Format)
		if err != nil {
			return nil, fmt.Errorf("parse child (i=%d): %w", i, err)
		}
		c[i] = v
	}
	return c, nil
}

// Data opens a reader for the original content of the file, and also returns
// the offset/size (relative to the data reader) of the corresponding data in
// the resource (this may be smaller than the file contents if the data was
// compressed). If the entry is a directory, an error is returned.
func (n Node) Data(data io.ReaderAt) (rc io.ReadCloser, fileOff int64, fileSz int64, err error) {
	if n.IsDir() {
		return nil, 0, 0, fmt.Errorf("is a directory, not a file")
	}

	if err := n.Flags.Valid(); err != nil {
		return nil, 0, 0, fmt.Errorf("invalid flags: %w", err)
	}

	var length uint32
	if err := binary.Read(io.NewSectionReader(data, int64(n.DataOffset), 4), binary.BigEndian, &length); err != nil {
		return nil, 0, 0, fmt.Errorf("read data length: %w", err)
	}

	r := io.NewSectionReader(data, int64(n.DataOffset)+4, int64(length))
	switch {
	case n.Flags.Has(NodeFlagCompressed):
		var zsz uint32 // note that this isn't strict; qUncompress will accept data longer
		if err := binary.Read(r, binary.BigEndian, &zsz); err != nil {
			return nil, 0, 0, fmt.Errorf("read qCompress original size header from zlib data: %w", err)
		}

		fileOff, fileSz = int64(n.DataOffset)+4+4, int64(length)
		zr, err := zlib.NewReader(r)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("open zlib reader: %w", err)
		}
		rc = zr
	case n.Flags.Has(NodeFlagCompressedZstd):
		fileOff, fileSz = int64(n.DataOffset)+4, int64(length)
		zr, err := zstd.NewReader(r)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("open zstd reader: %w", err)
		}
		rc = ioutil.NopCloser(zr)
	default:
		fileOff, fileSz = int64(n.DataOffset)+4, int64(length)
		rc = ioutil.NopCloser(r)
	}
	return rc, fileOff, fileSz, nil
}

// TODO: func (n Node) ReplaceData(data io.ReaderAt, dataW io.WriterAt, buf []byte) error

func (f NodeFlag) String() string {
	var x []string
	if f.Has(NodeFlagNone) {
		x = append(x, "None")
	}
	if f.Has(NodeFlagCompressed) {
		x = append(x, "Compressed")
	}
	if f.Has(NodeFlagDirectory) {
		x = append(x, "Directory")
	}
	if f.Has(NodeFlagCompressedZstd) {
		x = append(x, "CompressedZstd")
	}
	if r := f.remainder(); r != 0 {
		x = append(x, "0b"+strconv.FormatUint(uint64(r), 2))
	}
	return strings.Join(x, "|")
}

// Has returns true if the provided flag bits are set.
func (f NodeFlag) Has(v NodeFlag) bool {
	return f&v == v
}

// Valid checks if the combination of flags are valid. It does not check the
// format version.
func (f NodeFlag) Valid() error {
	if r := f.remainder(); r != 0 {
		return fmt.Errorf("flag contains unknown bits %#b", r)
	}
	if f.Has(NodeFlagCompressed) && f.Has(NodeFlagCompressedZstd) {
		return fmt.Errorf("flag cannot be Compressed and CompressedZstd at the same time")
	}
	return nil
}

func (f NodeFlag) remainder() NodeFlag {
	return f &^ (NodeFlagNone | NodeFlagCompressed | NodeFlagDirectory | NodeFlagCompressedZstd)
}
