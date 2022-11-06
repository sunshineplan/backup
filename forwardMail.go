package main

import (
	"flag"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/sunshineplan/forwarder"
	"github.com/sunshineplan/utils/mail"
)

var (
	addr = flag.String("addr", "", "Address")
	to   = flag.String("to", "", "Mail To")
)

var account = &forwarder.Account{IsTLS: true, Sender: &mail.Dialer{Port: 587}}

func main() {
	flag.StringVar(&account.Username, "user", "", "Username")
	flag.StringVar(&account.Password, "pass", "", "Password")
	flag.StringVar(&account.Sender.Server, "server", "", "Mail Host Server")
	flag.StringVar(&account.Sender.Account, "mail", "", "Mail Account")
	flag.StringVar(&account.Sender.Password, "password", "", "Mail Account Password")
	flag.Parse()

	host, port, err := net.SplitHostPort(*addr)
	if err != nil {
		log.Fatal(err)
	}
	account.Server = host
	account.Port, _ = strconv.Atoi(port)
	account.To = strings.Split(*to, ",")

	if _, err := account.Start(false); err != nil {
		log.Fatal(err)
	}
}
