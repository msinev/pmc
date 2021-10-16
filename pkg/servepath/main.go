package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

var WorkSource <-chan string

func pollFiles(dirname string, WorkSource chan<- string) {
	defer close(WorkSource)

	files, err := ioutil.ReadDir(dirname)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		if !file.IsDir() {
			WorkSource <- file.Name()
			log.Println("Queueing... " + file.Name())
		}

	}
	log.Println("Complete list of " + dirname + "... ")
}

func getHandler(w http.ResponseWriter, r *http.Request) {
	select {
	case inData, ok := <-WorkSource:
		if ok {
			w.Write([]byte(inData))
		} else {
			w.Write([]byte("."))
		}
	case <-time.After(1 * time.Second):
		w.Write([]byte(""))
	}

}

func main() {
	if len(os.Args) != 3 {
		println("Missing some arguments - [ binding address e.g :3000 ] [ path to serve e.g. /opt/data ] ")
		os.Exit(-1)
		return
	}
	PathParam := os.Args[2]
	BindAddr := os.Args[1]
	newTaskChan := make(chan string, 5)
	go pollFiles(PathParam, newTaskChan)
	WorkSource = newTaskChan
	http.HandleFunc("/get", getHandler)
	log.Printf("Serving path %s at %s\nCtrl-C to stop", PathParam, BindAddr)
	http.ListenAndServe(BindAddr, nil)
}
