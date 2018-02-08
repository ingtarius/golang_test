package main

import (
	"bufio"
	"database/sql"
	"flag"
	"fmt"
	_ "github.com/lib/pq"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	user        = "postgres"
	dbname      = "DB_NAME"
	imgAddr     = "IMG_URL_DOMAIN"
	rootPath    = "/IMG/ROOT/"
	tmpFilename = "/var/tmp/images_for_node"
	userId      = 33
)

var wg sync.WaitGroup
var client *http.Client
var count int64
var fail int64
var good int64
var lc int64
var rows struct {
	imageId string
}
var sizes []string

func init() {
	tr := &http.Transport{
		MaxIdleConns:        200,
		MaxIdleConnsPerHost: 200,
		IdleConnTimeout:     30 * time.Second,
		DisableCompression:  false,
	}
	client = &http.Client{Transport: tr}
}

func urlFromId(nodeId string, imageId int64, size string) string {
	fullName := fmt.Sprintf("https://%02s.%v/%s/%v.jpg", nodeId, imgAddr, size, imageId)
	return fullName
}

func pathFromId(shortID string) string {
	imagePath := fmt.Sprintf("%v/%v/%v/%v", rootPath, shortID[4:6], shortID[2:4], shortID[0:2])
	return imagePath
}

func saveFile(imageId int64, nodeId string) {
	for i := 0; i < len(sizes); i++ {
		shortId := fmt.Sprintf("%06d", imageId%1e6)
		path := pathFromId(shortId)
		link := urlFromId(nodeId, imageId, sizes[i])
		// check directory and create it
		if _, err := os.Stat(path); os.IsNotExist(err) {
			os.MkdirAll(path, os.ModePerm)
			os.Chown(path, userId, userId)
		}
		fullFile := fmt.Sprintf("%v/%v_%v.jpg", path, sizes[i], imageId)
		response, err := client.Get(link)
		if err != nil {
			log.Println("Error while downloading", link, "-", err)
			return
		}
		defer response.Body.Close()
		if response.StatusCode != 200 {
			fail += 1
		} else {
			output, err := os.Create(fullFile)
			//log.Println("Saved to ", fullFile)
			good += 1
			if err != nil {
				log.Println("Error while creating", fullFile, "-", err)
				return
			}
			defer output.Close()
			_, err = io.Copy(output, response.Body)
			if err != nil {
				log.Println("Error while downloading", link, "-", err)
				return
			}
		}
	}
}

func dumpImagesId(fileImg, host string, port int, user, dbname string, nodeId string) int64 {
	// make sql request and dump all to file
	// create file
	log.Println("# Make tmp file for image ids:", tmpFilename+nodeId)
	f, err := os.Create(fileImg)
	defer f.Close()
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s dbname=%s sslmode=disable", host, port, user, dbname)
	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		log.Fatal("Cant connect")
	}
	defer db.Close()
	log.Println("# Querying")
	QueryString := fmt.Sprintf("select image_id from items_images where image_id %% 100 IN (%s)", nodeId)
	//QueryString       := fmt.Sprintf("select image_id from items_images where image_id %% 100 IN (%s) LIMIT 10000", nodeId)
	rs, err := db.Query(QueryString)
	if err != nil {
		panic(err)
	}
	for rs.Next() {
		rs.Scan(&rows.imageId)
		f.WriteString(rows.imageId)
		f.WriteString("\n")
		lc++
	}
	defer rs.Close()
	f.Sync()
	log.Println("# Finish. Reading from this file")
	return lc
}

func main() {
	// Print start time
	log.Println(time.Now().Format(time.RFC850))
	// Add variables nodeId, workersCount, imageLevel and bouncer host + port
	nodeId := flag.String("node", "", "ID for image node (Required 00-99)")
	workersCount := flag.Int("workers", 56, "Count for workers. Try to use workers=cpu cores")
	imageLevel := flag.Int("imageLevel", 0, "1 or 2 image level. avi-app or avi-img (Required)")
	host := flag.String("host", "10.9.7.233", "host for DB request with bouncer")
	port := flag.Int("port", 6432, "bouncer port")
	flag.Parse()

	if *nodeId == "" || *imageLevel == 0 {
		flag.PrintDefaults()
		os.Exit(1)
	}
	if *imageLevel == 1 {
		sizes = []string{"140x105"}
	} else {
		sizes = []string{"640x480", "1280x960"}
	}
	fileImg := fmt.Sprintf(tmpFilename + *nodeId)
	// Declare channel for image ids
	// Request to DB and save into file, return total ids in file
	count = dumpImagesId(fileImg, *host, *port, user, dbname, *nodeId)
	imageIdJobs := make(chan int64, *workersCount)

	log.Println("# Total images", count)
	log.Println("# Starting goroutines")
	log.Printf("# Workers: %d", int(*workersCount))
	for w := 1; w <= *workersCount; w++ {
		go worker(imageIdJobs, *nodeId)
	}
	addToChannel(fileImg, imageIdJobs)
	wg.Wait()
	// Print stop time
	log.Println(time.Now().Format(time.RFC850))
	log.Println("# Failed images: ", fail)
	log.Println("# Good   images: ", good)
}

func addToChannel(fileImg string, imageIdJobs chan<- int64) {
	file, err := os.Open(fileImg)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		imageId, _ := strconv.ParseInt(scanner.Text(), 0, 64)
		imageIdJobs <- imageId
	}
	close(imageIdJobs)
}

func worker(imageIdJobs <-chan int64, nodeId string) {
	wg.Add(1)
	for {
		id, err := <-imageIdJobs
		if !err {
			break
		}
		saveFile(id, nodeId)
	}
	wg.Done()
}
