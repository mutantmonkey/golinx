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

	"github.com/adrg/xdg"
	"golang.org/x/net/proxy"
	"gopkg.in/yaml.v2"
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

func linx(config *Config, filepath string, ttl int, deleteKey string) {
	client := prepareProxyClient(config.Proxy)

	f, err := os.Open(filepath)
	if err != nil {
		log.Fatalf("Failed to open file: %v\n", err)
	}

	filename := path.Base(filepath)
	uploadUrl := fmt.Sprintf("%supload/%s", config.Server, filename)
	stat, err := f.Stat()
	if err != nil {
		log.Fatalf("Failed to stat file: %v\n", err)
	}
	reader := NewProgressReader(filename, bufio.NewReader(f), stat.Size())

	req, err := http.NewRequest("PUT", uploadUrl, reader)
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

		f, err := os.OpenFile(config.UploadLog, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0600)
		if err != nil {
			log.Fatalf("Failed to open upload log \"%s\" to write delete key \"%s\": %v", config.UploadLog, data.Delete_Key, err)
		}
		defer f.Close()

		_, err = f.WriteString(fmt.Sprintf("%s:%s\n", data.Filename, data.Delete_Key))
		if err != nil {
			log.Fatalf("Failed to write delete key \"%s\" to log: %v", data.Delete_Key, err)
		}
		f.Sync()
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
	var flags struct {
		deleteKey  string
		deleteMode bool
		ttl        int
		configPath string
		server     string
		proxy      string
		uploadLog  string
	}

	defaultConfigPath, err := xdg.ConfigFile("golinx/config.yml")
	if err != nil {
		log.Print("Unable to get XDG config file path: ", err)
		defaultConfigPath = ""
	}

	flag.StringVar(&flags.deleteKey, "deletekey", "",
		"The delete key to use for uploading or deleting a file")
	flag.BoolVar(&flags.deleteMode, "d", false,
		"Delete the specified files instead of uploading")
	flag.IntVar(&flags.ttl, "ttl", 0,
		"Time to live; the length of time in seconds before the file expires")
	flag.StringVar(&flags.configPath, "config", defaultConfigPath,
		"The path to the config file")
	flag.StringVar(&flags.server, "server", "",
		"URL to a linx server")
	flag.StringVar(&flags.proxy, "proxy", "",
		"URL of proxy used to access the server")
	flag.StringVar(&flags.uploadLog, "uploadlog", "",
		"Path to the upload log file")
	flag.Parse()

	if flags.configPath != "" {
		data, err := ioutil.ReadFile(flags.configPath)
		if err != nil {
			log.Print("Unable to read config file: ", err)
		} else {
			yaml.Unmarshal(data, &config)
		}
	}

	if flags.server != "" {
		config.Server = flags.server
	}

	if flags.proxy != "" {
		config.Proxy = flags.proxy
	}

	if flags.uploadLog != "" {
		config.UploadLog = flags.uploadLog
	}

	if config.Server == "" {
		log.Fatal("Required option server not specified in config or as a flag")
	}

	if lastChar := config.Server[len(config.Server)-1:]; lastChar != "/" {
		config.Server = config.Server + "/"
	}

	if flags.deleteMode {
		deleteKeys := getDeleteKeys(config)

		for _, deleteUrl := range flag.Args() {
			if flags.deleteKey == "" {
				u, err := url.Parse(deleteUrl)
				if err != nil {
					log.Fatalf("Failed to parse URL \"%s\": %v", deleteUrl, err)
				}

				if fileDeleteKey, exists := deleteKeys[u.Path[1:]]; exists {
					unlinx(config, deleteUrl, fileDeleteKey)
				} else {
					fmt.Printf("%s: no delete key found", deleteUrl)
				}
			} else {
				unlinx(config, deleteUrl, flags.deleteKey)
			}
		}
	} else {
		for _, filepath := range flag.Args() {
			linx(config, filepath, flags.ttl, flags.deleteKey)
		}
	}
}
