package main

import (
  "flag"
  _"fmt"
  "os"
  "path/filepath"
  "math/rand"
  "time"
  "github.com/hajimehoshi/go-mp3"
  "github.com/hajimehoshi/oto"
  "io"
)


var dir string
var files []string

// generate random number from total files in music path
func GetRandomNumber (numbers int) int {
  rand.Seed(time.Now().Unix())
  return rand.Intn(numbers)
}

func run ( MusicFile string ) error {
  // open file with music
  f, err := os.Open(MusicFile)
  if err != nil {
    return err
  }
  // mp3 magic
  d, err := mp3.NewDecoder(f)
  if err != nil {
    return err
  }
  // create new player
  p, err := oto.NewContext(d.SampleRate(), 2, 2, 8192)
  if err != nil {
    return err
  }
  player := p.NewPlayer()
  if _, err := io.Copy(player, d); err != nil {
    return err
  }
  player.Close()
  p.Close()
  f.Close()
  return nil
}

func main () {
  // parse args for music path, set defaut
  flag.StringVar(&dir, "dirname", "current", "Directory with music. Absolute path only!!!")
  flag.Parse()
  if dir == "current" {
    dir, _ = os.Getwd()
  }
  //fmt.Println("Search music in", dir)
  // Start generate file list from music dir
  err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
    files = append(files, path)
    return nil
  })
  if err != nil {
    panic(err)
  }
  // infinity loop
  for {
    //fmt.Println("Files in folder -", len(files))
    //fmt.Println("We will play files -", files[GetRandomNumber(len(files))])
    run(files[GetRandomNumber(len(files))])
    time.Sleep(1 * time.Second)
  }
}
