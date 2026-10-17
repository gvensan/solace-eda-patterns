// Command workshop is the single binary behind every dashboard scenario app.
//
// One binary rather than one per role: the Solace Go API links the native
// client library through cgo, so each extra binary costs another copy of it
// and another link step. Attendees wait for one build, once, and every
// scenario's actions are served by the same artifact.
//
// Usage: workshop <scenario> <role> [flags]
package main

import (
	"fmt"
	"os"
	"sort"

	"solace-workshop/apps/scenarios/access"
	"solace-workshop/apps/scenarios/backpressure"
	"solace-workshop/apps/scenarios/cdc"
	"solace-workshop/apps/scenarios/compete"
	"solace-workshop/apps/scenarios/cqrs"
	"solace-workshop/apps/scenarios/dlq"
	"solace-workshop/apps/scenarios/ecst"
	"solace-workshop/apps/scenarios/esrc"
	"solace-workshop/apps/scenarios/exclusive"
	"solace-workshop/apps/scenarios/filter"
	"solace-workshop/apps/scenarios/groups"
	"solace-workshop/apps/scenarios/guaranteed"
	"solace-workshop/apps/scenarios/p2p"
	"solace-workshop/apps/scenarios/partition"
	"solace-workshop/apps/scenarios/pubsub"
	"solace-workshop/apps/scenarios/reqreply"
	"solace-workshop/apps/scenarios/shock"
	"solace-workshop/apps/scenarios/streaming"
	"solace-workshop/apps/scenarios/topics"
)

// roles maps "<scenario> <role>" to the function that runs it. Adding a
// scenario means adding entries here and a package under scenarios/.
var roles = map[string]func([]string){
	"ef watch":          filter.Watch,
	"ef sensors":        filter.Sensors,
	"esrc statements":   esrc.Statements,
	"esrc ledger":       esrc.Ledger,
	"cdc search":        cdc.Search,
	"cdc connector":     cdc.Connector,
	"cdc inventory":     cdc.Inventory,
	"cqrs account":      cqrs.Account,
	"cqrs view":         cqrs.View,
	"cqrs write":        cqrs.Write,
	"cqrs checkout":     cqrs.Checkout,
	"ecst invoicing":    ecst.Invoicing,
	"ecst shipping":     ecst.Shipping,
	"ecst orders":       ecst.Orders,
	"ecst customers":    ecst.Customers,
	"ac app":            access.App,
	"rr till":           reqreply.Till,
	"rr pricing":        reqreply.Pricing,
	"ht stores":         topics.Stores,
	"ht team":           topics.Team,
	"es bikes":          streaming.Bikes,
	"es counter":        streaming.Counter,
	"es board":          streaming.Board,
	"ps shop":           pubsub.Shop,
	"ps subscriber":     pubsub.Subscriber,
	"sa onsale":         shock.OnSale,
	"sa booking":        shock.Booking,
	"bp display":        backpressure.Display,
	"bp reporter":       backpressure.Reporter,
	"bp sensors":        backpressure.Sensors,
	"compete checkout":  compete.Checkout,
	"compete worker":    compete.Worker,
	"dlq shop":          dlq.Shop,
	"dlq poison":        dlq.Poison,
	"dlq expiring":      dlq.Expiring,
	"dlq intake":        dlq.Intake,
	"dlq watch":         dlq.Watch,
	"exclusive publish": exclusive.Publish,
	"exclusive ledger":  exclusive.Ledger,
	"cg catalog":        groups.Catalog,
	"cg member":         groups.Member,
	"gd points":         guaranteed.Points,
	"gd tills":          guaranteed.Tills,
	"p2p send":          p2p.Send,
	"p2p print":         p2p.Print,
	"partition orders":  partition.Orders,
	"partition fulfil":  partition.Fulfil,
}

func main() {
	if len(os.Args) < 3 {
		usage()
		os.Exit(2)
	}
	key := os.Args[1] + " " + os.Args[2]
	run, ok := roles[key]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown scenario role: %s\n\n", key)
		usage()
		os.Exit(2)
	}
	run(os.Args[3:])
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: workshop <scenario> <role> [flags]")
	fmt.Fprintln(os.Stderr, "\nknown roles:")
	keys := make([]string, 0, len(roles))
	for k := range roles {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  %s\n", k)
	}
}
