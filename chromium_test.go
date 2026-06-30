package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// dataURL builds a data: URL for an inline HTML body with the given CSS color
// filling the viewport.
func dataURL(color string) string {
	return "data:text/html," +
		fmt.Sprintf("<body style='margin:0;width:100vw;height:100vh;background:%s'></body>", color)
}

// centerRGB samples the pixel at the middle of a frame.
func centerRGB(f *rgbFrame) (r, g, b byte) {
	o := ((f.h/2)*f.w + f.w/2) * 3
	return f.pix[o], f.pix[o+1], f.pix[o+2]
}

// TestScreencastDecodes drives the full frame pipeline headlessly: create a page,
// navigate to a solid-color document, run the JPEG screencast, and confirm the
// decoded RGB frame has the right dimensions and color.
func TestScreencastDecodes(t *testing.T) {
	b := testBrowser(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	const w, h = 320, 240
	p, err := newPage(ctx, b, dataURL("rgb(255,0,0)"), w, h, 80)
	if err != nil {
		t.Fatal(err)
	}
	defer p.close()

	// Poll until a red frame arrives (early frames may catch about:blank/load).
	deadline := time.Now().Add(10 * time.Second)
	var last *rgbFrame
	for time.Now().Before(deadline) {
		fr := p.next(500 * time.Millisecond)
		if fr == nil {
			continue
		}
		last = fr
		if r, g, bl := centerRGB(fr); r > 200 && g < 60 && bl < 60 {
			if fr.w != w || fr.h != h {
				t.Fatalf("frame size = %dx%d, want %dx%d", fr.w, fr.h, w, h)
			}
			if len(fr.pix) != w*h*3 {
				t.Fatalf("pix len = %d, want %d", len(fr.pix), w*h*3)
			}
			t.Logf("got %dx%d red frame, center=(%d,%d,%d)", fr.w, fr.h, r, g, bl)
			return
		}
	}
	if last != nil {
		r, g, bl := centerRGB(last)
		t.Fatalf("never saw a red frame; last %dx%d center=(%d,%d,%d)", last.w, last.h, r, g, bl)
	}
	t.Fatal("no frames received")
}

// TestStealthFingerprint checks the minimal anti-bot surface we rely on for
// Cloudflare/Turnstile: no HeadlessChrome marker, webdriver hidden, Chrome-like
// globals, populated languages/plugins, and visible activity state.
func TestStealthFingerprint(t *testing.T) {
	b := testBrowser(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	p, err := newPage(ctx, b, dataURL("rgb(0,0,0)"), 320, 200, 70)
	if err != nil {
		t.Fatal(err)
	}
	defer p.close()

	res, err := b.send(ctx, p.sessionID, "Runtime.evaluate", map[string]any{
		"expression": `(function () {
			return {
				webdriver: navigator.webdriver,
				userAgent: navigator.userAgent,
				brands: (navigator.userAgentData && navigator.userAgentData.brands || []).map(b => b.brand).join(","),
				languages: navigator.languages,
				plugins: navigator.plugins ? navigator.plugins.length : -1,
				hasChrome: !!window.chrome,
				hasRuntime: !!(window.chrome && window.chrome.runtime),
				visibilityState: document.visibilityState,
				hidden: document.hidden,
				hasFocus: document.hasFocus()
			};
		})()`,
		"returnByValue": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Result struct {
			Value struct {
				Webdriver       any      `json:"webdriver"`
				UserAgent       string   `json:"userAgent"`
				Brands          string   `json:"brands"`
				Languages       []string `json:"languages"`
				Plugins         int      `json:"plugins"`
				HasChrome       bool     `json:"hasChrome"`
				HasRuntime      bool     `json:"hasRuntime"`
				VisibilityState string   `json:"visibilityState"`
				Hidden          bool     `json:"hidden"`
				HasFocus        bool     `json:"hasFocus"`
			} `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(res, &got); err != nil {
		t.Fatal(err)
	}
	v := got.Result.Value
	if v.Webdriver != nil && v.Webdriver != false {
		t.Fatalf("navigator.webdriver = %#v, want nil/false", v.Webdriver)
	}
	if strings.Contains(v.UserAgent, "HeadlessChrome") {
		t.Fatalf("user agent still advertises headless: %q", v.UserAgent)
	}
	if strings.Contains(v.Brands, "HeadlessChrome") {
		t.Fatalf("brands still advertise headless: %q", v.Brands)
	}
	if len(v.Languages) == 0 || v.Languages[0] != "en-US" {
		t.Fatalf("languages = %v, want en-US first", v.Languages)
	}
	if v.Plugins < 1 {
		t.Fatalf("plugins length = %d, want >= 1", v.Plugins)
	}
	if !v.HasChrome || !v.HasRuntime {
		t.Fatalf("window.chrome/runtime missing: chrome=%v runtime=%v", v.HasChrome, v.HasRuntime)
	}
	if v.VisibilityState != "visible" || v.Hidden || !v.HasFocus {
		t.Fatalf("activity signals = state=%q hidden=%v focus=%v, want visible/false/true",
			v.VisibilityState, v.Hidden, v.HasFocus)
	}
}
