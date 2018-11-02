package main

import (
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	cloudflare_api_url = "https://api.cloudflare.com/client/v4/zones"
	window             = 1 * time.Minute
	offset             = 5 * time.Minute
)

var (
	client     *http.Client
	url        string
	start, end time.Time
)

func init() {
	tr := &http.Transport{
		MaxIdleConns:        200,
		MaxIdleConnsPerHost: 200,
		IdleConnTimeout:     30 * time.Second,
		DisableCompression:  false,
	}
	http.DefaultClient.Timeout = time.Second * 30
	client = &http.Client{Transport: tr}
}

func DownloadLogs(start time.Time, end time.Time, zoneid *string, fields *string, authkey *string, authmail *string, folder *string) string {
	url = string(cloudflare_api_url) + "/" + string(*zoneid) + "/logs/received?start=" + string(start.UTC().Format("2006-01-02T15:04:05")) + "Z&end=" + string(end.UTC().Format("2006-01-02T15:04:05")) + "Z&fields=" + string(*fields)
	//log.Println(url)
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add("X-Auth-Email", *authmail)
	req.Header.Add("X-Auth-Key", *authkey)
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error while downloading -", err)
		return "ERROR"
	}
	out, err := os.Create(*folder + "/" + string(start.Format("2006-01-02T15:04:05")) + "_" + string(end.Format("2006-01-02T15:04:05")) + ".txt")
	io.Copy(out, resp.Body)
	defer resp.Body.Close()
	defer out.Close()
	//log.Println(resp, err)
	return "DONE"
}

func main() {
	// Получаем переменные
	authkey := flag.String("authkey", "", "Auth key for Cloudflare (Required). Example: banana1234")
	authmail := flag.String("authmail", "", "Auth email for Cloudflare (Required). Example: monkey@zoo.com")
	zoneid := flag.String("zoneid", "", "Zone ID for Cloudflare (Required). Example: sdfgho8hioubhoi87")
	fields := flag.String("fields", "ClientRequestHost,ClientIP,ClientRequestURI", "List for fields in responce. See full list on CloudFlare site") // https://support.cloudflare.com/hc/en-us/articles/216672448-Enterprise-Log-Share-Logpull-REST-API
	folder := flag.String("folder", "./", "Folder to keep downloaded logs")
	flag.Parse()

	// Check for empty values
	if *authkey == "" || *authmail == "" || *zoneid == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}
	// check if folder exist
	_, err := os.Stat(*folder)
	if err != nil {
		panic(err)
	}
	//log.Println("# Auth key is", *authkey)
	//log.Println("# Auth email is", *authmail)
	//log.Println("# Zone ID is", *zoneid)
	//log.Println("# Fields is", *fields)
	//now := time.Now().UTC()
	for {
		//now := time.Now().UTC()
		now := time.Now()
		start = now.Add(-(offset + window))
		end = now.Add(-(offset))
		//fmt.Println(start.Format("2006-01-02T15:04:05"), end.Format("2006-01-02T15:04:05"))

		go DownloadLogs(start, end, zoneid, fields, authkey, authmail, folder)
		time.Sleep(window)
		//now = end
	}
}
