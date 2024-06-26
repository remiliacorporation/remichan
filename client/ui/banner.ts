import { BannerModal } from "../base"

export default () => {
	for (let id of ["options", "FAQ", "identity", "account"]) {
		highlightBanner(id)
	}
	new BannerModal(document.getElementById("FAQ"))
	new BannerModal(document.getElementById("bug"))

}

// Highlight options button by fading out and in, if no options are set
function highlightBanner(name: string) {
	const key = name + "_seen"
	if (localStorage.getItem(key)) {
		return
	}

	const el = document.querySelector('#banner-' + name)
	el.classList.add("blinking")

	el.addEventListener("click", () => {
		el.classList.remove("blinking")
		localStorage.setItem(key, '1')
	})
}
