package db

import (
	"database/sql"
	"testing"
	"time"

	"github.com/bakape/meguca/auth"
	"github.com/bakape/meguca/common"
	"github.com/bakape/meguca/config"
	. "github.com/bakape/meguca/test"
)

var (
	sampleUserID       = "123"
	sampleUserSession  = GenString(common.LenSession)
	samplePasswordHash = []byte{1, 2, 3}
)

func writeSampleUser(t *testing.T) {
	t.Helper()

	err := InTransaction(false, func(tx *sql.Tx) error {
		return RegisterAccount(tx, sampleUserID, samplePasswordHash)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func writeSampleSession(t *testing.T) {
	t.Helper()

	err := WriteLoginSession(sampleUserID, sampleUserSession)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRegisterAccount(t *testing.T) {
	assertTableClear(t, "accounts")

	// New user
	writeSampleUser(t)

	// User name already registered
	err := InTransaction(false, func(tx *sql.Tx) error {
		return RegisterAccount(tx, sampleUserID, samplePasswordHash)
	})
	if err != ErrUserNameTaken {
		UnexpectedError(t, err)
	}
}

func TestChangePassword(t *testing.T) {
	assertTableClear(t, "accounts")
	writeSampleUser(t)
	config.Set(config.Configs{})

	pass, err := GetPassword(sampleUserID)
	if err != nil {
		t.Fatal(err)
	}
	AssertEquals(t, pass, samplePasswordHash)

	newHash := []byte{1, 5, 51, 51, 3}
	err = ChangePassword(sampleUserID, newHash)
	if err != nil {
		t.Fatal(err)
	}
	pass, err = GetPassword(sampleUserID)
	if err != nil {
		t.Fatal(err)
	}
	AssertEquals(t, pass, newHash)
}

func TestLoginLogout(t *testing.T) {
	assertTableClear(t, "accounts")
	writeSampleUser(t)

	assertLoggedIn(t, sampleUserID, sampleUserSession, false)

	writeSampleSession(t)
	assertLoggedIn(t, sampleUserID, sampleUserSession, true)

	err := LogOut(sampleUserID, sampleUserSession)
	if err != nil {
		t.Fatal(err)
	}
	assertLoggedIn(t, sampleUserID, sampleUserSession, false)
}

func assertLoggedIn(t *testing.T, user, session string, std bool) {
	t.Helper()

	res, err := IsLoggedIn(user, session)
	if err != nil {
		t.Fatal(err)
	}
	if std {
		if !res {
			t.Fatal("not logged in")
		}
	} else if res {
		t.Fatal("logged in")
	}
}

func TestLogOutAll(t *testing.T) {
	assertTableClear(t, "accounts")
	writeSampleUser(t)
	writeSampleSession(t)

	err := LogOutAll(sampleUserID)
	if err != nil {
		t.Fatal(err)
	}
	assertLoggedIn(t, sampleUserID, sampleUserSession, false)
}

func TestGetPositions(t *testing.T) {
	assertTableClear(t, "accounts", "boards")
	writeSampleBoard(t)
	writeSampleUser(t)
	err := InTransaction(false, func(tx *sql.Tx) error {
		return WriteStaff(tx, "a", map[common.ModerationLevel][]string{
			common.BoardOwner: []string{sampleUserID},
		})
	})
	if err != nil {
		t.Fatal(err)
	}

	pos, err := FindPosition("a", sampleUserID)
	if err != nil {
		t.Fatal(err)
	}
	AssertEquals(t, pos, common.BoardOwner)

	owned, err := GetOwnedBoards(sampleUserID)
	if err != nil {
		t.Fatal(err)
	}
	AssertEquals(t, owned, []string{"a"})
}

func TestGetBanRecords(t *testing.T) {
	assertTableClear(t, "accounts", "boards")
	writeSampleUser(t)
	writeSampleBoard(t)
	writeSampleThread(t)

	const length = time.Hour * 20
	std := auth.BanRecord{
		Ban: auth.Ban{
			IP:    "::1",
			Board: "a",
		},
		ForPost: 1,
		By:      "me",
		Expires: time.Now().Add(length).UTC(),
	}

	err := Ban(std.Board, std.Reason, std.By, length, std.ForPost)
	if err != nil {
		t.Fatal(err)
	}

	ban, err := GetBanInfo(std.IP, std.Board)
	if err != nil {
		t.Fatal(err)
	}
	// Location is a pointer and can't be compared with reflection
	ban.Expires = std.Expires
	AssertEquals(t, ban, std)

	bans, err := GetBoardBans(std.Board)
	if err != nil {
		t.Fatal(err)
	}
	bans[0].Expires = std.Expires
	std.Type = "classic"
	AssertEquals(t, bans, []auth.BanRecord{std})
}
