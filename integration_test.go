package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

// TestEndToEndSHM drives the whole pixel path headlessly: a real Chromium frame
// is encoded through the shm transmitter, and the bytes a terminal would read
// back out of the shared-memory slot are verified to match the decoded frame
// exactly — the same fast path the PoC measured at 1100 FPS.
func TestEndToEndSHM(t *testing.T) {
	if !shmAvailable {
		t.Skip("shm unavailable on this build")
	}
	b := testBrowser(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	const w, h = 200, 120
	p, err := newPage(ctx, b, dataURL("rgb(0,128,255)"), w, h, 90)
	if err != nil {
		t.Fatal(err)
	}
	defer p.close()

	var fr *rgbFrame
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		f := p.next(500 * time.Millisecond)
		if f == nil {
			continue
		}
		if r, g, bl := centerRGB(f); bl > 200 && g > 80 && r < 80 {
			fr = f
			break
		}
	}
	if fr == nil {
		t.Fatal("never received the expected blue frame")
	}
	if fr.w != w || fr.h != h {
		t.Fatalf("frame %dx%d, want %dx%d", fr.w, fr.h, w, h)
	}

	spec := frameSpec{txW: fr.w, txH: fr.h, cols: 50, rows: 20, format: formatRGB}
	tx, err := newShmTx(spec, 3)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.close()

	esc, err := tx.encode(0, fr.pix)
	if err != nil {
		t.Fatal(err)
	}
	got := string(esc)
	for _, want := range []string{apcStart, "a=T", "f=24", "s=200", "v=120", "t=s", "S=72000"} {
		if !strings.Contains(got, want) {
			t.Fatalf("escape missing %q in: %q", want, got)
		}
	}

	// Slot 0 holds the payload; a terminal reads exactly len(pix) bytes from it.
	shmBytes, err := shmRead(tx.names[0], len(fr.pix))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(shmBytes, fr.pix) {
		t.Fatalf("shm bytes differ from frame (got %d bytes, want %d)", len(shmBytes), len(fr.pix))
	}
}

// TestPointerMap checks the terminal-to-page coordinate mapping in both cell and
// pixel modes, including the out-of-page (status row) rejection.
func TestPointerMap(t *testing.T) {
	cell := pointerMap{cols: 20, imgRows: 10, screenW: 200, screenH: 180, vw: 200, vh: 180}
	cases := []struct {
		x, y   int
		bx, by float64
		ok     bool
		name   string
	}{
		{1, 1, 5, 9, true, "top-left cell center"},
		{20, 10, 195, 171, true, "bottom-right cell center"},
		{5, 11, 0, 0, false, "status row rejected"},
		{21, 5, 0, 0, false, "beyond last column rejected"},
	}
	for _, c := range cases {
		bx, by, ok := cell.toBrowser(c.x, c.y)
		if ok != c.ok || (ok && (bx != c.bx || by != c.by)) {
			t.Errorf("%s: cell.toBrowser(%d,%d)=(%v,%v,%v) want (%v,%v,%v)",
				c.name, c.x, c.y, bx, by, ok, c.bx, c.by, c.ok)
		}
	}

	pix := pointerMap{screenW: 200, screenH: 180, vw: 200, vh: 180, pixel: true}
	if bx, by, ok := pix.toBrowser(50, 60); !ok || bx != 49 || by != 59 {
		t.Errorf("pixel.toBrowser(50,60)=(%v,%v,%v) want (49,59,true)", bx, by, ok)
	}
	if _, _, ok := pix.toBrowser(10, 200); ok {
		t.Error("pixel point below the image should be rejected")
	}
}

// TestDECRPMParse checks the mode-1016 DECRPM reply parser: it accepts only the
// exact CSI ?1016;Pm$y sequence, treats Pm in {1,2,3} as supported, waits when
// the reply is still arriving, and is not fooled by stray leading bytes.
func TestDECRPMParse(t *testing.T) {
	cases := []struct {
		in               string
		recognized, done bool
	}{
		{"\x1b[?1016;1$y", true, true},   // set
		{"\x1b[?1016;2$y", true, true},   // reset (still supported)
		{"\x1b[?1016;3$y", true, true},   // permanently set
		{"\x1b[?1016;0$y", false, true},  // not recognized
		{"\x1b[?1016;4$y", false, true},  // permanently reset => unusable
		{"\x1b[?1016;", false, false},    // incomplete: keep reading
		{"\x1b[?1016;1", false, false},   // no terminator yet
		{"\x1b[?1016;1$z", false, true},  // wrong terminator => unsupported
		{"random noise", false, false},   // prefix not present
		{"xy\x1b[?1016;1$y", true, true}, // stray leading bytes ignored
	}
	for _, c := range cases {
		r, d := scanDECRPM1016([]byte(c.in))
		if r != c.recognized || d != c.done {
			t.Errorf("scanDECRPM1016(%q) = (%v,%v), want (%v,%v)", c.in, r, d, c.recognized, c.done)
		}
	}
}
