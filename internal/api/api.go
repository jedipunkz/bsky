package api

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const discoverFeedURI = "at://did:plc:z72i7hdynmk6r22z27h6tvur/app.bsky.feed.generator/whats-hot"

// feedParams builds the query shared by the paginated feed endpoints.
func feedParams(limit int, cursor string) url.Values {
	params := url.Values{"limit": {strconv.Itoa(limit)}}
	if cursor != "" {
		params.Set("cursor", cursor)
	}
	return params
}

// getFeed calls an endpoint returning {feed, cursor}.
func (c *Client) getFeed(endpoint string, params url.Values) ([]FeedItem, string, error) {
	var resp feedResp
	if err := c.get(endpoint, params, &resp); err != nil {
		return nil, "", err
	}
	return resp.Feed, resp.Cursor, nil
}

func (c *Client) GetTimeline(limit int, cursor string) ([]FeedItem, string, error) {
	return c.getFeed("/app.bsky.feed.getTimeline", feedParams(limit, cursor))
}

func (c *Client) GetDiscoverFeed(limit int, cursor string) ([]FeedItem, string, error) {
	params := feedParams(limit, cursor)
	params.Set("feed", discoverFeedURI)
	return c.getFeed("/app.bsky.feed.getFeed", params)
}

func (c *Client) GetAuthorFeed(actor, filter string, limit int, cursor string) ([]FeedItem, string, error) {
	params := feedParams(limit, cursor)
	params.Set("actor", actor)
	params.Set("filter", filter)
	return c.getFeed("/app.bsky.feed.getAuthorFeed", params)
}

func (c *Client) SearchPosts(query string, limit int, cursor string) ([]FeedItem, string, error) {
	params := feedParams(limit, cursor)
	params.Set("q", query)

	var resp struct {
		Posts  []Post `json:"posts"`
		Cursor string `json:"cursor"`
	}
	if err := c.get("/app.bsky.feed.searchPosts", params, &resp); err != nil {
		return nil, "", err
	}
	items := make([]FeedItem, len(resp.Posts))
	for i, p := range resp.Posts {
		items[i] = FeedItem{Post: p}
	}
	return items, resp.Cursor, nil
}

func (c *Client) GetProfile(actor string) (*Profile, error) {
	var p Profile
	if err := c.get("/app.bsky.actor.getProfile", url.Values{"actor": {actor}}, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// createRecord writes a record to the user's repo and returns its URI.
func (c *Client) createRecord(collection string, rec record) (string, error) {
	rec.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	rec.Type = collection

	var resp struct {
		URI string `json:"uri"`
	}
	err := c.post("/com.atproto.repo.createRecord", map[string]any{
		"repo":       c.DID,
		"collection": collection,
		"record":     rec,
	}, &resp)
	return resp.URI, err
}

// deleteRecord removes the record identified by an at:// URI.
func (c *Client) deleteRecord(recordURI string) error {
	parts := strings.SplitN(strings.TrimPrefix(recordURI, "at://"), "/", 3)
	if len(parts) != 3 {
		return fmt.Errorf("invalid record URI: %s", recordURI)
	}
	return c.post("/com.atproto.repo.deleteRecord", map[string]any{
		"repo":       parts[0],
		"collection": parts[1],
		"rkey":       parts[2],
	}, nil)
}

func (c *Client) CreatePost(text string) error {
	_, err := c.createRecord("app.bsky.feed.post", record{Text: text})
	return err
}

func (c *Client) CreateReply(text, parentURI, parentCID string) error {
	parent := subjectRef{URI: parentURI, CID: parentCID}
	_, err := c.createRecord("app.bsky.feed.post", record{
		Text:  text,
		Reply: &replyRefer{Root: parent, Parent: parent},
	})
	return err
}

func (c *Client) Like(uri, cid string) (string, error) {
	return c.createRecord("app.bsky.feed.like", record{Subject: subjectRef{URI: uri, CID: cid}})
}

func (c *Client) Unlike(likeURI string) error {
	return c.deleteRecord(likeURI)
}

func (c *Client) Repost(uri, cid string) (string, error) {
	return c.createRecord("app.bsky.feed.repost", record{Subject: subjectRef{URI: uri, CID: cid}})
}

func (c *Client) Unrepost(repostURI string) error {
	return c.deleteRecord(repostURI)
}

func (c *Client) Follow(did string) (string, error) {
	return c.createRecord("app.bsky.graph.follow", record{Subject: did})
}

func (c *Client) Unfollow(followURI string) error {
	return c.deleteRecord(followURI)
}

func (c *Client) CreateBookmark(uri, cid string) error {
	return c.post("/app.bsky.bookmark.createBookmark", map[string]string{"uri": uri, "cid": cid}, nil)
}

func (c *Client) DeleteBookmark(postURI string) error {
	return c.post("/app.bsky.bookmark.deleteBookmark", map[string]string{"uri": postURI}, nil)
}

// bookmarkEntry is the getBookmarks item shape: the post lives under "item"
// rather than under "post" like the feed endpoints.
type bookmarkEntry struct {
	Item Post `json:"item"`
}

func (c *Client) GetBookmarks(limit int, cursor string) ([]FeedItem, string, error) {
	var resp struct {
		Bookmarks []bookmarkEntry `json:"bookmarks"`
		Cursor    string          `json:"cursor"`
	}
	if err := c.get("/app.bsky.bookmark.getBookmarks", feedParams(limit, cursor), &resp); err != nil {
		return nil, "", err
	}

	items := make([]FeedItem, 0, len(resp.Bookmarks))
	for _, bm := range resp.Bookmarks {
		if bm.Item.CID == "" {
			continue
		}
		items = append(items, FeedItem{Post: bm.Item})
	}
	return items, resp.Cursor, nil
}
