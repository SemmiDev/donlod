package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	flag.Parse()

	args := flag.Args()
	if len(args) <= 0 {
		log.Println("url empty or invalid")
		os.Exit(1)
	}

	url := args[0]
	if !validURL(url) {
		log.Println("url empty or invalid")
		os.Exit(1)
	}

	done := make(chan bool, 1)
	kill := make(chan os.Signal, 1)

	ticker := time.NewTicker(time.Millisecond * 500)
	fmt.Print("tunggu yah 😊")

	signal.Notify(kill, syscall.SIGINT, syscall.SIGTERM)

	go waitOSNotify(kill, done)
	go downloading(done, ticker)

	response, err := httpGet(url)
	if err != nil {
		done <- true
		log.Println("cannot perform a request")
		os.Exit(1)
	}

	defer response.Body.Close()

	// check status code
	if response.StatusCode != 200 {
		done <- true
		fmt.Printf("request fail, status = %d", response.StatusCode)
		os.Exit(1)
	}

	fileName := getFileName(url)

	// create output file
	file, err := os.Create(fileName)
	if err != nil {
		done <- true
		log.Println(err)
		os.Exit(1)
	}

	defer file.Close()

	err = readWrite(response.Body, file, done)
	if err != nil {
		done <- true
		log.Println(err)
		os.Exit(1)
	}

}

func waitOSNotify(kill chan os.Signal, done chan bool) {
	for range kill {
		log.Println("download interrupted")
		if err := sendDone(done); err != nil {
			log.Printf("failed to send done signal: %v", err)
		}
	}
}

// sendDone sends a message on the done channel and returns an error if the channel is closed.
func sendDone(done chan bool) error {
	select {
	case done <- true:
		return nil
	default:
		return errors.New("done channel is closed")
	}
}

func downloading(done chan bool, ticker *time.Ticker) {
	progress := 0
	for {
		select {
		case <-ticker.C:
			progress++
			if progress > 30 {
				progress = 1
			}
			fmt.Printf("\r[%-30s]", strings.Repeat("=", progress-1)+">")
		case <-done:
			fmt.Println()
			log.Println("Download complete!")
			fmt.Println("========================================")
			os.Exit(0)
			return
		}
	}
}

func httpGet(url string) (*http.Response, error) {
	transport := &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 5 * time.Second,
		IdleConnTimeout:     10 * time.Second,
	}

	httpClient := &http.Client{
		Transport: transport,
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	response, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func readWrite(in io.Reader, out io.Writer, done chan bool) error {
	buffer := make([]byte, 1024)
	reader := bufio.NewReader(in)

	for {
		line, err := reader.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return err
			}
		}

		// write
		_, err = out.Write(buffer[:line])
		if err != nil {
			return err
		}
	}

	done <- true

	return nil

}

func getFileName(urlParam string) string {
	urls := strings.Split(urlParam, "/")
	fileName := urls[len(urls)-1]

	return fileName
}

func validURL(rawurl string) bool {
	if strings.TrimSpace(rawurl) == "" {
		return false
	}

	parsedurl, err := url.Parse(rawurl)
	if err != nil || parsedurl.Scheme == "" || parsedurl.Host == "" {
		return false
	}
	return true
}
