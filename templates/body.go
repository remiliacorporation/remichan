package templates

import (
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/bakape/meguca/common"
	"github.com/bakape/meguca/config"
	"github.com/bakape/meguca/util"

	"github.com/valyala/quicktemplate"
)

// Embeddable URL types
const (
	youTube = iota
	soundCloud
	vimeo
	coub
	bitChute
)

var (
	linkRegexp      = regexp.MustCompile(`^>>(>*)(\d+)$`)
	referenceRegexp = regexp.MustCompile(`^>>>(>*)\/(\w+)\/$`)

	providers = map[int]string{
		youTube:    "YouTube",
		soundCloud: "SoundCloud",
		vimeo:      "Vimeo",
		coub:       "Coub",
		bitChute:   "BitChute",
	}
	embedPatterns = [...]struct {
		typ  int
		patt *regexp.Regexp
	}{
		{
			youTube,
			regexp.MustCompile(`https?:\/\/(?:[^\.]+\.)?(?:youtu\.be\/|youtube\.com\/embed\/|youtube\.com\/watch\?v=)[a-zA-Z0-9_-]+`),
		},
		{
			soundCloud,
			regexp.MustCompile(`https?:\/\/soundcloud.com\/.*`),
		},
		{
			vimeo,
			regexp.MustCompile(`https?:\/\/(?:www\.)?vimeo\.com\/.+`),
		},
		{
			coub,
			regexp.MustCompile(`https?:\/\/coub\.com\/view\/[a-zA-Z0-9-_]+`),
		},
		{
			bitChute,
			regexp.MustCompile(`https?:\/\/(?:[^\.]+\.)?(?:bitchute\.com\/embed\/|bitchute\.com\/video\/)[a-zA-Z0-9_-]+`),
		},
	}

	// URLs supported for linkification
	urlPrefixes = map[byte]string{
		'h': "http",
		'm': "magnet:?",
		'f': "ftp",
		'b': "bitcoin",
	}
)

type bodyContext struct {
	index bool     // Rendered for an index page
	state struct { // Body parser state
		spoiler, quote, code, bold, italic, red, blue, rbText, pyu bool
		successiveNewlines                                         uint
		iDice                                                      int
	}
	common.Post
	OP    uint64
	board string
	quicktemplate.Writer
}

// Render the text body of a post
func streambody(
	w *quicktemplate.Writer,
	p common.Post,
	op uint64,
	board string,
	index bool,
	rbText bool,
	pyu bool,
) {
	c := bodyContext{
		index:  index,
		Post:   p,
		OP:     op,
		board:  board,
		Writer: *w,
	}
	c.state.rbText = rbText
	c.state.pyu = pyu

	var fn func(string)
	if c.Editing {
		fn = c.parseOpenLine
	} else {
		fn = c.parseTerminatedLine
	}

	for i, l := range strings.Split(c.Body, "\n") {
		c.state.quote = false

		// Prevent successive empty lines
		if i != 0 && c.state.successiveNewlines < 2 {
			c.string("<br>")
		}
		if len(l) == 0 {
			c.state.successiveNewlines++
			continue
		}

		c.state.successiveNewlines = 0
		if l[0] == '>' {
			c.string("<em>")
			c.state.quote = true
		}
		if c.state.spoiler {
			c.string("<del>")
		}
		if c.state.bold {
			c.string("<b>")
		}
		if c.state.italic {
			c.string("<i>")
		}
		if c.state.red {
			c.string("<span class=\"red\">")
		}
		if c.state.blue {
			c.string("<span class=\"blue\">")
		}

		fn(l)

		if c.state.blue {
			c.string("</span>")
		}
		if c.state.red {
			c.string("</span>")
		}
		if c.state.italic {
			c.string("</i>")
		}
		if c.state.bold {
			c.string("</b>")
		}
		if c.state.spoiler {
			c.string("</del>")
		}
		if c.state.quote {
			c.string("</em>")
		}
	}
}

// Open and close any tags up to level, if they are set.
// Increment level by 1 for each tag deeper you go.
func (c *bodyContext) wrapTags(level int) {
	states := [...]bool{
		c.state.spoiler,
		c.state.bold,
		c.state.italic,
		c.state.red,
		c.state.blue,
	}
	opening := [...]string{
		"<del>",
		"<b>",
		"<i>",
		"<span class=\"red\">",
		"<span class=\"blue\">",
	}
	closing := [...]string{
		"</del>",
		"</b>",
		"</i>",
		"</span>",
		"</span>",
	}

	for i := len(states) - 1; i >= level; i-- {
		if states[i] {
			c.string(closing[i])
		}
	}
	if !states[level] {
		c.string(opening[level])
	}
	for i := level + 1; i < len(states); i++ {
		if states[i] {
			c.string(opening[i])
		}
	}
}

