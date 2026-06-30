package main

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

// TestProfilePersists verifies the -profile mechanism: a cookie written in one
// session is still present when a second browser opens the same user-data dir.
// This is what lets a one-time login (e.g. claude.ai) survive across runs.
func TestProfilePersists(t *testing.T) {
	chrome, err := findChrome()
	if err != nil {
		t.Skip("no chrome:", err)
	}
	dir, err := os.MkdirTemp("", "telectron-profile-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	attach := func(b *browser, ctx context.Context) string {
		t.Helper()
		tid, err := b.firstPageTarget(ctx)
		if err != nil {
			t.Fatal(err)
		}
		res, err := b.send(ctx, "", "Target.attachToTarget", map[string]any{"targetId": tid, "flatten": true})
		if err != nil {
			t.Fatal(err)
		}
		var a struct {
			SessionID string `json:"sessionId"`
		}
		json.Unmarshal(res, &a)
		if _, err := b.send(ctx, a.SessionID, "Network.enable", nil); err != nil {
			t.Fatal(err)
		}
		return a.SessionID
	}

	// Session 1: write a persistent cookie, then close (flushes to disk).
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	b1, err := launchBrowser(chrome, dir, 400, 300)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	sid := attach(b1, ctx)
	if _, err := b1.send(ctx, sid, "Network.setCookie", map[string]any{
		"name": "telectron_test", "value": "42",
		"domain": "example.com", "path": "/",
		"expires": float64(time.Now().Add(time.Hour).Unix()),
	}); err != nil {
		cancel()
		b1.close()
		t.Fatal(err)
	}
	cancel()
	b1.close() // Browser.close flushes the cookie store

	// Session 2: same dir — the cookie must still be there.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel2()
	b2, err := launchBrowser(chrome, dir, 400, 300)
	if err != nil {
		t.Fatal(err)
	}
	defer b2.close()
	sid2 := attach(b2, ctx2)
	res, err := b2.send(ctx2, sid2, "Network.getCookies", map[string]any{
		"urls": []string{"https://example.com/"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Cookies []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"cookies"`
	}
	if err := json.Unmarshal(res, &got); err != nil {
		t.Fatal(err)
	}
	for _, c := range got.Cookies {
		if c.Name == "telectron_test" && c.Value == "42" {
			t.Logf("cookie persisted across sessions in %s", dir)
			return
		}
	}
	t.Fatalf("cookie did not persist; got %+v", got.Cookies)
}
