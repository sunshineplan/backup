package main

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"flag"
	"log"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/go-ntlmssp"
	"github.com/sunshineplan/utils/httpproxy"
	"github.com/sunshineplan/utils/mail"
	"github.com/sunshineplan/utils/pop3"
	"github.com/vharitonsky/iniflags"
	"golang.org/x/net/publicsuffix"
)

var (
	addr   = flag.String("addr", "", "Address")
	domain = flag.String("domain", "", "Domain")
	user   = flag.String("user", "", "Username")
	pass   = flag.String("pass", "", "Password")
	to     = flag.String("to", "", "Mail To")
	proxy  = flag.String("proxy", "", "Proxy")
)

var dialer = &mail.Dialer{Port: 587}

var self string

func init() {
	var err error
	self, err = os.Executable()
	if err != nil {
		log.Fatalln("Failed to get self path:", err)
	}
}

func main() {
	flag.StringVar(&dialer.Server, "server", "", "Mail Host Server")
	flag.StringVar(&dialer.Account, "mail", "", "Mail Account")
	flag.StringVar(&dialer.Password, "password", "", "Mail Account Password")
	iniflags.SetConfigFile(filepath.Join(filepath.Dir(self), "config.ini"))
	iniflags.SetAllowMissingConfigFile(true)
	iniflags.SetAllowUnknownFlags(true)
	iniflags.Parse()

	if *domain == "" {
		host, _, err := net.SplitHostPort(*addr)
		if err != nil {
			log.Fatal(err)
		}
		*domain, err = publicsuffix.EffectiveTLDPlusOne(host)
		if err != nil {
			log.Fatal(err)
		}
	}

	if err := forwardMails(strings.Split(*to, ",")); err != nil {
		log.Fatal(err)
	}
}

func forwardMails(to []string) (err error) {
	var c *pop3.Client
	if *proxy == "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		c, err = pop3.DialTLS(ctx, *addr)
		if err != nil {
			return
		}
	} else {
		u, err := url.Parse(*proxy)
		if err != nil {
			return err
		}

		conn, err := httpproxy.New(u, nil).Dial("tcp", *addr)
		if err != nil {
			return err
		}

		host, _, err := net.SplitHostPort(*addr)
		if err != nil {
			return err
		}

		c, err = pop3.NewClient(tls.Client(conn, &tls.Config{ServerName: host}))
		if err != nil {
			return err
		}
	}
	defer c.Quit()

	if _, err = c.Cmd("AUTH NTLM", false); err != nil {
		return
	}

	b, err := ntlmssp.NewNegotiateMessage(*domain, "")
	if err != nil {
		return
	}

	s, err := c.Cmd(base64.StdEncoding.EncodeToString(b), false)
	if err != nil {
		return
	}

	b, err = base64.StdEncoding.DecodeString(s)
	if err != nil {
		return
	}
	b, err = ntlmssp.ProcessChallenge(b, *user, *pass)
	if err != nil {
		return
	}

	if _, err = c.Cmd(base64.StdEncoding.EncodeToString(b), false); err != nil {
		return
	}

	count, _, err := c.Stat()
	if err != nil {
		return
	}

	for id := 1; id <= count; id++ {
		s, err := c.Retr(id)
		if err != nil {
			log.Print(err)
			continue
		}

		if err := sendMail([]byte(s), to); err != nil {
			log.Print(err)
			continue
		}

		if err := c.Dele(id); err != nil {
			log.Print(err)
		}
	}

	return
}

func sendMail(b []byte, to []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	return dialer.SendMail(ctx, dialer.Account, to, b)
}
