// Package cdc implements the change data capture scenario.
//
// A warehouse's inventory app is old and nobody wants to change it. It writes
// stock changes to its own database and knows nothing about events. A change
// data capture connector reads the database's change log, the record every
// database keeps of each committed change, and publishes one event per
// change. The search index keeps its stock figures from those events.
//
// The database here is a file of committed changes, one JSON line each, with
// a log sequence number (LSN). Real connectors read a real database's log
// (the PostgreSQL write-ahead log, the MySQL binlog); the shape is the same:
// read in order, publish, remember how far you got.
package cdc

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"solace-workshop/apps/internal/solace"
	"solace.dev/go/messaging/pkg/solace/message"
)

const changeTopic = "cdc/inventory/stock/changed/v1/"

// skus is how many items the warehouse stocks.
const skus = 6

type change struct {
	LSN    int    `json:"lsn"`
	Table  string `json:"table"`
	Op     string `json:"op"`
	SKU    string `json:"sku"`
	Before int    `json:"before"`
	After  int    `json:"after"`
}

// stockRows is a stock table, one row per SKU in SKU order: the SKU, its
// stock and the number of the change that last set it. The inventory
// database and the search index report the same columns, so the page can
// compare them row by row.
func stockRows(stock map[string]int, last map[string]int) [][]string {
	skus := make([]string, 0, len(stock))
	for sku := range stock {
		skus = append(skus, sku)
	}
	sort.Strings(skus)
	rows := make([][]string, len(skus))
	for i, sku := range skus {
		rows[i] = []string{sku, strconv.Itoa(stock[sku]), "#" + strconv.Itoa(last[sku])}
	}
	return rows
}

// dataDir is where the inventory database and the connector's saved position
// live: under the dashboard's state folder, so Clear broker config removes
// them with the rest of the scenario's state.
func dataDir() string {
	base := os.Getenv("COCKPIT_STATE_DIR")
	if base == "" {
		base = "../../.state"
	}
	dir := filepath.Join(base, "change-data-capture")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

func readLog(path string) []change {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []change
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var c change
		if json.Unmarshal(sc.Bytes(), &c) == nil {
			out = append(out, c)
		}
	}
	return out
}

// Inventory is the legacy app. It sells and restocks, and writes every
// change to its database. It never connects to the broker.
func Inventory(args []string) {
	fs := flag.NewFlagSet("cdc inventory", flag.ExitOnError)
	role := fs.String("role", "inventory", "label for log lines")
	every := fs.Duration("every", time.Second, "pause between stock changes")
	forget := fs.Bool("forget", false, "delete the database and the connector's saved position, and exit")
	fs.Parse(args)

	path := filepath.Join(dataDir(), "inventory.log")
	if *forget {
		_ = os.Remove(path)
		_ = os.Remove(filepath.Join(dataDir(), "connector.position"))
		solace.Logf(*role, "deleted the inventory database and the connector's saved position")
		return
	}
	stock, last := map[string]int{}, map[string]int{}
	lsn := 0
	for _, c := range readLog(path) {
		stock[c.SKU] = c.After
		last[c.SKU] = c.LSN
		lsn = c.LSN
	}
	held := solace.NewHolding("The inventory database: stock per SKU", "sku", "stock", "last change")
	report := func() {
		note := "No changes yet."
		if lsn > 0 {
			note = fmt.Sprintf("Its change log holds %d committed changes. Latest: change #%d.", lsn, lsn)
		}
		held.Set(stockRows(stock, last), note)
	}
	report()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		solace.Errf(*role, "could not open the database: %s", err)
		os.Exit(1)
	}
	defer f.Close()
	if lsn == 0 {
		// The opening stock is committed like any other change, so it is in
		// the change log and reaches the search index the same way.
		for i := 1; i <= skus; i++ {
			sku := fmt.Sprintf("SKU-%02d", i)
			lsn++
			line, _ := json.Marshal(change{LSN: lsn, Table: "stock", Op: "insert", SKU: sku, Before: 0, After: 20})
			if _, err := f.Write(append(line, '\n')); err != nil {
				solace.Errf(*role, "could not write to the database: %s", err)
				os.Exit(1)
			}
			stock[sku], last[sku] = 20, lsn
		}
		_ = f.Sync()
		report()
		solace.Logf(*role, "new database: %d SKUs stocked with 20 each (changes #1 to #%d)", skus, lsn)
	} else {
		solace.Logf(*role, "database opened, last change was #%d", lsn)
	}
	solace.Logf(*role, "this app does not connect to the broker. It only writes to its database.")
	fmt.Println()

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	for {
		select {
		case <-stop:
			return
		case <-time.After(*every):
		}
		sku := fmt.Sprintf("SKU-%02d", 1+rand.Intn(skus))
		before, ok := stock[sku]
		if !ok {
			before = 20
		}
		after, what := before-1-rand.Intn(2), "sold"
		if after < 3 {
			after, what = before+15, "restocked"
		}
		lsn++
		line, _ := json.Marshal(change{LSN: lsn, Table: "stock", Op: "update", SKU: sku, Before: before, After: after})
		if _, err := f.Write(append(line, '\n')); err != nil {
			solace.Errf(*role, "could not write to the database: %s", err)
			continue
		}
		_ = f.Sync()
		stock[sku] = after
		last[sku] = lsn
		report()
		solace.Logf(*role, "%s %s: stock %d -> %d (change #%d written to the database)", what, sku, before, after, lsn)
	}
}

