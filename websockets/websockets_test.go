package websockets

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/bakape/meguca/auth"
	"github.com/bakape/meguca/common"
	"github.com/bakape/meguca/db"
	. "github.com/bakape/meguca/test"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-playground/log"
	"github.com/go-playground/log/handlers/console"
	"github.com/gorilla/websocket"
)

const (
	invalidMessage   = "invalid message:"
	onlyText         = "only text frames allowed"
	closeNormal      = "websocket: close 1000"
	invalidCharacter = "invalid character"
)

var (
	dialer = websocket.Dialer{}
	con    = console.New(true)
)

type mockWSServer struct {
	t          testing.TB
	server     *httptest.Server
	connSender chan *websocket.Conn
	sync.WaitGroup
}

func TestMain(m *testing.M) {
	close, err := db.LoadTestDB("websockets")
	if err != nil {
		panic(err)
	}

	log.AddHandler(con, log.AllLevels...)

	code := m.Run()
	err = close()
	if err != nil {
		panic(err)
	}
	os.Exit(code)
}

func newWSServer(t testing.TB) *mockWSServer {
	t.Helper()

	connSender := make(chan *websocket.Conn)
	handler := func(res http.ResponseWriter, req *http.Request) {
		conn, err := upgrader.Upgrade(res, req, nil)
		if err != nil {
			t.Fatal(err)
		}
		connSender <- conn
	}
	return &mockWSServer{
		t:          t,
		connSender: connSender,
		server:     httptest.NewServer(http.HandlerFunc(handler)),
	}
}

func (m *mockWSServer) Close() {
	m.t.Helper()

	m.server.CloseClientConnections()
	m.server.Close()
	close(m.connSender)
}

func (m *mockWSServer) NewClient() (*Client, *websocket.Conn) {
	m.t.Helper()

	wcl := dialServer(m.t, m.server)
	r := httptest.NewRequest("GET", "/", nil)
	ip, err := auth.GetIP(r)
	if err != nil {
		m.t.Fatal(err)
	}
	cl, err := newClient(<-m.connSender, r, ip)
	if err != nil {
		m.t.Fatal(err)
	}
	return cl, wcl
}

func dialServer(t testing.TB, sv *httptest.Server) *websocket.Conn {
	t.Helper()

	wcl, _, err := dialer.Dial(strings.Replace(sv.URL, "http", "ws", 1), nil)
	if err != nil {
		t.Fatal(err)
	}
	return wcl
}

func readListenErrors(t *testing.T, cl *Client, sv *mockWSServer) {
	t.Helper()

	defer sv.Done()
	if err := cl.listen(); err != nil && err != websocket.ErrCloseSent {
		t.Fatal(err)
	}
}

func assertMessage(t *testing.T, con *websocket.Conn, std string) {
	t.Helper()

	typ, msg, err := con.ReadMessage()
	if err != nil {
		t.Error(err)
	}
	if typ != websocket.TextMessage {
		t.Errorf("invalid received message format: %d", typ)
	}
	if s := string(msg); s != std {
		LogUnexpected(t, std, s)
	}
}

func assertWebsocketError(
	t *testing.T,
	conn *websocket.Conn,
	prefix string,
	sv *mockWSServer,
) {
	t.Helper()

	defer sv.Done()
	_, _, err := conn.ReadMessage()
	assertErrorPrefix(t, err, prefix)
}

func assertErrorPrefix(t *testing.T, err error, prefix string) {
	t.Helper()
	if errMsg := fmt.Sprint(err); !strings.HasPrefix(errMsg, prefix) {
		t.Fatalf("unexpected error prefix: `%s` : `%s`", prefix, errMsg)
	}
}

func captureLog(fn func()) string {
	buf := new(bytes.Buffer)
	con.SetWriter(buf)
	fn()
	con.SetWriter(os.Stdout)
	return buf.String()
}

func assertLog(t *testing.T, input, std string) {
	t.Helper()

	std = `\d+/\d+/\d+ \d+:\d+:\d+ ` + std
	if strings.HasPrefix(std, input) {
		LogUnexpected(t, std, input)
	}
}

func TestTestClose(t *testing.T) {
	t.Parallel()

	sv := newWSServer(t)
	defer sv.Close()
	cl, _ := sv.NewClient()
	std := errors.New("foo")

	sv.Add(1)
	go func() {
		defer sv.Done()
		if err := cl.listen(); err != std {
			UnexpectedError(t, err)
		}
	}()
	cl.Close(std)
	sv.Wait()

	// Already closed
	cl.Close(nil)
}

