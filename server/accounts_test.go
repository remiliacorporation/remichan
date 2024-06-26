package server

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bakape/meguca/auth"
	"github.com/bakape/meguca/common"
	"github.com/bakape/meguca/config"
	"github.com/bakape/meguca/db"
	. "github.com/bakape/meguca/test"
	"github.com/bakape/meguca/test/test_db"
)

const samplePassword = "123456"

var sampleLoginCreds = auth.SessionCreds{
	UserID:  "user1",
	Session: genSession(),
}

func writeSampleUser(t *testing.T) {
	t.Helper()

	hash, err := auth.BcryptHash(samplePassword, 3)
	if err != nil {
		t.Fatal(err)
	}
	writeAccount(t, sampleLoginCreds.UserID, hash)
	err = db.WriteLoginSession(
		sampleLoginCreds.UserID,
		sampleLoginCreds.Session,
	)
	if err != nil {
		t.Fatal(err)
	}
}

func writeAccount(t *testing.T, id string, hash []byte) {
	t.Helper()

	err := db.InTransaction(false, func(tx *sql.Tx) error {
		return db.RegisterAccount(tx, id, hash)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func genSession() string {
	return GenString(common.LenSession)
}

func TestIsLoggedIn(t *testing.T) {
	test_db.ClearTables(t, "accounts")

	hash, err := auth.BcryptHash(samplePassword, 3)
	if err != nil {
		t.Fatal(err)
	}
	writeAccount(t, "user1", hash)
	writeAccount(t, "user2", hash)

	token := genSession()
	if err := db.WriteLoginSession("user1", token); err != nil {
		t.Fatal(err)
	}

	cases := [...]struct {
		name, user, session string
		err                 error
	}{
		{"valid", "user1", token, nil},
		{"invalid session", "user2", genSession(), errAccessDenied},
		{"not registered", "nope", genSession(), errAccessDenied},
	}

	for i := range cases {
		c := cases[i]
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			rec, req := newPair("/")
			setLoginCookies(req, auth.SessionCreds{
				UserID:  c.user,
				Session: c.session,
			})
			_, err := isLoggedIn(rec, req)
			if err != c.err {
				LogUnexpected(t, c.err, err)
			}
		})
	}
}

func setLoginCookies(r *http.Request, creds auth.SessionCreds) {
	expires := time.Now().Add(time.Hour)
	r.AddCookie(&http.Cookie{
		Name:    "loginID",
		Value:   creds.UserID,
		Path:    "/",
		Expires: expires,
	})
	r.AddCookie(&http.Cookie{
		Name:    "session",
		Value:   creds.Session,
		Path:    "/",
		Expires: expires,
	})
}

func assertError(
	t *testing.T,
	rec *httptest.ResponseRecorder,
	code int,
	err error,
) {
	t.Helper()
	assertCode(t, rec, code)
	if err != nil {
		assertBody(t, rec, fmt.Sprintf("%d %s\n", code, err))
	}
}

func TestNotLoggedIn(t *testing.T) {
	test_db.ClearTables(t, "accounts", "boards")
	writeSampleBoard(t)

	fns := [...]http.HandlerFunc{servePrivateServerConfigs, changePassword}
	for i := range fns {
		fn := fns[i]
		t.Run("", func(t *testing.T) {
			t.Parallel()

			rec, req := newJSONPair(t, "/", boardActionRequest{
				Board: "a",
			})
			fn(rec, req)
			assertError(t, rec, 403, errAccessDenied)
		})
	}
}

func TestChangePassword(t *testing.T) {
	test_db.ClearTables(t, "accounts")
	writeSampleUser(t)
	config.Set(config.Configs{})

	const new = "654321"

	cases := [...]struct {
		name, old, new string
		code           int
		err            error
	}{
		{
			name: "wrong password",
			old:  "1234567",
			new:  new,
			code: 403,
			err:  common.ErrInvalidCreds,
		},
		{
			name: "new password too long",
			old:  samplePassword,
			new:  GenString(common.MaxLenPassword + 1),
			code: 400,
			err:  errInvalidPassword,
		},
		{
			name: "empty new password",
			old:  samplePassword,
			new:  "",
			code: 400,
			err:  errInvalidPassword,
		},
		{
			name: "correct password",
			old:  samplePassword,
			new:  new,
			code: 200,
		},
	}

	for i := range cases {
		c := cases[i]
		t.Run(c.name, func(t *testing.T) {
			msg := passwordChangeRequest{
				Old: c.old,
				New: c.new,
			}
			rec, req := newJSONPair(t, "/api/change-password", msg)
			setLoginCookies(req, sampleLoginCreds)

			router.ServeHTTP(rec, req)

			assertError(t, rec, c.code, c.err)
		})
	}

	// Assert new hash matches new password
	hash, err := db.GetPassword(sampleLoginCreds.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.BcryptCompare(new, hash); err != nil {
		t.Fatal(err)
	}
}

func TestRegistrationValidations(t *testing.T) {
	test_db.ClearTables(t, "accounts")

	cases := [...]struct {
		name, id, password string
		code               int
		err                error
	}{
		{
			name:     "no ID",
			id:       "",
			password: "123456",
			code:     400,
			err:      errInvalidUserID,
		},
		{
			name:     "id too long",
			id:       GenString(common.MaxLenUserID + 1),
			password: "123456",
			code:     400,
			err:      errInvalidUserID,
		},
		{
			name:     "no password",
			id:       "123",
			password: "",
			code:     400,
			err:      errInvalidPassword,
		},
		{
			name:     "password too long",
			id:       "123",
			password: GenString(common.MaxLenPassword + 1),
			code:     400,
			err:      errInvalidPassword,
		},
		{
			name:     "valid",
			id:       "123",
			password: "456",
			code:     200,
		},
		{
			name:     "id already taken",
			id:       "123",
			password: "456",
			code:     400,
			err:      errUserIDTaken,
		},
	}

	for i := range cases {
		c := cases[i]
		t.Run(c.name, func(t *testing.T) {
			rec, req := newJSONPair(t, "/api/register", loginCreds{
				ID:       c.id,
				Password: c.password,
			})
			router.ServeHTTP(rec, req)

			assertError(t, rec, c.code, c.err)
			if c.err == nil {
				assertLogin(t, rec, true)
			}
		})
	}
}

func assertLogin(t *testing.T, rec *httptest.ResponseRecorder, loggedIn bool) {
	t.Helper()

	// Extract cookies from recorder
	req := http.Request{
		Header: http.Header{
			"Cookie": rec.HeaderMap["Set-Cookie"],
		},
	}
	var userID, session string
	if c, err := req.Cookie("loginID"); err == nil {
		userID = c.Value
	}
	if c, err := req.Cookie("session"); err == nil {
		session = c.Value
	}

	assertLoginNoCookie(t, userID, session, loggedIn)
}

func assertLoginNoCookie(t *testing.T, userID, session string, loggedIn bool) {
	t.Helper()

	is, err := db.IsLoggedIn(userID, session)
	switch {
	case err != nil:
		t.Fatal(err)
	case is != loggedIn:
		t.Fatalf("unexpected session status: %t", is)
	}
}

func TestLogin(t *testing.T) {
	test_db.ClearTables(t, "accounts")

	const (
		id       = "123"
		password = "123456"
	)
	hash, err := auth.BcryptHash(password, 10)
	if err != nil {
		t.Fatal(err)
	}
	writeAccount(t, id, hash)

	cases := [...]struct {
		name, id, password string
		code               int
		err                error
	}{
		{
			name:     "invalid ID",
			id:       id + "1",
			password: password,
			code:     403,
			err:      common.ErrInvalidCreds,
		},
		{
			name:     "invalid password",
			id:       id,
			password: password + "1",
			code:     403,
			err:      common.ErrInvalidCreds,
		},
		{
			name:     "valid",
			id:       id,
			password: password,
			code:     200,
		},
	}

	for i := range cases {
		c := cases[i]
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			rec, req := newJSONPair(t, "/api/login", loginCreds{
				ID:       c.id,
				Password: c.password,
			})
			router.ServeHTTP(rec, req)

			assertError(t, rec, c.code, c.err)
			if c.err == nil {
				assertLogin(t, rec, true)
			}
		})
	}
}

