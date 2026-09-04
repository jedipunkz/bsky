package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	c := NewClient()
	c.baseURL = srv.URL
	c.SetSession("old-access", "refresh", "did:plc:me", "me.bsky.social")
	return c
}

func TestDo_RefreshesOnceAndRetries(t *testing.T) {
	var paths []string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/com.atproto.server.refreshSession":
			_ = json.NewEncoder(w).Encode(SessionResp{AccessJwt: "new-access", RefreshJwt: "new-refresh"})
		default:
			if r.Header.Get("Authorization") != "Bearer new-access" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"ExpiredToken"}`))
				return
			}
			_, _ = w.Write([]byte(`{"feed":[{"post":{"uri":"at://post"}}],"cursor":"next"}`))
		}
	})

	items, cursor, err := c.GetTimeline(50, "")
	if err != nil {
		t.Fatalf("GetTimeline: %v", err)
	}
	if len(items) != 1 || items[0].Post.URI != "at://post" || cursor != "next" {
		t.Fatalf("unexpected result: %+v cursor=%q", items, cursor)
	}
	want := []string{"/app.bsky.feed.getTimeline", "/com.atproto.server.refreshSession", "/app.bsky.feed.getTimeline"}
	if strings.Join(paths, ",") != strings.Join(want, ",") {
		t.Errorf("requests = %v, want %v", paths, want)
	}
	if c.accessJWT != "new-access" {
		t.Errorf("access token not updated: %q", c.accessJWT)
	}
}

func TestDo_StopsAfterOneRetry(t *testing.T) {
	var calls int
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path == "/com.atproto.server.refreshSession" {
			_ = json.NewEncoder(w).Encode(SessionResp{AccessJwt: "a", RefreshJwt: "r"})
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"ExpiredToken"}`))
	})

	if _, _, err := c.GetTimeline(50, ""); err == nil {
		t.Fatal("expected an error when the retry is rejected too")
	}
	// request, refresh, retry — and no further attempts.
	if calls != 3 {
		t.Errorf("made %d requests, want 3", calls)
	}
}

func TestDo_ReportsAPIError(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"InvalidRequest","message":"bad cursor"}`))
	})

	_, err := c.GetProfile("someone.bsky.social")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "InvalidRequest: bad cursor") {
		t.Errorf("error = %q, want it to carry the API message", err)
	}
}

func TestCreateRecord_SendsRepoCollectionAndRecord(t *testing.T) {
	var got map[string]any
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"uri":"at://did:plc:me/app.bsky.feed.like/1"}`))
	})

	uri, err := c.Like("at://post", "cid1")
	if err != nil {
		t.Fatalf("Like: %v", err)
	}
	if uri != "at://did:plc:me/app.bsky.feed.like/1" {
		t.Errorf("uri = %q", uri)
	}
	if got["repo"] != "did:plc:me" || got["collection"] != "app.bsky.feed.like" {
		t.Errorf("request = %+v", got)
	}
	rec, _ := got["record"].(map[string]any)
	if rec["$type"] != "app.bsky.feed.like" || rec["createdAt"] == "" {
		t.Errorf("record = %+v", rec)
	}
	if subject, _ := rec["subject"].(map[string]any); subject["uri"] != "at://post" {
		t.Errorf("subject = %+v", rec["subject"])
	}
	if _, hasText := rec["text"]; hasText {
		t.Error("a like record must not carry a text field")
	}
}

func TestDeleteRecord_SplitsURI(t *testing.T) {
	var got map[string]any
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
	})

	if err := c.Unlike("at://did:plc:me/app.bsky.feed.like/abc"); err != nil {
		t.Fatalf("Unlike: %v", err)
	}
	if got["repo"] != "did:plc:me" || got["collection"] != "app.bsky.feed.like" || got["rkey"] != "abc" {
		t.Errorf("request = %+v", got)
	}
	if err := c.Unlike("nonsense"); err == nil {
		t.Error("expected an error for a malformed record URI")
	}
}

func TestGetBookmarks_SkipsEntriesWithoutCID(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"bookmarks":[
			{"item":{"uri":"at://a","cid":"c1"}},
			{"item":{"uri":"at://b"}}
		],"cursor":"c"}`))
	})

	items, cursor, err := c.GetBookmarks(50, "")
	if err != nil {
		t.Fatalf("GetBookmarks: %v", err)
	}
	if len(items) != 1 || items[0].Post.URI != "at://a" || cursor != "c" {
		t.Errorf("items = %+v cursor = %q", items, cursor)
	}
}