func TestCloseMessageSending(t *testing.T) {
	t.Parallel()

	sv := newWSServer(t)
	defer sv.Close()
	cl, wcl := sv.NewClient()
	sv.Add(2)

	go readListenErrors(t, cl, sv)
	go assertWebsocketError(t, wcl, closeNormal, sv)
	cl.Close(nil)
	sv.Wait()
}

func TestHandleMessage(t *testing.T) {
	t.Parallel()

	sv := newWSServer(t)
	defer sv.Close()
	msg := []byte("natsutte tsuchatta")

	// Non-text message
	cl, _ := sv.NewClient()
	err := cl.handleMessage(websocket.BinaryMessage, msg)
	assertErrorPrefix(t, err, onlyText)

	// Message too short
	msg = []byte("0")
	cl, _ = sv.NewClient()
	assertHandlerError(t, cl, msg, invalidMessage)

	// Unparsable message type
	msg = []byte("nope")
	assertHandlerError(t, cl, msg, invalidMessage)

	// Not a sync message, when not synced
	msg = []byte("99no")
	assertHandlerError(t, cl, msg, invalidMessage)

	// No handler
	cl.gotFirstMessage = true
	assertHandlerError(t, cl, msg, invalidMessage)

	// Invalid inner message payload. Test proper type reflection of the
	// errInvalidMessage error type
	msg = []byte("30nope")
	assertHandlerError(t, cl, msg, invalidCharacter)
}

func assertHandlerError(t *testing.T, cl *Client, msg []byte, prefix string) {
	t.Helper()
	err := cl.handleMessage(websocket.TextMessage, msg)
	assertErrorPrefix(t, err, prefix)
}

func TestInvalidMessage(t *testing.T) {
	t.Parallel()

	sv := newWSServer(t)
	defer sv.Close()
	cl, wcl := sv.NewClient()

	sv.Add(1)
	go assertListenError(t, cl, onlyText, sv)
	if err := wcl.WriteMessage(websocket.BinaryMessage, []byte{1}); err != nil {
		t.Fatal(err)
	}
	assertMessage(t, wcl, `00"only text frames allowed"`)
	sv.Wait()
}

func assertListenError(
	t *testing.T,
	cl *Client,
	prefix string,
	sv *mockWSServer,
) {
	t.Helper()
	defer sv.Done()
	assertErrorPrefix(t, cl.listen(), prefix)
}

// Client properly closed connection with a control message
func TestClientClosure(t *testing.T) {
	t.Parallel()

	sv := newWSServer(t)
	defer sv.Close()
	cl, wcl := sv.NewClient()

	sv.Add(1)
	go readListenErrors(t, cl, sv)
	normalCloseWebClient(t, wcl)
	sv.Wait()
}

func normalCloseWebClient(t *testing.T, wcl *websocket.Conn) {
	t.Helper()

	msg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
	deadline := time.Now().Add(time.Second)
	err := wcl.WriteControl(websocket.CloseMessage, msg, deadline)
	if err != nil {
		t.Error(err)
	}
}

func TestHandler(t *testing.T) {
	t.Parallel()

	// Proper connection and client-side close
	sv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			Handler(w, r)
		}))
	defer sv.Close()
	wcl := dialServer(t, sv)
	normalCloseWebClient(t, wcl)
}

func TestSendMessage(t *testing.T) {
	t.Parallel()

	sv := newWSServer(t)
	defer sv.Close()
	cl, wcl := sv.NewClient()

	cases := [...]struct {
		typ common.MessageType
		msg string
	}{
		{common.MessageInsertPost, "01null"},  // 1 char type string
		{common.MessageSynchronise, "30null"}, // 2 char type string
	}

	for i := range cases {
		c := cases[i]
		if err := cl.sendMessage(c.typ, nil); err != nil {
			t.Error(err)
		}
		assertMessage(t, wcl, c.msg)
	}
}

func TestPinging(t *testing.T) {
	old := pingTimer
	pingTimer = time.Millisecond
	defer func() {
		pingTimer = old
	}()

	sv := newWSServer(t)
	defer sv.Close()
	cl, wcl := sv.NewClient()

	sv.Add(1)
	var once sync.Once
	wcl.SetPingHandler(func(_ string) error {
		once.Do(func() {
			sv.Done()
		})
		return nil
	})

	go wcl.ReadMessage()
	go cl.listen()
	sv.Wait()
	cl.Close(nil)
}
