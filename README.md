# pmc
Poor man's cluster

There are BIG clusters for the BIG data. Ceph, Mesos, Spark, Hadoop, Flink.. you name it. Complex tools and advanced setup, high salary and great resposibility, advanced perfomance and unbeatable performance. Blah... Blah... Blah... 

But what if you are a simple man with a big file collection... and you have several computers... 

# Case one - very stupid

Assume that you have a simple shared folder with your favorite movies and you need to recode all movies to new h.2xx latest and greatest slowest ever codec.
You have few coputers at home... or just near by. You want to run codec on all of them to accelerate process, but how to split files...  
Here is an idea.  

Run a file processing script on each machine you have mounted your file storage... 
```
#!/bin/bash
until [[ $fileprocess == "." ]]
  do
    date
    if [[ $fileprocess ]];then
      echo "Processing $fileprocess"
      ff-decoder-recoder /path/to where/files/mounted/$fileprocess
      
    else
      echo "Wait to start"
      sleep 1
    fi
  fileprocess=$(curl -s "http://192.168.1.199:3000/get")
  done
date
echo "Complete!"
```

But somehow you should get files from some this URL (aka REST service endpoint)
Assuming you have handler there getting the one file per request or "." if last file being already sent

```
func getHandler(w http.ResponseWriter, r *http.Request) {
	select {
	case inData,ok:=<-WorkSource:
		if(ok) {
			w.Write([]byte(inData))
		}else {
			w.Write([]byte("."))
		}
	case <-time.After(1 * time.Second):
		w.Write([]byte(""))
	}
}

func pollFiles(dirname string, WorkSource chan<- string) {
	defer close(WorkSource)

	files, err := ioutil.ReadDir(dirname)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		if !file.IsDir() {
			WorkSource<-file.Name()
			log.Println("Queueing... "+file.Name())
		}
	}
	log.Println("Complete list of "+dirname+"... ")
}

```
