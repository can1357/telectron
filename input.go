package main

import (
	"os"
	"time"
)

// Terminal input: a single reader goroutine owns all reads from the tty and
// demultiplexes three interleaved streams that arrive on the one fd:
//
//   - cursor-position reports (CSI ... R), the reply to the display loop's DSR
//     barrier, routed to terminal.cpr;
//   - SGR mouse reports (CSI < b ; x ; y M|m), mode 1006/1016;
//   - keystrokes (printable bytes, control bytes, and CSI/SS3 named keys).
//
// parseInput is a pure function over a byte buffer so the wire decoding is unit
// testable without a real terminal; inputReader wraps it with the I/O loop.

// mouseAction is the kind of pointer event a mouse report encodes.
type mouseAction uint8

const (
	mouseMove mouseAction = iota
	mouseDown
	mouseUp
	mouseWheel
)

// mouseInput is one decoded SGR mouse report. x,y are terminal coordinates:
// 1-based cells by default, or pixels when pixel mode (1016) is active.
type mouseInput struct {
	action           mouseAction
	button           int  // 0 left, 1 middle, 2 right
	held             bool // motion with a button down (drag)
	x, y             int
	wheel            int // -1 up, +1 down (mouseWheel only)
	shift, alt, ctrl bool
}

// keyInput is one decoded key: a printable rune (kindChar) or a named key.
type keyInput struct {
	r                rune   // valid when name == ""
	name             string // "Enter","Backspace","ArrowUp",... when set
	ctrl, alt, shift bool
}

func (k keyInput) named() bool { return k.name != "" }

// termEvent is exactly one of a mouse report or a key, tagged by which pointer
// is non-nil.
type termEvent struct {
	mouse *mouseInput
	key   *keyInput
}

// parseInput consumes complete tokens from buf, returning decoded events, the
// number of cursor-position reports seen, and how many bytes were consumed. An
// incomplete trailing escape sequence is left unconsumed for the next read.
func parseInput(buf []byte) (events []termEvent, cpr, consumed int) {
	i := 0
	for i < len(buf) {
		b := buf[i]
		if b != 0x1b {
			if ev, ok := decodeByte(b); ok {
				events = append(events, ev)
			}
			i++
			continue
		}
		// ESC: need at least one more byte to classify.
		if i+1 >= len(buf) {
			break // lone trailing ESC: defer (reader resolves via grace timeout)
		}
		switch buf[i+1] {
		case '[':
			n, evs, isCPR, ok := decodeCSI(buf[i:])
			if !ok {
				return events, cpr, i // incomplete CSI; keep for next read
			}
			if isCPR {
				cpr++
			}
			events = append(events, evs...)
			i += n
		case 'O':
			n, ev, ok := decodeSS3(buf[i:])
			if !ok {
				return events, cpr, i
			}
			if ev.key != nil {
				events = append(events, ev)
			}
			i += n
		default:
			// ESC + byte: Alt-modified key.
			if ev, ok := decodeByte(buf[i+1]); ok && ev.key != nil {
				ev.key.alt = true
				events = append(events, ev)
			}
			i += 2
		}
	}
	return events, cpr, i
}

// decodeByte maps a single non-ESC byte to a key event.
func decodeByte(b byte) (termEvent, bool) {
	switch {
	case b == 0x0d || b == 0x0a:
		return keyEvent(keyInput{name: "Enter"}), true
	case b == 0x7f || b == 0x08:
		return keyEvent(keyInput{name: "Backspace"}), true
	case b == 0x09:
		return keyEvent(keyInput{name: "Tab"}), true
	case b == 0x1b:
		return keyEvent(keyInput{name: "Escape"}), true
	case b >= 1 && b <= 26:
		// Ctrl-A..Ctrl-Z. Tab/Enter/Backspace handled above.
		return keyEvent(keyInput{r: rune('a' + b - 1), ctrl: true}), true
	case b >= 0x20 && b < 0x7f:
		return keyEvent(keyInput{r: rune(b)}), true
	}
	return termEvent{}, false
}

func keyEvent(k keyInput) termEvent { kk := k; return termEvent{key: &kk} }

