package db

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/bakape/meguca/common"
	"github.com/go-playground/log"
	"github.com/lib/pq"
)

type rowScanner interface {
	Scan(dest ...interface{}) error
}

// InTransaction runs a function inside a transaction and handles comminting and rollback on error.
// readOnly: the DBMS can optimise read-only transactions for better concurrency
//
// TODO: Get rid off readOnly param, once reader ported to output JSON
func InTransaction(readOnly bool, fn func(*sql.Tx) error) (err error) {
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{
		ReadOnly: readOnly,
	})
	if err != nil {
		return
	}

	err = fn(tx)
	if err != nil {
		tx.Rollback()
		return
	}
	return tx.Commit()
}

// Run fn on all returned rows in a query
func queryAll(q squirrel.SelectBuilder, fn func(r *sql.Rows) error,
) (err error) {
	r, err := q.Query()
	if err != nil {
		return
	}
	defer r.Close()

	for r.Next() {
		err = fn(r)
		if err != nil {
			return
		}
	}
	return r.Err()
}

// IsConflictError returns if an error is a unique key conflict error
func IsConflictError(err error) bool {
	return pqErrorCode(err) == "unique_violation"
}

// Extract error code, if error is a *pq.Error
func pqErrorCode(err error) string {
	if err, ok := err.(*pq.Error); ok {
		return err.Code.Name()
	}
	return ""
}

// Listen assigns a function to listen to Postgres notifications on a channel.
// Can't be used in tests.
func Listen(event string, fn func(msg string) error) (err error) {
	if common.IsTest {
		return
	}
	return ListenCancelable(event, nil, fn)
}

// Like listen, but is cancelable. Can be used in tests.
func ListenCancelable(event string, canceller <-chan struct{},
	fn func(msg string) error,
) (err error) {
	l := pq.NewListener(
		ConnArgs,
		time.Second,
		time.Second*10,
		func(_ pq.ListenerEventType, _ error) {},
	)
	err = l.Listen(event)
	if err != nil {
		return
	}

	go func() {
	again:
		select {
		case <-canceller:
			err := l.UnlistenAll()
			if err != nil {
				log.Errorf("unlistening database evenet id=`%s` error=`%s`\n",
					event, err)
			}
		case msg := <-l.Notify:
			if msg == nil {
				break
			}
			if err := fn(msg.Extra); err != nil {
				log.Errorf(
					"error on database event id=`%s` msg=`%s` error=`%s`\n",
					event, msg.Extra, err)
			}
			goto again
		}
	}()

	return
}

// Execute all SQL statement strings and return on first error, if any
func execAll(tx *sql.Tx, q ...string) error {
	for _, q := range q {
		if _, err := tx.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

// PostgreSQL notification message parse error
type ErrMsgParse string

func (e ErrMsgParse) Error() string {
	return fmt.Sprintf("unparsable message: `%s`", string(e))
}

// Split message containing a board and post/thread ID
func SplitBoardAndID(msg string) (board string, id uint64, err error) {
	split := strings.Split(msg, ",")
	if len(split) != 2 {
		goto fail
	}
	board = split[0]
	id, err = strconv.ParseUint(split[1], 10, 64)
	if err != nil {
		goto fail
	}
	return

fail:
	err = ErrMsgParse(msg)
	return
}

// Split message containing a list of uint64 numbers.
// Returns error, if message did not contain n integers.
func SplitUint64s(msg string, n int) (arr []uint64, err error) {
	parts := strings.Split(msg, ",")
	if len(parts) != n {
		goto fail
	}
	for _, p := range parts {
		i, err := strconv.ParseUint(p, 10, 64)
		if err != nil {
			goto fail
		}
		arr = append(arr, i)
	}
	return

fail:
	err = ErrMsgParse(msg)
	return
}

// Try to extract an exception message, if err is *pq.Error
func extractException(err error) string {
	if err == nil {
		return ""
	}

	pqErr, ok := err.(*pq.Error)
	if ok {
		return pqErr.Message
	}
	return ""
}

// Encode []uint64 tom postgres format
func encodeUint64Array(arr []uint64) string {
	b := []byte{'{'}
	for i, j := range arr {
		if i != 0 {
			b = append(b, ',')
		}
		b = strconv.AppendUint(b, j, 10)
	}
	return string(append(b, '}'))
}
