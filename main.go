// Command telectron renders a live headless Chromium page inside the terminal
// and feeds terminal mouse/keyboard input back into the page — a real,
// interactive browser drawn with the Kitty graphics protocol.
//
// Pixels travel the proven fast path from the kittyvideo PoC: Chromium paints
// offscreen, frames arrive over the Chrome DevTools Protocol screencast, and
// each decoded frame is published to the terminal through POSIX shared memory
// (the escape carries only the shm name) under a fixed image+placement id so
// frames replace in place flicker-free. A DSR barrier every pool frames keeps
// the terminal from reading a slot we are overwriting.
//
// Input flows the other way: a single reader demultiplexes the tty into the
// barrier's cursor-position replies, SGR mouse reports, and keystrokes; mouse
// reports are mapped from cells/pixels into page coordinates and injected via
// CDP Input events. Mouse input is pixel-precise where the terminal supports SGR
// pixel reporting (mode 1016, auto-detected), and the view follows terminal
// resizes.
//
// The backend is the CDP JPEG screencast (Page.startScreencast): a genuinely
// interactive browser-in-terminal, bounded by Chrome's screencast rate (tens of
// fps), not the PoC's raw transmission ceiling.
//
// Usage:
//
//	telectron [flags] <url>
//
// Ctrl-C quits (the terminal state is always restored); Ctrl-Q, Ctrl-Z, and
// Ctrl-\ also quit cleanly.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

func main() {
	quality := flag.Int("q", 80, "screencast JPEG quality (1-100)")
	pool := flag.Int("pool", 4, "shm slot pool size (barrier every pool frames)")
	mouse := flag.String("mouse", "auto", "mouse precision: auto|pixel|cell")
	mediumFlag := flag.String("medium", "auto", "transmission medium: auto|shm|file|tmpfile|direct")
	scale := flag.Float64("scale", 1.0, "render scale: viewport = scale * terminal pixel size")
	chromeFlag := flag.String("chrome", "", "path to Chrome/Chromium (default: auto-detect)")
	profileFlag := flag.String("profile", "", "persistent Chrome user-data dir (keeps logins across runs; default: ephemeral)")
	flag.Parse()

	url := flag.Arg(0)
	if url == "" {
		url = "https://news.ycombinator.com"
	}
	url = normalizeURL(url)

	if err := run(runOpts{
		url: url, quality: *quality, pool: *pool, mouseMode: *mouse,
		medium: *mediumFlag, scale: *scale, chromePath: *chromeFlag, profileDir: *profileFlag,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "telectron:", err)
		os.Exit(1)
	}
}

// runOpts is the parsed configuration for one session.
type runOpts struct {
	url        string
	quality    int
	pool       int
	mouseMode  string // auto|pixel|cell
	medium     string
	scale      float64
	chromePath string
	profileDir string
}

