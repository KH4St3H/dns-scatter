package proxy

import (
	"log"
	"net"
	"time"

	"github.com/miekg/dns"
	"github.com/mehrshad/dns-split/internal/rewriter"
)

type Server struct {
	listenAddr string
	upstream   string
	rewriter   *rewriter.Rewriter
	udpServer  *dns.Server
	tcpServer  *dns.Server
	client     *dns.Client
}

func NewServer(listenAddr, upstream string, rw *rewriter.Rewriter) *Server {
	return &Server{
		listenAddr: listenAddr,
		upstream:   upstream,
		rewriter:   rw,
		client: &dns.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (s *Server) Start() error {
	mux := dns.NewServeMux()
	mux.HandleFunc(".", s.handleQuery)

	s.udpServer = &dns.Server{
		Addr:    s.listenAddr,
		Net:     "udp",
		Handler: mux,
	}
	s.tcpServer = &dns.Server{
		Addr:    s.listenAddr,
		Net:     "tcp",
		Handler: mux,
	}

	errCh := make(chan error, 2)
	go func() { errCh <- s.udpServer.ListenAndServe() }()
	go func() { errCh <- s.tcpServer.ListenAndServe() }()

	// Wait briefly to catch immediate startup errors
	time.Sleep(50 * time.Millisecond)
	select {
	case err := <-errCh:
		return err
	default:
		log.Printf("server listening on %s (UDP+TCP)", s.listenAddr)
		return nil
	}
}

func (s *Server) Shutdown() {
	if s.udpServer != nil {
		s.udpServer.Shutdown()
	}
	if s.tcpServer != nil {
		s.tcpServer.Shutdown()
	}
}

func (s *Server) handleQuery(w dns.ResponseWriter, r *dns.Msg) {
	if len(r.Question) == 0 {
		dns.HandleFailed(w, r)
		return
	}

	q := r.Question[0]

	// Only rewrite TXT queries that match a known replacement
	if q.Qtype == dns.TypeTXT {
		restored, ok := s.rewriter.Restore(q.Name)
		if ok {
			s.handleRewrittenQuery(w, r, q.Name, restored)
			return
		}
	}

	// Non-TXT or non-matching: forward as-is
	s.forwardUpstream(w, r)
}

func (s *Server) handleRewrittenQuery(w dns.ResponseWriter, r *dns.Msg, replacementName, originalName string) {
	// Build a new query with the original domain
	upstreamMsg := r.Copy()
	upstreamMsg.Question[0].Name = originalName

	log.Printf("server: TXT %s → %s (restored)", replacementName, originalName)

	resp, _, err := s.client.Exchange(upstreamMsg, s.upstream)
	if err != nil {
		log.Printf("server: upstream error: %v", err)
		dns.HandleFailed(w, r)
		return
	}

	// Swap original domain back to replacement domain in the response
	resp.Question[0].Name = replacementName
	for _, rr := range resp.Answer {
		rewriteRRName(rr, originalName, replacementName)
	}
	for _, rr := range resp.Ns {
		rewriteRRName(rr, originalName, replacementName)
	}
	for _, rr := range resp.Extra {
		rewriteRRName(rr, originalName, replacementName)
	}

	resp.Id = r.Id
	w.WriteMsg(resp)
}

func (s *Server) forwardUpstream(w dns.ResponseWriter, r *dns.Msg) {
	network := "udp"
	if _, ok := w.RemoteAddr().(*net.TCPAddr); ok {
		network = "tcp"
	}
	client := &dns.Client{
		Net:     network,
		Timeout: 5 * time.Second,
	}
	resp, _, err := client.Exchange(r, s.upstream)
	if err != nil {
		log.Printf("server: upstream forward error: %v", err)
		dns.HandleFailed(w, r)
		return
	}
	resp.Id = r.Id
	w.WriteMsg(resp)
}

func rewriteRRName(rr dns.RR, from, to string) {
	h := rr.Header()
	if h.Name == from {
		h.Name = to
	}
}
