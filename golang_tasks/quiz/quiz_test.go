package main

import (
    "testing"
    "reflect"
    "sort"
)

func TestGenQuizOrder (t *testing.T) {

    count := 10
    question_order := make([]int, count)
    for i := 0; i < count; i++ {
        question_order[i] = i
    }

    result_shuffled := GenQuizOrder(question_order, true)
    result_not_shuffled := GenQuizOrder(question_order, false)

    if reflect.DeepEqual(question_order, result_not_shuffled) == false {
        t.Errorf("Slice is different, but shuffle is false!")
    }

    if reflect.DeepEqual(question_order, result_shuffled) == false {
        sort.Ints(result_shuffled)
        if reflect.DeepEqual(question_order, result_shuffled) == false {
            t.Errorf("Slice is different, but shuffle is true!")
        }
    } else {
        t.Errorf("Function return not shuffled slice")
   }
}

func TestgetAnswer (t *testing.T) {

    input := make(chan string, 1)
    text := "test"
    input <- text

    getAnswer(input)
    select {
        case answer := <-input:
            if answer != text {
                t.Errorf("Cant read from chan")
            }
        default:
                t.Errorf("Wrong!")
    }
}
