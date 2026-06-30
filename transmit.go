package main

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// transmitter turns a raw frame into the escape-code stream to write to the
// terminal, delivering the pixel bytes through a specific medium. Pooled media
// (file/tmp/shm) reuse a fixed set of slots; the caller MUST issue a barrier
// every poolSize() frames so the terminal has read slot N before it is reused.
type transmitter interface {
	// encode turns frame seq's raw pixels into the escape stream to write.
	encode(seq int, raw []byte) ([]byte, error)
	// poolSize is the slot count for pooled media, or 0 for unpooled (direct).
	poolSize() int
	// reshape updates the per-frame geometry (transmit size, cell box) without
	// disturbing the slot pool — used on a terminal resize. The caller MUST have
	// flushed the pool with a barrier first, since slots are reused at the new
	// size from the next frame.
	reshape(spec frameSpec)
	// close releases medium resources (temp files / shm objects).
	close()
}

// codec applies the optional zlib compression shared by all media.
type codec struct {
	compress bool
	zbuf     bytes.Buffer
	zw       *zlib.Writer
}

func newCodec(compress bool) *codec {
	c := &codec{compress: compress}
	if compress {
		c.zw, _ = zlib.NewWriterLevel(&c.zbuf, zlib.BestSpeed)
	}
	return c
}

// payload returns the bytes to deliver: raw, or a zlib stream over raw. The
// returned slice aliases internal scratch and is valid until the next call.
func (c *codec) payload(raw []byte) []byte {
	if !c.compress {
		return raw
	}
	c.zbuf.Reset()
	c.zw.Reset(&c.zbuf)
	_, _ = c.zw.Write(raw)
	_ = c.zw.Close()
	return c.zbuf.Bytes()
}

// newTransmitter builds the transmitter for a medium. For file/tmp media it
// creates a temp directory; for shm it allocates slot names.
func newTransmitter(m medium, spec frameSpec, pool int) (transmitter, error) {
	if pool < 1 {
		pool = 1
	}
	switch m {
	case mediumDirect:
		return &directTx{spec: spec, codec: newCodec(spec.compress)}, nil
	case mediumFile, mediumTmpFile:
		return newFileTx(m, spec, pool)
	case mediumSHM:
		if !shmAvailable {
			return nil, fmt.Errorf("shared-memory medium unavailable (build without cgo or non-darwin)")
		}
		return newShmTx(spec, pool)
	default:
		return nil, fmt.Errorf("unsupported medium")
	}
}

// directTx sends base64 pixel data inline (t=d), chunked per the spec.
type directTx struct {
	spec  frameSpec
	codec *codec
	b64   []byte
	out   bytes.Buffer
}

func (d *directTx) poolSize() int          { return 0 }
func (d *directTx) reshape(spec frameSpec) { d.spec = spec }
func (d *directTx) close()                 {}

func (d *directTx) encode(_ int, raw []byte) ([]byte, error) {
	p := d.codec.payload(raw)
	need := base64.StdEncoding.EncodedLen(len(p))
	if cap(d.b64) < need {
		d.b64 = make([]byte, need)
	}
	b64 := d.b64[:need]
	base64.StdEncoding.Encode(b64, p)
	d.out.Reset()
	writeDirect(&d.out, buildControl(d.spec, mediumDirect, 0), b64)
	return d.out.Bytes(), nil
}

// fileTx serves both the file (f) and temporary-file (t) media. It keeps a pool
// of files in a temp directory whose name contains "tty-graphics-protocol" so
// the path passes the terminal's temp-file safety check. The escape carries the
// base64 absolute (symlink-resolved) path so it matches the terminal's realpath.
type fileTx struct {
	m        medium
	spec     frameSpec
	codec    *codec
	dir      string
	paths    []string // absolute slot paths
	b64paths [][]byte // base64 of each slot path
	files    []*os.File
	out      bytes.Buffer
}