// Write string without escaping
func (c *bodyContext) string(s string) {
	c.N().S(s)
}

// Escape and write string
func (c *bodyContext) escape(s string) {
	c.E().S(s)
}

// Write a byte without heap allocations or escaping
func (c *bodyContext) byte(b byte) {
	buf := [1]byte{b}
	c.N().SZ(buf[:])
}

// Parse a line that is no longer being edited
func (c *bodyContext) parseTerminatedLine(line string) {
	c.parseCode(line, (*c).parseFragment)
}

// Detect code tags
func (c *bodyContext) parseCode(frag string, fn func(string)) {
	for {
		i := strings.Index(frag, "``")
		if i != -1 {
			c.formatCode(frag[:i], fn)
			frag = frag[i+2:]
			c.state.code = !c.state.code
		} else {
			c.formatCode(frag, fn)
			break
		}
	}
}

func (c *bodyContext) formatCode(frag string, fn func(string)) {
	if c.state.code {
		// Strip quotes
		for len(frag) != 0 && frag[0] == '>' {
			c.string(`&gt;`)
			frag = frag[1:]
		}
		c.N().Z(highlightSyntax(frag))
	} else {
		c.parseSpoilers(frag, fn)
	}
}

// Inject spoiler tags and call fn on the remaining parts
func (c *bodyContext) parseSpoilers(frag string, fn func(string)) {
	_fn := func(frag string) {
		c.parseBolds(frag, fn)
	}

	for {
		i := strings.Index(frag, "**")
		if i != -1 {
			_fn(frag[:i])
			c.wrapTags(0)
			c.state.spoiler = !c.state.spoiler
			frag = frag[i+2:]
		} else {
			_fn(frag)
			break
		}
	}
}

// Inject bold tags and call fn on the remaining parts
func (c *bodyContext) parseBolds(frag string, fn func(string)) {
	_fn := func(frag string) {
		c.parseItalics(frag, fn)
	}

	for {
		i := strings.Index(frag, "@@")
		if i != -1 {
			_fn(frag[:i])
			c.wrapTags(1)
			c.state.bold = !c.state.bold
			frag = frag[i+2:]
		} else {
			_fn(frag)
			break
		}
	}
}

// Inject italic tags and call fn on the remaining parts
func (c *bodyContext) parseItalics(frag string, fn func(string)) {
	_fn := func(frag string) {
		c.parseReds(frag, fn)
	}

	for {
		i := strings.Index(frag, "~~")
		if i != -1 {
			_fn(frag[:i])
			c.wrapTags(2)
			c.state.italic = !c.state.italic
			frag = frag[i+2:]
		} else {
			_fn(frag)
			break
		}
	}
}

// Inject red color tags and call fn on the remaining parts
func (c *bodyContext) parseReds(frag string, fn func(string)) {
	_fn := func(frag string) {
		c.parseBlues(frag, fn)
	}
	_rbText := func() {}

	if c.state.rbText {
		_rbText = func() {
			c.wrapTags(3)
			c.state.red = !c.state.red
		}
	}

	for {
		i := strings.Index(frag, "^r")
		if i != -1 {
			_fn(frag[:i])
			_rbText()
			frag = frag[i+2:]
		} else {
			_fn(frag)
			break
		}
	}
}

// Inject blue color tags and call fn on the remaining parts
func (c *bodyContext) parseBlues(frag string, fn func(string)) {
	_rbText := func() {}

	if c.state.rbText {
		_rbText = func() {
			c.wrapTags(4)
			c.state.blue = !c.state.blue
		}
	}

	for {
		i := strings.Index(frag, "^b")
		if i != -1 {
			fn(frag[:i])
			_rbText()
			frag = frag[i+2:]
		} else {
			fn(frag)
			break
		}
	}
}

