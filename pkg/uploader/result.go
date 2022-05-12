package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"sync"
	"time"
)

func readLoopResult(part io.Reader, fname string, code string, filetype string) error {
	dst, err := os.Create(fname)

	if err != nil {
		log.Printf("Created path error %s %v", fname, err)
		return err
	}

	var read int64
	lastBuff := false
	for !lastBuff {
		buffer := make([]byte, 100000)
		cBytes, err := part.Read(buffer)
		if err != io.EOF && err != nil {
			return err
		}
		if err == nil && cBytes <= 0 {
			return io.ErrNoProgress
		}

		read = read + int64(cBytes)

		//fmt.Printf("\r read: %v  length : %v \n", read, length)
		lastBuff = (err == io.EOF)

		count, err := dst.Write(buffer[0:cBytes])
		if err != nil {
			return err
		}

		if count != cBytes {
			log.Printf("Unable to write %d, only %d written", cBytes, count)
			return io.ErrShortWrite
		}
		ts := time.Now()
		SyncOperations <- func() {

			task, ok := IDs[code]
			if !ok || task == nil {
				return
			}
			task.TS = ts

			if task.ResultUpload == nil {
				return
			}
			up := task.ResultUpload[filetype]

			if up.Completed != nil {
				return
			}

			up.Uploaded = read

			if lastBuff {
				up.Completed = &ts
			}
		}

	}
	return io.EOF
}

var FileTypes map[string]int

func checkTypeOk(ftype string) bool {
	_, ok := FileTypes[ftype]
	return ok
}

func uploadResultFile(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		ReplyJSONOptions(w, r)
		return
	}

	keys, ok := r.URL.Query()["hcode"]
	code := ""
	if !ok || len(keys[0]) < 1 {
		log.Print("No hcode in URL")
	} else {
		code = keys[0]
	}
	log.Printf("Uploading file with HCODE %s", code)

	filetypes, ok := r.URL.Query()["type"]
	if len(filetypes) != 1 {
		if len(filetypes) == 0 {
			ReplyJSONString("\"File type missing\"", w, r)
			return
		}
		ReplyJSONString("\"File type count error\"", w, r)
		return
	}
	filetype := filetypes[0]
	if checkTypeOk(filetype) {
		ReplyJSONString("\"File type invalid\"", w, r)
		return
	}

	log.Printf("Uploading file with HCODE %s", code)

	initLoad := false
	aper := &initLoad

	mr, err := r.MultipartReader()
	if err != nil {
		ReplyJSONString("\""+err.Error()+"\"", w, r)
		return
	}

	length := r.ContentLength
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
		if task.ResultUpload[filetype] != nil {
			return
		}
		up := &UploadProgress{
			Size:      length,
			Uploaded:  0,
			Created:   ts,
			TS:        ts,
			Completed: nil,
		}

		task.ResultUpload[filetype] = up

		*aper = true
	}

	Signal.Wait()
	Mutex.Unlock()

	if !initLoad {
		ReplyJSONString("\"Already uploading\"", w, r)
		return
	}
	newPath := path.Join(BasePath, code)
	err = os.Mkdir(newPath, 0755)
	if err != nil {
		log.Printf("Failed to created path %s %v", newPath, err)
		ReplyJSONString("\"Impossible to create target path\"", w, r)
		return
	}
	log.Printf("Created path %s", newPath)
	//ticker := time.Tick(time.Millisecond) // <-- use this in production
	//ticker := time.Tick(time.Second) // this is for demo purpose with longer delay

	filePath := path.Join(newPath, "input.json")
	for {

		part, err := mr.NextPart()
		name := part.FormName()
		if name == "file" {
			err = readLoopResult(part, filePath, code, filetype)
			if err != io.EOF && err != nil {
				log.Printf("Error writing file %s %v", filePath, err)
				ReplyJSONString("\"Error writing file\"", w, r)
				break
			}
		} else {
			log.Printf("Ignoring field %s", name)
			buff, count, err := readLoopStub(part)
			log.Printf("Bytes read  %d, %s ", count, string(buff[0:Min(int(count), len(buff))]))
			if err != io.EOF {
				panic(err)
			}
			err = nil
		}
		//fn := part.FormName()

		if err == io.EOF {
			log.Printf("\nDone uploading %s", code)
			TaskQueueInsert(code)
			ReplyJSON([]byte("\"OK\""), w, r, false)
			break
		}

	}
}