func newFileTx(m medium, spec frameSpec, pool int) (*fileTx, error) {
	dir, err := os.MkdirTemp("", "tty-graphics-protocol-te-")
	if err != nil {
		return nil, err
	}
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real // macOS $TMPDIR is a symlink; resolve so it matches realpath
	}
	t := &fileTx{m: m, spec: spec, codec: newCodec(spec.compress), dir: dir}
	for k := 0; k < pool; k++ {
		p := filepath.Join(dir, "frame-"+strconv.Itoa(k))
		t.paths = append(t.paths, p)
		enc := make([]byte, base64.StdEncoding.EncodedLen(len(p)))
		base64.StdEncoding.Encode(enc, []byte(p))
		t.b64paths = append(t.b64paths, enc)
		// The reusable file medium keeps an open handle per slot; the temporary
		// medium recreates the file each frame because the terminal deletes it.
		if m == mediumFile {
			f, err := os.OpenFile(p, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o600)
			if err != nil {
				t.close()
				return nil, err
			}
			t.files = append(t.files, f)
		}
	}
	return t, nil
}

func (t *fileTx) poolSize() int          { return len(t.paths) }
func (t *fileTx) reshape(spec frameSpec) { t.spec = spec }

func (t *fileTx) close() {
	for _, f := range t.files {
		if f != nil {
			_ = f.Close()
		}
	}
	_ = os.RemoveAll(t.dir)
}

func (t *fileTx) encode(seq int, raw []byte) ([]byte, error) {
	slot := seq % len(t.paths)
	p := t.codec.payload(raw)
	if t.m == mediumFile {
		f := t.files[slot]
		if _, err := f.WriteAt(p, 0); err != nil {
			return nil, err
		}
		// Truncate to exactly len(p): with -z the payload length varies, and the
		// terminal reads S bytes / to EOF, so stale trailing bytes must go.
		if err := f.Truncate(int64(len(p))); err != nil {
			return nil, err
		}
	} else {
		// Temporary file: fresh each frame (the terminal unlinks it after read).
		if err := os.WriteFile(t.paths[slot], p, 0o600); err != nil {
			return nil, err
		}
	}
	t.out.Reset()
	writeLocal(&t.out, buildControl(t.spec, t.m, len(p)), t.b64paths[slot])
	return t.out.Bytes(), nil
}

// shmTx delivers frames via POSIX shared memory (t=s). Each frame (re)creates a
// slot's shm object, writes the payload, and lets the terminal unlink it after
// reading. Slot names are short to respect the darwin 31-char shm name limit.
type shmTx struct {
	spec    frameSpec
	codec   *codec
	names   []string
	b64name [][]byte
	out     bytes.Buffer
}

func newShmTx(spec frameSpec, pool int) (*shmTx, error) {
	t := &shmTx{spec: spec, codec: newCodec(spec.compress)}
	// POSIX shm names are system-wide; tag with the pid (base36) so concurrent
	// runs don't collide. Stays well under the darwin 31-char name limit.
	prefix := "/te" + strconv.FormatInt(int64(os.Getpid()), 36) + "-"
	for k := 0; k < pool; k++ {
		name := prefix + strconv.Itoa(k)
		t.names = append(t.names, name)
		enc := make([]byte, base64.StdEncoding.EncodedLen(len(name)))
		base64.StdEncoding.Encode(enc, []byte(name))
		t.b64name = append(t.b64name, enc)
		shmUnlink(name) // clear any stale object from a previous crashed run
	}
	return t, nil
}

func (t *shmTx) poolSize() int          { return len(t.names) }
func (t *shmTx) reshape(spec frameSpec) { t.spec = spec }

func (t *shmTx) close() {
	for _, n := range t.names {
		shmUnlink(n)
	}
}

func (t *shmTx) encode(seq int, raw []byte) ([]byte, error) {
	slot := seq % len(t.names)
	p := t.codec.payload(raw)
	if err := shmWrite(t.names[slot], p); err != nil {
		return nil, err
	}
	t.out.Reset()
	writeLocal(&t.out, buildControl(t.spec, mediumSHM, len(p)), t.b64name[slot])
	return t.out.Bytes(), nil
}