// Parse a line fragment
func (c *bodyContext) parseFragment(frag string) {
	// Leading and trailing punctuation, if any
	var leadPunct, trailPunct byte

	for i, word := range strings.Split(frag, " ") {
		if i != 0 {
			c.byte(' ')
		}

		// Strip leading and trailing punctuation and commit separately
		leadPunct, word, trailPunct = util.SplitPunctuationString(word)
		if leadPunct != 0 {
			c.byte(leadPunct)
		}
		if (strings.Count(word, "(") == strings.Count(word, ")")+1) &&
			(trailPunct == ')') && (strings.Contains(word, "http")) {
			word += ")"
			trailPunct = ' '
		}

		if len(word) == 0 {
			goto end
		}
		switch word[0] {
		case '#': // Hash commands
			if c.state.quote {
				goto end
			}
			if m := common.CommandRegexp.FindStringSubmatch(word); m != nil {
				c.parseCommands(string(m[1]))
				goto end
			}
		case '>': // Links
			if m := linkRegexp.FindStringSubmatch(word); m != nil {
				// Post links
				c.parsePostLink(m)
				goto end
			} else if m := referenceRegexp.FindStringSubmatch(word); m != nil {
				// Internal and custom reference URLs
				c.parseReference(m)
				goto end
			}
			fallthrough
		default:
			// Strip leading '>', if any
			leadingGt := 0
			stripped := word
			for len(stripped) != 0 && stripped[0] == '>' {
				stripped = stripped[1:]
				leadingGt++
			}

			// Generic HTTP(S) URLs and magnet links
			// Checking the first byte is much cheaper than a function call. Do
			// that first, as most cases won't match.
			if len(stripped) != 0 {
				pre, ok := urlPrefixes[stripped[0]]
				if ok && strings.HasPrefix(stripped, pre) {
					for i := 0; i < leadingGt; i++ {
						c.byte('>')
					}
					c.parseURL(stripped)
					goto end
				}
			}
		}

		c.escape(word)

	end:
		// Write trailing punctuation, if any
		if trailPunct != 0 {
			c.byte(trailPunct)
		}
	}
}

// Parse a potential link to a post
func (c *bodyContext) parsePostLink(m []string) {
	if c.Links == nil {
		c.string(m[0])
		return
	}

	id, _ := strconv.ParseUint(string(m[2]), 10, 64)
	var data common.Link
	for _, l := range c.Links {
		if l.ID == id {
			data = l
			break
		}
	}
	if data.ID == 0 {
		c.string(m[0])
		return
	}

	if len(m[1]) != 0 { // Write extra quotes
		c.string(m[1])
	}
	streampostLink(&c.Writer, data, c.index || data.OP != c.OP, c.index)
}

// Parse internal or customly set reference URL
func (c *bodyContext) parseReference(m []string) {
	var (
		m2   = string(m[2])
		href string
	)
	if config.IsBoard(m2) {
		href = fmt.Sprintf("/%s/", m2)
	} else if href = config.Get().Links[m2]; href != "" {
	} else {
		c.string(m[0])
		return
	}

	if len(m[1]) != 0 {
		c.string(m[1])
	}
	c.newTabLink(href, fmt.Sprintf(">>>/%s/", string(m[2])))
}

// Format and anchor link that opens in a new tab
func (c *bodyContext) newTabLink(href, text string) {
	c.string(`<a rel="noreferrer" href="`)
	c.escape(href)
	c.string(`" target="_blank">`)
	c.escape(text)
	c.string(`</a>`)
}

// Parse generic URLs and magnet links
func (c *bodyContext) parseURL(bit string) {
	s := string(bit)
	u, err := url.Parse(s)
	switch {
	case err != nil || u.Path == s: // Invalid or empty path
		c.escape(bit)
	case c.parseEmbeds(bit):
	case bit[0] == 'm': // Don't open a new tab for magnet links
		s = html.EscapeString(s)
		c.string(`<a rel="noreferrer" href="`)
		c.string(s)
		c.string(`">`)
		c.string(s)
		c.string(`</a>`)
	default:
		c.newTabLink(s, s)
	}
}

// Parse select embeddable URLs. Returns, if any found.
func (c *bodyContext) parseEmbeds(s string) bool {
	for _, t := range embedPatterns {
		if !t.patt.MatchString(s) {
			continue
		}

		c.string(`<em><a rel="noreferrer" class="embed" target="_blank" data-type="`)
		c.N().D(t.typ)
		c.string(`" href="`)
		c.escape(s)
		c.string(`">[`)
		c.string(providers[t.typ])
		c.string(`] ???</a></em>`)

		return true
	}
	return false
}

