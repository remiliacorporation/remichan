package db

import (
	"database/sql"
	"testing"

	"github.com/bakape/meguca/config"
	. "github.com/bakape/meguca/test"
)

func TestLoadConfigs(t *testing.T) {
	config.Clear()
	std := config.Configs{
		Public: config.Public{
			Mature: true,
		},
	}
	err := WriteConfigs(std)
	if err != nil {
		t.Fatal(err)
	}

	if err := loadConfigs(); err != nil {
		t.Fatal(err)
	}

	AssertEquals(t, config.Get(), &std)
}

func TestUpdateOnRemovedBoard(t *testing.T) {
	assertTableClear(t, "boards")
	config.Clear()
	config.SetBoardConfigs(config.BoardConfigs{
		ID: "a",
	})

	if err := updateBoardConfigs("a"); err != nil {
		t.Fatal(err)
	}

	AssertEquals(
		t,
		config.GetBoardConfigs("a"),
		config.BoardConfContainer{},
	)
	AssertEquals(t, config.GetBoards(), []string{})
}

func TestUpdateOnAddBoard(t *testing.T) {
	assertTableClear(t, "boards")
	config.Clear()

	std := BoardConfigs{
		BoardConfigs: config.BoardConfigs{
			ID: "a",
			BoardPublic: config.BoardPublic{
				ForcedAnon: true,
				Banners:    []uint16{},
			},
			Eightball: []string{"yes"},
		},
	}
	err := InTransaction(false, func(tx *sql.Tx) error {
		return WriteBoard(tx, std)
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := updateBoardConfigs("a"); err != nil {
		t.Fatal(err)
	}

	AssertEquals(
		t,
		config.GetBoardConfigs("a").BoardConfigs,
		std.BoardConfigs,
	)
	AssertEquals(t, config.GetBoards(), []string{"a"})
}

func TestUpdateBoardConfigs(t *testing.T) {
	assertTableClear(t, "boards")
	config.Clear()

	std := BoardConfigs{
		BoardConfigs: config.BoardConfigs{
			ID: "a",
			BoardPublic: config.BoardPublic{
				ForcedAnon: true,
				Banners:    []uint16{},
			},
			Eightball: []string{"yes"},
		},
	}
	err := InTransaction(false, func(tx *sql.Tx) error {
		return WriteBoard(tx, std)
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := loadBoardConfigs(); err != nil {
		t.Fatal(err)
	}

	AssertEquals(
		t,
		config.GetBoardConfigs("a").BoardConfigs,
		std.BoardConfigs,
	)

	conf := std.BoardConfigs
	conf.Title = "foo"
	err = UpdateBoard(conf)
	if err != nil {
		t.Fatal(err)
	}

	if err := updateBoardConfigs("a"); err != nil {
		t.Fatal(err)
	}

	std.Title = "foo"
	AssertEquals(
		t,
		config.GetBoardConfigs("a").BoardConfigs,
		std.BoardConfigs,
	)
}
