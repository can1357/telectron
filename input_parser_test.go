package main

import "testing"

func TestParseInput(t *testing.T) {
	parse := func(input string) ([]termEvent, int, int) {
		t.Helper()
		events, cpr, consumed := parseInput([]byte(input))
		return events, cpr, consumed
	}
	wantConsumed := func(t *testing.T, got, want int) {
		t.Helper()
		if got != want {
			t.Fatalf("consumed = %d, want %d", got, want)
		}
	}
	wantCPR := func(t *testing.T, got, want int) {
		t.Helper()
		if got != want {
			t.Fatalf("cpr = %d, want %d", got, want)
		}
	}
	wantEventCount := func(t *testing.T, events []termEvent, want int) {
		t.Helper()
		if len(events) != want {
			t.Fatalf("len(events) = %d, want %d", len(events), want)
		}
	}
	wantMouse := func(t *testing.T, events []termEvent, i int) *mouseInput {
		t.Helper()
		if i >= len(events) {
			t.Fatalf("events[%d] missing; len(events) = %d", i, len(events))
		}
		if events[i].mouse == nil {
			t.Fatalf("events[%d].mouse is nil, want mouse event", i)
		}
		if events[i].key != nil {
			t.Fatalf("events[%d].key is non-nil on mouse event", i)
		}
		return events[i].mouse
	}
	wantKey := func(t *testing.T, events []termEvent, i int) *keyInput {
		t.Helper()
		if i >= len(events) {
			t.Fatalf("events[%d] missing; len(events) = %d", i, len(events))
		}
		if events[i].key == nil {
			t.Fatalf("events[%d].key is nil, want key event", i)
		}
		if events[i].mouse != nil {
			t.Fatalf("events[%d].mouse is non-nil on key event", i)
		}
		return events[i].key
	}
	checkMouse := func(t *testing.T, input string, want mouseInput) {
		t.Helper()
		events, cpr, consumed := parse(input)
		wantCPR(t, cpr, 0)
		wantConsumed(t, consumed, len(input))
		wantEventCount(t, events, 1)
		got := wantMouse(t, events, 0)
		if got.action != want.action {
			t.Fatalf("mouse.action = %v, want %v", got.action, want.action)
		}
		if got.button != want.button {
			t.Fatalf("mouse.button = %d, want %d", got.button, want.button)
		}
		if got.held != want.held {
			t.Fatalf("mouse.held = %v, want %v", got.held, want.held)
		}
		if got.x != want.x || got.y != want.y {
			t.Fatalf("mouse position = (%d,%d), want (%d,%d)", got.x, got.y, want.x, want.y)
		}
		if got.wheel != want.wheel {
			t.Fatalf("mouse.wheel = %d, want %d", got.wheel, want.wheel)
		}
		if got.shift != want.shift || got.alt != want.alt || got.ctrl != want.ctrl {
			t.Fatalf("mouse modifiers shift/alt/ctrl = %v/%v/%v, want %v/%v/%v", got.shift, got.alt, got.ctrl, want.shift, want.alt, want.ctrl)
		}
	}
	checkNamedKey := func(t *testing.T, input, wantName string) {
		t.Helper()
		events, cpr, consumed := parse(input)
		wantCPR(t, cpr, 0)
		wantConsumed(t, consumed, len(input))
		wantEventCount(t, events, 1)
		got := wantKey(t, events, 0)
		if got.name != wantName {
			t.Fatalf("key.name = %q, want %q", got.name, wantName)
		}
		if got.r != 0 {
			t.Fatalf("key.r = %q, want zero rune for named key %q", got.r, wantName)
		}
	}

	t.Run("SGR mouse reports", func(t *testing.T) {
		checkMouse(t, "\x1b[<0;10;5M", mouseInput{action: mouseDown, button: 0, x: 10, y: 5})
		checkMouse(t, "\x1b[<0;10;5m", mouseInput{action: mouseUp, button: 0, x: 10, y: 5})
		checkMouse(t, "\x1b[<35;20;30M", mouseInput{action: mouseMove, button: 3, held: false, x: 20, y: 30})
		checkMouse(t, "\x1b[<32;7;8M", mouseInput{action: mouseMove, button: 0, held: true, x: 7, y: 8})
		checkMouse(t, "\x1b[<64;5;5M", mouseInput{action: mouseWheel, x: 5, y: 5, wheel: -1})
		checkMouse(t, "\x1b[<65;5;5M", mouseInput{action: mouseWheel, button: 0, x: 5, y: 5, wheel: 1})
	})

	t.Run("SGR mouse modifiers", func(t *testing.T) {
		checkMouse(t, "\x1b[<16;1;1M", mouseInput{action: mouseDown, button: 0, x: 1, y: 1, ctrl: true})
		checkMouse(t, "\x1b[<4;1;1M", mouseInput{action: mouseDown, button: 0, x: 1, y: 1, shift: true})
		checkMouse(t, "\x1b[<8;1;1M", mouseInput{action: mouseDown, button: 0, x: 1, y: 1, alt: true})
	})

	t.Run("cursor position report", func(t *testing.T) {
		events, cpr, consumed := parse("\x1b[2;3R")
		wantCPR(t, cpr, 1)
		wantConsumed(t, consumed, len("\x1b[2;3R"))
		wantEventCount(t, events, 0)
	})

	t.Run("printable keys", func(t *testing.T) {
		events, cpr, consumed := parse("abc")
		wantCPR(t, cpr, 0)
		wantConsumed(t, consumed, len("abc"))
		wantEventCount(t, events, 3)
		for i, want := range []rune{'a', 'b', 'c'} {
			got := wantKey(t, events, i)
			if got.r != want {
				t.Fatalf("events[%d].key.r = %q, want %q", i, got.r, want)
			}
			if got.name != "" {
				t.Fatalf("events[%d].key.name = %q, want empty", i, got.name)
			}
		}
	})

	t.Run("named keys", func(t *testing.T) {
		cases := []struct {
			input string
			name  string
		}{
			{input: "\r", name: "Enter"},
			{input: "\x7f", name: "Backspace"},
			{input: "\t", name: "Tab"},
			{input: "\x1b[A", name: "ArrowUp"},
			{input: "\x1b[B", name: "ArrowDown"},
			{input: "\x1b[C", name: "ArrowRight"},
			{input: "\x1b[D", name: "ArrowLeft"},
			{input: "\x1b[3~", name: "Delete"},
			{input: "\x1b[H", name: "Home"},
			{input: "\x1b[5~", name: "PageUp"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				checkNamedKey(t, tc.input, tc.name)
			})
		}
	})

	t.Run("ctrl letter", func(t *testing.T) {
		events, cpr, consumed := parse("\x01")
		wantCPR(t, cpr, 0)
		wantConsumed(t, consumed, len("\x01"))
		wantEventCount(t, events, 1)
		got := wantKey(t, events, 0)
		if got.r != 'a' {
			t.Fatalf("key.r = %q, want %q", got.r, 'a')
		}
		if !got.ctrl {
			t.Fatalf("key.ctrl = false, want true")
		}
		if got.name != "" || got.alt || got.shift {
			t.Fatalf("key name/alt/shift = %q/%v/%v, want empty/false/false", got.name, got.alt, got.shift)
		}
	})

	t.Run("modified arrow", func(t *testing.T) {
		events, cpr, consumed := parse("\x1b[1;5C")
		wantCPR(t, cpr, 0)
		wantConsumed(t, consumed, len("\x1b[1;5C"))
		wantEventCount(t, events, 1)
		got := wantKey(t, events, 0)
		if got.name != "ArrowRight" {
			t.Fatalf("key.name = %q, want %q", got.name, "ArrowRight")
		}
		if !got.ctrl {
			t.Fatalf("key.ctrl = false, want true")
		}
		if got.alt || got.shift {
			t.Fatalf("key alt/shift = %v/%v, want false/false", got.alt, got.shift)
		}
	})

	t.Run("alt char", func(t *testing.T) {
		events, cpr, consumed := parse("\x1bd")
		wantCPR(t, cpr, 0)
		wantConsumed(t, consumed, len("\x1bd"))
		wantEventCount(t, events, 1)
		got := wantKey(t, events, 0)
		if got.r != 'd' {
			t.Fatalf("key.r = %q, want %q", got.r, 'd')
		}
		if !got.alt {
			t.Fatalf("key.alt = false, want true")
		}
		if got.name != "" || got.ctrl || got.shift {
			t.Fatalf("key name/ctrl/shift = %q/%v/%v, want empty/false/false", got.name, got.ctrl, got.shift)
		}
	})

	t.Run("partial trailing sequences", func(t *testing.T) {
		events, cpr, consumed := parse("\x1b[<0;10")
		wantCPR(t, cpr, 0)
		wantConsumed(t, consumed, 0)
		wantEventCount(t, events, 0)

		complete := "\x1b[<0;1;1M"
		partial := "\x1b[<0;2"
		events, cpr, consumed = parse(complete + partial)
		wantCPR(t, cpr, 0)
		wantConsumed(t, consumed, len(complete))
		wantEventCount(t, events, 1)
		got := wantMouse(t, events, 0)
		if got.action != mouseDown || got.button != 0 || got.x != 1 || got.y != 1 {
			t.Fatalf("mouse event = action %v button %d pos (%d,%d), want mouseDown button 0 pos (1,1)", got.action, got.button, got.x, got.y)
		}
	})

	t.Run("mixed stream", func(t *testing.T) {
		events, cpr, consumed := parse("a\x1b[<0;3;4M\r")
		wantCPR(t, cpr, 0)
		wantConsumed(t, consumed, len("a\x1b[<0;3;4M\r"))
		wantEventCount(t, events, 3)

		first := wantKey(t, events, 0)
		if first.r != 'a' || first.name != "" {
			t.Fatalf("events[0] key = r %q name %q, want r 'a' and empty name", first.r, first.name)
		}
		mid := wantMouse(t, events, 1)
		if mid.action != mouseDown || mid.button != 0 || mid.x != 3 || mid.y != 4 {
			t.Fatalf("events[1] mouse = action %v button %d pos (%d,%d), want mouseDown button 0 pos (3,4)", mid.action, mid.button, mid.x, mid.y)
		}
		last := wantKey(t, events, 2)
		if last.name != "Enter" {
			t.Fatalf("events[2].key.name = %q, want %q", last.name, "Enter")
		}
	})
}
