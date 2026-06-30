package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"
)

// Chrome DevTools Protocol client over Chromium's --remote-debugging-pipe.
//
// The pipe transport carries CDP as NUL-delimited JSON messages on two extra
// file descriptors: the browser reads commands from fd 3 and writes
// replies+events to fd 4. We pass our pipe ends through exec.Cmd.ExtraFiles
// (index 0 -> child fd 3, index 1 -> child fd 4), avoiding a websocket, an open
// debugging port, and any third-party dependency.
//
// All sessions are flattened onto the one pipe: a command/event for a page
// target carries a sessionId, so a single reader goroutine demultiplexes
// browser-level and page-level traffic. Reference:
// https://chromedevtools.github.io/devtools-protocol/

// cdpError is the protocol-level error returned in a command reply.
type cdpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data,omitempty"`
}

func (e *cdpError) Error() string {
	if e.Data != "" {
		return fmt.Sprintf("cdp error %d: %s (%s)", e.Code, e.Message, e.Data)
	}
	return fmt.Sprintf("cdp error %d: %s", e.Code, e.Message)
}

// cdpCommand is an outbound request. omitempty keeps params/sessionId off the
// wire when unset, which Chrome's stricter parsers require.
type cdpCommand struct {
	ID        int    `json:"id"`
	Method    string `json:"method"`
	Params    any    `json:"params,omitempty"`
	SessionID string `json:"sessionId,omitempty"`
}

// cdpReply is an inbound message: a command result (has id) or an event (has
// method). Either kind may carry a sessionId identifying the page session.
type cdpReply struct {
	ID        int             `json:"id"`
	Method    string          `json:"method"`
	Params    json.RawMessage `json:"params"`
	Result    json.RawMessage `json:"result"`
	Error     *cdpError       `json:"error"`
	SessionID string          `json:"sessionId"`
}

// eventHandler receives an event's raw params and its originating sessionId.
type eventHandler func(sessionID string, params json.RawMessage)

// browser is a live CDP connection to a spawned Chromium process.
type browser struct {
	cmd     *exec.Cmd
	in      *os.File // we write commands here (child fd 3)
	out     *os.File // we read replies/events here (child fd 4)
	bw      *bufio.Writer
	wmu     sync.Mutex // serializes writes to in
	done    chan struct{}
	stealth bool // apply anti-automation mitigations before navigation

	mu      sync.Mutex
	nextID  int
	pending map[int]chan cdpReply
	// handlers are keyed by "sessionId\x00method"; "" sessionId matches any.
	handlers map[string][]eventHandler
	closeErr error
}

// chromeFlags are the launch flags for headless offscreen rendering with a
// stable, scrollbar-free screencast surface. We always suppress Chrome's most
// obvious automation signal at the source.
func chromeFlags(dataDir string, w, h int) []string {
	return []string{
		"--remote-debugging-pipe",
		"--headless=new",
		"--user-data-dir=" + dataDir,
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-extensions",
		"--disable-background-networking",
		"--disable-background-timer-throttling",
		"--disable-backgrounding-occluded-windows",
		"--disable-renderer-backgrounding",
		"--disable-features=Translate,site-per-process",
		"--disable-blink-features=AutomationControlled",
		"--hide-scrollbars",
		"--force-color-profile=srgb",
		"--mute-audio",
		fmt.Sprintf("--window-size=%d,%d", w, h),
		"about:blank",
	}
}

// findChrome locates a Chromium-family binary: TELECTRON_CHROME / CHROME env
// first, then common install paths, then PATH.
func findChrome() (string, error) {
	if p := os.Getenv("TELECTRON_CHROME"); p != "" {
		return p, nil
	}
	if p := os.Getenv("CHROME"); p != "" {
		return p, nil
	}
	var candidates []string
	if runtime.GOOS == "darwin" {
		candidates = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		}
	} else {
		candidates = []string{
			"/usr/bin/google-chrome", "/usr/bin/google-chrome-stable",
			"/usr/bin/chromium", "/usr/bin/chromium-browser",
			"/usr/bin/microsoft-edge", "/usr/bin/brave-browser",
		}
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, nil
		}
	}
	for _, name := range []string{"google-chrome", "chromium", "chromium-browser", "microsoft-edge", "brave-browser"} {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("no Chromium-family browser found; set TELECTRON_CHROME to its path")
}

