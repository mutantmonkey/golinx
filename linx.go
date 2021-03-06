package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/adrg/xdg"
	"gopkg.in/cheggaaa/pb.v1"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Server    string
	ApiKey    string
	Proxy     string
	UploadLog string
	Headers   []string
}

type LinxJSON struct {
	Filename   string
	Url        string
	Delete_Key string
	Expiry     string
	Size       string
}

// Type used for passing multiple parameters using the same flag
type multiStringFlag []string

func (m *multiStringFlag) String() string {
	return "[" + strings.Join(*m, " ") + "]"
}

func (m *multiStringFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

func linx(config *Config, filename string, size int64, f io.Reader, ttl int, deleteKey string) (data LinxJSON, err error) {
	client := prepareProxyClient(config.Proxy)

	bar := pb.New(int(size))
	bar.SetUnits(pb.U_BYTES)
	bar.Prefix(fmt.Sprintf("%.20s ", filename))
	bar.Start()

	reader := bar.NewProxyReader(bufio.NewReader(f))

	req, err := http.NewRequest("PUT", config.Server+"upload/"+filename, reader)
	if err != nil {
		err = fmt.Errorf("Failed to create request: %v", err)
		return
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("User-Agent", "golinx")
	req.Header.Add("Linx-Expiry", strconv.Itoa(ttl))
	req.Header.Add("Linx-Randomize", "yes")

	addRequestHeaders(config.Headers, req)

	if config.ApiKey != "" {
		req.Header.Set("Linx-Api-Key", config.ApiKey)
	}
	if deleteKey != "" {
		req.Header.Add("Linx-Delete-Key", deleteKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		err = fmt.Errorf("Failed to issue request: %v", err)
		return
	}
	defer resp.Body.Close()

	bar.Finish()

	if resp.StatusCode != 200 {
		err = fmt.Errorf("Upload failed: %s", resp.Status)
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		err = fmt.Errorf("Failed to read the body: %v", err)
		return
	}

	err = json.Unmarshal(body, &data)
	if err != nil {
		err = fmt.Errorf("Unable to unmarshal JSON: %v", err)
		return
	}

	if deleteKey != "" || config.UploadLog != "" {
		fmt.Printf("%s\n", data.Url)

		var ulog *os.File
		ulog, err = os.OpenFile(config.UploadLog, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0600)
		if err != nil {
			err = fmt.Errorf("Failed to open upload log \"%s\" to write delete key \"%s\": %v", config.UploadLog, data.Delete_Key, err)
			return
		}
		defer ulog.Close()

		_, err = ulog.WriteString(fmt.Sprintf("%s:%s\n", data.Filename, data.Delete_Key))
		if err != nil {
			err = fmt.Errorf("Failed to write delete key \"%s\" to log: %v", data.Delete_Key, err)
			return
		}
		ulog.Sync()
	} else {
		fmt.Printf("%-40s  delete key: %s\n", data.Url, data.Delete_Key)
	}

	return
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

	addRequestHeaders(config.Headers, req)

	if config.ApiKey != "" {
		req.Header.Set("Linx-Api-Key", config.ApiKey)
	}

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
		deleteKey      string
		deleteMode     bool
		ttl            int
		configPath     string
		server         string
		proxy          string
		uploadLog      string
		makeCollection bool
		headers        multiStringFlag
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
	flag.BoolVar(&flags.makeCollection, "collection", false,
		"Create a collection when uploading multiple files")
	flag.Var(&flags.headers, "H",
		"Additional headers to add to the request; multiple -H flags are accepted (optional)")
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

	if len(flags.headers) > 0 {
		config.Headers = append(config.Headers, flags.headers...)
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
		var uploads []string

		for _, filepath := range flag.Args() {
			f, err := os.Open(filepath)
			if err != nil {
				log.Fatalf("Failed to open file: %v\n", err)
			}
			defer f.Close()

			stat, err := f.Stat()
			if err != nil {
				log.Fatalf("Failed to stat file: %v\n", err)
			}

			result, err := linx(config, stat.Name(), stat.Size(), f, flags.ttl, flags.deleteKey)
			if err != nil {
				log.Fatalf("Unable to upload file: %v", err)
			}

			if flags.makeCollection == true {
				uploads = append(uploads, result.Url)
			}
		}

		if flags.makeCollection == true {
			reader := strings.NewReader(strings.Join(uploads, "\n"))
			_, err = linx(config, "linx.collection", int64(reader.Len()), reader, flags.ttl, flags.deleteKey)
			if err != nil {
				log.Fatalf("Unable to upload collection: %v", err)
			}
		}
	}
}
