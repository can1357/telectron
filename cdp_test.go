package main

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

// testBrowser launches a headless Chromium for a test, skipping when none is
// installed, and registers teardown.
func testBrowser(t *testing.T) *browser {
	t.Helper()
	chrome, err := findChrome()
	if err != nil {
		t.Skip("no chrome:", err)
	}
	dir, err := os.MkdirTemp("", "telectron-test-")
	if err != nil {
		t.Fatal(err)
	}
	b, err := launchBrowser(chrome, dir, 800, 600)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatal(err)
	}
	t.Cleanup(func() {
		b.close()
		os.RemoveAll(dir)
	})
	return b
}

// TestCDPVersion proves the NUL-framed pipe transport end to end: a real Chrome
// over fds 3/4 answers a browser-level command.
func TestCDPVersion(t *testing.T) {
	b := testBrowser(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	res, err := b.send(ctx, "", "Browser.getVersion", nil)
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Product  string `json:"product"`
		Revision string `json:"revision"`
	}
	if err := json.Unmarshal(res, &v); err != nil {
		t.Fatal(err)
	}
	if v.Product == "" {
		t.Fatalf("empty product in %s", res)
	}
	t.Logf("connected to %s", v.Product)
}
