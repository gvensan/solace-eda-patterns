// Package datasync implements the data synchronization scenario.
//
// A company is moving from its old CRM to a new one, and for a while people
// edit customers in both. Each CRM keeps its own customer table, publishes
// every edit made in it, and applies the edits the other one publishes. Two
// rules keep the tables the same:
//
//   - Origin: an edit applied from the other side is not published again.
//     Without it, an edit echoes back and forth forever.
//   - Version: every edit raises the record's version, and a CRM applies an
//     incoming edit only if it is newer than what it has. Two edits with the
//     same version are settled the same way on both sides (the old CRM wins),
//     so both end with the same record.
//
// Each CRM saves its table to a file after every change, as a database
// would, so a restart keeps it.
package datasync

import (
	"crypto/sha1"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"solace-workshop/apps/internal/solace"
	sol "solace.dev/go/messaging/pkg/solace"
	"solace.dev/go/messaging/pkg/solace/message"
)

const topicBase = "dsync/crm/customer/changed/v1/"

var phones = []string{"555-0101", "555-0142", "555-0177", "555-0190", "555-0123", "555-0168"}

type record struct {
	Customer string `json:"customer"`
	Phone    string `json:"phone"`
	Version  int    `json:"version"`
	Origin   string `json:"origin"`
}

// newer reports whether a should replace b: higher version, or the same
// version from the old CRM.
func newer(a, b record) bool {
	if a.Version != b.Version {
		return a.Version > b.Version
	}
	return a.Origin == "legacy" && b.Origin != "legacy"
}

func checksum(table map[string]record) string {
	keys := make([]string, 0, len(table))
	for k := range table {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha1.New()
	for _, k := range keys {
		r := table[k]
		fmt.Fprintf(h, "%s|%s|%d|%s;", r.Customer, r.Phone, r.Version, r.Origin)
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:6]
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// stateDir is where each CRM keeps its table: under the dashboard's state
// folder, so Clear broker config removes them with the scenario's state.
func stateDir() string {
	base := os.Getenv("COCKPIT_STATE_DIR")
	if base == "" {
		base = "../../.state"
	}
	dir := filepath.Join(base, "data-synchronization")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

// CRM is one side of the sync. --side names it (legacy or new) and the other
// side is the one it applies edits from. --echo publishes every change it
// applies as if it were a fresh edit, which is what a database trigger does
// when it cannot tell a sync write from a user's write.
func CRM(args []string) {
	fs := flag.NewFlagSet("dsync crm", flag.ExitOnError)
	side := fs.String("side", "", "legacy or new")
	every := fs.Duration("every", 3*time.Second, "pause between edits made in this CRM")
	echo := fs.Bool("echo", false, "publish changes applied from the other side again, with a new version")
	forget := fs.Bool("forget", false, "delete both CRMs' customer tables, and exit")
	clientName := fs.String("client-name", "", "client name the broker reports this app by (default dsync-<side>)")
	fs.Parse(args)
	other := map[string]string{"legacy": "new", "new": "legacy"}[*side]
	if *forget {
		for _, f := range []string{"legacy-crm.json", "new-crm.json"} {
			_ = os.Remove(filepath.Join(stateDir(), f))
		}
		solace.Logf("crm", "deleted both CRMs' customer tables")
		return
	}
	if other == "" {
		fmt.Fprintln(os.Stderr, "usage: workshop dsync crm --side legacy|new")
		os.Exit(2)
	}
	role := *side
	user := "svc-dsync-" + *side

	path := filepath.Join(stateDir(), *side+"-crm.json")

	var mu sync.Mutex
	table := map[string]record{}
	if b, err := os.ReadFile(path); err == nil && json.Unmarshal(b, &table) == nil && len(table) > 0 {
		solace.Logf(role, "opened its customer table: %d customers", len(table))
	} else {
		for i := 0; i < 6; i++ {
			c := fmt.Sprintf("C-%02d", i+1)
			table[c] = record{Customer: c, Phone: phones[i], Version: 1, Origin: "legacy"}
		}
		solace.Logf(role, "new customer table, the same 6 customers as the other CRM")
	}
	save := func() {
		b, _ := json.Marshal(table)
		_ = os.WriteFile(path, b, 0o644)
	}

	if *clientName == "" {
		*clientName = "dsync-" + *side
	}
	svc := solace.ConnectAs(role, user, *clientName)
	defer svc.Disconnect()
	pub := solace.StartPersistentPublisher(role, user, svc)
	defer solace.Terminate(pub)
	sent, made := 0, 0
	publish := func(r record) {
		b, _ := json.Marshal(r)
		if err := solace.PublishConfirmed(svc, pub, topicBase+*side+"/"+r.Customer, string(b)); err != nil {
			solace.Errf(role, "%s: could not publish the change: %s", r.Customer, err)
			return
		}
		sent++
	}

	var rcv sol.PersistentMessageReceiver = solace.BindQueue(role, user, svc, "q.dsync."+*side, solace.QueueOpts{ClientAck: true})
	defer solace.Terminate(rcv)
	if *echo {
		solace.Logf(role, "applying edits from the %s CRM, and publishing every applied change again", other)
	} else {
		solace.Logf(role, "applying edits from the %s CRM; applied changes are not published again", other)
	}
	fmt.Println()

	if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
		var in record
		if err := json.Unmarshal([]byte(solace.PayloadOf(msg)), &in); err != nil {
			solace.Errf(role, "not a customer change: %s", err)
		} else {
			// A write to the database takes a moment.
			time.Sleep(100 * time.Millisecond)
			mu.Lock()
			have := table[in.Customer]
			switch {
			case newer(in, have):
				if *echo {
					in.Version++
					in.Origin = *side
				}
				table[in.Customer] = in
				save()
				solace.Logf(role, "applied %s from %s: phone %s (version %d)", in.Customer, other, in.Phone, in.Version)
				if *echo {
					publish(in)
				}
			case in.Version == have.Version && in.Phone == have.Phone:
				// The same record; nothing to do.
			default:
				solace.Logf(role, "kept my %s (version %d %s) over the %s CRM's version %d", in.Customer, have.Version, have.Origin, other, in.Version)
			}
			mu.Unlock()
		}
		if err := rcv.Ack(msg); err != nil {
			solace.Errf(role, "could not acknowledge: %s", err)
		}
	}); err != nil {
		solace.Errf(role, "could not register the message handler: %s", err)
		os.Exit(1)
	}

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	edit := time.NewTicker(*every)
	defer edit.Stop()
	report := time.NewTicker(5 * time.Second)
	defer report.Stop()
	for {
		select {
		case <-stop:
			return
		case <-edit.C:
			mu.Lock()
			c := fmt.Sprintf("C-%02d", 1+rand.Intn(6))
			r := table[c]
			r.Phone = fmt.Sprintf("555-%04d", rand.Intn(10000))
			r.Version++
			r.Origin = *side
			table[c] = r
			save()
			made++
			publish(r)
			mu.Unlock()
			solace.Logf(role, "edited %s here: phone %s (version %d), published", c, r.Phone, r.Version)
		case <-report.C:
			mu.Lock()
			sum := checksum(table)
			s, m := sent, made
			sent, made = 0, 0
			high := 0
			for _, r := range table {
				if r.Version > high {
					high = r.Version
				}
			}
			mu.Unlock()
			solace.Logf(role, "last 5 s: %s made here, %s sent. Table checksum %s, highest version %d",
				plural(m, "edit"), plural(s, "change"), sum, high)
		}
	}
}
