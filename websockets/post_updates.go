package websockets

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/bakape/meguca/common"
	"github.com/bakape/meguca/config"
	"github.com/bakape/meguca/db"
	"github.com/bakape/meguca/parser"
	"github.com/bakape/meguca/util"
)

var (
	errNoPostOpen    = errors.New("no post open")
	errEmptyPost     = errors.New("post body empty")
	errTooManyLines  = errors.New("too many lines in post body")
	errSpliceTooLong = errors.New("splice text too long")
	errSpliceNOOP    = errors.New("splice NOOP")
	errTextOnly      = errors.New("text only board")
	errHasImage      = errors.New("post already has image")
)

// Error created, when client supplies invalid splice coordinates to server
type errInvalidSpliceCoords struct {
	body string
	req  spliceRequestString
}

func (e errInvalidSpliceCoords) Error() string {
	return fmt.Sprintf("invalid splice coordinates: %#v", e)
}

// Like spliceRequest, but with a string Text field. Used for internal
// conversions between []rune and string.
type spliceRequestString struct {
	spliceCoords
	Text string `json:"text"`
}

// Common part of a splice request and a splice response
type spliceCoords struct {
	Start uint `json:"start"`
	Len   uint `json:"len"`
}

// Response to a spliceRequest. Sent to all listening clients.
type spliceMessage struct {
	ID uint64 `json:"id"`
	spliceRequestString
}

// Request or to replace the current line's text starting at an exact position
// in the current line
type spliceRequest struct {
	spliceCoords
	Text []rune
}

// Custom unmarshaling of string -> []rune
func (s *spliceRequest) UnmarshalJSON(buf []byte) error {
	var tmp spliceRequestString
	if err := json.Unmarshal(buf, &tmp); err != nil {
		return err
	}
	*s = spliceRequest{
		spliceCoords: tmp.spliceCoords,
		Text:         []rune(tmp.Text),
	}
	return nil
}

// Custom marshaling of []rune -> string
func (s spliceRequest) MarshalJSON() ([]byte, error) {
	return json.Marshal(spliceRequestString{
		spliceCoords: s.spliceCoords,
		Text:         string(s.Text),
	})
}

// Append a rune to the body of the open post
func (c *Client) appendRune(data []byte) (err error) {
	has, err := c.hasPost()
	switch {
	case err != nil:
		return
	case !has:
		return
	case c.post.len+1 > common.MaxLenBody:
		return common.ErrBodyTooLong
	}

	var char rune
	err = decodeMessage(data, &char)
	switch {
	case err != nil:
		return
	case char == 0:
		return common.ErrContainsNull
	case char == '\n':
		c.post.lines++
		if c.post.lines > common.MaxLinesBody {
			return errTooManyLines
		}
	}
	err = parser.IsPrintable(char, true)
	if err != nil {
		return
	}

	msg, err := common.EncodeMessage(
		common.MessageAppend,
		[2]uint64{c.post.id, uint64(char)},
	)
	if err != nil {
		return
	}

	c.post.body = append(c.post.body, string(char)...)
	c.post.len++
	return c.updateBody(msg, 1)
}

// Send message to thread update feed and writes the open post's buffer to the
// embedded database. Requires locking of c.openPost.
// n specifies the number of characters updated.
func (c *Client) updateBody(msg []byte, n int) error {
	c.feed.SetOpenBody(c.post.id, string(c.post.body), msg)
	c.incrementSpamScore(uint(n) * config.Get().CharScore)
	return db.SetOpenBody(c.post.id, c.post.body)
}

// Increment the spam score for this IP by score. If the client requires a new
// solved captcha, send a notification.
func (c *Client) incrementSpamScore(score uint) {
	db.IncrementSpamScore(c.ip, score)
}

// Remove one character from the end of the line in the open post
func (c *Client) backspace() error {
	has, err := c.hasPost()
	switch {
	case err != nil:
		return err
	case !has:
		return nil
	case c.post.len == 0:
		return errEmptyPost
	}

	msg, err := common.EncodeMessage(common.MessageBackspace, c.post.id)
	if err != nil {
		return err
	}

	r, lastRuneLen := utf8.DecodeLastRune(c.post.body)
	c.post.body = c.post.body[:len(c.post.body)-lastRuneLen]
	if r == '\n' {
		c.post.lines--
	}
	c.post.len--

	return c.updateBody(msg, 1)
}

// Close an open post and parse the last line, if needed.
func (c *Client) closePost() (err error) {
	if c.post.id == 0 {
		return errNoPostOpen
	}

	var (
		links []common.Link
		com   []common.Command
	)
	if c.post.len != 0 {
		links, com, err = parser.ParseBody(c.post.body, c.post.board, c.post.op,
			c.post.id, c.ip, false)
		if err != nil {
			return
		}
	}

	err = db.ClosePost(c.post.id, c.post.op, string(c.post.body), links, com)
	if err != nil {
		return
	}

	err = CheckRouletteBan(com, c.post.board, c.post.op, c.post.id)
	c.post = openPost{}
	return
}

