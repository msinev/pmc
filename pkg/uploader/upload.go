package main

import (
	"compress/gzip"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

type UploadProgress struct {
	Size      int64
	Uploaded  int64
	TS        time.Time
	Created   time.Time
	Completed *time.Time
}

type TaskProgress struct {
	TS        time.Time
	Created   *time.Time
	Queued    *time.Time
	Started   *time.Time
	Completed *time.Time
	Stalled   *time.Time
}

type TheTask struct {
	InitialUpload *UploadProgress
	ResultUpload  map[string]*UploadProgress
	Execute       *TaskProgress
	TS            time.Time
}

var IDs map[string]*TheTask

//
var IDCount int
var IDPrefix string
var IDMutex sync.RWMutex
var SyncOperations chan func()

func GetEnvDefault(env string, def string) string {
	val := os.Getenv(env)
	if val != "" {
		println("Returning actual \"" + val + "\" for " + env)
		return val
	}
	return def
}

func isZipAllowed(r *http.Request) bool {
	ae := r.Header.Values("Accept-Encoding")
	for _, v := range ae {
		if strings.HasPrefix(v, "gzip") {
			return true
		}
	}
	return false
}

const MinZipDataSize = 300

func ReplyJSONString(data string, w http.ResponseWriter, r *http.Request) {
	//Somehow CORS headers needed to be here as well might be not all of headers but... just in case keeep it
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST")
	w.Header().Set("Access-Control-Max-Age", "300")
	w.Header().Set("Content-Type", "application/json")
	log.Printf("Reply JSON error %s", data)
	w.Write([]byte(data))
}

func ReplyErrorString(data string, code int, w http.ResponseWriter, r *http.Request) {
	//Somehow CORS headers needed to be here as well might be not all of headers but... just in case keeep it
	w.WriteHeader(code)
	w.Write([]byte(data))
}

func ReplyJSON(data []byte, w http.ResponseWriter, r *http.Request, zip bool) {
	//Somehow CORS headers needed to be here as well might be not all of headers but... just in case keeep it
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST")
	w.Header().Set("Access-Control-Max-Age", "300")
	w.Header().Set("Content-Type", "application/json")
	if zip && len(data) > MinZipDataSize && isZipAllowed(r) {
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		gz.Write(data)
		gz.Flush()
		gz.Close()
	} else {
		w.Write(data)
	}
}

func ReplyJSONOptions(w http.ResponseWriter, r *http.Request) {
	//	v:=r.Header.Get("Origin")
	//	if len(v)==0 {
	//		v="*"
	//	}

	w.Header().Set("Access-Control-Allow-Origin", GetEnvDefault("Allow-Origin", "*"))
	w.Header().Set("Access-Control-Allow-Headers", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST")
	w.Header().Set("Access-Control-Max-Age", "300")
	w.WriteHeader(http.StatusNoContent)
	//w.Header().Set("Content-Type", "application/json")
	//w.Write([]byte("\"PREFLIGHT CORS REPLY\""))

}

func handleHTTPMethodError(w http.ResponseWriter) {
	http.Error(w, "Method not allowed! Use only GET,POST or OPTIONS", http.StatusMethodNotAllowed)
}

func uploadPage(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(fmt.Sprintf("<html><title>Go upload</title><body><form action='http://localhost:8080/receive' method='post' enctype='multipart/form-data'><label for='file'>Filename:</label><input type='file' name='file' id='file'><input type='submit' value='Upload' ></form></body></html>")))
}

func Executor() {
	IDs = make(map[string]*TheTask, 100)
	opCount := 0

	for v := range SyncOperations {
		opCount++
		//start := time.Now()
		//log.Printf("Operation %d started", opCount)
		v()
		//end := time.Now()
		//log.Printf("Time operation %d %v", opCount, end.Sub(start))
	}
}

const TaskThreshold = 100

func check(hCode string) {
	time.Sleep(10 * time.Second)
	SyncOperations <- func() {
		up, ok := IDs[hCode]
		if !ok || up == nil {
			return
		}
		TSNow := time.Now()
		if TSNow.Sub(up.TS) > TaskThreshold*time.Second {
			log.Printf("Removing task %s", hCode)
			delete(IDs, hCode)
			return
		}
		// log.Printf("Need repeat checking later %s", hCode)
		go check(hCode)
	}
}

func uploadInitID(w http.ResponseWriter, r *http.Request) {

	if r.Method == "OPTIONS" {
		ReplyJSONOptions(w, r)
		return
	}

	if r.Method != "POST" && r.Method != "GET" {
		handleHTTPMethodError(w)
		return
	}

	IDMutex.Lock()
	IDCount++
	c := IDCount
	IDMutex.Unlock()

	scode := fmt.Sprintf("%s==123==SecretCode==%d", IDPrefix, c)
	h := sha1.New()
	h.Write([]byte(scode))
	hID := h.Sum(nil)
	hCode := fmt.Sprintf("%x", hID)

	ts := time.Now()
	SyncOperations <- func() {
		IDs[hCode] =
			&TheTask{
				TS:            ts,
				InitialUpload: nil,
				ResultUpload:  map[string]*UploadProgress{},
				Execute:       nil,
			}

		go check(hCode)
	}
	log.Printf("Initializing HCODE %s", hCode)
	ReplyJSON([]byte("\""+hCode+"\""), w, r, false)
}

type UploadState struct {
	Code     string
	Done     bool
	Size     int64
	Uploaded int64
}

func dequeTaskID(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		ReplyJSONOptions(w, r)
		return
	}

	if r.Method != "POST" && r.Method != "GET" {
		handleHTTPMethodError(w)
		return
	}

	task := TaskQueueRetrive()
	if task == nil {
		ReplyJSONString("\"\"", w, r)
	} else {
		ReplyJSONString("\""+*task+"\"", w, r)
	}

}

