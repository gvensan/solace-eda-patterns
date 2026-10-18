// Package gateway implements the event gateway scenario.
//
// A delivery company's events arrive in three protocols. Its vans report
// their position over MQTT, the protocol their telematics boxes speak. A
// partner's order portal posts new orders over plain HTTP. Both land on the
// same broker as events, and the dispatch service reads them from one queue
// in the Solace API, without knowing how each arrived. Out the other side,
// the broker posts each order to the billing system's webhook over HTTP:
// a REST delivery point, with no Solace client in billing at all.
//
// The broker is the gateway: MQTT and REST are its own services, not a
// separate process. The van speaks MQTT 3.1.1 directly (a small client in
// this file, so the workshop needs no extra library), and the partner uses
// net/http.
package gateway

import (
	"bufio"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"solace-workshop/apps/internal/solace"
	"solace.dev/go/messaging/pkg/solace/message"
)

var cdataReason = regexp.MustCompile(`<reason><!\[CDATA\[(.*?)\]\]></reason>`)

func brokerHost() string { return solace.Env("SOLACE_HOST", "localhost") }

// ------------------------------------------------------------------ MQTT

// mqtt is just enough MQTT 3.1.1 to connect and publish at QoS 1.
type mqtt struct {
	conn net.Conn
	r    *bufio.Reader
	id   uint16
}

func encodeLength(n int) []byte {
	var out []byte
	for {
		b := byte(n % 128)
		n /= 128
		if n > 0 {
			b |= 0x80
		}
		out = append(out, b)
		if n == 0 {
			return out
		}
	}
}

func str(s string) []byte {
	b := make([]byte, 2, 2+len(s))
	binary.BigEndian.PutUint16(b, uint16(len(s)))
	return append(b, s...)
}

func packet(header byte, body []byte) []byte {
	return append(append([]byte{header}, encodeLength(len(body))...), body...)
}

func (m *mqtt) read() (byte, []byte, error) {
	h, err := m.r.ReadByte()
	if err != nil {
		return 0, nil, err
	}
	n, mul := 0, 1
	for {
		b, err := m.r.ReadByte()
		if err != nil {
			return 0, nil, err
		}
		n += int(b&0x7f) * mul
		if b&0x80 == 0 {
			break
		}
		mul *= 128
	}
	body := make([]byte, n)
	_, err = io.ReadFull(m.r, body)
	return h, body, err
}

var connackReasons = map[byte]string{
	1: "unacceptable protocol version",
	2: "identifier rejected",
	3: "server unavailable",
	4: "bad user name or password",
	5: "not authorized",
}

func dialMQTT(addr, clientID, user, password string) (*mqtt, error) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, err
	}
	m := &mqtt{conn: conn, r: bufio.NewReader(conn)}
	body := append(str("MQTT"), 4, 0x80|0x40|0x02, 0, 60)
	body = append(body, str(clientID)...)
	body = append(body, str(user)...)
	body = append(body, str(password)...)
	if _, err := conn.Write(packet(0x10, body)); err != nil {
		conn.Close()
		return nil, err
	}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	h, ack, err := m.read()
	if err != nil {
		conn.Close()
		return nil, err
	}
	if h>>4 != 2 || len(ack) < 2 {
		conn.Close()
		return nil, fmt.Errorf("expected CONNACK, got packet type %d", h>>4)
	}
	if ack[1] != 0 {
		conn.Close()
		return nil, fmt.Errorf("CONNACK return code %d: %s", ack[1], connackReasons[ack[1]])
	}
	return m, nil
}

// publish sends one QoS 1 message and waits for its PUBACK, which the broker
// sends once it has stored the message.
func (m *mqtt) publish(topic, payload string) error {
	m.id++
	if m.id == 0 {
		m.id = 1
	}
	body := str(topic)
	body = binary.BigEndian.AppendUint16(body, m.id)
	body = append(body, payload...)
	if _, err := m.conn.Write(packet(0x32, body)); err != nil {
		return err
	}
	_ = m.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	h, ack, err := m.read()
	if err != nil {
		return err
	}
	if h>>4 != 4 || len(ack) < 2 || binary.BigEndian.Uint16(ack) != m.id {
		return errors.New("no PUBACK for the message")
	}
	return nil
}

func (m *mqtt) close() {
	_, _ = m.conn.Write([]byte{0xE0, 0})
	m.conn.Close()
}

// Van reports its position over MQTT every --every, at QoS 1.
func Van(args []string) {
	fs := flag.NewFlagSet("gw van", flag.ExitOnError)
	role := fs.String("role", "van", "label for log lines")
	van := fs.String("van", "V-07", "van id")
	user := fs.String("user", "svc-gw-vans", "client username to connect as")
	clientName := fs.String("client-name", "gw-van", "MQTT client id")
	every := fs.Duration("every", time.Second, "pause between reports")
	fs.Parse(args)

	addr := net.JoinHostPort(brokerHost(), solace.Env("SOLACE_MQTT_PORT", "1883"))
	m, err := dialMQTT(addr, *clientName, *user, solace.Env("SOLACE_CLIENT_PASSWORD", "solace-workshop"))
	if err != nil {
		solace.Errf(*role, "could not connect over MQTT to %s: %s", addr, err)
		os.Exit(1)
	}
	defer m.close()
	topic := "gw/fleet/van/" + *van + "/location"
	solace.Logf(*role, "connected over MQTT to %s as %s; reporting to %s at QoS 1", addr, *user, topic)
	fmt.Println()

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	lat, lon := 51.5072, -0.1276
	for n := 1; ; n++ {
		lat += (rand.Float64() - 0.5) / 500
		lon += (rand.Float64() - 0.5) / 500
		payload := fmt.Sprintf(`{"van":"%s","n":%d,"lat":%.4f,"lon":%.4f}`, *van, n, lat, lon)
		if err := m.publish(topic, payload); err != nil {
			solace.Errf(*role, "report #%d not stored: %s", n, err)
			os.Exit(1)
		}
		solace.Logf(*role, "report #%d: %.4f, %.4f sent over MQTT, PUBACK received", n, lat, lon)
		select {
		case <-stop:
			return
		case <-time.After(*every):
		}
	}
}

