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
	"sync"
	"time"

	"github.com/Azure/go-ntlmssp"
	"github.com/sunshineplan/utils/httpproxy"
	"github.com/sunshineplan/utils/pop3"
	"github.com/sunshineplan/utils/smtp"
	"github.com/vharitonsky/iniflags"
	"golang.org/x/net/publicsuffix"
)

var (
	addr     = flag.String("addr", "", "Address")
	domain   = flag.String("domain", "", "Domain")
	user     = flag.String("user", "", "Username")
	pass     = flag.String("pass", "", "Password")
	server   = flag.String("server", "", "Mail Host Server")
	account  = flag.String("mail", "", "Mail Account")
	password = flag.String("password", "", "Mail Account Password")
	to       = flag.String("to", "", "Mail To")
	proxy    = flag.String("proxy", "", "Proxy")
)

var smtpClient *smtp.Client
var u *url.URL
var once sync.Once

var self string

func init() {
	var err error
	self, err = os.Executable()
	if err != nil {
		log.Fatalln("Failed to get self path:", err)
	}
}

func main() {
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

	var err error
	if *proxy != "" {
		u, err = url.Parse(*proxy)
		if err != nil {
			log.Fatal(err)
		}
	}

	if err = forwardMails(strings.Split(*to, ",")); err != nil {
		log.Fatal(err)
	}
}

func forwardMails(to []string) error {
	var pop3Client *pop3.Client
	if *proxy == "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		var err error
		pop3Client, err = pop3.DialTLS(ctx, *addr)
		if err != nil {
			return err
		}
	} else {
		conn, err := httpproxy.New(u, nil).Dial("tcp", *addr)
		if err != nil {
			return err
		}

		host, _, err := net.SplitHostPort(*addr)
		if err != nil {
			return err
		}

		pop3Client, err = pop3.NewClient(tls.Client(conn, &tls.Config{ServerName: host}))
		if err != nil {
			return err
		}
	}
	defer pop3Client.Quit()

	if _, err := pop3Client.Cmd("AUTH NTLM", false); err != nil {
		return err
	}

	b, err := ntlmssp.NewNegotiateMessage(*domain, "")
	if err != nil {
		return err
	}

	s, err := pop3Client.Cmd(base64.StdEncoding.EncodeToString(b), false)
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

	if _, err = pop3Client.Cmd(base64.StdEncoding.EncodeToString(b), false); err != nil {
		return err
	}

	count, _, err := pop3Client.Stat()
	if err != nil {
		return err
	}

	for id := 1; id <= count; id++ {
		s, err := pop3Client.Retr(id)
		if err != nil {
			log.Print(err)
			continue
		}

		once.Do(connectSMTP)
		if err := smtpClient.Send(*account, to, []byte(s)); err != nil {
			log.Print(err)
			continue
		}

		if err := pop3Client.Dele(id); err != nil {
			log.Print(err)
		}
	}
	smtpClient.Quit()

	return nil
}

func connectSMTP() {
	var err error
	if *proxy == "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		smtpClient, err = smtp.Dial(ctx, *server+":587")
	} else {
		var conn net.Conn
		conn, err = httpproxy.New(u, nil).Dial("tcp", *server+":587")
		if err != nil {
			log.Fatal(err)
		}

		smtpClient, err = smtp.NewClient(conn, *server)
	}
	if err != nil {
		log.Fatal(err)
	}

	if err = smtpClient.Auth(&smtp.Auth{Username: *account, Password: *password, Server: *server}); err != nil {
		log.Fatal(err)
	}
}
