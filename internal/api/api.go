package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const baseURL = "https://bsky.social/xrpc"

type Client struct {
	accessJWT  string
	refreshJWT string
	DID        string
	Handle     string
	http       *http.Client
	onRefresh  func(accessJWT, refreshJWT string)
}

func (c *Client) SetOnRefresh(fn func(accessJWT, refreshJWT string)) {
	c.onRefresh = fn
}

func NewClient() *Client {
	return &Client{
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) SetSession(accessJWT, refreshJWT, did, handle string) {
	c.accessJWT = accessJWT
	c.refreshJWT = refreshJWT
	c.DID = did
	c.Handle = handle
}

func (c *Client) IsAuthenticated() bool {
	return c.accessJWT != ""
}

type createSessionReq struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

type SessionResp struct {
	AccessJwt  string `json:"accessJwt"`
	RefreshJwt string `json:"refreshJwt"`
	DID        string `json:"did"`
	Handle     string `json:"handle"`
}

func (c *Client) CreateSession(identifier, password string) (*SessionResp, error) {
	body, _ := json.Marshal(createSessionReq{Identifier: identifier, Password: password})
	resp, err := c.http.Post(baseURL+"/com.atproto.server.createSession", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("login failed: %s", string(data))
	}
	var s SessionResp
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	c.accessJWT = s.AccessJwt
	c.refreshJWT = s.RefreshJwt
	c.DID = s.DID
	c.Handle = s.Handle
	return &s, nil
}

func (c *Client) RefreshSession() error {
	req, _ := http.NewRequest("POST", baseURL+"/com.atproto.server.refreshSession", nil)
	req.Header.Set("Authorization", "Bearer "+c.refreshJWT)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return fmt.Errorf("refresh failed: %s", string(data))
	}
	var s SessionResp
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	c.accessJWT = s.AccessJwt
	c.refreshJWT = s.RefreshJwt
	if c.onRefresh != nil {
		c.onRefresh(s.AccessJwt, s.RefreshJwt)
	}
	return nil
}

func isExpiredToken(data []byte) bool {
	var e struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(data, &e)
	return e.Error == "ExpiredToken"
}

type Author struct {
	DID         string `json:"did"`
	Handle      string `json:"handle"`
	DisplayName string `json:"displayName"`
}

type PostRecord struct {
	Text      string `json:"text"`
	CreatedAt string `json:"createdAt"`
}

type PostViewer struct {
	Like   string `json:"like"`   // URI of user's like record, empty if not liked
	Repost string `json:"repost"` // URI of user's repost record, empty if not reposted
}

type Post struct {
	URI         string     `json:"uri"`
	CID         string     `json:"cid"`
	Author      Author     `json:"author"`
	Record      PostRecord `json:"record"`
	LikeCount   int        `json:"likeCount"`
	RepostCount int        `json:"repostCount"`
	ReplyCount  int        `json:"replyCount"`
	Viewer      PostViewer `json:"viewer"`
}

type FeedItem struct {
	Post Post `json:"post"`
}

type timelineResp struct {
	Feed   []FeedItem `json:"feed"`
	Cursor string     `json:"cursor"`
}

func (c *Client) GetTimeline(limit int, cursor string) ([]FeedItem, string, error) {
	u := fmt.Sprintf("%s/app.bsky.feed.getTimeline?limit=%d", baseURL, limit)
	if cursor != "" {
		u += "&cursor=" + url.QueryEscape(cursor)
	}
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("Authorization", "Bearer "+c.accessJWT)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 401 || isExpiredToken(data) {
		if err := c.RefreshSession(); err != nil {
			return nil, "", fmt.Errorf("session expired, please re-login")
		}
		return c.GetTimeline(limit, cursor)
	}
	if resp.StatusCode != 200 {
		return nil, "", fmt.Errorf("timeline error: %s", string(data))
	}
	var tr timelineResp
	if err := json.Unmarshal(data, &tr); err != nil {
		return nil, "", err
	}
	return tr.Feed, tr.Cursor, nil
}