// decodeCSI decodes one CSI sequence beginning at s[0]==ESC, s[1]=='['. It
// returns bytes consumed, any events, whether it was a cursor-position report,
// and ok=false if the sequence is incomplete.
func decodeCSI(s []byte) (n int, events []termEvent, isCPR bool, ok bool) {
	// Find the final byte (0x40-0x7e) terminating the sequence.
	end := -1
	for j := 2; j < len(s); j++ {
		if s[j] >= 0x40 && s[j] <= 0x7e {
			end = j
			break
		}
	}
	if end < 0 {
		return 0, nil, false, false
	}
	final := s[end]
	body := s[2:end] // parameter+intermediate bytes
	n = end + 1

	if len(body) > 0 && body[0] == '<' {
		if ev, ok := decodeMouse(body[1:], final); ok {
			return n, []termEvent{ev}, false, true
		}
		return n, nil, false, true
	}
	if final == 'R' {
		return n, nil, true, true // cursor-position report (DSR reply)
	}
	if k, ok := decodeNamedCSI(body, final); ok {
		return n, []termEvent{keyEvent(k)}, false, true
	}
	return n, nil, false, true // recognized-but-ignored CSI
}

// decodeMouse decodes an SGR mouse body "b;x;y" with terminator 'M' (press) or
// 'm' (release).
func decodeMouse(body []byte, final byte) (termEvent, bool) {
	cb, x, y, ok := three(body)
	if !ok {
		return termEvent{}, false
	}
	m := mouseInput{
		x: x, y: y,
		shift: cb&4 != 0,
		alt:   cb&8 != 0,
		ctrl:  cb&16 != 0,
	}
	switch {
	case cb&64 != 0: // wheel
		m.action = mouseWheel
		if cb&3 == 0 {
			m.wheel = -1 // up
		} else {
			m.wheel = 1 // down
		}
	case cb&32 != 0: // motion
		m.action = mouseMove
		m.button = cb & 3
		m.held = cb&3 != 3
	default:
		m.button = cb & 3
		if final == 'M' {
			m.action = mouseDown
		} else {
			m.action = mouseUp
		}
	}
	mm := m
	return termEvent{mouse: &mm}, true
}

// decodeNamedCSI maps a CSI body+final to a named key. body may carry a
// "1;<mod>" prefix for modified arrows/navigation keys.
func decodeNamedCSI(body []byte, final byte) (keyInput, bool) {
	mod := 0
	// "1;5C" style: split params; the 2nd param is the modifier code.
	if i := indexByte(body, ';'); i >= 0 {
		if m, ok := atoiBytes(body[i+1:]); ok {
			mod = m
		}
		body = body[:i]
	}
	k := keyInput{}
	applyMod(&k, mod)
	switch final {
	case 'A':
		k.name = "ArrowUp"
	case 'B':
		k.name = "ArrowDown"
	case 'C':
		k.name = "ArrowRight"
	case 'D':
		k.name = "ArrowLeft"
	case 'H':
		k.name = "Home"
	case 'F':
		k.name = "End"
	case '~':
		num, _ := atoiBytes(body)
		switch num {
		case 1, 7:
			k.name = "Home"
		case 2:
			k.name = "Insert"
		case 3:
			k.name = "Delete"
		case 4, 8:
			k.name = "End"
		case 5:
			k.name = "PageUp"
		case 6:
			k.name = "PageDown"
		default:
			return keyInput{}, false
		}
	default:
		return keyInput{}, false
	}
	return k, true
}

// decodeSS3 decodes an SS3 sequence (ESC O x), used for some function/cursor
// keys in application mode.
func decodeSS3(s []byte) (n int, ev termEvent, ok bool) {
	if len(s) < 3 {
		return 0, termEvent{}, false
	}
	k := keyInput{}
	switch s[2] {
	case 'A':
		k.name = "ArrowUp"
	case 'B':
		k.name = "ArrowDown"
	case 'C':
		k.name = "ArrowRight"
	case 'D':
		k.name = "ArrowLeft"
	case 'H':
		k.name = "Home"
	case 'F':
		k.name = "End"
	default:
		return 3, termEvent{}, true // consume but ignore
	}
	return 3, keyEvent(k), true
}

// applyMod sets modifier flags from the SGR/CSI modifier code (1 + bitmask:
// 1 shift, 2 alt, 4 ctrl).
func applyMod(k *keyInput, mod int) {
	if mod <= 1 {
		return
	}
	m := mod - 1
	k.shift = m&1 != 0
	k.alt = m&2 != 0
	k.ctrl = m&4 != 0
}

