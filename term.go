package main

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// terminal owns the controlling tty and its geometry, and knows how to map
// pixels to character cells for Kitty graphics placement.
//
// We talk to /dev/tty directly rather than os.Stdout so the program works even
// when stdout is redirected, and so the Chromium pipe never collides with
// terminal control I/O.
//
// Reads and writes use the one *os.File concurrently: the display loop writes
// frames and the DSR barrier request, while the input reader (see input.go)
// reads keystrokes and mouse reports plus the barrier's cursor-position reply.
// os.File is safe for concurrent read/write; the two directions never overlap.
type terminal struct {
	f        *os.File // /dev/tty, or a fallback writer when not a tty
	isTTY    bool
	cols     int           // grid width in cells
	rows     int           // grid height in cells
	xpix     int           // text-area width in pixels
	ypix     int           // text-area height in pixels
	cellW    int           // pixels per cell, horizontal
	cellH    int           // pixels per cell, vertical
	oldState *unix.Termios // saved termios, for restore()
	cpr      chan struct{} // cursor-position reports, fed by the input reader
}

// fallback cell size for terminals that don't report pixel geometry.
const (
	fallbackCellW = 9
	fallbackCellH = 18
)

// isRemote reports whether we're attached over ssh, where the terminal shares
// neither memory nor a filesystem with us, so local media (shm/file) can't work.
func isRemote() bool {
	return os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_TTY") != ""
}

// openTerminal opens the controlling tty and queries its geometry. When no tty
// is available (sandboxed/redirected runs) it returns a terminal backed by the
// given writer with synthesized geometry so headless runs still work.
func openTerminal(headlessOut *os.File) (*terminal, error) {
	if os.Getenv("TELECTRON_HEADLESS") != "" {
		t := &terminal{f: headlessOut, isTTY: false}
		t.synthesize()
		return t, nil
	}
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		t := &terminal{f: headlessOut, isTTY: false}
		t.synthesize()
		return t, nil
	}
	t := &terminal{f: f, isTTY: true, cpr: make(chan struct{}, 1)}
	if err := t.querySize(); err != nil {
		t.synthesize()
	}
	return t, nil
}

