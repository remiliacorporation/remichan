// Form input field rendering

package templates

import (
	"html"
	"strconv"
	"strings"

	"github.com/bakape/meguca/lang"
	"github.com/valyala/quicktemplate"
)

// Input field types
type inputType uint8

const (
	_bool inputType = iota
	_number
	_string
	_select
	_textarea
	_password
	_map
	_array
	_image
	_metashortcut
	_shortcut
	_range
	_hr
)

// Spec of an option passed into the rendering function
type inputSpec struct {
	Type                                   inputType
	Required, Placeholder, NoID, WrapLabel bool
	Min, Max, MaxLength, Rows              int
	ID, Pattern, Autocomplete              string
	Options                                []string
	Val                                    interface{}
}

// Returns a copy of s, that will render wrapped indside a label
func (s inputSpec) wrap() inputSpec {
	s.NoID = true
	s.WrapLabel = true
	return s
}

// For constructing various HTML input forms
type formWriter struct {
	quicktemplate.Writer
	lang lang.Pack
}

// Write an element attribute to the buffer
func (w *formWriter) attr(key, val string) {
	w.N().S(` `)
	w.N().S(key)
	if val != "" {
		w.N().S(`="`)
		w.N().S(val)
		w.N().S(`"`)
	}
}

func (w *formWriter) typ(val string) {
	w.attr("type", val)
}

// Write an input element from the spec to the buffer
func (w *formWriter) input(spec inputSpec) {
	cont := false
	switch spec.Type {
	case _select:
		w.sel(spec)
	case _textarea:
		w.textArea(spec)
	case _map:
		streamrenderMap(&w.Writer, spec)
	case _array:
		streamrenderArray(&w.Writer, spec)
	case _shortcut:
		w.N().S("Alt+")
		cont = true
	case _metashortcut:
		cont = true
	case _hr:
		w.N().S("<hr>")
	default:
		cont = true
	}
	if !cont {
		return
	}

	w.tag("input", spec)

	switch spec.Type {
	case _bool:
		w.typ("checkbox")
		if spec.Val != nil && spec.Val.(bool) {
			w.attr("checked", "")
		}
	case _number:
		w.typ("number")
		if spec.Val != nil {
			cast := uint64(spec.Val.(uint))
			w.attr("value", strconv.FormatUint(cast, 10))
		}
		w.attr("min", strconv.Itoa(spec.Min))
		if spec.Max != 0 {
			w.attr("max", strconv.Itoa(spec.Max))
		}
	case _range:
		w.typ("range")
		w.attr("min", strconv.Itoa(spec.Min))
		w.attr("max", strconv.Itoa(spec.Max))
	case _password, _string:
		if spec.Type == _string {
			w.typ("text")
		} else {
			w.typ("password")
		}
		if spec.Val != nil {
			w.attr("value", html.EscapeString(spec.Val.(string)))
		}
		if spec.Pattern != "" {
			w.attr("pattern", spec.Pattern)
		}
		if spec.MaxLength != 0 {
			w.attr("maxlength", strconv.Itoa(spec.MaxLength))
		}
		if spec.Autocomplete != "" {
			w.attr("autocomplete", spec.Autocomplete)
		}
	case _image:
		w.typ("file")
		w.attr("accept", "image/png,image/gif,image/jpeg")
	case _metashortcut, _shortcut:
		w.attr("maxlength", "5")
		w.attr("class", "shortcut")
	}

	w.N().S(`>`)
}

// Write the element tag and the common parts of all input element types to
// buffer
func (w *formWriter) tag(tag string, spec inputSpec) {
	w.N().S(`<`)
	w.N().S(tag)
	w.attr("name", spec.ID)
	if !spec.NoID { // To not conflict with non-unique labels
		w.attr("id", spec.ID)
	}
	w.attr("title", w.lang.Forms[spec.ID][1])
	if spec.Placeholder {
		w.attr("placeholder", w.lang.Forms[spec.ID][0])
	}
	if spec.Required {
		w.attr("required", "")
	}
}

// Write a select element to buffer
func (w *formWriter) sel(spec inputSpec) {
	w.tag("select", spec)
	w.N().S(`>`)

	var val string
	if spec.Val != nil {
		val = spec.Val.(string)
	}

	for _, o := range spec.Options {
		w.N().S("<option")
		w.attr("value", o)
		if o == val {
			w.attr("selected", "selected")
		}
		w.N().S(`>`)

		label, ok := w.lang.Options[spec.ID]
		if !ok {
			label = o
		}
		w.N().S(label)

		w.N().S("</option>")
	}

	w.N().S("</select>")
}

// Render a text area input element
func (w *formWriter) textArea(spec inputSpec) {
	w.tag("textarea", spec)
	if spec.MaxLength != 0 {
		w.attr("maxlength", strconv.Itoa(spec.MaxLength))
	}
	w.attr("rows", strconv.Itoa(spec.Rows))
	w.N().S(`>`)

	switch spec.Val.(type) {
	case string:
		w.E().S(spec.Val.(string))
	case []string:
		w.E().S(strings.Join(spec.Val.([]string), "\n"))
	}

	w.N().S("</textarea>")
}

// Write an input element label from the spec to the buffer
func (w *formWriter) label(spec inputSpec, inside *func()) {
	ln := w.lang.Forms[spec.ID]

	w.N().S("<label")
	if !spec.NoID {
		w.attr("for", spec.ID)
	}
	w.attr("title", ln[1])
	w.N().S(`>`)
	if inside != nil {
		(*inside)()
	}
	w.N().S(ln[0])
	w.N().S("</label>")
}

// Render a table containing {label input_element} pairs
func streamtable(qw *quicktemplate.Writer, specs []inputSpec) {
	w := formWriter{
		Writer: *qw,
		lang:   lang.Get(),
	}
	w.N().S("<table>")

	for _, spec := range specs {
		w.N().S("<tr><td>")
		if spec.Type != _hr {
			w.label(spec, nil)
		}
		w.N().S("</td><td>")
		w.input(spec)
		w.N().S("</td></tr>")
	}

	w.N().S("</table>")
}

// Render a single input element
func streaminput(qw *quicktemplate.Writer, spec inputSpec, lang lang.Pack) {
	w := formWriter{
		Writer: *qw,
		lang:   lang,
	}
	if spec.WrapLabel {
		f := func() {
			w.input(spec)
		}
		w.label(spec, &f)
	} else {
		w.input(spec)
		if !spec.Placeholder {
			w.label(spec, nil)
		}
	}
	w.N().S(`<br>`)
}

// Render the options inputs of an options panel
func streamoptions(qw *quicktemplate.Writer, specs []inputSpec, ln lang.Pack) {
	w := formWriter{
		Writer: *qw,
		lang:   ln,
	}
	for _, s := range specs {
		w.input(s)
		if s.Type != _hr {
			w.label(s, nil)
			w.N().S(`<br>`)
		}
	}
}
