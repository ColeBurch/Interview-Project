package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"
)

// connectTimeout doubles as the hang guard: cancelling the request context
// unblocks any read that is waiting on a frame the server never sent.
const connectTimeout = 5 * time.Second

type conn struct {
	reader *bufio.Reader
	body   io.ReadCloser
	cancel context.CancelFunc
}

func (c *conn) close() {
	c.cancel()
	c.body.Close()
}

// next blocks until the server pushes the next viewer list.
//
// Every assertion below waits on a real frame instead of sleeping, so the test
// is deterministic: if a broadcast is missing the read times out and fails.
func (c *conn) next(t *testing.T) []string {
	t.Helper()

	for {
		line, err := c.reader.ReadString('\n')
		if err != nil {
			t.Fatalf("waiting for viewer list: %v", err)
		}
		data, ok := strings.CutPrefix(strings.TrimRight(line, "\r\n"), "data: ")
		if !ok {
			continue // SSE comment, blank separator, or a field we don't use
		}
		var names []string
		if err := json.Unmarshal([]byte(data), &names); err != nil {
			t.Fatalf("malformed payload %q: %v", data, err)
		}
		return names
	}
}

func newTestServer(t *testing.T) (*app, string) {
	t.Helper()

	a := &app{
		logger: slog.New(slog.DiscardHandler),
		hub:    newHub(),
	}
	srv := httptest.NewServer(a.routes())
	t.Cleanup(srv.Close)
	return a, srv.URL
}

func connect(t *testing.T, base, report, name string) *conn {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	target := fmt.Sprintf("%s/reports/%s/presence?name=%s", base, report, url.QueryEscape(name))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		cancel()
		t.Fatalf("build request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel()
		t.Fatalf("connect %s: %v", name, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		cancel()
		t.Fatalf("connect %s: status %d, want 200", name, resp.StatusCode)
	}

	c := &conn{reader: bufio.NewReader(resp.Body), body: resp.Body, cancel: cancel}
	t.Cleanup(c.close)
	return c
}

func assertViewers(t *testing.T, got, want []string, when string) {
	t.Helper()

	if !slices.Equal(got, want) {
		t.Fatalf("%s: viewers = %v, want %v", when, got, want)
	}
}

// TestPresenceLifecycle is the one check that matters: arriving viewers show up
// for everyone already watching, and leaving viewers disappear.
func TestPresenceLifecycle(t *testing.T) {
	a, base := newTestServer(t)

	cole := connect(t, base, "weekly-performance", "Cole")
	assertViewers(t, cole.next(t), []string{"Cole"}, "first viewer sees themselves")

	// A second viewer must appear on the first viewer's stream without the
	// first viewer doing anything.
	sarah := connect(t, base, "weekly-performance", "Sarah")
	assertViewers(t, cole.next(t), []string{"Cole", "Sarah"}, "Sarah arrives")
	assertViewers(t, sarah.next(t), []string{"Cole", "Sarah"}, "Sarah's own first frame")

	// Dropping the connection is the only "leave" signal there is.
	sarah.close()
	assertViewers(t, cole.next(t), []string{"Cole"}, "Sarah leaves")

	// Presence is per person, not per tab: Cole opening a second tab must not
	// make him show up twice.
	coleSecondTab := connect(t, base, "weekly-performance", "Cole")
	assertViewers(t, cole.next(t), []string{"Cole"}, "Cole opens a second tab")
	assertViewers(t, coleSecondTab.next(t), []string{"Cole"}, "second tab's own frame")

	// ...and closing one of two tabs must not evict him.
	coleSecondTab.close()
	assertViewers(t, cole.next(t), []string{"Cole"}, "Cole closes his second tab")

	// Viewers are scoped to a report, so Jake on another report is invisible
	// here and Cole is invisible there.
	connect(t, base, "q3-campaign", "Jake")
	assertViewers(t, a.hub.viewers("weekly-performance"), []string{"Cole"}, "other report leaks in")
	assertViewers(t, a.hub.viewers("q3-campaign"), []string{"Jake"}, "own report")
}

func TestPresenceRequiresName(t *testing.T) {
	_, base := newTestServer(t)

	resp, err := http.Get(base + "/reports/weekly-performance/presence")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// A name is echoed into every other viewer's browser, so its length is bounded
// at the boundary rather than trusted.
func TestPresenceTruncatesLongNames(t *testing.T) {
	a, base := newTestServer(t)

	connect(t, base, "weekly-performance", strings.Repeat("é", maxNameLen+10))

	viewers := a.hub.viewers("weekly-performance")
	if len(viewers) != 1 {
		t.Fatalf("viewers = %v, want exactly one", viewers)
	}
	if got := len([]rune(viewers[0])); got != maxNameLen {
		t.Fatalf("name length = %d runes, want %d", got, maxNameLen)
	}
	// Truncating a multi-byte name by bytes would leave a replacement char.
	if strings.ContainsRune(viewers[0], '�') {
		t.Fatalf("name %q was split mid-rune", viewers[0])
	}
}
