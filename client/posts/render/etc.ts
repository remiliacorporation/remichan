// Miscellaneous post component rendering functions

import { page, mine } from '../../state'
import lang from '../../lang'
import { makeAttrs, pluralize } from "../../util"
import { PostLink } from "../../common"

// Render a link to other posts
export function renderPostLink(link: PostLink): string {
    const cross = link.op !== page.thread,
        url = `${cross ? `/${link.board}/${link.op}` : ""}#p${link.id}`
    let html = `<a class="post-link" data-id="${link.id}" href="${url}">>>${link.id}`
    if (cross && page.thread) {
        html += " ➡"
    }
    if (mine.has(link.id)) { // Post, I made
        html += ' ' + lang.posts["you"]
    }
    html += `</a><a class="hash-link" href="${url}"> #</a>`
    return html
}

// Render a temporary link for open posts
export function renderTempLink(id: number): string {
    const attrs = {
        class: "post-link temp",
        "data-id": id.toString(),
        href: `#p${id}`,
    }
    let html = `<a ${makeAttrs(attrs)}>>>${id}`
    if (mine.has(id)) {
        html += ' ' + lang.posts["you"]
    }
    html += "</a>"
    return html
}

// Renders readable elapsed time since post. Numbers are in seconds.
export function relativeTime(then: number): string {
    const now = Math.floor(Date.now() / 1000)
    let time = Math.floor((now - then) / 60),
        isFuture = false
    if (time < 1) {
        if (time > -5) { // Assume to be client clock imprecision
            return lang.posts["justNow"]
        }
        isFuture = true
        time = -time
    }

    const divide = [60, 24, 30, 12],
        unit = ['minute', 'hour', 'day', 'month']
    for (let i = 0; i < divide.length; i++) {
        if (time < divide[i]) {
            return ago(time, lang.plurals[unit[i]], isFuture)
        }
        time = Math.floor(time / divide[i])
    }

    return ago(time, lang.plurals["year"], isFuture)
}

// Renders "56 minutes ago" or "in 56 minutes" like relative time text
function ago(time: number, units: [string, string], isFuture: boolean): string {
    const count = pluralize(time, units)
    if (isFuture) {
        return `${lang.posts["in"]} ${count}`
    }
    return `${count} ${lang.posts["ago"]}`
}