// launchBrowser spawns Chromium with the pipe transport wired to fds 3/4 and
// starts the reader goroutine. dataDir is the user-data dir; w,h size the
// initial window. Stealth mitigations are always enabled.
func launchBrowser(chromePath, dataDir string, w, h int) (*browser, error) {
	// child fd 3 reads commands -> child holds the read end, we hold the write.
	cmdR, cmdW, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	// child fd 4 writes output -> child holds the write end, we hold the read.
	evtR, evtW, err := os.Pipe()
	if err != nil {
		cmdR.Close()
		cmdW.Close()
		return nil, err
	}

	cmd := exec.Command(chromePath, chromeFlags(dataDir, w, h)...)
	cmd.ExtraFiles = []*os.File{cmdR, evtW} // -> child fd 3, fd 4
	// Keep Chrome's own stderr off our tty; surface it only on launch failure.
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		cmdR.Close()
		cmdW.Close()
		evtR.Close()
		evtW.Close()
		return nil, fmt.Errorf("start chrome: %w", err)
	}
	// The child owns its ends now; closing ours lets EOF propagate on exit.
	cmdR.Close()
	evtW.Close()

	b := &browser{
		cmd:      cmd,
		in:       cmdW,
		out:      evtR,
		bw:       bufio.NewWriterSize(cmdW, 1<<16),
		done:     make(chan struct{}),
		stealth:  true,
		nextID:   0,
		pending:  make(map[int]chan cdpReply),
		handlers: make(map[string][]eventHandler),
	}
	go b.readLoop()
	return b, nil
}

// readLoop consumes NUL-delimited JSON from the browser, routing replies to
// waiting callers and events to registered handlers until the pipe closes.
func (b *browser) readLoop() {
	defer close(b.done)
	r := bufio.NewReaderSize(b.out, 1<<20)
	for {
		raw, err := r.ReadBytes(0)
		if len(raw) > 1 {
			msg := raw[:len(raw)-1] // strip trailing NUL
			var rep cdpReply
			if json.Unmarshal(msg, &rep) == nil {
				b.dispatch(rep)
			}
		}
		if err != nil {
			b.failPending(fmt.Errorf("cdp pipe closed: %w", err))
			return
		}
	}
}

// dispatch routes one decoded message: a result/error to its pending caller, an
// event to all matching handlers (exact sessionId and wildcard).
func (b *browser) dispatch(rep cdpReply) {
	if rep.ID != 0 {
		b.mu.Lock()
		ch := b.pending[rep.ID]
		delete(b.pending, rep.ID)
		b.mu.Unlock()
		if ch != nil {
			ch <- rep
		}
		return
	}
	if rep.Method == "" {
		return
	}
	b.mu.Lock()
	hs := append([]eventHandler(nil), b.handlers[rep.SessionID+"\x00"+rep.Method]...)
	hs = append(hs, b.handlers["\x00"+rep.Method]...)
	b.mu.Unlock()
	for _, h := range hs {
		h(rep.SessionID, rep.Params)
	}
}

// failPending unblocks every in-flight caller when the connection dies.
func (b *browser) failPending(err error) {
	b.mu.Lock()
	b.closeErr = err
	for id, ch := range b.pending {
		ch <- cdpReply{ID: id, Error: &cdpError{Message: err.Error()}}
		delete(b.pending, id)
	}
	b.mu.Unlock()
}

// on registers a handler for an event method. sessionID "" matches events from
// any session; a specific id matches only that page session.
func (b *browser) on(sessionID, method string, h eventHandler) {
	b.mu.Lock()
	key := sessionID + "\x00" + method
	b.handlers[key] = append(b.handlers[key], h)
	b.mu.Unlock()
}

