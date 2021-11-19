package main

import (
	"context"
	"encoding/base64"
	"flag"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/go-ntlmssp"
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
	flag.StringVar(&dialer.Host, "host", "", "Mail Host Server")
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

func forwardMails(to []string) error {
	c, err := pop3.NewClient(*addr, true)
	if err != nil {
		return err
	}
	defer c.Quit()

	if _, err := c.Cmd("AUTH NTLM", false); err != nil {
		return err
	}

	b, err := ntlmssp.NewNegotiateMessage(*domain, "")
	if err != nil {
		return err
	}

	s, err := c.Cmd(base64.StdEncoding.EncodeToString(b), false)
	if err != nil {
		return err
	}

	b, err = base64.StdEncoding.DecodeString(s)
	if err != nil {
		return err
	}
	b, err = ntlmssp.ProcessChallenge(b, *user, *pass)
	if err != nil {
		return err
	}

	if _, err := c.Cmd(base64.StdEncoding.EncodeToString(b), false); err != nil {
		return err
	}

	count, _, err := c.Stat()
	if err != nil {
		return err
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

	return nil
}

func sendMail(b []byte, to []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	return dialer.SendMail(ctx, dialer.Account, to, b)
}
