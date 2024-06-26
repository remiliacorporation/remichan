package websockets

import (
	"database/sql"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/bakape/meguca/common"
	"github.com/bakape/meguca/db"
	. "github.com/bakape/meguca/test"
	"github.com/bakape/meguca/test/test_db"
	"github.com/bakape/meguca/websockets/feeds"
)

// Sample wall of text
const longPost = `Shut the fuck up. I'm so tired of being disrespected on this
goddamn website. All I wanted to do was post my opinion. MY OPINION. But no,
you little bastards think it's "hilarious" to mock those with good opinions.
My opinion. while not absolute, is definitely worth the respect to formulate
an ACTUAL FUCKING RESPONSE AND NOT JUST A SHORT MEME OF A REPLY. I've been on
this site for 6 months: 6 MONTHS and I have never felt this wronged. It boils
me up that I could spend so much time thinking and putting effort into things
while you shits sit around (probably jerking off to Gardevoir or whatever
furbait you like) and make fun of the intellectuals of this world. You're
laughing at me? Good for fucking you. Literally no one cares that your little
brain is to underdeveloped and rotted to comprehend this game...THIS GREAT
GREAT GAME. I could sit here all day whining, but I won't. I'm NOT a whiner.
I'm a realist and an intellectual. I know when to call it quits and to leave
the babybrains to themselves. I'm done with this goddamn site and you goddamn
immature children. I have lived my life up until this point having to deal
with memesters and idiots like you. I know how you work. I know that you all
think you're "epik trolls" but you're not. You think you baited me? NAH. I've
never taken any bait. This is my 100% real opinion divorced from anger. I'm
calm, I'm serene. I LAUGH when people imply I'm intellectually low enough to
take bait. I always choose to reply just to spite you. I won. I've always won.
Losing is not in my skillset. So you're probably gonna reply "lol epik
trolled" or "u mad bro" but once you've done that you've shown me I've won.
I've tricked the trickster and conquered memery. I live everyday growing
stronger to fight you plebs and low level trolls who are probably 11 (baby,
you gotta be 18 to use 4chan). But whatever, I digress. It's just fucking
annoying that I'm never taken serious on this site, goddamn.`

var (
	samplePost = db.Post{
		StandalonePost: common.StandalonePost{
			Post: common.Post{
				Editing: true,
				ID:      2,
				Body:    "abc",
				Time:    time.Now().Unix(),
			},
			OP:    1,
			Board: "a",
		},
	}
)

func TestLineEmpty(t *testing.T) {
	t.Parallel()

	sv := newWSServer(t)
	defer sv.Close()

	cl, _ := sv.NewClient()
	cl.post.id = 1
	cl.post.time = time.Now().Unix()
	if err := cl.backspace(); err != errEmptyPost {
		t.Errorf("unexpected error by %s: %s", "Client.backspace", err)
	}
}

func TestAppendBodyTooLong(t *testing.T) {
	t.Parallel()

	sv := newWSServer(t)
	defer sv.Close()

	cl, _ := sv.NewClient()
	cl.post = openPost{
		id:   1,
		time: time.Now().Unix(),
		len:  common.MaxLenBody,
	}
	if err := cl.appendRune(nil); err != common.ErrBodyTooLong {
		UnexpectedError(t, err)
	}
}

func TestAppendRune(t *testing.T) {
	feeds.Clear()
	test_db.ClearTables(t, "boards")
	test_db.WriteSampleBoard(t)
	test_db.WriteSampleThread(t)
	writeSamplePost(t)

	sv := newWSServer(t)
	defer sv.Close()
	cl, _ := sv.NewClient()
	registerClient(t, cl, 1, "a")
	cl.post = openPost{
		id:    2,
		op:    1,
		len:   3,
		board: "a",
		time:  time.Now().Unix(),
		body:  []byte("abc"),
	}

	if err := cl.appendRune([]byte("100")); err != nil {
		t.Fatal(err)
	}

	assertOpenPost(t, cl, 4, "abcd")
	awaitFlush()
	assertBody(t, 2, "abcd")
}

func awaitFlush() {
	time.Sleep(time.Millisecond * 400)
}