func (c *Client) GetDiscoverFeed(limit int, cursor string) ([]FeedItem, string, error) {
	feedURI := "at://did:plc:z72i7hdynmk6r22z27h6tvur/app.bsky.feed.generator/whats-hot"
	u := fmt.Sprintf("%s/app.bsky.feed.getFeed?feed=%s&limit=%d", baseURL, feedURI, limit)
	if cursor != "" {
		u += "&cursor=" + url.QueryEscape(cursor)
	}
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("Authorization", "Bearer "+c.accessJWT)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 401 || isExpiredToken(data) {
		if err := c.RefreshSession(); err != nil {
			return nil, "", fmt.Errorf("session expired")
		}
		return c.GetDiscoverFeed(limit, cursor)
	}
	if resp.StatusCode != 200 {
		return nil, "", fmt.Errorf("discover feed error: %s", string(data))
	}
	var tr timelineResp
	if err := json.Unmarshal(data, &tr); err != nil {
		return nil, "", err
	}
	return tr.Feed, tr.Cursor, nil
}

func (c *Client) createRecord(collection string, record interface{}) (string, error) {
	body, _ := json.Marshal(struct {
		Repo       string      `json:"repo"`
		Collection string      `json:"collection"`
		Record     interface{} `json:"record"`
	}{
		Repo:       c.DID,
		Collection: collection,
		Record:     record,
	})
	req, _ := http.NewRequest("POST", baseURL+"/com.atproto.repo.createRecord", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+c.accessJWT)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 401 || isExpiredToken(data) {
		if err := c.RefreshSession(); err != nil {
			return "", fmt.Errorf("session expired")
		}
		return c.createRecord(collection, record)
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("create record failed: %s", string(data))
	}
	var result struct {
		URI string `json:"uri"`
	}
	_ = json.Unmarshal(data, &result)
	return result.URI, nil
}

func (c *Client) deleteRecord(recordURI string) error {
	trimmed := strings.TrimPrefix(recordURI, "at://")
	parts := strings.SplitN(trimmed, "/", 3)
	if len(parts) != 3 {
		return fmt.Errorf("invalid record URI: %s", recordURI)
	}
	body, _ := json.Marshal(struct {
		Repo       string `json:"repo"`
		Collection string `json:"collection"`
		Rkey       string `json:"rkey"`
	}{
		Repo:       parts[0],
		Collection: parts[1],
		Rkey:       parts[2],
	})
	req, _ := http.NewRequest("POST", baseURL+"/com.atproto.repo.deleteRecord", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+c.accessJWT)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 401 || isExpiredToken(data) {
		if err := c.RefreshSession(); err != nil {
			return fmt.Errorf("session expired")
		}
		return c.deleteRecord(recordURI)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("delete record failed: %s", string(data))
	}
	return nil
}

type postRecord struct {
	Type      string `json:"$type"`
	Text      string `json:"text"`
	CreatedAt string `json:"createdAt"`
}

func (c *Client) CreatePost(text string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := c.createRecord("app.bsky.feed.post", postRecord{
		Type:      "app.bsky.feed.post",
		Text:      text,
		CreatedAt: now,
	})
	return err
}

type subjectRef struct {
	URI string `json:"uri"`
	CID string `json:"cid"`
}

type likeRecord struct {
	Type      string     `json:"$type"`
	Subject   subjectRef `json:"subject"`
	CreatedAt string     `json:"createdAt"`
}

func (c *Client) Like(uri, cid string) (string, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	return c.createRecord("app.bsky.feed.like", likeRecord{
		Type:      "app.bsky.feed.like",
		Subject:   subjectRef{URI: uri, CID: cid},
		CreatedAt: now,
	})
}

func (c *Client) Unlike(likeURI string) error {
	return c.deleteRecord(likeURI)
}

type repostRecord struct {
	Type      string     `json:"$type"`
	Subject   subjectRef `json:"subject"`
	CreatedAt string     `json:"createdAt"`
}

func (c *Client) Repost(uri, cid string) (string, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	return c.createRecord("app.bsky.feed.repost", repostRecord{
		Type:      "app.bsky.feed.repost",
		Subject:   subjectRef{URI: uri, CID: cid},
		CreatedAt: now,
	})
}

func (c *Client) Unrepost(repostURI string) error {
	return c.deleteRecord(repostURI)
}

type createBookmarkReq struct {
	URI string `json:"uri"`
	CID string `json:"cid"`
}

func (c *Client) CreateBookmark(uri, cid string) error {
	body, _ := json.Marshal(createBookmarkReq{URI: uri, CID: cid})
	req, _ := http.NewRequest("POST", baseURL+"/app.bsky.bookmark.createBookmark", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+c.accessJWT)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == 401 {
		if err := c.RefreshSession(); err != nil {
			return fmt.Errorf("session expired")
		}
		return c.CreateBookmark(uri, cid)
	}
	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create bookmark failed: %s", string(data))
	}
	return nil
}

