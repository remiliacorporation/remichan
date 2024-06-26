// Core websocket message handlers

import { handlers, message, connSM, connEvent } from './connection'
import { posts, page } from './state'
import { Post, FormModel, PostView, lightenThread } from './posts'
import { PostLink, Command, PostData, ImageData, ModerationEntry } from "./common"
import { postAdded } from "./ui"
import { incrementPostCount } from "./page"
import { posterName } from "./options"
import { OverlayNotification } from "./ui"

// Message for splicing the contents of the current line
export type SpliceResponse = {
	id: number
	start: number
	len: number
	text: string
}

type CloseMessage = {
	id: number
	links: PostLink[] | null
	commands: Command[] | null
}

// Message for inserting images into an open post
interface ImageMessage extends ImageData {
	id: number
}

interface ModerationMessage extends ModerationEntry {
	id: number
}

// Run a function on a model, if it exists
function handle(id: number, fn: (m: Post) => void) {
	const model = posts.get(id)
	if (model) {
		fn(model)
	}
}

// Insert a post into the models and DOM
export function insertPost(data: PostData) {
	// R/a/dio song name override
	if (posterName()) {
		data.name = posterName()
	}

	const existing = posts.get(data.id)
	if (existing) {
		if (existing instanceof FormModel) {
			existing.onAllocation(data)
			incrementPostCount(true, !!data["image"]);
		}
		return
	}

	const model = new Post(data)
	model.op = page.thread
	model.board = page.board
	posts.add(model)
	const view = new PostView(model, null)

	if (!model.editing) {
		model.propagateLinks()
	}

	// Find last allocated post and insert after it
	const last = document
		.getElementById("thread-container")
		.lastElementChild
	if (last.id === "p0") {
		last.before(view.el)
	} else {
		last.after(view.el)
	}

	postAdded(model)
	incrementPostCount(true, !!data["image"]);
	lightenThread();
}

export default () => {
	handlers[message.invalid] = (msg: string) => {

		// TODO: More user-friendly critical error reporting

		alert(msg)
		connSM.feed(connEvent.error)
		throw msg
	}

	handlers[message.insertPost] = insertPost

	handlers[message.insertImage] = (msg: ImageMessage) =>
		handle(msg.id, m => {
			delete msg.id
			if (!m.image) {
				incrementPostCount(false, true)
			}
			m.insertImage(msg)
		})

	handlers[message.spoiler] = (id: number) =>
		handle(id, m =>
			m.spoilerImage())

	handlers[message.append] = ([id, char]: [number, number]) =>
		handle(id, m =>
			m.append(char))

	handlers[message.backspace] = (id: number) =>
		handle(id, m =>
			m.backspace())

	handlers[message.splice] = (msg: SpliceResponse) =>
		handle(msg.id, m =>
			m.splice(msg))

	handlers[message.closePost] = ({ id, links, commands }: CloseMessage) =>
		handle(id, m => {
			if (links) {
				m.links = links
				m.propagateLinks()
			}
			if (commands) {
				m.commands = commands
			}
			m.closePost()
		})

	handlers[message.moderatePost] = (msg: ModerationMessage) =>
		handle(msg.id, m =>
			m.applyModeration(msg))

	handlers[message.redirect] = (url: string) =>
		location.href = url

	handlers[message.notification] = (text: string) =>
		new OverlayNotification(text)
}