// notify sends a command without awaiting its reply: used for high-rate,
// result-less traffic — input injection and screencast frame acks — where
// blocking on a round trip would add latency or, worse, stall the caller. The
// reply still arrives and is dropped (no pending entry), so ids stay unique.
func (b *browser) notify(sessionID, method string, params any) error {
	b.mu.Lock()
	if b.closeErr != nil {
		err := b.closeErr
		b.mu.Unlock()
		return err
	}
	b.nextID++
	id := b.nextID
	b.mu.Unlock()

	buf, err := json.Marshal(cdpCommand{ID: id, Method: method, Params: params, SessionID: sessionID})
	if err != nil {
		return err
	}
	b.wmu.Lock()
	_, werr := b.bw.Write(buf)
	if werr == nil {
		werr = b.bw.WriteByte(0)
	}
	if werr == nil {
		werr = b.bw.Flush()
	}
	b.wmu.Unlock()
	return werr
}

// send issues a command and blocks for its reply. sessionID "" targets the
// browser; otherwise it targets that attached page session.
func (b *browser) send(ctx context.Context, sessionID, method string, params any) (json.RawMessage, error) {
	b.mu.Lock()
	if b.closeErr != nil {
		err := b.closeErr
		b.mu.Unlock()
		return nil, err
	}
	b.nextID++
	id := b.nextID
	ch := make(chan cdpReply, 1)
	b.pending[id] = ch
	b.mu.Unlock()

	cmd := cdpCommand{ID: id, Method: method, Params: params, SessionID: sessionID}
	buf, err := json.Marshal(cmd)
	if err != nil {
		b.clearPending(id)
		return nil, err
	}
	b.wmu.Lock()
	_, werr := b.bw.Write(buf)
	if werr == nil {
		werr = b.bw.WriteByte(0)
	}
	if werr == nil {
		werr = b.bw.Flush()
	}
	b.wmu.Unlock()
	if werr != nil {
		b.clearPending(id)
		return nil, fmt.Errorf("write %s: %w", method, werr)
	}

	select {
	case rep := <-ch:
		if rep.Error != nil {
			return nil, fmt.Errorf("%s: %w", method, rep.Error)
		}
		return rep.Result, nil
	case <-ctx.Done():
		b.clearPending(id)
		return nil, ctx.Err()
	}
}

// firstPageTarget returns the id of the browser's existing top-level page target
// (created at launch). That page is active and screencast-capable, unlike a
// freshly created background target. It polls briefly because the page may not
// be registered the instant the pipe connects.
func (b *browser) firstPageTarget(ctx context.Context) (string, error) {
	deadline := time.Now().Add(5 * time.Second)
	for {
		res, err := b.send(ctx, "", "Target.getTargets", nil)
		if err != nil {
			return "", fmt.Errorf("getTargets: %w", err)
		}
		var r struct {
			TargetInfos []struct {
				TargetID string `json:"targetId"`
				Type     string `json:"type"`
			} `json:"targetInfos"`
		}
		if err := json.Unmarshal(res, &r); err != nil {
			return "", err
		}
		for _, t := range r.TargetInfos {
			if t.Type == "page" {
				return t.TargetID, nil
			}
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("no page target appeared")
		}
		select {
		case <-time.After(50 * time.Millisecond):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}

func (b *browser) clearPending(id int) {
	b.mu.Lock()
	delete(b.pending, id)
	b.mu.Unlock()
}

// close terminates the browser process and tears down the pipes.
func (b *browser) close() {
	if b.cmd == nil || b.cmd.Process == nil {
		return
	}
	// Ask Chrome to exit cleanly, then ensure it is gone.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	_, _ = b.send(ctx, "", "Browser.close", nil)
	cancel()
	select {
	case <-b.done:
	case <-time.After(2 * time.Second):
		_ = b.cmd.Process.Kill()
	}
	b.in.Close()
	b.out.Close()
	_ = b.cmd.Wait()
}
