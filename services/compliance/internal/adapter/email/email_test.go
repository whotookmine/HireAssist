package email

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"
)

// A minimal SMTP server that records the envelope sender and the message.
func fakeSMTP(t *testing.T) (addr string, got chan string) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	got = make(chan string, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		r, w := bufio.NewReader(c), c
		say := func(s string) { _, _ = w.Write([]byte(s + "\r\n")) }
		say("220 fake")
		var log strings.Builder
		for data := false; ; {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			log.WriteString(line)
			switch {
			case data && line == ".\r\n":
				data = false
				say("250 ok")
			case data:
			case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"):
				say("250 fake")
			case strings.HasPrefix(line, "DATA"):
				data = true
				say("354 go")
			case strings.HasPrefix(line, "QUIT"):
				say("221 bye")
				got <- log.String()
				return
			default:
				say("250 ok")
			}
		}
	}()
	t.Cleanup(func() { ln.Close() })
	return ln.Addr().String(), got
}

func TestNotifyUsesTheBareAddressInTheEnvelope(t *testing.T) {
	addr, got := fakeSMTP(t)
	s := SMTP{Addr: addr, From: "HireAssist <no-reply@hireassist.local>"}
	if err := s.Notify(context.Background(), "somchai@acme.example", "Stale", "body"); err != nil {
		t.Fatal(err)
	}
	log := <-got
	for _, want := range []string{"MAIL FROM:<no-reply@hireassist.local>", "RCPT TO:<somchai@acme.example>", "From: HireAssist <no-reply@hireassist.local>", "Subject: Stale"} {
		if !strings.Contains(log, want) {
			t.Errorf("missing %q in\n%s", want, log)
		}
	}
}

func TestNotifyRefusesHeaderInjection(t *testing.T) {
	if err := (SMTP{Addr: "127.0.0.1:1", From: "a@b"}).Notify(context.Background(), "x@y\r\nBcc: z@w", "s", "b"); err == nil {
		t.Error("accepted a recipient with a line break")
	}
}