func writeSamplePost(t testing.TB) {
	t.Helper()
	err := db.InTransaction(false, func(tx *sql.Tx) error {
		return db.WritePost(tx, samplePost)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func assertOpenPost(t *testing.T, cl *Client, len int, buf string) {
	t.Helper()
	if l := cl.post.len; l != len {
		t.Errorf("unexpected openPost body length: %d", l)
	}
	if s := string(cl.post.body); s != buf {
		t.Errorf("unexpected openPost buffer contents: `%s`", s)
	}
}

func assertBody(t *testing.T, id uint64, body string) {
	t.Helper()

	post, err := db.GetPost(id)
	if err != nil {
		t.Fatal(err)
	}
	if post.Body != body {
		LogUnexpected(t, body, post.Body)
	}
}

func BenchmarkAppend(b *testing.B) {
	feeds.Clear()
	test_db.ClearTables(b, "boards")
	test_db.WriteSampleBoard(b)
	test_db.WriteSampleThread(b)
	writeSamplePost(b)

	sv := newWSServer(b)
	defer sv.Close()
	cl, _ := sv.NewClient()
	cl.post = openPost{
		id:   2,
		op:   1,
		len:  3,
		time: time.Now().Unix(),
		body: []byte("abc"),
	}
	registerClient(b, cl, 1, "a")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := cl.appendRune([]byte("100")); err != nil {
			b.Fatal(err)
		}
	}
}

func TestClosePostWithHashCommand(t *testing.T) {
	feeds.Clear()
	test_db.ClearTables(t, "boards")
	test_db.WriteSampleBoard(t)
	test_db.WriteSampleThread(t)

	post := db.Post{
		StandalonePost: common.StandalonePost{
			Post: common.Post{
				ID:   2,
				Body: "#flip",
			},
			OP: 1,
		},
	}
	err := db.InTransaction(false, func(tx *sql.Tx) error {
		return db.WritePost(tx, post)
	})
	if err != nil {
		t.Fatal(err)
	}

	sv := newWSServer(t)
	defer sv.Close()
	cl, _ := sv.NewClient()
	cl.post = openPost{
		id:    2,
		op:    1,
		len:   5,
		board: "a",
		time:  time.Now().Unix(),
		body:  []byte("#flip"),
	}

	if err := cl.closePost(); err != nil {
		t.Fatal(err)
	}

	t.Run("command type", func(t *testing.T) {
		t.Parallel()

		post, err := db.GetPost(2)
		if err != nil {
			t.Fatal(err)
		}
		if len(post.Commands) == 0 {
			t.Fatal("no commands written")
		}
		if typ := post.Commands[0].Type; typ != common.Flip {
			t.Errorf("unexpected command type: %d", typ)
		}
	})
}

func TestClosePostWithLinks(t *testing.T) {
	feeds.Clear()
	test_db.ClearTables(t, "boards")
	test_db.WriteSampleBoard(t)
	test_db.WriteSampleThread(t)

	thread := db.Thread{
		ID:    21,
		Board: "a",
	}
	op := db.Post{
		StandalonePost: common.StandalonePost{
			Post: common.Post{
				ID: 21,
			},
			OP: 21,
		},
	}
	if err := db.WriteThread(thread, op); err != nil {
		t.Fatal(err)
	}

	posts := [...]db.Post{
		{
			StandalonePost: common.StandalonePost{
				Post: common.Post{
					ID:   2,
					Body: " >>22 ",
				},
				Board: "a",
				OP:    1,
			},
		},
		{
			StandalonePost: common.StandalonePost{
				Post: common.Post{
					ID: 22,
				},
				OP:    21,
				Board: "c",
			},
		},
	}
	err := db.InTransaction(false, func(tx *sql.Tx) error {
		for _, p := range posts {
			if err := db.WritePost(tx, p); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	sv := newWSServer(t)
	defer sv.Close()
	cl, _ := sv.NewClient()
	cl.post = openPost{
		id:    2,
		op:    1,
		len:   3,
		board: "a",
		time:  time.Now().Unix(),
		body:  []byte(" >>22 "),
	}
	setBoardConfigs(t, false)

	if err := cl.closePost(); err != nil {
		t.Fatal(err)
	}

	post, err := db.GetPost(2)
	if err != nil {
		t.Fatal(err)
	}
	AssertEquals(t, post.Links, []common.Link{
		{
			ID:    22,
			OP:    21,
			Board: "a",
		},
	})
}

func TestBackspace(t *testing.T) {
	feeds.Clear()
	test_db.ClearTables(t, "boards")
	test_db.WriteSampleBoard(t)
	test_db.WriteSampleThread(t)
	writeSamplePost(t)

	sv := newWSServer(t)
	defer sv.Close()
	cl, _ := sv.NewClient()
	registerClient(t, cl, 1, "a")
	cl.post = openPost{
		id:   2,
		op:   1,
		len:  3,
		time: time.Now().Unix(),
		body: []byte("abc"),
	}

	if err := cl.backspace(); err != nil {
		t.Fatal(err)
	}

	assertOpenPost(t, cl, 2, "ab")
	awaitFlush()
	assertBody(t, 2, "ab")
}

func TestClosePost(t *testing.T) {
	feeds.Clear()
	test_db.ClearTables(t, "boards")
	test_db.WriteSampleBoard(t)
	test_db.WriteSampleThread(t)
	writeSamplePost(t)

	sv := newWSServer(t)
	defer sv.Close()
	cl, _ := sv.NewClient()
	registerClient(t, cl, 1, "a")
	cl.post = openPost{
		id:    2,
		op:    1,
		len:   3,
		board: "a",
		body:  []byte("abc"),
	}
	cl.feed.InsertPost(samplePost.Post, nil)

	if err := cl.closePost(); err != nil {
		t.Fatal(err)
	}

	AssertEquals(t, cl.post, openPost{})
	assertBody(t, 2, "abc")
	assertPostClosed(t, 2)
}

func assertPostClosed(t *testing.T, id uint64) {
	t.Helper()

	post, err := db.GetPost(id)
	if err != nil {
		t.Fatal(err)
	}
	if post.Editing {
		t.Error("post not closed")
	}
}

func TestSpliceValidityChecks(t *testing.T) {
	t.Parallel()

	sv := newWSServer(t)
	defer sv.Close()
	cl, _ := sv.NewClient()
	cl.post = openPost{
		id:   2,
		time: time.Now().Unix(),
	}

	var tooLong string
	for i := 0; i < 2001; i++ {
		tooLong += "a"
	}

	cases := [...]struct {
		name       string
		start, len uint
		text, line string
		err        error
	}{
		{
			"exceeds buffer bounds",
			2, 1,
			"", "abc",
			&errInvalidSpliceCoords{
				body: "",
				req: spliceRequestString{
					spliceCoords: spliceCoords{
						Start: 2,
						Len:   1,
					},
					Text: "",
				},
			},
		},
		{"NOOP", 0, 0, "", "", errSpliceNOOP},
		{"too long", 0, 0, tooLong, "", errSpliceTooLong},
	}

	for i := range cases {
		c := cases[i]
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			req := spliceRequest{
				spliceCoords: spliceCoords{
					Start: c.start,
					Len:   c.len,
				},
				Text: []rune(c.text),
			}
			AssertEquals(t, c.err, cl.spliceText(marshalJSON(t, req)))
		})
	}
}

func TestSplice(t *testing.T) {
	feeds.Clear()
	test_db.ClearTables(t, "boards")
	test_db.WriteSampleBoard(t)
	setBoardConfigs(t, false)

	const longSplice = `Never gonna give you up Never gonna let you down Never gonna run around and desert you Never gonna make you cry Never gonna say goodbye Never gonna tell a lie and hurt you `

	sv := newWSServer(t)
	defer sv.Close()

	cases := [...]struct {
		name                   string
		start, len             uint
		text, init, final, log string
	}{
		{
			name:  "append to empty body",
			start: 0,
			len:   0,
			text:  "abc",
			init:  "",
			final: "abc",
			log:   `05{"id":2,"start":0,"len":0,"text":"abc"}`,
		},
		{
			name:  "remove one char",
			start: 0,
			len:   1,
			text:  "",
			init:  "abc",
			final: "bc",
			log:   `05{"id":2,"start":0,"len":1,"text":""}`,
		},
		{
			name:  "remove one multibyte char",
			start: 2,
			len:   1,
			text:  "",
			init:  "αΒΓΔ",
			final: "αΒΔ",
			log:   `05{"id":2,"start":2,"len":1,"text":""}`,
		},
		{
			name:  "inject into the middle",
			start: 2,
			len:   0,
			text:  "abc",
			init:  "abc",
			final: "ababcc",
			log:   `05{"id":2,"start":2,"len":0,"text":"abc"}`,
		},
		{
			name:  "inject multibyte char into the middle",
			start: 2,
			len:   0,
			text:  "Δ",
			init:  "αΒΓ",
			final: "αΒΔΓ",
			log:   `05{"id":2,"start":2,"len":0,"text":"Δ"}`,
		},
		{
			name:  "injection exceeds max body length",
			start: 1943,
			len:   0,
			text:  longSplice,
			init:  longPost,
			final: longPost[:1943] + longSplice[:57],
			log:   `05{"id":2,"start":1943,"len":-1,"text":"Never gonna give you up Never gonna let you down Never go"}`,
		},
		{
			name:  "append exceeds max body length",
			start: 1951,
			len:   0,
			text:  longSplice + "\n",
			init:  longPost,
			final: longPost + longSplice[:49],
			log:   `05{"id":2,"start":1951,"len":-1,"text":"Never gonna give you up Never gonna let you down "}`,
		},
	}

	for i := range cases {
		c := cases[i]
		t.Run(c.name, func(t *testing.T) {
			test_db.ClearTables(t, "threads")
			test_db.WriteSampleThread(t)

			post := db.Post{
				StandalonePost: common.StandalonePost{
					Post: common.Post{
						Editing: true,
						ID:      2,
						Body:    c.init,
					},
					Board: "a",
					OP:    1,
				},
			}
			err := db.InTransaction(false, func(tx *sql.Tx) error {
				return db.WritePost(tx, post)
			})
			if err != nil {
				t.Fatal(err)
			}

			cl, _ := sv.NewClient()
			registerClient(t, cl, 1, "a")
			cl.post = openPost{
				id:    2,
				op:    1,
				len:   utf8.RuneCountInString(c.init),
				board: "a",
				time:  time.Now().Unix(),
				body:  []byte(c.init),
			}

			req := spliceRequest{
				spliceCoords: spliceCoords{
					Start: c.start,
					Len:   c.len,
				},
				Text: []rune(c.text),
			}

			if err := cl.spliceText(marshalJSON(t, req)); err != nil {
				t.Fatal(err)
			}

			assertOpenPost(t, cl, utf8.RuneCountInString(c.final), c.final)
			awaitFlush()
			assertBody(t, 2, c.final)
		})
	}
}

func TestCloseOldOpenPost(t *testing.T) {
	feeds.Clear()
	test_db.ClearTables(t, "boards")
	test_db.WriteSampleBoard(t)
	test_db.WriteSampleThread(t)

	then := time.Now().Add(time.Minute * -30).Unix()
	post := db.Post{
		StandalonePost: common.StandalonePost{
			Post: common.Post{
				Editing: true,
				ID:      2,
				Time:    then,
			},
			OP: 1,
		},
	}
	err := db.InTransaction(false, func(tx *sql.Tx) error {
		return db.WritePost(tx, post)
	})
	if err != nil {
		t.Fatal(err)
	}

	sv := newWSServer(t)
	defer sv.Close()
	cl, _ := sv.NewClient()
	cl.post = openPost{
		id:   2,
		op:   1,
		time: then,
	}

	has, err := cl.hasPost()
	switch {
	case err != nil:
		t.Fatal(err)
	case has:
		t.Error("client has open post")
	}

	assertPostClosed(t, 2)
}

func TestInsertImageIntoPostWithImage(t *testing.T) {
	t.Parallel()

	sv := newWSServer(t)
	defer sv.Close()
	cl, _ := sv.NewClient()
	cl.post = openPost{
		id:       1,
		time:     time.Now().Unix(),
		hasImage: true,
	}
	if err := cl.insertImage(nil); err != errHasImage {
		UnexpectedError(t, err)
	}
}

func TestInsertImageOnTextOnlyBoard(t *testing.T) {
	setBoardConfigs(t, true)

	sv := newWSServer(t)
	defer sv.Close()
	cl, _ := sv.NewClient()
	cl.post = openPost{
		id:    1,
		board: "a",
		time:  time.Now().Unix(),
	}

	req := ImageRequest{
		Name:  "foo.jpeg",
		Token: "123",
	}
	if err := cl.insertImage(marshalJSON(t, req)); err != errTextOnly {
		UnexpectedError(t, err)
	}
}

func TestInsertImage(t *testing.T) {
	feeds.Clear()
	test_db.ClearTables(t, "boards", "images")
	test_db.WriteSampleBoard(t)
	test_db.WriteSampleThread(t)
	writeSampleImage(t)
	setBoardConfigs(t, false)

	post := db.Post{
		StandalonePost: common.StandalonePost{
			Post: common.Post{
				ID: 2,
			},
			Board: "a",
			OP:    1,
		},
	}
	err := db.InTransaction(false, func(tx *sql.Tx) error {
		return db.WritePost(tx, post)
	})
	if err != nil {
		t.Fatal(err)
	}

	var token string
	err = db.InTransaction(false, func(tx *sql.Tx) (err error) {
		token, err = db.NewImageToken(tx, stdJPEG.SHA1)
		return
	})
	if err != nil {
		t.Fatal(err)
	}

	sv := newWSServer(t)
	defer sv.Close()
	cl, _ := sv.NewClient()
	registerClient(t, cl, 1, "a")
	cl.post = openPost{
		id:    2,
		board: "a",
		op:    1,
		time:  time.Now().Unix(),
	}

	req := ImageRequest{
		Name:  "foo.jpeg",
		Token: token,
	}
	if err := cl.insertImage(marshalJSON(t, req)); err != nil {
		t.Fatal(err)
	}

	if !cl.post.hasImage {
		t.Error("no image flag on openPost")
	}
}

func writeSampleImage(t *testing.T) {
	t.Helper()
	if err := db.WriteImage(stdJPEG); err != nil {
		t.Fatal(err)
	}
}
