// Message types of the WebSocket communication protocol
export const enum message {
	invalid,

	// 1 - 29 modify post model state
	insertPost,
	append,
	backspace,
	splice,
	closePost,
	insertImage,
	spoiler,
	moderatePost,

	// >= 30 are miscellaneous and do not write to post models
	synchronise = 30,
	reclaim,

	// Send new post ID to client
	postID,

	// Concatenation of multiple websocket messages to reduce transport overhead
	concat,

	// Invokes no operation on the server. Used to test the client's connection
	// in situations, when you can't be certain the client is still connected.
	NOOP,

	// Transmit current synced IP count to client
	syncCount,

	// Send current server Unix time to client
	serverTime,

	// Redirect the client to a specific board
	redirect,

	// Send a notification to a client
	notification,

	// Notification about needing a captcha on the next post allocation
	captcha,

	// Data concerning live random video feed
	meguTV,

	// Used by the client to send it's protocol version and by the server to
	// send server and board configurations
	configs,
}

export type MessageHandler = (msg: {}) => void

// Websocket message handlers. Each handler responds to its distinct message
// type.
export const handlers: { [type: number]: MessageHandler } = {}