// ------------------------------------------------------------------ REST

// Partner posts a new order over HTTP every --every. The URL path is the
// topic; Solace-Delivery-Mode: persistent makes it a guaranteed message, and
// the broker answers 200 once it has stored it.
func Partner(args []string) {
	fs := flag.NewFlagSet("gw partner", flag.ExitOnError)
	role := fs.String("role", "partner", "label for log lines")
	user := fs.String("user", "svc-gw-partner", "client username for HTTP basic authentication")
	password := fs.String("password", "", "password (default: the workshop's client password)")
	every := fs.Duration("every", 3*time.Second, "pause between orders")
	topic := fs.String("topic", "gw/partner/order/created/v1", "topic to post under; the order id is added")
	fs.Parse(args)
	if *password == "" {
		*password = solace.Env("SOLACE_CLIENT_PASSWORD", "solace-workshop")
	}

	base := "http://" + net.JoinHostPort(brokerHost(), solace.Env("SOLACE_REST_PORT", "9000"))
	solace.Logf(*role, "posting every %s to %s/TOPIC/%s/<order>", *every, base, *topic)
	fmt.Println()
	client := &http.Client{Timeout: 5 * time.Second}

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	seq := int(time.Now().Unix() % 10000)
	for n := 1; ; n++ {
		order := fmt.Sprintf("O-%05d", seq*10+n)
		body := fmt.Sprintf(`{"order":"%s","parcels":%d}`, order, 1+rand.Intn(4))
		req, _ := http.NewRequest(http.MethodPost, base+"/TOPIC/"+*topic+"/"+order, strings.NewReader(body))
		req.SetBasicAuth(*user, *password)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Solace-Delivery-Mode", "persistent")
		resp, err := client.Do(req)
		if err != nil {
			solace.Errf(*role, "%s: %s", order, err)
		} else {
			reply, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				solace.Logf(*role, "%s posted over HTTP: %s", order, resp.Status)
			} else {
				// The broker's answer is an XML document; its reason is the part
				// worth a log line.
				reason := strings.TrimSpace(string(reply))
				if m := cdataReason.FindStringSubmatch(reason); m != nil {
					reason = m[1]
				}
				solace.Errf(*role, "%s refused over HTTP: %d (%s)", order, resp.StatusCode, reason)
			}
		}
		select {
		case <-stop:
			return
		case <-time.After(*every):
		}
	}
}

// ------------------------------------------------------------- consumers

// Dispatch reads one queue with the Solace API, whatever protocol each
// event arrived in.
func Dispatch(args []string) {
	fs := flag.NewFlagSet("gw dispatch", flag.ExitOnError)
	user := fs.String("user", "svc-gw-dispatch", "client username to connect as")
	role := fs.String("role", "dispatch", "label for log lines")
	clientName := fs.String("client-name", "gw-dispatch", "client name the broker reports this app by")
	queue := fs.String("queue", "q.gw.dispatch", "queue to read")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	rcv := solace.BindQueue(*role, *user, svc, *queue, solace.QueueOpts{ClientAck: true})
	defer solace.Terminate(rcv)
	fmt.Println()
	if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
		p := solace.PayloadOf(msg)
		t := msg.GetDestinationName()
		if strings.HasPrefix(t, "gw/fleet/") {
			n, _ := solace.FieldInt(p, "n")
			solace.Logf(*role, "%s position #%d (sent over MQTT, read here over SMF)", solace.FieldString(p, "van"), n)
		} else {
			solace.Logf(*role, "%s, %d parcel(s) to collect (sent over HTTP, read here over SMF)", solace.FieldString(p, "order"), func() int { v, _ := solace.FieldInt(p, "parcels"); return v }())
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

// Billing is a plain HTTP server with no Solace client. The broker's REST
// delivery point posts each order to it, and treats any 2xx as delivered.
func Billing(args []string) {
	fs := flag.NewFlagSet("gw billing", flag.ExitOnError)
	role := fs.String("role", "billing", "label for log lines")
	port := fs.String("port", solace.Env("SOLACE_WEBHOOK_PORT", "18099"), "port to listen on")
	// No Solace connection, so this is only the name signal_app.sh finds
	// the process by.
	_ = fs.String("client-name", "gw-billing", "name the workshop's scripts find this app by")
	fs.Parse(args)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(io.LimitReader(r.Body, 4096))
		order := solace.FieldString(string(body), "order")
		solace.Logf(*role, "%s %s from the broker: %s billed", r.Method, r.URL.Path, order)
		w.WriteHeader(http.StatusOK)
	})
	srv := &http.Server{Addr: ":" + *port, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	solace.Logf(*role, "webhook listening on port %s; no Solace client in this app", *port)
	fmt.Println()
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			solace.Errf(*role, "%s", err)
			os.Exit(1)
		}
	}()
	solace.WaitForStop()
	_ = srv.Close()
}
