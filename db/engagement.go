// Package db handles all core database interactions of the server
package db

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/go-playground/log"
)

type EngagementType int
type EngagementEvent struct {
	Wallet   string
	JWT      string
	Activity EngagementType
	IP       string
	Board    string
}

const (
	MadeReply  EngagementType = 1
	MadeThread EngagementType = 2
	DailyLogin EngagementType = 3
)

func trackBoards(boards_serialized string, engagement EngagementEvent) (err error) {
	var boards_map map[string]int

	if boards_serialized != "" {
		err = json.Unmarshal([]byte(boards_serialized), &boards_map)
		if err != nil {
			log.Errorf("Could not unmarshal serialized board data")
			return
		}
	} else {
		boards_map = make(map[string]int)
	}

	boards_map[engagement.Board]++

	data, err := json.Marshal(boards_map)
	if err != nil {
		log.Errorf("Could not marshal serialized board data")
		return
	}

	update_q := sq.Update("engagement").
		Set("boards", string(data)).
		Where("address = ?", engagement.Wallet)

	_, err = update_q.Exec()
	if err != nil {
		log.Errorf("Could not update wallet's board stats")
		log.Errorf(err.Error())
		return
	}

	log.Infof("updated wallet's board stats")
	return
}

func trackIP(seenDate int, logins int, engagement EngagementEvent) (err error) {
	var (
		ips string
	)

	if seenDate != time.Now().Day() {
		q := sq.Update("engagement").
			Set("seenDate", time.Now().Day()).
			Set("logins", logins+1).
			Where("address = ?", engagement.Wallet)
		_, err = q.Exec()
		if err != nil {
			log.Errorf("Could not update seenDate")
			log.Errorf(err.Error())
			return
		}
	}

	// Add this IP to user list
	err = sq.Select("ips").
		From("wallet_ips").
		Where("address = ?", engagement.Wallet).
		Scan(&ips)

	if ips == "" {
		ips = engagement.IP
	} else {
		ipArray := strings.Split(ips, ",")

		for _, ip := range ipArray {
			if ip == engagement.IP {
				// No need to add
				return
			}
		}

		// Why do slice methods not work? May need to pull in old package
		// from before slices were part of the standard library
		ipArray = append(ipArray, engagement.IP)
		uniqueIPs := make(map[string]bool)
		for _, ip := range ipArray {
			uniqueIPs[ip] = true
		}
		ipArray = []string{}
		for ip := range uniqueIPs {
			ipArray = append(ipArray, ip)
		}

		ips = strings.Join(ipArray, ",")
	}

	insert_q := sq.Insert("wallet_ips").
		Columns("address", "ips").
		Values(engagement.Wallet, ips).
		Suffix("ON CONFLICT (address) DO UPDATE SET ips = ?", ips)

	// update_q := sq.Update("wallet_ips").
	// 	Set("ips", ips).
	// 	Where("address = ?", engagement.Wallet)

	_, err = insert_q.Exec()
	if err != nil {
		log.Errorf("Could not update wallet's known ips")
		log.Errorf(err.Error())
		return
	}

	log.Infof("Updated wallet's ips %s %s", engagement.Wallet, ips)
	return
}

// CLEANUP SOON
func TrackEngagement(engagement EngagementEvent) (err error) {
	var (
		column            string
		seenDate          int
		logins            int
		boards_serialized string
		value             = 1
	)
	switch engagement.Activity {
	case MadeThread:
		column = "threads"
	case MadeReply:
		column = "posts"
	case DailyLogin:
		column = "logins"
	default:
		log.Errorf("unknown engagement type")
		return
	}

	q := sq.Insert("engagement").
		Columns("address", column).
		Values(engagement.Wallet, value).
		Suffix("ON CONFLICT (address) DO UPDATE SET " + column + " = engagement." + column + " + 1")

	_, err = q.Exec()
	if err != nil {
		log.Errorf("Could not push engagement metric update")
		log.Errorf(err.Error())
		return
	}

	err = sq.Select("seenDate", "logins", "boards").
		From("engagement").
		Where("address = ?", engagement.Wallet).
		Scan(&seenDate, &logins, &boards_serialized)

	log.Infof("Pushed engagement update")

	err = trackIP(seenDate, logins, engagement)
	if err != nil {
		log.Errorf("Could not attach IP to this wallet")
		log.Errorf(err.Error())
		return
	}

	err = trackBoards(boards_serialized, engagement)
	if err != nil {
		log.Errorf("Could not track board stats")
		log.Errorf(err.Error())
		return
	}

	return
}