// querySize fills geometry from the kernel via TIOCGWINSZ.
func (t *terminal) querySize() error {
	ws, err := unix.IoctlGetWinsize(int(t.f.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return err
	}
	t.cols, t.rows = int(ws.Col), int(ws.Row)
	t.xpix, t.ypix = int(ws.Xpixel), int(ws.Ypixel)
	if t.cols == 0 || t.rows == 0 {
		return fmt.Errorf("terminal reported zero grid size")
	}
	if t.xpix == 0 || t.ypix == 0 {
		t.cellW, t.cellH = fallbackCellW, fallbackCellH
		t.xpix, t.ypix = t.cols*t.cellW, t.rows*t.cellH
	} else {
		t.cellW, t.cellH = t.xpix/t.cols, t.ypix/t.rows
	}
	return nil
}

// synthesize provides a usable default geometry for headless runs.
func (t *terminal) synthesize() {
	if t.cols == 0 {
		t.cols, t.rows = 200, 50
	}
	t.cellW, t.cellH = fallbackCellW, fallbackCellH
	t.xpix, t.ypix = t.cols*t.cellW, t.rows*t.cellH
}

func (t *terminal) write(b []byte) (int, error)    { return t.f.Write(b) }
func (t *terminal) writeStr(s string) (int, error) { return t.f.WriteString(s) }

const (
	hideCursor = "\x1b[?25l"
	showCursor = "\x1b[?25h"
	cursorHome = "\x1b[H"
	clearAll   = "\x1b[2J"
	altScreen  = "\x1b[?1049h"
	mainScreen = "\x1b[?1049l"

	// Mouse tracking: any-motion reporting (1003) with SGR extended coordinates
	// (1006). 1016 upgrades SGR coordinates from cells to pixels.
	mouseOn       = "\x1b[?1003h\x1b[?1006h"
	mouseOff      = "\x1b[?1006l\x1b[?1003l"
	mousePixelOn  = "\x1b[?1016h"
	mousePixelOff = "\x1b[?1016l"
)

// statusAt writes s at the given 1-based row, then returns the cursor home,
// without disturbing the saved drawing origin.
func (t *terminal) statusAt(row int, s string) {
	fmt.Fprintf(t.f, "\x1b[%d;1H\x1b[2K%s%s", row, s, cursorHome)
}

// enableMouse turns on mouse reporting; pixel adds cell-to-pixel coordinate
// precision (mode 1016) where the terminal supports it.
func (t *terminal) enableMouse(pixel bool) {
	if !t.isTTY {
		return
	}
	t.writeStr(mouseOn)
	if pixel {
		t.writeStr(mousePixelOn)
	}
}

// disableMouse turns off all mouse reporting modes we set.
func (t *terminal) disableMouse() {
	if !t.isTTY {
		return
	}
	t.writeStr(mousePixelOff)
	t.writeStr(mouseOff)
}

// detectPixelMouse reports whether the terminal supports SGR pixel mouse
// reporting (mode 1016). Without it, mouse reports are cell-granular, so clicks
// land at the cell center — up to half a cell from the real pointer. It enables
// 1016, asks DECRQM whether the mode is recognized, then disables it again
// (enableMouse sets the final state). It runs before the input reader starts, so
// the reply is read synchronously here. Returns false (use cell mode) if the
// terminal does not answer or does not recognize the mode.
func (t *terminal) detectPixelMouse() bool {
	if !t.isTTY {
		return false
	}
	t.writeStr(mousePixelOn + "\x1b[?1016$p") // enable, then request its status
	ok := t.awaitDECRPM(250 * time.Millisecond)
	t.writeStr(mousePixelOff)
	return ok
}

// awaitDECRPM reads the DECRPM reply to our mode-1016 query synchronously and
// reports whether the mode is recognized (Pm != 0). It waits for the exact
// CSI ? 1016 ; Pm $ y sequence rather than the first 'y' byte, so a stray
// keystroke arriving during startup cannot cause a false negative. Bounded by a
// read deadline so an unsupported terminal that stays silent falls back to cell
// mode.
func (t *terminal) awaitDECRPM(timeout time.Duration) bool {
	buf := make([]byte, 64)
	var acc []byte
	_ = t.f.SetReadDeadline(time.Now().Add(timeout))
	defer t.f.SetReadDeadline(time.Time{})
	for {
		n, err := t.f.Read(buf)
		if n > 0 {
			acc = append(acc, buf[:n]...)
			if v, done := scanDECRPM1016(acc); done {
				return v
			}
		}
		if err != nil {
			return false
		}
	}
}

// scanDECRPM1016 looks for the mode-1016 DECRPM reply (ESC [ ? 1016 ; Pm $ y) in
// b. It returns (recognized, true) once the full reply is present, or
// (false, false) to signal "keep reading" while it may still be arriving.
func scanDECRPM1016(b []byte) (recognized, done bool) {
	prefix := []byte("\x1b[?1016;")
	i := bytes.Index(b, prefix)
	if i < 0 {
		return false, false // not seen yet; keep reading
	}
	j := i + len(prefix)
	k := j
	for k < len(b) && b[k] >= '0' && b[k] <= '9' {
		k++
	}
	if k == j {
		return false, k < len(b) // non-digit where a value was expected => malformed, stop
	}
	if k+1 >= len(b) {
		return false, false // wait for the "$y" terminator
	}
	if b[k] == '$' && b[k+1] == 'y' {
		v, _ := atoiBytes(b[j:k])
		// 1=set, 2=reset, 3=permanently set => usable; 0=not recognized,
		// 4=permanently reset => cannot enable.
		return v >= 1 && v <= 3, true
	}
	return false, true // unexpected terminator => treat as unsupported
}

// makeRaw puts the tty into a cbreak-style mode: non-canonical and no-echo so
// input and the barrier's cursor-position reply are read byte-for-byte without
// the kernel line-buffering or echoing them, and so keystrokes don't scribble
// over the video. ISIG is deliberately kept enabled so Ctrl-C always raises
// SIGINT and tears the session down cleanly no matter what a goroutine is doing
// (Ctrl-C/Ctrl-Z/Ctrl-\ are therefore not delivered to the page). No-op when
// not a tty.
func (t *terminal) makeRaw() error {
	if !t.isTTY {
		return nil
	}
	old, err := unix.IoctlGetTermios(int(t.f.Fd()), unix.TIOCGETA)
	if err != nil {
		return err
	}
	t.oldState = old
	raw := *old
	raw.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP |
		unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	raw.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.IEXTEN
	raw.Cflag &^= unix.CSIZE | unix.PARENB
	raw.Cflag |= unix.CS8
	raw.Cc[unix.VMIN] = 1  // block for at least one byte; the poller wakes us
	raw.Cc[unix.VTIME] = 0 // no inter-byte timeout; reads are poller-driven
	return unix.IoctlSetTermios(int(t.f.Fd()), unix.TIOCSETA, &raw)
}

// restore returns the tty to its saved state. Safe to call if makeRaw wasn't.
func (t *terminal) restore() {
	if t.oldState != nil {
		_ = unix.IoctlSetTermios(int(t.f.Fd()), unix.TIOCSETA, t.oldState)
		t.oldState = nil
	}
}

// barrier issues a Device Status Report (cursor position) and blocks until the
// input reader reports the terminal's reply. Terminals process their input
// stream strictly in order, so the reply is emitted only after the parser has
// consumed every byte written before the request — a true "the terminal has
// ingested all prior frames" synchronization point. Returns false if not a tty
// or no reply arrives within the deadline.
//
// The reply itself is parsed by the input reader (input.go), which routes the
// cursor-position report to t.cpr; this avoids two goroutines reading the tty.
func (t *terminal) barrier() bool {
	if !t.isTTY || t.cpr == nil {
		return false
	}
	// Drop any stale signal so we wait for *this* request's reply.
	select {
	case <-t.cpr:
	default:
	}
	if _, err := t.f.WriteString("\x1b[6n"); err != nil {
		return false
	}
	select {
	case <-t.cpr:
		return true
	case <-time.After(500 * time.Millisecond):
		return false
	}
}

func (t *terminal) close() {
	if t.isTTY {
		_ = t.f.Close()
	}
}
