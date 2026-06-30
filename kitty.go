package main

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// Kitty graphics protocol primitives.
//
// A frame is one image transmitted-and-displayed in a single action (a=T) under
// a fixed image+placement id so each frame replaces the previous in place (the
// spec guarantees same i,p replacement is flicker-free). C=1 pins the cursor so
// the grid never scrolls; q=2 suppresses per-frame responses.
//
// The payload reaches the terminal through one of four media (the t key):
//
//	direct (d)      base64 pixel data inline in the escape, chunked <=4096 bytes.
//	file (f)        a regular file; escape carries the base64 path. Not deleted.
//	temporary (t)   a temp file; terminal deletes it after reading.
//	shared mem (s)  a POSIX shm object; terminal reads then unlinks it.
//
// Reference: https://sw.kovidgoyal.net/kitty/graphics-protocol/
const (
	apcStart  = "\x1b_G"
	apcEnd    = "\x1b\\"
	imageID   = 1
	placeID   = 1
	chunkSize = 4096 // max base64 bytes per direct escape, per spec
)

// pixelFormat is the raw payload layout (the f key).
type pixelFormat int

const (
	formatRGB  pixelFormat = 24 // 3 bytes/pixel
	formatRGBA pixelFormat = 32 // 4 bytes/pixel
)

func (f pixelFormat) bpp() int {
	if f == formatRGBA {
		return 4
	}
	return 3
}

func (f pixelFormat) label() string {
	if f == formatRGBA {
		return "rgba"
	}
	return "rgb"
}

// medium is the transmission medium (the t key).
type medium int

const (
	mediumDirect medium = iota
	mediumFile
	mediumTmpFile
	mediumSHM
)

func (m medium) tKey() string {
	switch m {
	case mediumFile:
		return "f"
	case mediumTmpFile:
		return "t"
	case mediumSHM:
		return "s"
	default:
		return "d"
	}
}

func (m medium) String() string {
	switch m {
	case mediumFile:
		return "file"
	case mediumTmpFile:
		return "tmpfile"
	case mediumSHM:
		return "shm"
	default:
		return "direct"
	}
}

// parseMedium maps a flag value to a medium.
func parseMedium(s string) (medium, error) {
	switch s {
	case "auto", "":
		// Shared memory is fastest where the client and terminal share memory and
		// a filesystem: a local terminal with cgo. Fall back to inline stdio over
		// ssh (no shared fs/mem) or in a cgo-less build.
		if shmAvailable && !isRemote() {
			return mediumSHM, nil
		}
		return mediumDirect, nil
	case "direct", "d":
		return mediumDirect, nil
	case "file", "f":
		return mediumFile, nil
	case "tmpfile", "tmp", "t":
		return mediumTmpFile, nil
	case "shm", "s":
		return mediumSHM, nil
	default:
		return 0, fmt.Errorf("unknown medium %q (want auto|direct|file|tmpfile|shm)", s)
	}
}

// frameSpec holds the per-stream invariant geometry and encoding choices.
// txW/txH are the transmitted pixel dimensions; cols/rows are the on-screen cell
// box the terminal scales the image into.
type frameSpec struct {
	txW, txH   int
	cols, rows int
	format     pixelFormat
	compress   bool
}

// buildControl assembles the comma-separated control-data keys for one frame.
// payloadSize is the byte count of the (possibly compressed) data and is sent
// as S for the local media so the terminal reads exactly that many bytes.
func buildControl(s frameSpec, m medium, payloadSize int) string {
	var b strings.Builder
	b.Grow(80)
	b.WriteString("a=T,q=2,C=1,i=")
	b.WriteString(strconv.Itoa(imageID))
	b.WriteString(",p=")
	b.WriteString(strconv.Itoa(placeID))
	b.WriteString(",f=")
	b.WriteString(strconv.Itoa(int(s.format)))
	b.WriteString(",s=")
	b.WriteString(strconv.Itoa(s.txW))
	b.WriteString(",v=")
	b.WriteString(strconv.Itoa(s.txH))
	b.WriteString(",c=")
	b.WriteString(strconv.Itoa(s.cols))
	b.WriteString(",r=")
	b.WriteString(strconv.Itoa(s.rows))
	if m != mediumDirect {
		b.WriteString(",t=")
		b.WriteString(m.tKey())
		b.WriteString(",S=")
		b.WriteString(strconv.Itoa(payloadSize))
	}
	if s.compress {
		b.WriteString(",o=z")
	}
	return b.String()
}

// writeDirect emits the chunked base64 escapes for an inline (t=d) payload.
// b64 is the base64-encoded (already compressed if requested) pixel data.
func writeDirect(out *bytes.Buffer, control string, b64 []byte) {
	first := true
	for off := 0; off < len(b64); off += chunkSize {
		end := off + chunkSize
		if end > len(b64) {
			end = len(b64)
		}
		last := end == len(b64)
		out.WriteString(apcStart)
		if first {
			out.WriteString(control)
			out.WriteString(",m=")
			first = false
		} else {
			out.WriteString("m=")
		}
		if last {
			out.WriteByte('0')
		} else {
			out.WriteByte('1')
		}
		out.WriteByte(';')
		out.Write(b64[off:end])
		out.WriteString(apcEnd)
	}
}

// writeLocal emits the single escape for a local medium (file/tmp/shm): control
// keys plus the base64-encoded path or shm name as the payload.
func writeLocal(out *bytes.Buffer, control string, b64name []byte) {
	out.WriteString(apcStart)
	out.WriteString(control)
	out.WriteByte(';')
	out.Write(b64name)
	out.WriteString(apcEnd)
}

// cleanupImage frees only this app's frame image (id=imageID) and its
// placement, leaving any unrelated terminal graphics untouched.
func cleanupImage() string {
	return apcStart + "a=d,d=I,i=" + strconv.Itoa(imageID) + ",q=2" + apcEnd
}
