package main

import (
	"bufio"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"
)

var (
	SourceFile string
)

func main() {
	// Get params
	folder := flag.String("folder", "./", "Folder to keep downloaded logs")
	flag.Parse()

	// check if folder exist
	_, err := os.Stat(*folder)
	if err != nil {
		panic(err)
	}

	log.Println("Starting...")
	for {
		files, err := ioutil.ReadDir(*folder)
		if err != nil {
			log.Println(os.Stderr, err)
			os.Exit(1)
		}
		OldestTime := time.Now()
		for _, file := range files {
			if file.Mode().IsRegular() && file.ModTime().Before(OldestTime) && strings.Contains(file.Name(), ".txt") {
				SourceFile = file.Name()
				OldestTime = file.ModTime()
			}
		}
		log.Println(OldestTime, SourceFile)
		// Начинаем работать с выбранным файлом. Без горутин, чтобы не миксовать строчки в приемнике
		file, err := os.Open(*folder + "/" + SourceFile)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			log.Println(scanner.Text())
		}

		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}
		err = os.Remove(*folder + "/" + SourceFile)
	}
}