func TestLogout(t *testing.T) {
	test_db.ClearTables(t, "accounts")
	id, tokens := writeSampleSessions(t)

	cases := [...]struct {
		name, token string
		code        int
		err         error
	}{
		{
			name:  "not logged in",
			token: genSession(),
			code:  403,
			err:   errAccessDenied,
		},
		{
			name:  "valid",
			token: tokens[0],
			code:  200,
		},
	}

	for i := range cases {
		c := cases[i]
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			rec, req := newJSONPair(t, "/api/logout", nil)
			setLoginCookies(req, auth.SessionCreds{
				UserID:  id,
				Session: c.token,
			})
			router.ServeHTTP(rec, req)

			assertError(t, rec, c.code, c.err)

			if c.err == nil {
				assertLoginNoCookie(t, id, tokens[0], false)
				assertLoginNoCookie(t, id, tokens[1], true)
			}
		})
	}
}

func writeSampleSessions(t *testing.T) (string, [2]string) {
	t.Helper()

	const id = "123"
	tokens := [2]string{genSession(), genSession()}
	hash, err := auth.BcryptHash("foo", 3)
	if err != nil {
		t.Fatal(err)
	}

	writeAccount(t, id, hash)
	for _, token := range tokens {
		if err := db.WriteLoginSession(id, token); err != nil {
			t.Fatal(err)
		}
	}

	return id, tokens
}

func TestLogoutAll(t *testing.T) {
	test_db.ClearTables(t, "accounts")
	id, tokens := writeSampleSessions(t)

	rec, req := newJSONPair(t, "/api/logout-all", nil)
	setLoginCookies(req, auth.SessionCreds{
		UserID:  id,
		Session: tokens[0],
	})
	router.ServeHTTP(rec, req)

	assertCode(t, rec, 200)
	for _, tok := range tokens {
		assertLoginNoCookie(t, id, tok, false)
	}
}
