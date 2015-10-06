package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"

	"golang.org/x/net/proxy"
)

type Config struct {
	Server    string
	Proxy     string
	UploadLog string
}

type LinxJSON struct {
	Filename   string
	Url        string
	Delete_Key string
	Expiry     string
	Size       string
}

func prepareProxyClient(proxyUrl string) *http.Client {
	var dialer proxy.Dialer

	dialer = proxy.Direct

	if proxyUrl != "" {
		u, err := url.Parse(proxyUrl)
		if err != nil {
			log.Fatalf("Failed to parse proxy URL: %v\n", err)
		}

		dialer, err = proxy.FromURL(u, dialer)
		if err != nil {
			log.Fatalf("Failed to obtain proxy dialer: %v\n", err)
		}
	}

	transport := &http.Transport{Dial: dialer.Dial}
	return &http.Client{Transport: transport}
}

func linx(config *Config, filepath string, ttl int, deleteKey string) {
	client := prepareProxyClient(config.Proxy)

	f, err := os.Open(filepath)
	if err != nil {
		log.Fatalf("Failed to open file: %v\n", err)
	}

	uploadUrl := fmt.Sprintf("%supload/%s", config.Server, path.Base(filepath))

	req, err := http.NewRequest("PUT", uploadUrl, bufio.NewReader(f))
	if err != nil {
		log.Fatalf("Failed to create request: %v\n", err)
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("User-Agent", "golinx")
	req.Header.Add("Linx-Expiry", strconv.Itoa(ttl))
	req.Header.Add("Linx-Randomize", "yes")

	if deleteKey != "" {
		req.Header.Add("Linx-Delete-Key", deleteKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Failed to issue request: %v\n", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Fatalf("Upload failed: %s\n", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read the body: %v\n", err)
	}

	var data LinxJSON
	err = json.Unmarshal(body, &data)

	if deleteKey != "" || config.UploadLog != "" {
		fmt.Printf("%s\n", data.Url)
	} else {
		fmt.Printf("%-40s  delete key: %s\n", data.Url, data.Delete_Key)
	}
}

func unlinx(config *Config, url string, deleteKey string) bool {
	client := prepareProxyClient(config.Proxy)

	if !strings.HasPrefix(url, config.Server) {
		log.Fatalf("\"%s\" is not a valid URL for the configured server\n", url)
		return false
	}

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		log.Fatalf("Failed to create request: %v\n", err)
	}

	req.Header.Add("User-Agent", "golinx")
	req.Header.Add("Linx-Delete-Key", deleteKey)

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Failed to issue request: %v\n", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		fmt.Printf("%s: deleted\n", url)
		return true
	} else {
		fmt.Printf("%s: deletion failed: %s\n", url, resp.Status)
		return false
	}
}

func main() {
	config := &Config{}
	var deleteKey string
	var deleteMode bool
	var ttl int

	flag.StringVar(&deleteKey, "deletekey", "",
		"The delete key to use for uploading or deleting a file")
	flag.BoolVar(&deleteMode, "d", false,
		"Delete the specified files instead of uploading")
	flag.IntVar(&ttl, "ttl", 0,
		"Time to live; the length of time in seconds before the file expires")
	flag.StringVar(&config.Server, "server", "http://127.0.0.1:8080/",
		"URL to a linx server")
	flag.StringVar(&config.Proxy, "proxy", "",
		"URL of proxy used to access the server")
	flag.Parse()

	if lastChar := config.Server[len(config.Server)-1:]; lastChar != "/" {
		config.Server = config.Server + "/"
	}

	if deleteMode {
		for _, url := range flag.Args() {
			unlinx(config, url, deleteKey)
		}
	} else {
		for _, filepath := range flag.Args() {
			linx(config, filepath, ttl, deleteKey)
		}
	}
}
