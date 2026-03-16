package proxy

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/miekg/dns"
	"github.com/mehrshad/dns-split/internal/rewriter"
)

type Client struct {
	listenAddr string
	serverAddr string
	rewriter   *rewriter.Rewriter
	udpServer  *dns.Server
	tcpServer  *dns.Server
	client     *dns.Client

	// Track in-flight query rewrites: queryID → {original, replacement}
	inflight sync.Map
}

type inflightEntry struct {
	originalName    string
	replacementName string
}

func NewClient(listenAddr, serverAddr string, rw *rewriter.Rewriter) *Client {
	return &Client{
		listenAddr: listenAddr,
		serverAddr: serverAddr,
		rewriter:   rw,
		client: &dns.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (c *Client) Start() error {
	mux := dns.NewServeMux()
	mux.HandleFunc(".", c.handleQuery)

	c.udpServer = &dns.Server{
		Addr:    c.listenAddr,
		Net:     "udp",
		Handler: mux,
	}
	c.tcpServer = &dns.Server{
		Addr:    c.listenAddr,
		Net:     "tcp",
		Handler: mux,
	}

	errCh := make(chan error, 2)
	go func() { errCh <- c.udpServer.ListenAndServe() }()
	go func() { errCh <- c.tcpServer.ListenAndServe() }()

	time.Sleep(50 * time.Millisecond)
	select {
	case err := <-errCh:
		return err
	default:
		log.Printf("client listening on %s (UDP+TCP)", c.listenAddr)
		return nil
	}
}

func (c *Client) Shutdown() {
	if c.udpServer != nil {
		c.udpServer.Shutdown()
	}
	if c.tcpServer != nil {
		c.tcpServer.Shutdown()
	}
}

func (c *Client) handleQuery(w dns.ResponseWriter, r *dns.Msg) {
	if len(r.Question) == 0 {
		dns.HandleFailed(w, r)
		return
	}

	q := r.Question[0]

	// Only rewrite TXT queries
	if q.Qtype == dns.TypeTXT {
		rewritten, replacement := c.rewriter.Replace(q.Name)
		if replacement != "" {
			c.handleRewrittenQuery(w, r, q.Name, rewritten, replacement)
			return
		}
	}

	// Non-TXT or non-matching: forward as-is to server
	c.forwardToServer(w, r)
}

func (c *Client) handleRewrittenQuery(w dns.ResponseWriter, r *dns.Msg, originalName, rewrittenName, replacement string) {
	// Build rewritten query
	outMsg := r.Copy()
	outMsg.Question[0].Name = rewrittenName

	// Use a unique key for tracking (we use a fresh ID to avoid collisions)
	outMsg.Id = dns.Id()
	key := fmt.Sprintf("%d-%s", outMsg.Id, rewrittenName)
	c.inflight.Store(key, &inflightEntry{
		originalName:    originalName,
		replacementName: rewrittenName,
	})
	defer c.inflight.Delete(key)

	log.Printf("client: TXT %s → %s (rewritten)", originalName, rewrittenName)

	network := "udp"
	if _, ok := w.RemoteAddr().(*net.TCPAddr); ok {
		network = "tcp"
	}
	client := &dns.Client{
		Net:     network,
		Timeout: 5 * time.Second,
	}

	resp, _, err := client.Exchange(outMsg, c.serverAddr)
	if err != nil {
		log.Printf("client: server error: %v", err)
		dns.HandleFailed(w, r)
		return
	}

	// Swap replacement domain back to original in the response
	resp.Question[0].Name = originalName
	for _, rr := range resp.Answer {
		rewriteRRName(rr, rewrittenName, originalName)
	}
	for _, rr := range resp.Ns {
		rewriteRRName(rr, rewrittenName, originalName)
	}
	for _, rr := range resp.Extra {
		rewriteRRName(rr, rewrittenName, originalName)
	}

	resp.Id = r.Id
	w.WriteMsg(resp)
}

func (c *Client) forwardToServer(w dns.ResponseWriter, r *dns.Msg) {
	network := "udp"
	if _, ok := w.RemoteAddr().(*net.TCPAddr); ok {
		network = "tcp"
	}
	client := &dns.Client{
		Net:     network,
		Timeout: 5 * time.Second,
	}
	resp, _, err := client.Exchange(r, c.serverAddr)
	if err != nil {
		log.Printf("client: forward error: %v", err)
		dns.HandleFailed(w, r)
		return
	}
	resp.Id = r.Id
	w.WriteMsg(resp)
}
