package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const baseURL = "https://bsky.social/xrpc"

type Client struct {
	accessJWT  string
	refreshJWT string
	DID        string
	Handle     string
	http       *http.Client
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
	return nil
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

type Post struct {
	URI         string     `json:"uri"`
	CID         string     `json:"cid"`
	Author      Author     `json:"author"`
	Record      PostRecord `json:"record"`
	LikeCount   int        `json:"likeCount"`
	RepostCount int        `json:"repostCount"`
	ReplyCount  int        `json:"replyCount"`
}

type FeedItem struct {
	Post Post `json:"post"`
}

type timelineResp struct {
	Feed   []FeedItem `json:"feed"`
	Cursor string     `json:"cursor"`
}

func (c *Client) GetTimeline(limit int) ([]FeedItem, error) {
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/app.bsky.feed.getTimeline?limit=%d", baseURL, limit), nil)
	req.Header.Set("Authorization", "Bearer "+c.accessJWT)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 401 {
		if err := c.RefreshSession(); err != nil {
			return nil, fmt.Errorf("session expired, please re-login")
		}
		return c.GetTimeline(limit)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("timeline error: %s", string(data))
	}
	var tr timelineResp
	if err := json.Unmarshal(data, &tr); err != nil {
		return nil, err
	}
	return tr.Feed, nil
}

func (c *Client) GetDiscoverFeed(limit int) ([]FeedItem, error) {
	feedURI := "at://did:plc:z72i7hdynmk6r22z27h6tvur/app.bsky.feed.generator/whats-hot"
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/app.bsky.feed.getFeed?feed=%s&limit=%d", baseURL, feedURI, limit), nil)
	req.Header.Set("Authorization", "Bearer "+c.accessJWT)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 401 {
		if err := c.RefreshSession(); err != nil {
			return nil, fmt.Errorf("session expired")
		}
		return c.GetDiscoverFeed(limit)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("discover feed error: %s", string(data))
	}
	var tr timelineResp
	if err := json.Unmarshal(data, &tr); err != nil {
		return nil, err
	}
	return tr.Feed, nil
}

func (c *Client) createRecord(collection string, record interface{}) error {
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
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create record failed: %s", string(data))
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
	return c.createRecord("app.bsky.feed.post", postRecord{
		Type:      "app.bsky.feed.post",
		Text:      text,
		CreatedAt: now,
	})
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

func (c *Client) Like(uri, cid string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return c.createRecord("app.bsky.feed.like", likeRecord{
		Type:      "app.bsky.feed.like",
		Subject:   subjectRef{URI: uri, CID: cid},
		CreatedAt: now,
	})
}

type repostRecord struct {
	Type      string     `json:"$type"`
	Subject   subjectRef `json:"subject"`
	CreatedAt string     `json:"createdAt"`
}

func (c *Client) Repost(uri, cid string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return c.createRecord("app.bsky.feed.repost", repostRecord{
		Type:      "app.bsky.feed.repost",
		Subject:   subjectRef{URI: uri, CID: cid},
		CreatedAt: now,
	})
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

func (c *Client) CreateReply(text, parentURI, parentCID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return c.createRecord("app.bsky.feed.post", replyPostRecord{
		Type:      "app.bsky.feed.post",
		Text:      text,
		CreatedAt: now,
		Reply: replyRef{
			Root:   subjectRef{URI: parentURI, CID: parentCID},
			Parent: subjectRef{URI: parentURI, CID: parentCID},
		},
	})
}
