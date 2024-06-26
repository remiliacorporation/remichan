// Synchronization management message handlers

package websockets

import (
	"github.com/bakape/meguca/auth"
	"github.com/bakape/meguca/cache"
	"github.com/bakape/meguca/common"
	"github.com/bakape/meguca/config"
	"github.com/bakape/meguca/db"
	"github.com/bakape/meguca/websockets/feeds"

	"golang.org/x/crypto/bcrypt"
)

type syncRequest struct {
	Last100, Catalog      bool
	Page, ProtocolVersion uint
	Thread                uint64
	Board                 string
}

type reclaimRequest struct {
	ID       uint64
	Password string
}

// Synchronise the client to a certain thread, assign it's ID and prepare to
// receive update messages.
func (c *Client) synchronise(data []byte) error {
	var msg syncRequest
	err := decodeMessage(data, &msg)
	switch {
	case err != nil:
		return err
	case !auth.IsBoard(msg.Board):
		return common.ErrInvalidBoard(msg.Board)
	case msg.Thread != 0:
		valid, err := db.ValidateOP(msg.Thread, msg.Board)
		switch {
		case err != nil:
			return err
		case !valid:
			return common.ErrInvalidThread(msg.Thread, msg.Board)
		}
	}

	err = db.IsBanned(msg.Board, c.ip)
	if err != nil {
		return err
	}

	if msg.ProtocolVersion == common.ProtocolVersion {
		buf, err := common.EncodeMessage(common.MessageConfigs,
			config.GetBoardConfigs(msg.Board).BoardConfigs)
		if err != nil {
			return err
		}
		err = c.send(buf)
		if err != nil {
			return err
		}
	}

	return c.registerSync(msg)
}

// Register fresh client sync or change from previous sync
func (c *Client) registerSync(req syncRequest) (err error) {
	if c.post.id != 0 {
		err = c.closePreviousPost()
		if err != nil {
			return
		}
	}

	c.feed, err = feeds.SyncClient(c, req.Thread, req.Board)
	if err != nil || req.Thread != 0 {
		return
	}
	if req.ProtocolVersion != common.ProtocolVersion {
		return c.sendMessage(common.MessageSynchronise, nil)
	}

	// Send board post data over websocket
	key := cache.BoardKey(req.Board, int64(req.Page), !req.Catalog)
	var f cache.FrontEnd
	if req.Catalog {
		f = cache.CatalogFE
	} else {
		f = cache.BoardPageFE
	}
	json, _, _, err := cache.GetJSONAndData(key, f)
	return c.send(common.PrependMessageType(common.MessageSynchronise, json))
}

// Reclaim an open post after connection loss or navigating away.
//
// TODO: Technically there is no locking performed so a single post may be open
// by multiple clients. This opens us up to some exploits, but nothing severe.
// Still need to think of a solution.
func (c *Client) reclaimPost(data []byte) error {
	if err := c.closePreviousPost(); err != nil {
		return err
	}

	var req reclaimRequest
	if err := decodeMessage(data, &req); err != nil {
		return err
	}

	hash, err := db.GetPostPassword(req.ID)
	switch {
	case err != nil:
		return err
	case hash == nil:
		return c.sendMessage(common.MessageReclaim, 1)
	}

	switch err = auth.BcryptCompare(req.Password, hash); err {
	case nil:
	case bcrypt.ErrMismatchedHashAndPassword:
		return c.sendMessage(common.MessageReclaim, 1)
	default:
		return err
	}

	post, err := db.GetPost(req.ID)
	switch {
	case err != nil:
		return err
	case !post.Editing:
		return c.sendMessage(common.MessageReclaim, 1)
	}

	c.post.init(post)
	c.feed.InsertPost(post.Post, nil)

	return c.sendMessage(common.MessageReclaim, 0)
}
