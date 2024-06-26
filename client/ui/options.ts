import { TabbedModal } from '../base'
import {
	models, default as options, optionType, storeBackground, storeMascot
} from '../options'
import { load, hook } from '../util'
import lang from '../lang'
import { clearHidden } from "../posts"
import { hidden } from "../state"

// Only instance of the options panel
export let panel: OptionsPanel

// View of the options panel
export default class OptionsPanel extends TabbedModal {
	private hidden: HTMLElement
	private import: HTMLInputElement

	constructor() {
		super(document.getElementById("options"))
		panel = this
		this.hidden = document.getElementById('hidden')
		this.import = this.el
			.querySelector("#importSettings") as HTMLInputElement

		this.onClick({
			'#export': () =>
				this.exportConfigs(),
			'#import': e =>
				this.importConfigs(e),
			'#hidden': clearHidden,
		})
		this.on('change', e =>
			this.applyChange(e))

		this.renderHidden(hidden.size)
		this.assignValues()

		hook("renderOptionValue", ([id, type, val]) =>
			this.assignValue(id, type, val))
		hook("renderHiddenCount", n =>
			this.renderHidden(n))
	}

	// Assign loaded option settings to the respective elements in the options
	// panel
	private assignValues() {
		for (let id in models) {
			const model = models[id],
				val = model.get()
			this.assignValue(id, model.spec.type, val)
		}
	}

	// Assign a single option value. Called on changes to the options externally
	// not from the options panel
	private assignValue(id: string, type: optionType, val: any) {
		if (type == optionType.none) {
			return
		}
		const el = document.getElementById(id) as HTMLInputElement

		switch (type) {
			case optionType.checkbox:
				el.checked = val as boolean
				break
			case optionType.number:
			case optionType.menu:
			case optionType.range:
			case optionType.textarea:
				el.value = val as string || ""
				break
			case optionType.shortcut:
				if(val == 13) {
					el.value = "Enter"
				} else {
					el.value = String.fromCodePoint(val as number).toUpperCase()
				}
				break
		}
		// 'image' type simply falls through, as those don't need to be set
	}

	// Propagate options panel changes through
	// options-panel -> options -> OptionModel
	private applyChange(event: Event) {
		const el = event.target as HTMLInputElement,
			id = el.getAttribute('id'),
			model = models[id]

		// Not an option input element
		if (!model) {
			return
		}

		let val: boolean | string | number
		switch (model.spec.type) {
			case optionType.checkbox:
				val = el.checked
				break
			case optionType.number:
			case optionType.range:
				val = parseInt(el.value)
				break
			case optionType.menu:
			case optionType.textarea:
				val = el.value
				break
			case optionType.shortcut:
				if(el.value.toLowerCase() == "enter") {
					val = 13;
				} else {
					val = el.value.toUpperCase().codePointAt(0)
				}
				break
			case optionType.image:
				// Not recorded. Extracted directly by the handler
				const file = (el as any).files[0]
				el.value = ""
				switch (id) {
					case "userBGImage":
						storeBackground(file)
						break
					case "mascotImage":
						storeMascot(file)
						break
				}
				return
		}

		options[id] = val
	}

	// Dump options to JSON file and upload to user
	private exportConfigs() {
		const a = document.getElementById('export')
		const blob = new Blob([JSON.stringify(localStorage)], {
			type: 'octet/stream'
		})
		a.setAttribute('href', window.URL.createObjectURL(blob))
		a.setAttribute('download', 'meguca-config.json')
	}

	// Import options from uploaded JSON file
	private importConfigs(event: Event) {
		// Proxy to hidden file input
		this.import.click()
		const handler = () =>
			this.importConfigFile()
		this.import.addEventListener("change", handler, { once: true })
	}

	// After the file has been uploaded, parse it and import the configs
	private async importConfigFile() {
		const reader = new FileReader()
		reader.readAsText(this.import.files[0])
		const event = await load(reader) as any

		// In case of corruption
		let json: { [key: string]: string }
		try {
			json = JSON.parse(event.target.result)
		}
		catch (err) {
			alert(lang.ui["importCorrupt"])
			return
		}

		localStorage.clear()
		for (let key in json) {
			localStorage.setItem(key, json[key])
		}
		alert(lang.ui["importDone"])
		location.reload()
	}

	// Render Hidden posts counter
	private renderHidden(count: number) {
		const el = this.hidden
		el.textContent = el.textContent.replace(/\d+$/, count.toString())
	}
}
