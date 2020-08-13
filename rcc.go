package qrc

import (
	"encoding/binary"
	"fmt"
	"io"
)

// RCCHeaderMagic identifies a RCC file.
var RCCHeaderMagic = [4]byte{'q', 'r', 'e', 's'}

// RCCHeader is the header of a Qt resource file in the binary format. Note that
// the offsets are relative to the start of the file (i.e. the start of the
// header).
type RCCHeader struct {
	Magic         [4]byte
	FormatVersion int32
	TreeOffset    int32
	DataOffset    int32
	NamesOffset   int32

	// FormatVersion >= 3
	OverallFlags int32
}

// ParseRCCHeader parses the RCC header. If the magic bytes are invalid, an
// error is returned. If an error occurs, any number of bytes may have been read
// from the reader.
func ParseRCCHeader(r io.Reader) (*RCCHeader, error) {
	var h RCCHeader

	if err := binary.Read(r, binary.BigEndian, &h.Magic); err != nil {
		return nil, fmt.Errorf("read magic: %w", err)
	}

	if h.Magic != RCCHeaderMagic {
		return nil, fmt.Errorf("invalid magic %#v", h.Magic)
	}

	if err := binary.Read(r, binary.BigEndian, &h.FormatVersion); err != nil {
		return nil, fmt.Errorf("read format version: %w", err)
	}

	if err := binary.Read(r, binary.BigEndian, &h.TreeOffset); err != nil {
		return nil, fmt.Errorf("read tree offset: %w", err)
	}

	if err := binary.Read(r, binary.BigEndian, &h.DataOffset); err != nil {
		return nil, fmt.Errorf("read data offset: %w", err)
	}

	if err := binary.Read(r, binary.BigEndian, &h.NamesOffset); err != nil {
		return nil, fmt.Errorf("read names offset: %w", err)
	}

	if h.FormatVersion >= 3 {
		if err := binary.Read(r, binary.BigEndian, &h.OverallFlags); err != nil {
			return nil, fmt.Errorf("read overall flags: %w", err)
		}
	}

	if h.FormatVersion > 3 {
		return nil, fmt.Errorf("unsupported format version %d", &h.FormatVersion)
	}

	return &h, nil
}
