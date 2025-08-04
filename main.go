package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/miekg/dns"
)

var serverPort int
var replyWindowMsec int
var upstreamServer string

var replyWindow time.Duration

func flags() {
	flag.IntVar(&serverPort, "port", 5353, "tcp and udp port to listen on for dns requests")
	flag.IntVar(&replyWindowMsec, "reply_window", 500, "millisecond duration to wait and collect dns responses")
	flag.StringVar(&upstreamServer, "upstream", "8.8.8.8:53", "upstream dns server to send requests to")

	flag.Parse()
}

// the one at the end is usually right
func getTheLastOne(_ *dns.Msg, replies []*dns.Msg) (*dns.Msg, error) {
	return replies[len(replies)-1], nil
}

// GFW doesnt send responses with multiple answers
func returnOnlyRoundRobin(_ *dns.Msg, replies []*dns.Msg) (*dns.Msg, error) {
	for _, r := range replies {
		if len(r.Answer) > 1 {
			return r, nil
		}
	}

	return nil, errors.New("no response had multiple answers")
}

func keyForMsg(m *dns.Msg) string {
	out := make([]string, len(m.Answer))
	m = m.Copy()
	for i, a := range m.Answer {
		a.Header().Ttl = 0
		out[i] = a.String()
	}
	return strings.Join(out, "|")
}

// send a query multiple times and return the response is the same
func findSameInMultipleCalls(query *dns.Msg, replies []*dns.Msg) (*dns.Msg, error) {
	client := new(dns.Client)
	resps2, err := client.ExchangePooled(query.Copy(), upstreamServer, replyWindow)
	if err != nil {
		return nil, err
	}

	dupFind := map[string]*dns.Msg{}

	// seed the map with the first set of responses
	for _, r := range replies {
		dupFind[keyForMsg(r)] = r
	}

	// if any exist, its the right one
	for _, r := range resps2 {
		if _, ok := dupFind[keyForMsg(r)]; ok {
			return r, nil
		}
	}

	return nil, errors.New("no duplicates found between the two responses")
}

// check if only one has a non-mutated name
func findNameIntegrity(query *dns.Msg, replies []*dns.Msg) (*dns.Msg, error) {
	origName := query.Question[0].Name

	valid := []*dns.Msg{}

	for _, r := range replies {
		for _, a := range r.Answer {
			if a.Header().Name == origName {
				valid = append(valid, r)
			}
		}
	}

	if len(valid) == 1 {
		return valid[0], nil
	}
	return nil, fmt.Errorf("inconclusive result (%d were correct)", len(valid))
}

func forwardQuestion(req *dns.Msg) ([]*dns.Msg, error) {
	client := new(dns.Client)
	resp, err := client.ExchangePooled(req, upstreamServer, replyWindow)
	return resp, err
}

type hueristicCallback func(query *dns.Msg, replies []*dns.Msg) (*dns.Msg, error)

var handlers = map[string]hueristicCallback{
	"Last one":         getTheLastOne,
	"RR check":         returnOnlyRoundRobin,
	"Compare multiple": findSameInMultipleCalls,
	"Name was intact":  findNameIntegrity,
}

func handleDnsQueries(w dns.ResponseWriter, r *dns.Msg) {
	req := new(dns.Msg)
	r = r.Copy()
	req.MsgHdr = r.MsgHdr
	req.Question = r.Question

	log.Printf("got %d questions from client", len(r.Question))
	for _, q := range r.Question {
		log.Printf("    %s", q.Name)
	}

	resps, err := forwardQuestion(req)
	if err != nil {
		log.Printf("dns forward failed rip: %v", err)
		return
	}

	for name, heu := range handlers {
		resp, err := heu(req, resps)
		if err == nil {
			log.Printf("heu '%s' succeeded", name)
			resp.SetReply(r)
			w.WriteMsg(resp)
			return
		}

		log.Printf("heu '%s' failed: %v", name, err)
	}

	// couldn't get a valid response, NX it
	m := new(dns.Msg)
	m.SetRcode(r, dns.RcodeNameError)
	w.WriteMsg(m)
}

func start(srv *dns.Server) {
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("couldn't start listener: %v", err)
	}
}

func main() {
	flags()

	replyWindow = time.Millisecond * time.Duration(replyWindowMsec)

	udpSrv := &dns.Server{
		Addr: fmt.Sprintf(":%d", serverPort),
		Net:  "udp",
	}
	tcpSrv := &dns.Server{
		Addr: fmt.Sprintf(":%d", serverPort),
		Net:  "tcp",
	}

	dns.HandleFunc(".", handleDnsQueries)

	log.Printf("starting server on %d/udp and %d/tcp\n", serverPort, serverPort)
	go start(udpSrv)
	go start(tcpSrv)

	log.Printf("server running, query will be upstreamed to %s. reply window is %dms", upstreamServer, replyWindowMsec)
	log.Printf("%d heuristics available", len(handlers))

	for k, _ := range handlers {
		log.Printf("  - %s", k)
	}

	// spin the main thread while the server threads are running
	for {
		time.Sleep(1 * time.Second)
	}
}