// run sets up the terminal and browser, then drives the display and input loops
// until the user quits, restoring the terminal on every exit path.
func run(o runOpts) error {
	term, err := openTerminal(os.Stdout)
	if err != nil {
		return err
	}
	defer term.close()

	imgRows, vw, vh := viewportFor(term, o.scale)
	if vw < 16 || vh < 16 {
		return fmt.Errorf("terminal too small (%dx%d px)", vw, vh)
	}

	chrome := o.chromePath
	if chrome == "" {
		if chrome, err = findChrome(); err != nil {
			return err
		}
	}
	dir := o.profileDir
	if dir == "" {
		dir, err = os.MkdirTemp("", "telectron-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir) // ephemeral: clean up
	} else if err = os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	b, err := launchBrowser(chrome, dir, vw, vh)
	if err != nil {
		return err
	}
	defer b.close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	p, err := newPage(ctx, b, o.url, vw, vh, o.quality)
	cancel()
	if err != nil {
		return err
	}
	defer p.close()

	med, err := parseMedium(o.medium)
	if err != nil {
		return err
	}
	if !term.isTTY {
		med = mediumDirect // no tty => no barrier => pooled media are unsafe
	}
	spec := frameSpec{txW: vw, txH: vh, cols: term.cols, rows: imgRows, format: formatRGB}
	tx, err := newTransmitter(med, spec, o.pool)
	if err != nil {
		return err
	}
	defer tx.close()

	done := make(chan struct{})
	shutdown := sync.OnceFunc(func() { close(done) })

	// Catch every quit signal BEFORE entering raw/alt-screen mode so a Ctrl-C
	// (SIGINT, still raised since ISIG is kept), Ctrl-\ (SIGQUIT), Ctrl-Z
	// (SIGTSTP), or SIGTERM is routed to an orderly shutdown instead of
	// default-terminating or suspending with the terminal stuck in raw/alt-screen
	// mode. Ctrl-Z quits cleanly rather than suspending — expected for a
	// fullscreen graphics TUI.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGTSTP)
	go func() {
		select {
		case <-sigCh:
			shutdown()
		case <-done:
		}
	}()

	if err := term.makeRaw(); err != nil {
		return fmt.Errorf("enter raw mode: %w", err)
	}
	restore := sync.OnceFunc(func() {
		term.disableMouse()
		term.writeStr(cleanupImage() + showCursor + mainScreen)
		term.restore()
	})
	defer restore()
	term.writeStr(altScreen + hideCursor + clearAll)

	pixel := resolvePixelMode(term, o.mouseMode)
	term.enableMouse(pixel)

	var pmPtr atomic.Pointer[pointerMap]
	pm := pointerMapFor(term, imgRows, vw, vh, pixel)
	pmPtr.Store(&pm)

	// Terminal resizes arrive as SIGWINCH; debounce to a single pending signal
	// the display loop applies between frames.
	resizeCh := make(chan struct{}, 1)
	winchCh := make(chan os.Signal, 1)
	signal.Notify(winchCh, syscall.SIGWINCH)
	go func() {
		for {
			select {
			case <-done:
				return
			case <-winchCh:
				select {
				case resizeCh <- struct{}{}:
				default:
				}
			}
		}
	}()

	var reader *inputReader
	if term.isTTY {
		reader = newInputReader(term)
		go reader.run()
		go inputLoop(reader, p, &pmPtr, shutdown, done)
	}

	sess := &session{
		term: term, page: p, tx: tx, spec: spec, o: o,
		med: med, pixel: pixel, imgRows: imgRows, pm: &pmPtr,
	}
	loopErr := sess.display(resizeCh, done)

	// Quit requested (or transmission failed): tear down input first so nothing
	// else writes to the tty, then restore and surface any error.
	if reader != nil {
		reader.stop()
	}
	restore()
	return loopErr
}

// viewportFor computes the page viewport: the image fills all but the reserved
// status row, at exactly that cell box's pixel size (so terminal and page
// coordinates line up 1:1), times the render scale.
func viewportFor(term *terminal, scale float64) (imgRows, vw, vh int) {
	imgRows = term.rows - 1
	if imgRows < 1 {
		imgRows = term.rows
	}
	vw = scaled(term.cols*term.cellW, scale)
	vh = scaled(imgRows*term.cellH, scale)
	return
}

// resolvePixelMode decides whether to inject pixel-precise mouse coordinates:
// forced by the -mouse flag, else auto-detected from the terminal's SGR-pixel
// (1016) support. Cell mode is the universal fallback (clicks land at the cell
// center, up to half a cell off).
func resolvePixelMode(term *terminal, mode string) bool {
	switch mode {
	case "pixel":
		return true
	case "cell":
		return false
	default:
		return term.detectPixelMouse()
	}
}

// session holds the mutable display state so a terminal resize can rebuild the
// transmitter and geometry in place.
type session struct {
	term      *terminal
	page      *page
	tx        transmitter
	spec      frameSpec
	o         runOpts
	med       medium
	pixel     bool
	imgRows   int
	pm        *atomic.Pointer[pointerMap]
	barrierOK bool // a DSR barrier has succeeded at least once
}

// display pulls decoded frames, transmits each through the medium, applies a
// pending resize between frames, and keeps the status row updated. Returns nil on
// quit, or an error when transmission cannot proceed safely.
func (s *session) display(resizeCh <-chan struct{}, done <-chan struct{}) error {
	winStart, winFrames := time.Now(), 0
	lastStatus := winStart
	seq := 0
	for {
		select {
		case <-done:
			return nil
		case <-resizeCh:
			if s.reconfigure(done) {
				seq = 0 // fresh pool sequence only after a flushed reshape
				winStart, winFrames, lastStatus = time.Now(), 0, time.Now()
			}
			continue
		default:
		}
		fr := s.page.next(250 * time.Millisecond)
		if fr == nil {
			continue
		}
		if fr.w != s.spec.txW || fr.h != s.spec.txH {
			fr = resizeRGB(fr, s.spec.txW, s.spec.txH)
		}
		esc, err := s.tx.encode(seq, fr.pix)
		if err != nil {
			return fmt.Errorf("transmit frame %d: %w", seq, err)
		}
		s.term.writeStr(cursorHome)
		if _, err := s.term.write(esc); err != nil {
			return nil // tty closed underneath us (shutdown): stop quietly
		}
		seq++
		winFrames++
		if every := s.tx.poolSize(); every > 0 && seq%every == 0 {
			if !s.syncBarrier(done) {
				select {
				case <-done:
					return nil
				default:
					return fmt.Errorf("terminal does not answer the DSR sync barrier; "+
						"%s medium cannot stay in sync — rerun with -medium direct", s.med)
				}
			}
		}
		if now := time.Now(); now.Sub(lastStatus) >= 250*time.Millisecond {
			fps := float64(winFrames) / now.Sub(winStart).Seconds()
			s.term.statusAt(s.term.rows, fmt.Sprintf(
				" telectron  %s  %dx%d  |  %5.1f fps  |  %s  (Ctrl-C quits) ",
				s.med, s.spec.txW, s.spec.txH, fps, truncate(s.o.url, 50)))
			winStart, winFrames, lastStatus = now, 0, now
		}
	}
}

// syncBarrier blocks until the terminal acknowledges the DSR barrier, so a pooled
// shm slot is never reused before the terminal has read it. A transient stall —
// e.g. while a window resize pauses the terminal — is retried until it answers
// again rather than tolerated by sending more frames (which would overwrite an
// unread slot). Returns false only on shutdown, or when the terminal has never
// answered at all (DSR unsupported), where pooled media cannot be used safely.
func (s *session) syncBarrier(done <-chan struct{}) bool {
	if s.term.barrier() {
		s.barrierOK = true
		return true
	}
	if !s.barrierOK {
		// Give DSR a few chances at startup before declaring it unsupported.
		for i := 0; i < 3; i++ {
			if stopping(done) {
				return false
			}
			if s.term.barrier() {
				s.barrierOK = true
				return true
			}
		}
		return false // DSR unsupported: caller fails closed
	}
	// Worked before: this is a transient stall. Wait it out instead of reusing an
	// unread slot. Ctrl-C still breaks out via done.
	for !stopping(done) {
		if s.term.barrier() {
			return true
		}
	}
	return false
}

// stopping reports whether shutdown has been requested.
func stopping(done <-chan struct{}) bool {
	select {
	case <-done:
		return true
	default:
		return false
	}
}

// reconfigure adapts to a terminal resize: re-read geometry, resize the page
// viewport, flush the slot pool, reshape the transmitter to the new frame size,
// re-place the image, and publish the new coordinate mapping. It returns true
// only when it fully applied a new geometry (after the barrier flush), so the
// caller resets the slot sequence only when that is safe. Every failure is
// tolerated (keep running at the old size) so a resize never crashes the session.
func (s *session) reconfigure(done <-chan struct{}) bool {
	if err := s.term.querySize(); err != nil {
		return false
	}
	imgRows, vw, vh := viewportFor(s.term, s.o.scale)
	if vw < 16 || vh < 16 {
		return false
	}
	if vw == s.spec.txW && vh == s.spec.txH && imgRows == s.imgRows {
		return false // spurious SIGWINCH, nothing changed
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err := s.page.setViewport(ctx, vw, vh)
	cancel()
	if err != nil {
		return false // keep streaming at the old size
	}
	// Flush the pool before reusing its slots at the new size: the terminal must
	// finish reading every slot written so far, or it would read a slot whose
	// pixels and declared dimensions no longer agree. Bail on shutdown.
	if s.tx.poolSize() > 0 && !s.syncBarrier(done) {
		return false
	}
	newSpec := frameSpec{txW: vw, txH: vh, cols: s.term.cols, rows: imgRows, format: formatRGB}
	s.tx.reshape(newSpec) // same slot pool, new geometry — no unlink/recreate race
	s.spec = newSpec
	s.imgRows = imgRows
	s.term.writeStr(cleanupImage() + clearAll) // drop the stale-size placement
	pm := pointerMapFor(s.term, imgRows, vw, vh, s.pixel)
	s.pm.Store(&pm)
	return true
}

// inputLoop maps each terminal event into a page action, intercepting the quit
// chord. It loads the current pointer map per event so a resize remaps cleanly.
func inputLoop(reader *inputReader, p *page, pm *atomic.Pointer[pointerMap], shutdown func(), done <-chan struct{}) {
	held := -1
	for {
		select {
		case <-done:
			return
		case ev, ok := <-reader.events:
			if !ok {
				return
			}
			if isQuit(ev) {
				shutdown()
				return
			}
			handleInput(ev, p, *pm.Load(), &held)
		}
	}
}

// isQuit reports the Ctrl-Q quit chord (Ctrl-C is handled as SIGINT).
func isQuit(ev termEvent) bool {
	return ev.key != nil && ev.key.ctrl && (ev.key.r == 'q')
}

// handleInput injects one terminal event into the page.
func handleInput(ev termEvent, p *page, pm pointerMap, held *int) {
	if ev.key != nil {
		p.sendKey(*ev.key)
		return
	}
	mi := ev.mouse
	bx, by, ok := pm.toBrowser(mi.x, mi.y)
	if !ok {
		return // event over the status row, outside the page
	}
	mods := cdpModifiers(mi)
	switch mi.action {
	case mouseDown:
		p.mousePressed(bx, by, mi.button, mods, 1)
		*held = mi.button
	case mouseUp:
		p.mouseReleased(bx, by, mi.button, mods, 1)
		*held = -1
	case mouseMove:
		mask := 0
		if mi.held {
			mask = mouseButtons[mi.button%3].mask
		}
		p.mouseMoved(bx, by, mask, mods)
	case mouseWheel:
		dy := float64(scrollStep)
		if mi.wheel < 0 {
			dy = -dy
		}
		p.mouseWheel(bx, by, 0, dy, mods)
	}
}

// scrollStep is the device-pixel scroll distance per wheel notch.
const scrollStep = 80

// cdpModifiers packs a mouse report's modifier flags into the CDP bitmask
// (Alt=1 Ctrl=2 Shift=8).
func cdpModifiers(mi *mouseInput) int {
	m := 0
	if mi.alt {
		m |= 1
	}
	if mi.ctrl {
		m |= 2
	}
	if mi.shift {
		m |= 8
	}
	return m
}

// pointerMap converts terminal mouse coordinates to page device pixels. The page
// occupies the top-left cols x imgRows cells; screenW/screenH are that box's size
// in terminal pixels and vw/vh the page viewport, so the mapping is a pure
// proportional scale that stays correct under any cell size or render scale.
type pointerMap struct {
	cols, imgRows    int // cell grid the image occupies
	screenW, screenH int // image extent in terminal pixels
	vw, vh           int // page viewport (transmit) pixels
	pixel            bool
}

// pointerMapFor builds the mapping for the current geometry.
func pointerMapFor(term *terminal, imgRows, vw, vh int, pixel bool) pointerMap {
	return pointerMap{
		cols: term.cols, imgRows: imgRows,
		screenW: term.cols * term.cellW, screenH: imgRows * term.cellH,
		vw: vw, vh: vh, pixel: pixel,
	}
}

// toBrowser maps a terminal coordinate to a page pixel, reporting ok=false when
// the point is outside the rendered page (e.g. the status row). Pixel mode (SGR
// 1016) uses the exact pixel within the image; cell mode uses the cell center.
func (m pointerMap) toBrowser(x, y int) (float64, float64, bool) {
	if x < 1 || y < 1 {
		return 0, 0, false
	}
	var fx, fy float64 // fraction across the image in [0,1)
	if m.pixel {
		if y-1 >= m.screenH {
			return 0, 0, false // below the image, in the status row
		}
		fx = float64(x-1) / float64(m.screenW)
		fy = float64(y-1) / float64(m.screenH)
	} else {
		if y > m.imgRows || x > m.cols {
			return 0, 0, false
		}
		fx = (float64(x) - 0.5) / float64(m.cols)
		fy = (float64(y) - 0.5) / float64(m.imgRows)
	}
	bx := clampf(fx*float64(m.vw), 0, float64(m.vw-1))
	by := clampf(fy*float64(m.vh), 0, float64(m.vh-1))
	return bx, by, true
}

func clampf(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// resizeRGB nearest-neighbor scales a frame to w x h. It is a safety net for the
// brief window during a resize where an in-flight frame's size differs from the
// transmit dimensions; the common path returns fr unchanged.
func resizeRGB(fr *rgbFrame, w, h int) *rgbFrame {
	if fr.w == w && fr.h == h {
		return fr
	}
	out := make([]byte, w*h*3)
	for y := 0; y < h; y++ {
		sy := y * fr.h / h
		for x := 0; x < w; x++ {
			sx := x * fr.w / w
			si := (sy*fr.w + sx) * 3
			di := (y*w + x) * 3
			out[di] = fr.pix[si]
			out[di+1] = fr.pix[si+1]
			out[di+2] = fr.pix[si+2]
		}
	}
	return &rgbFrame{w: w, h: h, pix: out}
}

// scaled multiplies a pixel dimension by the render scale, keeping it positive
// and even (JPEG chroma subsampling prefers even dimensions).
func scaled(v int, scale float64) int {
	if scale <= 0 {
		scale = 1
	}
	n := int(float64(v) * scale)
	if n < 1 {
		n = 1
	}
	return n &^ 1
}

// normalizeURL prepends https:// to a bare host and leaves schemes (http, file,
// data, ...) untouched.
func normalizeURL(u string) string {
	for i := 0; i < len(u); i++ {
		c := u[i]
		if c == ':' {
			return u // has a scheme
		}
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '+' || c == '-' || c == '.') {
			break
		}
	}
	return "https://" + u
}

// truncate shortens s to n runes with an ellipsis; n<=0 returns "".
func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