// Parse a hash command
func (c *bodyContext) parseCommands(bit string) {
	// Guard against invalid dice rolls
	if c.Commands == nil || c.state.iDice > len(c.Commands)-1 {
		c.writeInvalidCommand(bit)
		return
	}

	formatting := "<strong>"
	inner := make([]byte, 0, 32)
	val := c.Commands[c.state.iDice]

	// Protect from index shifts on boardConfig.pyu toggle
	if !c.state.pyu {
		switch val.Type {
		case common.Pyu, common.Pcount:
			c.state.iDice++
			c.writeInvalidCommand(bit)
			return
		}
	}

	switch bit {
	case "flip":
		var s string
		if val.Flip {
			s = "flap"
		} else {
			s = "flop"
		}
		inner = append(inner, s...)
		c.state.iDice++
	case "8ball":
		inner = append(inner, html.EscapeString(val.Eightball)...)
		c.state.iDice++
	case "pyu", "pcount", "rcount":
		switch val.Type {
		case common.Pyu, common.Pcount:
			// Protect from index shifts on boardConfig.pyu toggle
			if !c.state.pyu {
				c.writeInvalidCommand(bit)
				return
			}
			fallthrough
		case common.Rcount:
			inner = strconv.AppendUint(inner, val.Pyu, 10)
			c.state.iDice++
			break
		default:
			c.writeInvalidCommand(bit)
		}
	case "roulette":
		inner = strconv.AppendUint(inner, uint64(val.Roulette[0]), 10)
		inner = append(inner, "/"...)
		inner = strconv.AppendUint(inner, uint64(val.Roulette[1]), 10)
		// set formatting if the poster died
		if val.Roulette[0] == 1 {
			formatting = "<strong class=\"dead\">"
		}
		c.state.iDice++
	default:
		if strings.HasPrefix(bit, "sw") {
			c.formatSyncwatch(val.SyncWatch)
			c.state.iDice++
			return
		}

		// Validate dice
		m := common.DiceRegexp.FindStringSubmatch(bit)
		rolls := 1
		if m[1] != "" {
			var err error
			if rolls, err = strconv.Atoi(m[1]); err != nil || rolls > 10 {
				c.writeInvalidCommand(bit)
				return
			}
		}
		sides, err := strconv.Atoi(m[2])
		if err != nil || sides > common.MaxDiceSides {
			c.writeInvalidCommand(bit)
			return
		}

		c.state.iDice++
		var sum uint64
		for i, roll := range val.Dice {
			if i != 0 {
				inner = append(inner, " + "...)
			}
			sum += uint64(roll)
			inner = strconv.AppendUint(inner, uint64(roll), 10)
		}
		if len(val.Dice) > 1 {
			inner = append(inner, " = "...)
			inner = strconv.AppendUint(inner, sum, 10)
		}

		formatting = getRollFormatting(uint64(rolls), uint64(sides), sum)
	}

	c.string(formatting)
	c.string(`#`)
	c.string(bit)
	c.string(` (`)
	c.N().Z(inner)
	c.string(`)</strong>`)
}

func getRollFormatting(numberOfDice uint64, facesPerDie uint64, sum uint64) string {
	maxRoll := numberOfDice * facesPerDie
	// no special formatting for small rolls
	if maxRoll < 10 || facesPerDie == 1 {
		return "<strong>"
	}

	if maxRoll == sum {
		return "<strong class=\"super_roll\">"
	} else if sum == numberOfDice {
		return "<strong class=\"kuso_roll\">"
	} else if sum == 69 || sum == 6969 {
		return "<strong class=\"lewd_roll\">"
	} else if checkEm(sum) {
		if sum < 100 {
			return "<strong class=\"dubs_roll\">"
		} else if sum < 1000 {
			return "<strong class=\"trips_roll\">"
		} else if sum < 10000 {
			return "<strong class=\"quads_roll\">"
		} else { // QUINTS!!!
			return "<strong class=\"rainbow_roll\">"
		}
	}
	return "<strong>"
}

// If num is made of the same digit repeating
func checkEm(num uint64) bool {
	if num < 10 {
		return false
	}
	digit := num % 10
	for {
		num /= 10
		if num == 0 {
			return true
		}
		if num%10 != digit {
			return false
		}
	}
}

// Format a synchronized time counter
func (c *bodyContext) formatSyncwatch(val [5]uint64) {
	c.string(`<em><strong class="embed syncwatch" data-hour=`)
	c.uint64(val[0])
	c.string(` data-min=`)
	c.uint64(val[1])
	c.string(` data-sec=`)
	c.uint64(val[2])
	c.string(` data-start=`)
	c.uint64(val[3])
	c.string(` data-end=`)
	c.uint64(val[4])
	c.string(`>syncwatch</strong></em>`)
}

func (c *bodyContext) uint64(i uint64) {
	c.string(strconv.FormatUint(i, 10))
}

// If command validation failed, simply write the string
func (c *bodyContext) writeInvalidCommand(s string) {
	c.byte('#')
	c.escape(s)
}

// Parse a line that is still being edited
func (c *bodyContext) parseOpenLine(line string) {
	c.parseCode(line, func(s string) {
		c.escape(s)
	})
}
