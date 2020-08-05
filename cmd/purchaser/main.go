package main

import (
	"flag"
	"github.com/mattkimber/purchaser/internal/processor"
	"log"
)

func init() {
	flag.Parse()
}

func main() {
	files := flag.Args()
	for _, file := range files {
		err := processor.Process(file)
		if err != nil {
			log.Panicf("could not open file: %v", err)
		}
	}
}