// Connector reads the inventory database's change log in order, publishes
// each change as an event, and saves the number of the last change the broker
// accepted. After a restart it carries on from there. --from-start ignores the
// saved position, as a connector does when it has lost it.
func Connector(args []string) {
	fs := flag.NewFlagSet("cdc connector", flag.ExitOnError)
	user := fs.String("user", "svc-cdc-connector", "client username to connect as")
	role := fs.String("role", "connector", "label for log lines")
	clientName := fs.String("client-name", "cdc-connector", "client name the broker reports this app by")
	fromStart := fs.Bool("from-start", false, "ignore the saved position and read the change log from the beginning")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	pub := solace.StartPersistentPublisher(*role, *user, svc)
	defer solace.Terminate(pub)

	logPath := filepath.Join(dataDir(), "inventory.log")
	posPath := filepath.Join(dataDir(), "connector.position")
	done := 0
	held := solace.NewHolding("Its saved position in the change log", "position", "change")
	report := func(latest int) {
		held.Set([][]string{
			{"saved position: published up to", "#" + strconv.Itoa(done)},
			{"latest change in the database", "#" + strconv.Itoa(latest)},
			{"still to publish", strconv.Itoa(max(latest-done, 0))},
		}, "")
	}
	saved, readErr := os.ReadFile(posPath)
	pos, parseErr := strconv.Atoi(strings.TrimSpace(string(saved)))
	if readErr == nil && parseErr == nil && !*fromStart {
		done = pos
		solace.Logf(*role, "saved position found: resuming after change #%d", done)
	} else if *fromStart {
		solace.Logf(*role, "no saved position: reading the change log from the first change")
	} else {
		solace.Logf(*role, "first run: reading the change log from the first change")
	}
	fmt.Println()

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	for {
		changes := readLog(logPath)
		latest := 0
		if len(changes) > 0 {
			latest = changes[len(changes)-1].LSN
		}
		report(latest)
		for _, c := range changes {
			if c.LSN <= done {
				continue
			}
			payload, _ := json.Marshal(c)
			if err := solace.PublishConfirmed(svc, pub, changeTopic+c.SKU, string(payload)); err != nil {
				solace.Errf(*role, "change #%d not accepted: %s", c.LSN, err)
				break
			}
			done = c.LSN
			if err := solace.WriteFileAtomic(posPath, []byte(strconv.Itoa(done))); err != nil {
				solace.Errf(*role, "could not save the position: %s", err)
			}
			report(latest)
			solace.Logf(*role, "change #%d published: %s %d -> %d", c.LSN, c.SKU, c.Before, c.After)
		}
		select {
		case <-stop:
			return
		case <-time.After(300 * time.Millisecond):
		}
	}
}

// Search keeps the stock figure per SKU from the change events. It applies a
// change only if it is newer than the last one it applied for that SKU, so a
// change delivered twice is noticed and ignored.
func Search(args []string) {
	fs := flag.NewFlagSet("cdc search", flag.ExitOnError)
	user := fs.String("user", "svc-cdc-search", "client username to connect as")
	role := fs.String("role", "search", "label for log lines")
	clientName := fs.String("client-name", "cdc-search", "client name the broker reports this app by")
	queue := fs.String("queue", "q.cdc.search", "queue with stock change events")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	rcv := solace.BindQueue(*role, *user, svc, *queue, solace.QueueOpts{ClientAck: true})
	defer solace.Terminate(rcv)
	fmt.Println()

	applied, stock := map[string]int{}, map[string]int{}
	held := solace.NewHolding("The search index: stock per SKU", "sku", "stock", "last change")
	ignored := 0
	report := func() {
		note := ""
		if ignored > 0 {
			note = fmt.Sprintf("%d repeated changes ignored: already applied.", ignored)
		}
		held.Set(stockRows(stock, applied), note)
	}
	report()
	if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
		var c change
		if err := json.Unmarshal([]byte(solace.PayloadOf(msg)), &c); err != nil {
			solace.Errf(*role, "not a change event: %s", err)
		} else if c.LSN <= applied[c.SKU] {
			ignored++
			report()
			solace.Logf(*role, "change #%d for %s again: already applied, ignored", c.LSN, c.SKU)
		} else {
			applied[c.SKU] = c.LSN
			stock[c.SKU] = c.After
			report()
			solace.Logf(*role, "%s shows %d in stock (change #%d)", c.SKU, c.After, c.LSN)
		}
		if err := rcv.Ack(msg); err != nil {
			solace.Errf(*role, "could not acknowledge: %s", err)
		}
	}); err != nil {
		solace.Errf(*role, "could not register the message handler: %s", err)
		os.Exit(1)
	}
	solace.WaitForStop()
}
