// Post and image hover previews

import { posts, getModel, page, storeSeenPost } from "../state"
import options from "../options"
import {
	setAttrs, getClosestID, fetchJSON, hook, emitChanges, ChangeEmitter
} from "../util"
import { Post } from "./model"
import ImageHandler, { sourcePath } from "./images"
import PostView from "./view"
import { PostData, fileTypes, isExpandable } from "../common"

interface MouseMove extends ChangeEmitter {
	event: MouseEvent
}

const overlay = document.querySelector("#hover-overlay")

// Currently displayed preview, if any
let postPreview: PostPreview,
	imagePreview: HTMLElement

// Centralized mousemove target tracking
const mouseMove = emitChanges<MouseMove>({
	event: {
		target: null,
	},
} as MouseMove)

// Post hover preview view
class PostPreview extends ImageHandler {
	public el: HTMLElement
	private clickHandler: EventListener
	private observer: MutationObserver
	private parent: HTMLElement
	private source: HTMLElement
	private sourceModel: Post

	constructor(model: Post, parent: HTMLElement) {
		const { el } = model.view
		super({ el: clonePost(el) })
		this.parent = parent
		this.model = Object.assign({}, model)
		this.sourceModel = model
		this.source = el

		// Remove on parent click
		this.clickHandler = () =>
			this.remove()
		parent.addEventListener("click", this.clickHandler, {
			passive: true,
		})

		// Propagate post updates to clone
		this.observer = new MutationObserver(() =>
			this.renderUpdates())
		this.observer.observe(el, {
			childList: true,
			attributes: true,
			characterData: true,
			subtree: true,
		})

		this.render()
	}

	private render() {
		// Stop any playing audio or video
		const media = this.el.querySelector("audio, video") as HTMLMediaElement
		if (media) {
			media.pause()
		}

		// Remove any inline expanded posts
		for (let el of this.el.querySelectorAll("article")) {
			el.remove()
		}

		// Remove any existing reverse post link highlights due to link inline
		// expansion
		for (let el of this.el.querySelectorAll("a.post-link.referenced")) {
			el.classList.remove("referenced")
		}

		// Underline reverse post links in preview
		const patt = new RegExp(`[>\/]` + getClosestID(this.parent))
		for (let el of this.el.querySelectorAll("a.post-link")) {
			if (!patt.test(el.textContent)) {
				continue
			}
			el.classList.add("referenced")
		}

		// Contract any expanded open thumbnails
		const img = this.sourceModel.image
		if (img && img.expanded) {
			// Clone parent model's image and render contracted thumbnail
			this.model.image = Object.assign({}, this.sourceModel.image)
			this.contractImage(null, false)
		}

		const fc = overlay.firstChild
		if (fc !== this.el) {
			if (fc) {
				fc.remove()
			}
			overlay.append(this.el)
		}

		this.position()

		// Highlight target post, if present
		this.sourceModel.view.setHighlight(true)
	}

	// Position the preview element relative to it's parent link
	private position() {
		const rect = this.parent.getBoundingClientRect()

		// The preview will never take up more than 100% screen width, so no
		// need for checking horizontal overflow. Must be applied before
		// reading the height, so it takes into account post resizing to
		// viewport edge.
		this.el.style.left = rect.left + "px"

		const height = this.el.offsetHeight
		let top = rect.top - height - 5

		// If post gets cut off at the top, put it bellow the link
		if (top < 0) {
			top += height + 23
		}
		this.el.style.top = top + "px"
	}

	// Reclone and reposition on update. This is pretty expensive, but good
	// enough, because only one post will ever be previewed at a time
	private renderUpdates() {
		const el = clonePost(this.source)
		this.el.replaceWith(el)
		this.el = el
		this.render()
	}

	// Remove reference to this view from the parent element and module
	public remove() {
		this.observer.disconnect()
		this.parent.removeEventListener("click", this.clickHandler)
		postPreview = null
		super.remove()
		this.sourceModel.view.setHighlight(false)
	}
}

// Clear any previews
function clear() {
	if (postPreview) {
		postPreview.remove()
		postPreview = null
	}
	if (imagePreview) {
		imagePreview.remove()
		imagePreview = null
	}
}

// Clone a post element as a preview
function clonePost(el: HTMLElement): HTMLElement {
	const preview = el.cloneNode(true) as HTMLElement
	preview.removeAttribute("id")
	preview.classList.add("preview")
	return preview
}

function renderImagePreview(event: MouseEvent) {
	if (!options.imageHover || page.catalog) {
		return
	}
	const target = event.target as HTMLElement
	let bypass = !(target.matches && target.matches("figure img")),
		post: Post
	if (!bypass) {
		post = getModel(target)
		bypass = !post
			|| post.image.expanded
			|| post.image.thumb_type === fileTypes.noFile
	}
	if (bypass) {
		if (imagePreview) {
			imagePreview.remove()
			imagePreview = null
		}
		return
	}

	let tag: string
	if (isExpandable(post.image.file_type)) {
		switch (post.image.file_type) {
			case fileTypes.webm:
				if (!options.webmHover) {
					return clear()
				}
				tag = "video"
				break
			case fileTypes.mp4:
			case fileTypes.ogg:
				// No video OGG and MP4 are treated just like MP3
				if (!options.webmHover || !post.image.video) {
					return clear()
				}
				tag = "video"
				break
			default:
				tag = "img"
		}
	} else {
		// Nothing to preview for these
		return clear()
	}

	const el = document.createElement(tag)
	setAttrs(el, {
		src: sourcePath(post.image.sha1, post.image.file_type),
	});
	if (tag === 'video') {
		setAttrs(el, {
			autoplay: "",
			loop: "",
		});
	}
	imagePreview = el
	if (tag === "video") {
		(el as HTMLVideoElement).volume = options.audioVolume / 100
	}
	overlay.append(el)

	// Force Chrome 73+ to do a repaint. Otherwise you get invisible images.
	el.onload = () =>
		el.style.transform = "translateZ(1px)";
}

async function renderPostPreview(event: MouseEvent) {
	let target = event.target as HTMLElement
	if (!target.matches || !target.matches("a.post-link, .hash-link")) {
		return
	}
	if (target.classList.contains("hash-link")) {
		target = target.previousElementSibling as HTMLElement
	}
	if (target.matches("em.expanded > a")) {
		return
	}
	const id = parseInt(target.getAttribute("data-id"))
	if (!id) {
		return
	}

	let post = posts.get(id)
	if (!post) {
		// Try to fetch from server, if this post is not currently displayed
		// due to lastN or in a different thread
		const [data] = await fetchJSON<PostData>(`/json/post/${id}`)
		if (data) {
			post = new Post(data)
			new PostView(post, null)
		} else {
			return
		}
	} else if (!post.seenOnce) {
		post.seenOnce = true
		storeSeenPost(post.id, post.op)
	}
	postPreview = new PostPreview(post, target)
}

// Bind mouse movement event listener
function onMouseMove(event: MouseEvent) {
	if (event.target !== mouseMove.event.target) {
		clear()
		mouseMove.event = event
	}
}

export default () => {
	document.addEventListener("mousemove", onMouseMove, {
		passive: true,
	})
	mouseMove.onChange("event", renderPostPreview)
	mouseMove.onChange("event", renderImagePreview)

	// Clear previews, when an image is expanded
	hook("imageExpanded", clear)
}

