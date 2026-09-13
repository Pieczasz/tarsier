package main

import (
	"log"
	"os"
)

func usage() {
	log.Println("usage: badshop-api [--config path]") // notwant: logs/unstructured-logging
	os.Exit(2)
}