// three parses "a;b;c" of unsigned integers.
func three(b []byte) (x, y, z int, ok bool) {
	i := indexByte(b, ';')
	if i < 0 {
		return
	}
	j := indexByte(b[i+1:], ';')
	if j < 0 {
		return
	}
	j += i + 1
	var o1, o2, o3 bool
	x, o1 = atoiBytes(b[:i])
	y, o2 = atoiBytes(b[i+1 : j])
	z, o3 = atoiBytes(b[j+1:])
	return x, y, z, o1 && o2 && o3
}

func indexByte(b []byte, c byte) int {
	for i, x := range b {
		if x == c {
			return i
		}
	}
	return -1
}

// atoiBytes parses an unsigned decimal integer, ok=false on empty/non-digit.
func atoiBytes(b []byte) (int, bool) {
	if len(b) == 0 {
		return 0, false
	}
	n := 0
	for _, c := range b {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}

// inputReader runs the single tty read loop, routing CPRs to the terminal's
// barrier channel and key/mouse events to its events channel.
type inputReader struct {
	t           *terminal
	events      chan termEvent
	closed      chan struct{}
	buf         []byte
	pendingMove termEvent // coalesced latest mouse motion, awaiting room
}

func newInputReader(t *terminal) *inputReader {
	return &inputReader{
		t:      t,
		events: make(chan termEvent, 512),
		closed: make(chan struct{}),
	}
}

// run reads until the tty closes. Returns when the fd errors (e.g. terminal
// closed at shutdown). A lone trailing ESC is resolved to Escape after a short
// grace window so the Escape key works without swallowing real sequences.
func (r *inputReader) run() {
	defer close(r.events)
	tmp := make([]byte, 4096)
	deadlineSet := false
	for {
		select {
		case <-r.closed:
			return
		default:
		}
		if len(r.buf) == 1 && r.buf[0] == 0x1b {
			_ = r.t.f.SetReadDeadline(time.Now().Add(30 * time.Millisecond))
			deadlineSet = true
		} else if deadlineSet {
			_ = r.t.f.SetReadDeadline(time.Time{})
			deadlineSet = false
		}
		n, err := r.t.f.Read(tmp)
		if n > 0 {
			r.buf = append(r.buf, tmp[:n]...)
			evs, cpr, consumed := parseInput(r.buf)
			r.buf = append(r.buf[:0], r.buf[consumed:]...)
			for k := 0; k < cpr; k++ {
				select {
				case r.t.cpr <- struct{}{}:
				default:
				}
			}
			r.deliver(evs)
		}
		if err != nil {
			if os.IsTimeout(err) {
				if len(r.buf) == 1 && r.buf[0] == 0x1b {
					r.buf = r.buf[:0]
					r.send(keyEvent(keyInput{name: "Escape"}))
				}
				continue
			}
			return
		}
	}
}

// deliver routes a batch of parsed events to the consumer without ever blocking
// the tty reader. Mouse motion is coalesced to the latest position (older moves
// are redundant), and every send is non-blocking, so a flood of 1003 motion can
// never wedge the reader and starve barrier (CPR) routing — a stalled consumer
// drops redundant moves, not clicks/keys, which are low-volume.
func (r *inputReader) deliver(evs []termEvent) {
	for _, e := range evs {
		if e.mouse != nil && e.mouse.action == mouseMove {
			r.pendingMove = e // overwrite: only the newest hover/drag matters
			continue
		}
		r.flushMove() // keep "move then click" ordering
		r.send(e)
	}
	r.flushMove()
}

// flushMove enqueues the coalesced latest motion, if any and if room remains.
func (r *inputReader) flushMove() {
	if r.pendingMove.mouse == nil {
		return
	}
	if r.trySend(r.pendingMove) {
		r.pendingMove = termEvent{}
	}
}

// send delivers one event best-effort (non-blocking).
func (r *inputReader) send(e termEvent) { r.trySend(e) }

// trySend attempts a non-blocking enqueue; false if the channel is full or the
// reader is shutting down.
func (r *inputReader) trySend(e termEvent) bool {
	select {
	case r.events <- e:
		return true
	case <-r.closed:
		return false
	default:
		return false
	}
}

func (r *inputReader) stop() {
	select {
	case <-r.closed:
	default:
		close(r.closed)
	}
	_ = r.t.f.SetReadDeadline(time.Now()) // unblock a pending Read
}
