package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/sunshineplan/cipher"
	"github.com/sunshineplan/utils/archive"
	"github.com/sunshineplan/utils/progressbar"
	"github.com/sunshineplan/utils/txt"
)

const url = "https://github.com/sunshineplan/%s/archive/main.zip"

var repos []string

var key string
var file string

func init() {
	var err error
	repos, err = txt.ReadFile("backup")
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	flag.StringVar(&key, "key", "", "")
	flag.StringVar(&file, "file", "source.code", "")
	flag.Parse()

	pb := progressbar.New(len(repos))
	pb.Start()

	var mu sync.Mutex
	var source []archive.File
	for _, i := range repos {
		go func(repo string) {
			defer pb.Add(1)

			fs, err := download(repo)
			if err != nil {
				log.Fatal(err)
			}

			mu.Lock()
			source = append(source, fs...)
			mu.Unlock()
		}(i)
	}
	pb.Done()

	var buf bytes.Buffer
	err := archive.Pack(&buf, archive.ZIP, source...)
	if err != nil {
		log.Fatal(err)
	}

	err = os.WriteFile(file, cipher.Encrypt([]byte(key), buf.Bytes()), 0666)
	if err != nil {
		log.Fatal(err)
	}
}

func download(repo string) ([]archive.File, error) {
	resp, err := http.Get(fmt.Sprintf(url, repo))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("%s: %s", repo, resp.Status)
	}

	return archive.Unpack(resp.Body)
}
