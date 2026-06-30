package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"runtime"
	"strings"
	"time"
)

// page is one attached CDP page session: it drives navigation, owns the
// screencast frame stream (decoded to packed RGB), and injects input.
//
// Frames flow Chrome -> Page.screencastFrame (base64 JPEG) -> tiny event handler
// (ack + hand off) -> decoder goroutine (JPEG -> RGB) -> frames channel. The
// handler stays minimal so it never blocks the single CDP reader goroutine, and
// every frame is acked immediately or Chrome stops after a small backlog.
type page struct {
	b         *browser
	sessionID string
	targetID  string
	w, h      int

	rawCh  chan string    // base64 JPEG awaiting decode (drops oldest under load)
	frames chan *rgbFrame // newest decoded frame (cap 1, latest wins)
	stopCh chan struct{}
}

// rgbFrame is one decoded frame as packed 24-bit RGB, row-major, no padding.
type rgbFrame struct {
	w, h int
	pix  []byte // len == w*h*3
}

// screencastFrame is the payload of a Page.screencastFrame event. SessionID is a
// per-frame ack token (an int), distinct from the CDP page session id.
type screencastFrame struct {
	Data      string `json:"data"`
	SessionID int    `json:"sessionId"`
}

// newPage attaches a flattened session to the browser's existing active page,
// locks the viewport to w x h, navigates to url, and starts the JPEG screencast.
// It reuses the launch-time page rather than creating one: a headless
// background target reports "not attached to an active page" to startScreencast.
func newPage(ctx context.Context, b *browser, url string, w, h, quality int) (*page, error) {
	targetID, err := b.firstPageTarget(ctx)
	if err != nil {
		return nil, err
	}
	attached, err := b.send(ctx, "", "Target.attachToTarget", map[string]any{
		"targetId": targetID, "flatten": true,
	})
	if err != nil {
		return nil, fmt.Errorf("attachToTarget: %w", err)
	}
	var at struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(attached, &at); err != nil {
		return nil, err
	}

	p := &page{
		b: b, sessionID: at.SessionID, targetID: targetID, w: w, h: h,
		rawCh:  make(chan string, 1),
		frames: make(chan *rgbFrame, 1),
		stopCh: make(chan struct{}),
	}

	for _, m := range []string{"Page.enable", "Runtime.enable", "DOM.enable"} {
		if _, err := b.send(ctx, p.sessionID, m, nil); err != nil {
			return nil, fmt.Errorf("%s: %w", m, err)
		}
	}
	if b.stealth {
		if err := applyStealth(ctx, b, p.sessionID); err != nil {
			return nil, fmt.Errorf("applyStealth: %w", err)
		}
	}
	if err := p.setViewport(ctx, w, h); err != nil {
		return nil, err
	}
	// Keep the headless page treated as focused+visible so it paints
	// continuously and accepts input focus.
	_, _ = b.send(ctx, p.sessionID, "Emulation.setFocusEmulationEnabled",
		map[string]any{"enabled": true})

	p.b.on(p.sessionID, "Page.screencastFrame", p.onFrame)
	go p.decodeLoop()

	// Wait for the navigation to commit before screencasting: an uncommitted
	// page answers startScreencast with "not attached to an active page".
	loaded := make(chan struct{}, 1)
	signal := func(string, json.RawMessage) {
		select {
		case loaded <- struct{}{}:
		default:
		}
	}
	p.b.on(p.sessionID, "Page.loadEventFired", signal)
	p.b.on(p.sessionID, "Page.frameStoppedLoading", signal)
	if err := p.navigate(ctx, url); err != nil {
		return nil, err
	}
	select {
	case <-loaded:
	case <-time.After(8 * time.Second):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	_, _ = b.send(ctx, "", "Target.activateTarget", map[string]any{"targetId": p.targetID})
	_, _ = b.send(ctx, p.sessionID, "Page.bringToFront", nil)

	var scErr error
	for i := 0; i < 8; i++ {
		if _, scErr = b.send(ctx, p.sessionID, "Page.startScreencast", map[string]any{
			"format": "jpeg", "quality": quality,
			// Cap generously so the frame size always equals the viewport; a
			// later resize is then just an Emulation.setDeviceMetricsOverride,
			// with no screencast restart.
			"maxWidth": 8192, "maxHeight": 8192, "everyNthFrame": 1,
		}); scErr == nil {
			return p, nil
		}
		select {
		case <-time.After(300 * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return nil, fmt.Errorf("startScreencast: %w", scErr)
}

const stealthAcceptLanguage = "en-US,en;q=0.9"

// stealthScript is the full puppeteer-derived document-start stealth suite
// (webdriver, window.chrome, canvas/webgl/audio/font/screen/iframe/worker
// fingerprint normalization, locale, plugins, codecs, hardware, and native
// toString masking). It is injected on every new document before the page's own
// JS runs. Generated from the puppeteer stealth sources; see stealth.js.
//
//go:embed stealth.js
var stealthScript string

// applyStealth removes the high-signal automation fingerprints before the first
// navigation: clean UA + client hints on the wire (Network) and in JS
// (Emulation), plus the document-start patch suite on every new document/frame.
// The UA overrides are load-bearing for bot challenges, so their failure is
// fatal rather than swallowed.
func applyStealth(ctx context.Context, b *browser, sessionID string) error {
	override, err := buildUserAgentOverride(ctx, b)
	if err != nil {
		return err
	}
	if _, err := b.send(ctx, sessionID, "Page.addScriptToEvaluateOnNewDocument", map[string]any{
		"source": stealthScript,
	}); err != nil {
		return fmt.Errorf("inject stealth script: %w", err)
	}
	_, _ = b.send(ctx, sessionID, "Network.enable", nil)
	if _, err := b.send(ctx, sessionID, "Network.setUserAgentOverride", override); err != nil {
		return fmt.Errorf("network user-agent override: %w", err)
	}
	if _, err := b.send(ctx, sessionID, "Emulation.setUserAgentOverride", override); err != nil {
		return fmt.Errorf("emulation user-agent override: %w", err)
	}
	return nil
}

// buildUserAgentOverride returns a normal Chrome UA and matching client-hints
// metadata, derived from this browser's real version but with the Headless
// marker removed.
func buildUserAgentOverride(ctx context.Context, b *browser) (map[string]any, error) {
	res, err := b.send(ctx, "", "Browser.getVersion", nil)
	if err != nil {
		return nil, err
	}
	var v struct {
		UserAgent string `json:"userAgent"`
		Product   string `json:"product"`
	}
	if err := json.Unmarshal(res, &v); err != nil {
		return nil, err
	}
	userAgent := strings.Replace(v.UserAgent, "HeadlessChrome/", "Chrome/", 1)
	legacyVersion := "0"
	if i := strings.Index(userAgent, "Chrome/"); i >= 0 {
		rest := userAgent[i+len("Chrome/"):]
		if j := strings.IndexByte(rest, ' '); j >= 0 {
			rest = rest[:j]
		}
		if rest != "" {
			legacyVersion = rest
		}
	}
	fullVersion := legacyVersion
	major := strings.SplitN(legacyVersion, ".", 2)[0]
	platform := "Linux"
	platformFull := "Linux"
	platformVersion := ""
	jsPlatform := "Linux"
	switch {
	case strings.Contains(userAgent, "Mac OS X"):
		platform, platformFull, jsPlatform = "macOS", "macOS", "MacIntel"
		if m := between(userAgent, "Mac OS X ", ")"); m != "" {
			platformVersion = strings.ReplaceAll(m, "_", ".")
		}
	case strings.Contains(userAgent, "Windows NT "):
		platform, platformFull, jsPlatform = "Windows", "Windows", "Win32"
		platformVersion = between(userAgent, "Windows NT ", ";")
	case strings.Contains(userAgent, "Android "):
		platform, platformFull, jsPlatform = "Android", "Android", "Android"
		platformVersion = between(userAgent, "Android ", ";")
	}
	architecture, bitness := "", ""
	switch runtime.GOARCH {
	case "arm64":
		architecture, bitness = "arm", "64"
	case "amd64":
		architecture, bitness = "x86", "64"
	}
	brands := []map[string]string{
		{"brand": "Not A;Brand", "version": "99"},
		{"brand": "Chromium", "version": major},
		{"brand": "Google Chrome", "version": major},
	}
	fullVersionList := []map[string]string{
		{"brand": "Not A;Brand", "version": "99.0.0.0"},
		{"brand": "Chromium", "version": fullVersion},
		{"brand": "Google Chrome", "version": fullVersion},
	}
	return map[string]any{
		"userAgent":      userAgent,
		"platform":       jsPlatform,
		"acceptLanguage": stealthAcceptLanguage,
		"userAgentMetadata": map[string]any{
			"brands":          brands,
			"fullVersion":     fullVersion,
			"fullVersionList": fullVersionList,
			"platform":        platformFull,
			"platformVersion": platformVersion,
			"architecture":    architecture,
			"bitness":         bitness,
			"model":           "",
			"mobile":          platform == "Android",
		},
	}, nil
}

func between(s, left, right string) string {
	i := strings.Index(s, left)
	if i < 0 {
		return ""
	}
	i += len(left)
	j := strings.Index(s[i:], right)
	if j < 0 {
		return ""
	}
	return s[i : i+j]
}

// setViewport locks the rendering surface to exactly w x h device pixels.
func (p *page) setViewport(ctx context.Context, w, h int) error {
	p.w, p.h = w, h
	_, err := p.b.send(ctx, p.sessionID, "Emulation.setDeviceMetricsOverride", map[string]any{
		"width": w, "height": h, "deviceScaleFactor": 1, "mobile": false,
		"screenWidth": w, "screenHeight": h,
	})
	return err
}

// navigate loads url and returns once the navigation command is accepted (not
// awaiting full load; screencast frames stream as content paints).
func (p *page) navigate(ctx context.Context, url string) error {
	_, err := p.b.send(ctx, p.sessionID, "Page.navigate", map[string]any{"url": url})
	if err != nil {
		return fmt.Errorf("navigate %s: %w", url, err)
	}
	return nil
}

// onFrame is the screencast event handler. It MUST stay tiny: ack immediately so
// Chrome keeps streaming, then hand the base64 payload to the decoder. The raw
// queue is latest-wins — if the decoder is still busy, the older queued frame is
// evicted so we always decode the freshest paint rather than a stale backlog.
func (p *page) onFrame(_ string, params json.RawMessage) {
	var f screencastFrame
	if json.Unmarshal(params, &f) != nil {
		return
	}
	_ = p.b.notify(p.sessionID, "Page.screencastFrameAck",
		map[string]any{"sessionId": f.SessionID})
	for {
		select {
		case p.rawCh <- f.Data:
			return
		default:
			select {
			case <-p.rawCh: // evict the stale frame, retry with the fresh one
			default:
			}
		}
	}
}

// decodeLoop turns queued base64 JPEG into RGB frames, publishing only the
// newest to the frames channel (older undisplayed frames are discarded).
func (p *page) decodeLoop() {
	for {
		select {
		case <-p.stopCh:
			return
		case data := <-p.rawCh:
			fr, err := decodeJPEG(data)
			if err != nil {
				continue
			}
			p.publish(fr)
		}
	}
}

// publish puts fr in the frames channel, evicting any stale undisplayed frame so
// the display loop always renders the latest content.
func (p *page) publish(fr *rgbFrame) {
	for {
		select {
		case p.frames <- fr:
			return
		default:
			select {
			case <-p.frames: // drop the stale frame, retry with the fresh one
			default:
			}
		}
	}
}

// decodeJPEG base64-decodes and JPEG-decodes one screencast payload into packed
// RGB.
func decodeJPEG(b64 string) (*rgbFrame, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	img, err := jpeg.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	return toRGB(img), nil
}

// toRGB packs any decoded image into row-major 24-bit RGB. The JPEG decoder
// yields *image.YCbCr; that path avoids the per-pixel interface dispatch of the
// generic fallback.
func toRGB(img image.Image) *rgbFrame {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	pix := make([]byte, w*h*3)
	switch src := img.(type) {
	case *image.YCbCr:
		o := 0
		for y := 0; y < h; y++ {
			yy := b.Min.Y + y
			for x := 0; x < w; x++ {
				xx := b.Min.X + x
				yi := src.YOffset(xx, yy)
				ci := src.COffset(xx, yy)
				r, g, bl := yCbCrToRGB(src.Y[yi], src.Cb[ci], src.Cr[ci])
				pix[o], pix[o+1], pix[o+2] = r, g, bl
				o += 3
			}
		}
	case *image.RGBA:
		o := 0
		for y := 0; y < h; y++ {
			row := src.PixOffset(b.Min.X, b.Min.Y+y)
			for x := 0; x < w; x++ {
				pix[o] = src.Pix[row]
				pix[o+1] = src.Pix[row+1]
				pix[o+2] = src.Pix[row+2]
				o += 3
				row += 4
			}
		}
	default:
		o := 0
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
				pix[o] = byte(r >> 8)
				pix[o+1] = byte(g >> 8)
				pix[o+2] = byte(bl >> 8)
				o += 3
			}
		}
	}
	return &rgbFrame{w: w, h: h, pix: pix}
}

// yCbCrToRGB is the JFIF YCbCr->RGB conversion (matches color.YCbCrToRGB) with
// integer math, inlined to keep the hot decode loop allocation- and call-free.
func yCbCrToRGB(y, cb, cr uint8) (uint8, uint8, uint8) {
	yy := int32(y) * 0x10101
	cb1 := int32(cb) - 128
	cr1 := int32(cr) - 128
	r := (yy + 91881*cr1) >> 16
	g := (yy - 22554*cb1 - 46802*cr1) >> 16
	bl := (yy + 116130*cb1) >> 16
	return clamp8(r), clamp8(g), clamp8(bl)
}

func clamp8(v int32) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

// next returns the freshest decoded frame, or nil if none is ready or the page
// stopped. It blocks up to wait for the first/next frame.
func (p *page) next(wait time.Duration) *rgbFrame {
	select {
	case fr := <-p.frames:
		return fr
	case <-time.After(wait):
		return nil
	case <-p.stopCh:
		return nil
	}
}

// close stops the screencast and the decoder goroutine.
func (p *page) close() {
	select {
	case <-p.stopCh:
	default:
		close(p.stopCh)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	_, _ = p.b.send(ctx, p.sessionID, "Page.stopScreencast", nil)
	cancel()
}

// CDP input injection. All injection is fire-and-forget (notify): pointer/key
// events need no reply and must not add a round-trip of latency per event.
//
// CDP bitmasks: modifiers Alt=1 Ctrl=2 Meta=4 Shift=8; buttons left=1 right=2
// middle=4.

// mouseButtons maps a 0/1/2 button index to its name and mask bit.
var mouseButtons = [3]struct {
	name string
	mask int
}{
	{"left", 1}, {"middle", 4}, {"right", 2},
}

// mouseMoved reports a pointer move at (x,y); buttonsMask carries any held
// buttons so the page sees drags, not just hovers.
func (p *page) mouseMoved(x, y float64, buttonsMask, mods int) {
	_ = p.b.notify(p.sessionID, "Input.dispatchMouseEvent", map[string]any{
		"type": "mouseMoved", "x": x, "y": y,
		"buttons": buttonsMask, "modifiers": mods,
	})
}

// mousePressed / mouseReleased dispatch a button transition at (x,y).
func (p *page) mousePressed(x, y float64, button, mods, clickCount int) {
	b := mouseButtons[button%3]
	_ = p.b.notify(p.sessionID, "Input.dispatchMouseEvent", map[string]any{
		"type": "mousePressed", "x": x, "y": y,
		"button": b.name, "buttons": b.mask,
		"clickCount": clickCount, "modifiers": mods,
	})
}

func (p *page) mouseReleased(x, y float64, button, mods, clickCount int) {
	b := mouseButtons[button%3]
	_ = p.b.notify(p.sessionID, "Input.dispatchMouseEvent", map[string]any{
		"type": "mouseReleased", "x": x, "y": y,
		"button": b.name, "buttons": 0,
		"clickCount": clickCount, "modifiers": mods,
	})
}

// mouseWheel scrolls by (dx,dy) device pixels at (x,y).
func (p *page) mouseWheel(x, y, dx, dy float64, mods int) {
	_ = p.b.notify(p.sessionID, "Input.dispatchMouseEvent", map[string]any{
		"type": "mouseWheel", "x": x, "y": y,
		"deltaX": dx, "deltaY": dy, "modifiers": mods,
	})
}

// sendKey dispatches a key as a keyDown/keyUp pair. Printable keys carry text so
// they both insert characters and fire key handlers; named keys carry the DOM
// key/code and virtual key code so editing and navigation work.
func (p *page) sendKey(k keyInput) {
	km := mapKey(k)
	mods := keyModifiers(k)
	down := map[string]any{
		"modifiers": mods, "key": km.key, "code": km.code,
		"windowsVirtualKeyCode": km.vk, "nativeVirtualKeyCode": km.vk,
	}
	if km.text != "" {
		down["type"] = "keyDown"
		down["text"] = km.text
		down["unmodifiedText"] = km.text
	} else {
		down["type"] = "rawKeyDown"
	}
	_ = p.b.notify(p.sessionID, "Input.dispatchKeyEvent", down)

	up := map[string]any{
		"type": "keyUp", "modifiers": mods, "key": km.key, "code": km.code,
		"windowsVirtualKeyCode": km.vk, "nativeVirtualKeyCode": km.vk,
	}
	_ = p.b.notify(p.sessionID, "Input.dispatchKeyEvent", up)
}

// keyModifiers packs a keyInput's modifier flags into the CDP bitmask.
func keyModifiers(k keyInput) int {
	m := 0
	if k.alt {
		m |= 1
	}
	if k.ctrl {
		m |= 2
	}
	if k.shift {
		m |= 8
	}
	return m
}

// keyData is the CDP fields for one key: DOM key, DOM code, virtual key code,
// and the text it inserts (empty for non-text keys).
type keyData struct {
	key, code string
	vk        int
	text      string
}

// namedKeys maps our parser's named keys to their CDP key data.
var namedKeys = map[string]keyData{
	"Enter":      {"Enter", "Enter", 13, "\r"},
	"Backspace":  {"Backspace", "Backspace", 8, ""},
	"Tab":        {"Tab", "Tab", 9, ""},
	"Escape":     {"Escape", "Escape", 27, ""},
	"ArrowUp":    {"ArrowUp", "ArrowUp", 38, ""},
	"ArrowDown":  {"ArrowDown", "ArrowDown", 40, ""},
	"ArrowLeft":  {"ArrowLeft", "ArrowLeft", 37, ""},
	"ArrowRight": {"ArrowRight", "ArrowRight", 39, ""},
	"Home":       {"Home", "Home", 36, ""},
	"End":        {"End", "End", 35, ""},
	"PageUp":     {"PageUp", "PageUp", 33, ""},
	"PageDown":   {"PageDown", "PageDown", 34, ""},
	"Insert":     {"Insert", "Insert", 45, ""},
	"Delete":     {"Delete", "Delete", 46, ""},
}

// mapKey resolves a parsed key to its CDP fields. Modified printable keys
// (Ctrl/Alt held) drop the inserted text so shortcuts fire instead of typing.
func mapKey(k keyInput) keyData {
	if k.named() {
		if d, ok := namedKeys[k.name]; ok {
			return d
		}
		return keyData{key: k.name, code: k.name}
	}
	r := k.r
	d := keyData{key: string(r), code: codeForRune(r), vk: vkForRune(r), text: string(r)}
	if k.ctrl || k.alt {
		d.text = ""
	}
	return d
}

// codeForRune returns the physical DOM code for a rune, best-effort.
func codeForRune(r rune) string {
	switch {
	case r >= 'a' && r <= 'z':
		return "Key" + string(r-'a'+'A')
	case r >= 'A' && r <= 'Z':
		return "Key" + string(r)
	case r >= '0' && r <= '9':
		return "Digit" + string(r)
	case r == ' ':
		return "Space"
	}
	return ""
}

// vkForRune returns the Windows virtual key code for a rune, best-effort (0 when
// unknown; the inserted text still carries the character).
func vkForRune(r rune) int {
	switch {
	case r >= 'a' && r <= 'z':
		return int(r - 'a' + 'A')
	case r >= 'A' && r <= 'Z':
		return int(r)
	case r >= '0' && r <= '9':
		return int(r)
	case r == ' ':
		return 32
	}
	return 0
}