// CheckRouletteBan meme bans if the poster lost at #roulette
func CheckRouletteBan(commands []common.Command, board string, thread uint64, id uint64) error {
	for _, command := range commands {
		if command.Type == common.Roulette {
			if command.Roulette[0] == 1 {
				err := db.Ban(board, "lost at #roulette", "system",
					time.Hour, id)
				if err != nil {
					return err
				}

				return db.InTransaction(false, func(tx *sql.Tx) error {
					return db.IncrementRcount(tx, thread)
				})
			}
		}
	}
	return nil
}

// Splice the text in the open post
func (c *Client) spliceText(data []byte) error {
	if has, err := c.hasPost(); err != nil {
		return err
	} else if !has {
		return nil
	}

	var req spliceRequest
	err := decodeMessage(data, &req)
	if err != nil {
		return err
	}
	err = parser.IsPrintableRunes(req.Text, true)
	if err != nil {
		return err
	}

	// Validate
	switch {
	case err != nil:
		return err
	case req.Start > common.MaxLenBody,
		req.Len > common.MaxLenBody,
		int(req.Start+req.Len) > c.post.len:
		return &errInvalidSpliceCoords{
			body: string(c.post.body),
			req: spliceRequestString{
				spliceCoords: spliceCoords{
					Start: req.Start,
					Len:   req.Len,
				},
				Text: string(req.Text),
			},
		}
	case req.Len == 0 && len(req.Text) == 0:
		return errSpliceNOOP // This does nothing. Client-side error.
	case len(req.Text) > common.MaxLenBody:
		return errSpliceTooLong // Nice try, kid
	}

	for _, r := range req.Text {
		if r == 0 {
			return common.ErrContainsNull
		}
	}

	var (
		old = []rune(string(c.post.body))
		end = append(req.Text, old[req.Start+req.Len:]...)
	)
	c.post.len += -int(req.Len) + len(req.Text)
	res := spliceMessage{
		ID: c.post.id,
		spliceRequestString: spliceRequestString{
			spliceCoords: req.spliceCoords,
			Text:         string(req.Text),
		},
	}

	// If it goes over the max post length, trim the end
	exceeding := c.post.len - common.MaxLenBody
	if exceeding > 0 {
		end = end[:len(end)-exceeding]
		res.Len = uint(len(old[int(req.Start):]))
		res.Text = string(end)
		c.post.len = common.MaxLenBody
	}

	msg, err := common.EncodeMessage(common.MessageSplice, res)
	if err != nil {
		return err
	}

	// Need to prevent modifications to the original slice, as there might be
	// concurrent reads in the update feed.
	c.post.body = util.CloneBytes(c.post.body)

	byteStartPos := 0
	for _, r := range old[:req.Start] {
		byteStartPos += utf8.RuneLen(r)
	}
	c.post.body = append(c.post.body[:byteStartPos], string(end)...)

	c.post.countLines()
	if c.post.lines > common.MaxLinesBody {
		return errTooManyLines
	}

	// +1, so you can't spam zero insert splices to infinity
	return c.updateBody(msg, len(res.Text)+1)
}

// Insert and image into an existing open post
// Note: Spam score is now incremented on image thumbnailing, not assignment to
// post.
func (c *Client) insertImage(data []byte) (err error) {
	has, err := c.hasPost()
	switch {
	case err != nil:
		return
	case !has:
		return errNoPostOpen
	case c.post.hasImage:
		return errHasImage
	}

	var req ImageRequest
	err = decodeMessage(data, &req)
	if err != nil {
		return
	}

	if config.GetBoardConfigs(c.post.board).TextOnly {
		return errTextOnly
	}

	err = formatImageName(&req.Name)
	if err != nil {
		return
	}
	var msg []byte
	err = db.InTransaction(false, func(tx *sql.Tx) (err error) {
		msg, err = db.InsertImage(tx, c.post.id, req.Token, req.Name,
			req.Spoiler)
		return
	})
	if err != nil {
		return
	}
	c.post.hasImage = true
	c.post.isSpoilered = req.Spoiler
	c.feed.InsertImage(c.post.id, req.Spoiler,
		common.PrependMessageType(common.MessageInsertImage, msg))

	return
}

// Spoiler an already inserted image in an unclosed post
func (c *Client) spoilerImage() (err error) {
	has, err := c.hasPost()
	switch {
	case err != nil:
		return err
	case !has:
		return errNoPostOpen
	case !c.post.hasImage:
		return errors.New("post does not have an image")
	case c.post.isSpoilered:
		return errors.New("already spoilered")
	}

	err = db.SpoilerImage(c.post.id, c.post.op)
	if err != nil {
		return
	}
	msg, err := common.EncodeMessage(common.MessageSpoiler, c.post.id)
	if err != nil {
		return
	}
	c.feed.SpoilerImage(c.post.id, msg)

	return
}