func (c *Client) DeleteBookmark(postURI string) error {
	body, _ := json.Marshal(struct {
		URI string `json:"uri"`
	}{URI: postURI})
	req, _ := http.NewRequest("POST", baseURL+"/app.bsky.bookmark.deleteBookmark", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+c.accessJWT)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == 401 {
		if err := c.RefreshSession(); err != nil {
			return fmt.Errorf("session expired")
		}
		return c.DeleteBookmark(postURI)
	}
	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete bookmark failed: %s", string(data))
	}
	return nil
}

type bookmarkItemView struct {
	URI         string     `json:"uri"`
	CID         string     `json:"cid"`
	Author      Author     `json:"author"`
	Record      PostRecord `json:"record"`
	LikeCount   int        `json:"likeCount"`
	RepostCount int        `json:"repostCount"`
	ReplyCount  int        `json:"replyCount"`
}

type bookmarkEntry struct {
	Subject   struct {
		URI string `json:"uri"`
		CID string `json:"cid"`
	} `json:"subject"`
	CreatedAt string           `json:"createdAt"`
	Item      bookmarkItemView `json:"item"`
}

type getBookmarksResp struct {
	Bookmarks []bookmarkEntry `json:"bookmarks"`
	Cursor    string          `json:"cursor"`
}

func (c *Client) GetBookmarks(limit int, cursor string) ([]FeedItem, string, error) {
	u := fmt.Sprintf("%s/app.bsky.bookmark.getBookmarks?limit=%d", baseURL, limit)
	if cursor != "" {
		u += "&cursor=" + url.QueryEscape(cursor)
	}
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("Authorization", "Bearer "+c.accessJWT)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 401 || isExpiredToken(data) {
		if err := c.RefreshSession(); err != nil {
			return nil, "", fmt.Errorf("session expired")
		}
		return c.GetBookmarks(limit, cursor)
	}
	if resp.StatusCode != 200 {
		return nil, "", fmt.Errorf("get bookmarks error: %s", string(data))
	}
	var br getBookmarksResp
	if err := json.Unmarshal(data, &br); err != nil {
		return nil, "", err
	}
	items := make([]FeedItem, 0, len(br.Bookmarks))
	for _, bm := range br.Bookmarks {
		if bm.Item.CID == "" {
			continue
		}
		items = append(items, FeedItem{
			Post: Post{
				URI:         bm.Item.URI,
				CID:         bm.Item.CID,
				Author:      bm.Item.Author,
				Record:      bm.Item.Record,
				LikeCount:   bm.Item.LikeCount,
				RepostCount: bm.Item.RepostCount,
				ReplyCount:  bm.Item.ReplyCount,
			},
		})
	}
	return items, br.Cursor, nil
}

type replyRef struct {
	Root   subjectRef `json:"root"`
	Parent subjectRef `json:"parent"`
}

type replyPostRecord struct {
	Type      string   `json:"$type"`
	Text      string   `json:"text"`
	CreatedAt string   `json:"createdAt"`
	Reply     replyRef `json:"reply"`
}

type searchPostsResp struct {
	Posts  []Post `json:"posts"`
	Cursor string `json:"cursor"`
}

func (c *Client) SearchPosts(query string, limit int) ([]FeedItem, error) {
	u := fmt.Sprintf("%s/app.bsky.feed.searchPosts?q=%s&limit=%d", baseURL, url.QueryEscape(query), limit)
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("Authorization", "Bearer "+c.accessJWT)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 401 || isExpiredToken(data) {
		if err := c.RefreshSession(); err != nil {
			return nil, fmt.Errorf("session expired")
		}
		return c.SearchPosts(query, limit)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("search error: %s", string(data))
	}
	var sr searchPostsResp
	if err := json.Unmarshal(data, &sr); err != nil {
		return nil, err
	}
	items := make([]FeedItem, len(sr.Posts))
	for i, p := range sr.Posts {
		items[i] = FeedItem{Post: p}
	}
	return items, nil
}

func (c *Client) CreateReply(text, parentURI, parentCID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := c.createRecord("app.bsky.feed.post", replyPostRecord{
		Type:      "app.bsky.feed.post",
		Text:      text,
		CreatedAt: now,
		Reply: replyRef{
			Root:   subjectRef{URI: parentURI, CID: parentCID},
			Parent: subjectRef{URI: parentURI, CID: parentCID},
		},
	})
	return err
}
