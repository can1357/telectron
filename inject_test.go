package main

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"
	"time"
)

func TestInjectClickChangesDOM(t *testing.T) {
	b := testBrowser(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pageURL := "data:text/html," + url.PathEscape(`<body style='margin:0'><button id=b style='position:fixed;left:0;top:0;width:100vw;height:100vh' onclick="window.__clicked=(window.__clicked||0)+1">x</button></body>`)
	p, err := newPage(ctx, b, pageURL, 400, 300, 80)
	if err != nil {
		t.Fatal(err)
	}
	defer p.close()

	gotFrame := false
	for range 2 {
		if p.next(2*time.Second) != nil {
			gotFrame = true
		}
	}
	if !gotFrame {
		t.Fatal("no screencast frame received before click injection")
	}

	p.mousePressed(200, 150, 0, 0, 1)
	p.mouseReleased(200, 150, 0, 0, 1)

	deadline := time.Now().Add(3 * time.Second)
	var observed float64
	for time.Now().Before(deadline) {
		res, err := b.send(ctx, p.sessionID, "Runtime.evaluate", map[string]any{
			"expression":    "window.__clicked||0",
			"returnByValue": true,
		})
		if err != nil {
			t.Fatal(err)
		}
		var got struct {
			Result struct {
				Value float64 `json:"value"`
			} `json:"result"`
		}
		if err := json.Unmarshal(res, &got); err != nil {
			t.Fatal(err)
		}
		observed = got.Result.Value
		if observed >= 1 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("click did not update DOM; window.__clicked = %v", observed)
}

func TestInjectKeyTypesText(t *testing.T) {
	b := testBrowser(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pageURL := "data:text/html," + url.PathEscape(`<body style='margin:0'><input id=i autofocus style='width:100vw;height:100vh;font-size:40px'><script>document.getElementById('i').focus()</script></body>`)
	p, err := newPage(ctx, b, pageURL, 400, 300, 80)
	if err != nil {
		t.Fatal(err)
	}
	defer p.close()

	gotFrame := false
	for range 2 {
		if p.next(2*time.Second) != nil {
			gotFrame = true
		}
	}
	if !gotFrame {
		t.Fatal("no screencast frame received before key injection")
	}

	p.mousePressed(200, 150, 0, 0, 1)
	p.mouseReleased(200, 150, 0, 0, 1)
	p.sendKey(keyInput{r: 'h'})
	p.sendKey(keyInput{r: 'i'})

	deadline := time.Now().Add(3 * time.Second)
	observed := ""
	for time.Now().Before(deadline) {
		res, err := b.send(ctx, p.sessionID, "Runtime.evaluate", map[string]any{
			"expression":    "document.getElementById('i').value",
			"returnByValue": true,
		})
		if err != nil {
			t.Fatal(err)
		}
		var got struct {
			Result struct {
				Value string `json:"value"`
			} `json:"result"`
		}
		if err := json.Unmarshal(res, &got); err != nil {
			t.Fatal(err)
		}
		observed = got.Result.Value
		if observed == "hi" {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("typed keys did not update DOM; input value = %q", observed)
}
