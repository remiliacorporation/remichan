// Package common contains common shared types, variables and constants used
// throughout the project
package common

// ParseBody forwards parser.ParseBody to avoid cyclic imports in db/upkeep
// TODO: Clean up this function signature
var ParseBody func([]byte, string, uint64, uint64, string, bool) ([]Link, []Command, error)

// Board is defined to enable marshalling optimizations and sorting by sticky
// threads
type Board struct {
	Pages   int      `json:"pages"`
	Threads []Thread `json:"threads"`
}

func (b Board) Len() int {
	return len(b.Threads)
}

func (b Board) Swap(i, j int) {
	b.Threads[i], b.Threads[j] = b.Threads[j], b.Threads[i]
}

func (b Board) Less(i, j int) bool {
	// So it gets sorted with sticky threads first
	return b.Threads[i].Sticky
}

// Thread is a transport/export wrapper that stores both the thread metadata,
// its opening post data and its contained posts. The composite type itself is
// not stored in the database.
type Thread struct {
	Abbrev     bool   `json:"abbrev"`
	Sticky     bool   `json:"sticky"`
	Locked     bool   `json:"locked"`
	PostCount  uint32 `json:"post_count"`
	ImageCount uint32 `json:"image_count"`
	UpdateTime int64  `json:"update_time"`
	BumpTime   int64  `json:"bump_time"`
	Subject    string `json:"subject"`
	Board      string `json:"board"`
	Post
	Posts []Post `json:"posts"`
}

// Post is a generic post exposed publically through the JSON API. Either OP or
// reply.
type Post struct {
	Editing    bool              `json:"editing"`
	Moderated  bool              `json:"-"`
	Sage       bool              `json:"sage"`
	Auth       ModerationLevel   `json:"auth"`
	ID         uint64            `json:"id"`
	Time       int64             `json:"time"`
	Body       string            `json:"body"`
	Flag       string            `json:"flag"`
	Name       string            `json:"name"`
	Trip       string            `json:"trip"`
	Image      *Image            `json:"image"`
	Links      []Link            `json:"links"`
	Commands   []Command         `json:"commands"`
	Moderation []ModerationEntry `json:"moderation"`
}

// Return if post has been deleted by staff
func (p *Post) IsDeleted() bool {
	for _, l := range p.Moderation {
		if l.Type == DeletePost {
			return true
		}
	}
	return false
}

// Link describes a link from one post to another
type Link struct {
	ID    uint64 `json:"id"`
	OP    uint64 `json:"op"`
	Board string `json:"board"`
}

// StandalonePost is a post view that includes the "op" and "board" fields,
// which are not exposed though Post, but are required for retrieving a post
// with unknown parenthood.
type StandalonePost struct {
	Post
	OP    uint64 `json:"op"`
	Board string `json:"board"`
}
