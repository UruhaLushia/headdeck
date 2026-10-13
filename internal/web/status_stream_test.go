package web

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func readStatusFrame(scanner *bufio.Scanner) (string, map[string]json.RawMessage, error) {
	var kind string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			kind = strings.TrimPrefix(line, "event: ")
		}
		if strings.HasPrefix(line, "data: ") {
			var data map[string]json.RawMessage
			err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &data)
			return kind, data, err
		}
	}
	return "", nil, fmt.Errorf("status stream closed: %v", scanner.Err())
}

func TestStatusStreamDiffAndRevocation(t *testing.T) {
	srv, _, _ := testWebServer(t)
	cookie := login(t, srv)
	for _, path := range []string{"/api/status/stream", "/api/events"} {
		out := request(t, srv, "GET", path, "", map[string]string{"Origin": "https://foreign.example"}, []*http.Cookie{cookie})
		if out.Code != http.StatusForbidden {
			t.Fatalf("foreign origin %s: got %d", path, out.Code)
		}
	}
	server := httptest.NewUnstartedServer(srv)
	server.Config.WriteTimeout = 30 * time.Millisecond
	server.Start()
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/status/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(cookie)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("status stream: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	scanner := bufio.NewScanner(resp.Body)
	kind, data, err := readStatusFrame(scanner)
	if err != nil || kind != "snapshot" || data["headscale"] == nil || data["server"] == nil {
		t.Fatalf("missing initial status: %s %v %v", kind, data, err)
	}
	started := time.Now()
	kind, data, err = readStatusFrame(scanner)
	if err != nil || kind != "diff" || data["principal"] != nil || data["server"] == nil {
		t.Fatalf("expected changed fields after WriteTimeout: %s %v %v", kind, data, err)
	}
	if time.Since(started) < 14*time.Second {
		t.Fatal("status is sampled too frequently")
	}
	if err := srv.sessions.revoke(cookie.Value); err != nil {
		t.Fatal(err)
	}
	if kind, _, err = readStatusFrame(scanner); err == nil {
		t.Fatalf("revoked session remains subscribed: %s", kind)
	}
}
