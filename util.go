package main

import (
	"bufio"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"golang.org/x/net/proxy"
)

func getDeleteKeys(config *Config) (keys map[string]string) {
	keys = make(map[string]string)

	f, err := os.Open(config.UploadLog)
	if err != nil {
		log.Print("Could not open upload log: ", err)
		return
	}

	scanner := bufio.NewScanner(bufio.NewReader(f))
	for scanner.Scan() {
		lineSlice := strings.SplitN(scanner.Text(), ":", 2)
		if len(lineSlice) == 2 {
			keys[lineSlice[0]] = lineSlice[1]
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal("Scanner error: ", err)
	}

	return
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

func addRequestHeaders(headers []string, req *http.Request) error {
	for _, header := range headers {
		h := strings.SplitN(header, ": ", 2)
		req.Header.Add(h[0], h[1])
	}

	return nil
}
