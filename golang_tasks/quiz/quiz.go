package main

import (
	"bufio"
	"os"
	"io"
    "time"
	"fmt"
	"strings"
	"encoding/csv"
	"flag"
	"log"
    "math/rand"
)

var good_answer int
var bad_answer  int
var user_answer string
var result string

type Quiz struct {
	Question string		`json:"question"`
	Answer	 string		`json:answer"`
}

func GenQuizOrder(slice []int, shuffle bool) []int {
  // fill slice of numbers. 0, 1, 2..len
  for k := range slice {
      slice[k] = k
  }
  ret := make([]int, len(slice)) // make empty copy of slice
  // check for shuffling
  if shuffle == true {
    r := rand.New(rand.NewSource(time.Now().Unix()))
    n := len(slice)
    for i := 0; i < n; i++ {
      randIndex := r.Intn(len(slice))
      ret[i] = slice[randIndex]
      slice = append(slice[:randIndex], slice[randIndex+1:]...)
    }
  } else {
      ret = slice
  }
  return ret
}

func getAnswer(input chan string) {
        in := bufio.NewReader(os.Stdin)
        result, _ := in.ReadString('\n')
        result = strings.TrimSuffix(result, "\n")
        input <- result
}

func main () {
	FileName := flag.String("filename", "problems.csv", "file name for file")
    Timer := flag.Int("timer", 30, "time to answer")
    Shuffle := flag.Bool("shuffle", true, "shuffle questions or not")
	flag.Parse()
	// Open files for test
	csvFile, error := os.Open(*FileName)
	if error != nil {
		log.Fatal(error)
	}
	r := csv.NewReader(bufio.NewReader(csvFile))

	var round []Quiz
    // generate slice of questions from csv
	for {
		line, error := r.Read()
		if error == io.EOF {
			break
		} else if error !=nil {
			log.Fatal(error)
		}
		round = append(round, Quiz{
			Question: line[0],
			Answer:   line[1],
			},)
	}
	// start quiz
    // generate random order for questions
    question_order := make([]int, len(round)) // create array of 0, len = len(round)
    question_order = GenQuizOrder(question_order, *Shuffle) // shuffle
    fmt.Println(question_order)
    // create channel for user input
    input := make(chan string, 1)

    // start rounds, step by step
    for i := range question_order {
        fmt.Println("Question is:", round[question_order[i]].Question)

        go getAnswer(input)
        select {
        case user_answer = <-input:
            // do nothing, we have answer
        case <-time.After(time.Duration(*Timer) * time.Second):
            user_answer = "" // set empty responce
        }
        //fmt.Println("User say:", user_answer, "right answer is:", round[i].Answer)
        if strings.ToLower(round[question_order[i]].Answer) == strings.ToLower(user_answer) {
            good_answer++
        } else {
            bad_answer++
        }
    }
    close(input)
    fmt.Println("In this quiz we have", len(round), "questions")
    fmt.Println("Right answers is:", good_answer, "and wrong:", bad_answer)
}
