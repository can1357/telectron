package main

import (
	"context"
	"net/url"
	"testing"
	"time"
)

// animURL is a continuously repainting page (background-color keyframes — a paint
// property), percent-encoded so no raw '#' or space corrupts the data: URL. A raw
// '#' would start the URL fragment and silently truncate the document.
var animURL = "data:text/html," + url.PathEscape(
	"<style>@keyframes f{from{background:rgb(250,0,0)}to{background:rgb(0,0,250)}}"+
		"body{margin:0;height:100vh;animation:f .4s infinite alternate}</style><body></body>")

// TestFrameStreaming proves the full frame pipeline streams continuously rather
// than delivering a single static paint: an animating page must yield many
// distinct decoded frames through screencast -> ack -> decode -> publish.
func TestFrameStreaming(t *testing.T) {
	b := testBrowser(t)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	p, err := newPage(ctx, b, animURL, 200, 120, 60)
	if err != nil {
		t.Fatal(err)
	}
	defer p.close()

	pulled, distinct, lastR := 0, 0, -1
	deadline := time.Now().Add(2500 * time.Millisecond)
	for time.Now().Before(deadline) {
		fr := p.next(300 * time.Millisecond)
		if fr == nil {
			continue
		}
		pulled++
		if r, _, _ := centerRGB(fr); lastR < 0 || absInt(int(r)-lastR) > 10 {
			distinct++
			lastR = int(r)
		}
	}
	t.Logf("frames pulled=%d, distinct-by-color=%d", pulled, distinct)
	if distinct < 10 {
		t.Fatalf("expected continuous streaming (>=10 distinct frames), got %d distinct (pulled %d)", distinct, pulled)
	}
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