func uploadShowID(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		ReplyJSONOptions(w, r)
		return
	}

	if r.Method != "POST" && r.Method != "GET" {
		handleHTTPMethodError(w)
		return
	}

	keys, ok := r.URL.Query()["hcode"]
	code := ""
	if !ok || len(keys[0]) < 1 {
		r.ParseForm()
		code = r.Form.Get("hcode")
	} else {
		code = keys[0]
	}

	percent := UploadState{Code: code,
		Done:     false,
		Size:     0,
		Uploaded: 0,
	}

	var Mutex sync.Mutex
	var Signal *sync.Cond
	Signal = sync.NewCond(&Mutex)
	aper := &percent

	Mutex.Lock()

	ts := time.Now()
	SyncOperations <- func() {
		Mutex.Lock()
		defer Mutex.Unlock()
		defer Signal.Signal()
		task, ok := IDs[code]
		if !ok || task == nil {
			return
		}

		if task.InitialUpload != nil {
			return
		}

		task.TS = ts

		up := task.InitialUpload
		if up.Size != 0 {
			aper.Size = up.Size
			aper.Uploaded = up.Uploaded
			aper.Done = up.Completed != nil
			aper.Code = code
		}
	}
	Signal.Wait()
	Mutex.Unlock()
	data, err := json.Marshal(&percent)
	if err != nil {
		ReplyErrorString(err.Error(), http.StatusInternalServerError, w, r)
	}
	ReplyJSON(data, w, r, false)
}

func getByID(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		ReplyJSONOptions(w, r)
		return
	}

	if r.Method != "POST" && r.Method != "GET" {
		handleHTTPMethodError(w)
		return
	}

	keys, ok := r.URL.Query()["hcode"]
	code := ""
	if !ok || len(keys[0]) < 1 {
		r.ParseForm()
		code = r.Form.Get("hcode")
	} else {
		code = keys[0]
	}

	available := false
	fok := &available

	var Mutex sync.Mutex
	var Signal *sync.Cond
	Signal = sync.NewCond(&Mutex)

	Mutex.Lock()

	ts := time.Now()
	SyncOperations <- func() {
		Mutex.Lock()
		defer Mutex.Unlock()
		defer Signal.Signal()
		task, ok := IDs[code]
		if !ok || task == nil {
			return
		}
		task.TS = ts
		if task.InitialUpload == nil {
			return
		}
		*fok = true
	}
	Signal.Wait()
	Mutex.Unlock()
	if !available {
		ReplyErrorString("Task or file is missing", http.StatusNotFound, w, r)
		return
	}

	newPath := path.Join(BasePath, code)
	filePath := path.Join(newPath, "input.json")
	f, err := os.Open(filePath)
	if err != nil {
		ReplyErrorString("File not opened", http.StatusNotFound, w, r)
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		ReplyErrorString("File not opened", http.StatusNotFound, w, r)
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST")
	w.Header().Set("Access-Control-Max-Age", "300")
	w.Header().Set("Content-Type", MIMEType(code, "input"))
	r.Header.Add("Content-Length", strconv.Itoa(int(fi.Size())))
	io.Copy(w, f)

}

/*
func parseForm(w http.ResponseWriter, r *http.Request) *Upload {
	var upload Upload

	mp_rdr, err := r.MultipartReader()
	if err != nil {
		serverError(w, "error reading multipart: "+err.Error())
		return nil
	}

	for {
		part, err := mp_rdr.NextPart()

		if err == io.EOF {
			break
		}

		switch part.FormName() {
		case "file":
			upload.Data = readPart(part)
		case "filename":
			upload.Name = string(readPart(part))
		case "password":
			upload.Password = string(readPart(part))
		case "mode":
			upload.Mode = string(readPart(part))
		default:
			serverError(w, "invalid form part: "+part.FormName())
			return nil
		}
	}

	return &upload
}
*/

const apiPrefix = "/upload/v1/"

func isDirectory(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	return fileInfo.IsDir(), err
}

var BasePath string

func MIMEType(code string, ftype string) string {
	return "image/jpeg"
}

func main() {
	//
	if len(os.Args) != 3 {
		println("[File path] [Listen URL]")
		os.Exit(-1)
		return
	}
	//
	BasePath = os.Args[1]
	isDirOK, err := isDirectory(BasePath)
	//
	if !isDirOK || err != nil {
		println("Not an existing  path " + BasePath)
		os.Exit(-2)
		return
	}
	//
	initTaskQueue()
	//
	initFileTypes()
	//
	SyncOperations = make(chan func(), 2)
	IDPrefix = time.Now().String()
	go Executor()
	mux := http.NewServeMux()
	//
	mux.HandleFunc("/", uploadPage)
	mux.HandleFunc(apiPrefix+"receive", uploadInitialFile)
	mux.HandleFunc(apiPrefix+"init", uploadInitID)
	mux.HandleFunc(apiPrefix+"stat", uploadShowID)
	mux.HandleFunc(apiPrefix+"nextTask", dequeTaskID)
	mux.HandleFunc(apiPrefix+"get", getByID)
	mux.HandleFunc(apiPrefix+"result", uploadResultFile)
	//mux.HandleFunc(apiPrefix+"put", uploadResultFile)
	//
	http.ListenAndServe(os.Args[2], mux)
	//
}
