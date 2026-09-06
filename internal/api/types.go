package api

type Author struct {
	DID         string `json:"did"`
	Handle      string `json:"handle"`
	DisplayName string `json:"displayName"`
}

type ReplyRef struct {
	Root   subjectRef `json:"root"`
	Parent subjectRef `json:"parent"`
}

type PostRecord struct {
	Text      string    `json:"text"`
	CreatedAt string    `json:"createdAt"`
	Reply     *ReplyRef `json:"reply,omitempty"`
	Facets    []Facet   `json:"facets,omitempty"`
}

// FacetFeature is one annotation on a text range. Only the link feature is
// used here; mentions and tags carry other fields.
type FacetFeature struct {
	Type string `json:"$type"`
	URI  string `json:"uri"`
}

// FacetIndex is a UTF-8 *byte* range into PostRecord.Text, not a rune range.
type FacetIndex struct {
	ByteStart int `json:"byteStart"`
	ByteEnd   int `json:"byteEnd"`
}

type Facet struct {
	Index    FacetIndex     `json:"index"`
	Features []FacetFeature `json:"features"`
}

// LinkURI returns the target of the facet's link feature, or "" when the facet
// annotates something else (a mention or a tag).
func (f Facet) LinkURI() string {
	for _, feat := range f.Features {
		if feat.Type == "app.bsky.richtext.facet#link" && feat.URI != "" {
			return feat.URI
		}
	}
	return ""
}

type ProfileViewer struct {
	Following string `json:"following"` // URI of follow record, empty if not following
}

type Profile struct {
	DID            string        `json:"did"`
	Handle         string        `json:"handle"`
	DisplayName    string        `json:"displayName"`
	FollowersCount int           `json:"followersCount"`
	FollowsCount   int           `json:"followsCount"`
	PostsCount     int           `json:"postsCount"`
	Viewer         ProfileViewer `json:"viewer"`
}

type PostViewer struct {
	Like   string `json:"like"`   // URI of user's like record, empty if not liked
	Repost string `json:"repost"` // URI of user's repost record, empty if not reposted
}

type EmbedImageView struct {
	Thumb    string `json:"thumb"`
	Fullsize string `json:"fullsize"`
	Alt      string `json:"alt"`
}

type PostEmbedView struct {
	Type   string           `json:"$type"`
	Images []EmbedImageView `json:"images"`
	// recordWithMedia nests images under "media"
	Media *PostEmbedView `json:"media"`
}

// EmbedImages returns images regardless of whether they are top-level or
// nested inside a recordWithMedia embed.
func (e *PostEmbedView) EmbedImages() []EmbedImageView {
	if e == nil {
		return nil
	}
	if len(e.Images) > 0 {
		return e.Images
	}
	if e.Media != nil {
		return e.Media.Images
	}
	return nil
}

type Post struct {
	URI         string         `json:"uri"`
	CID         string         `json:"cid"`
	Author      Author         `json:"author"`
	Record      PostRecord     `json:"record"`
	LikeCount   int            `json:"likeCount"`
	RepostCount int            `json:"repostCount"`
	ReplyCount  int            `json:"replyCount"`
	Viewer      PostViewer     `json:"viewer"`
	Embed       *PostEmbedView `json:"embed"`
}

type FeedItem struct {
	Post Post `json:"post"`
}

// subjectRef is a strong reference to another record (a post being liked,
// reposted or replied to).
type subjectRef struct {
	URI string `json:"uri"`
	CID string `json:"cid"`
}

// record is the shape written to the user's repo. Fields that do not apply to a
// given collection are omitted.
type record struct {
	Type      string      `json:"$type"`
	CreatedAt string      `json:"createdAt"`
	Text      string      `json:"text,omitempty"`
	Subject   any         `json:"subject,omitempty"`
	Reply     *replyRefer `json:"reply,omitempty"`
}

type replyRefer struct {
	Root   subjectRef `json:"root"`
	Parent subjectRef `json:"parent"`
}

// feedResp is the shape shared by getTimeline, getFeed and getAuthorFeed.
type feedResp struct {
	Feed   []FeedItem `json:"feed"`
	Cursor string     `json:"cursor"`
}
