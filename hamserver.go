package main

import (
//	"fmt"
//	"golang.org/x/oauth2"
	"bufio"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

/*
	opts, err := oauth2.New(
    		oauth2.Client("plalonde@acm.org", "=J}PFv5Z~g7P"),
    		oauth2.Endpoint(
        		"https://api.spark.io/oauth/auth",
        		"https://api.spark.io/oauth/token",
		),
    	)

	if err != nil {
		log.Fatal(err)
	}

	// Redirect user to consent page to ask for permission
	// for the scopes specified above.
	url := opts.AuthCodeURL("state", "online", "auto")
	fmt.Printf("Visit the URL for the auth dialog: %v", url)
	
	var code string
	if _, err = fmt.Scan(&code); err != nil {
    	log.Fatal(err)
	}
	t, err := opts.NewTransportFromCode(code)
	if err != nil {
    		log.Fatal(err)
	}

	client := http.Client{Transport: t};
*/

type HamData struct {
	Time string `json:"time,omitempty"`
	Temp float32 `json:"temp"`
	Humid float32 `json:"humid"`
	Light float32 `json:"light"`
}

type CoreInfo struct {
	LastApp string `json:"last_app"`
	LastHeard string `json:"last_heard"`
	Connected bool `json:"connected"`
	DeviceID string `json:"deviceID"`
}

type SparkResult struct {
	Cmd string `json:"cmd"`
	Name string `json:"name"`
	Result string `json:"result"`
	CoreInfo CoreInfo `json:"coreInfo"`
}



func getHamData(logchan chan<- []byte, w http.ResponseWriter, r *http.Request) {
	req, err := http.NewRequest("GET", "https://api.spark.io/v1/devices/48ff71065067555020561287/haminator", nil);
	req.Header = map[string][]string{
		"Authorization": {"Bearer ba591fe058983749ba78cdebb9bd3f87bbb7c438"},
	}

	resp, err := http.DefaultClient.Do(req);
	if err != nil {
		log.Fatal(err)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
    		log.Fatal(err)
	}

	var spark SparkResult
	err = json.Unmarshal(body, &spark)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	} 

	var hamData HamData
	err = json.Unmarshal([]byte(spark.Result), &hamData)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	} 

	hamData.Time = spark.CoreInfo.LastHeard;
log.Println(hamData)
	out, err := json.Marshal(hamData)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	logchan <- out
	w.Write(out)
}

func setupLogger(filename string) chan<- []byte {
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		log.Fatal(err)
	}
	logchan := make(chan []byte)
	go func() {
		defer f.Close()
		for {
			sample := <-logchan
			sample = append(sample, '\n')
			n, err := f.Write(sample)
			if err != nil {
				log.Println("Failed to write full sample:", err, n, len(sample))
			}
			f.Sync()
		}
	}()
	return logchan
}
			
func getAllHamData(w http.ResponseWriter, r *http.Request) {
	f, err := os.OpenFile("ham.json", os.O_RDONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer  f.Close()
	
	in := bufio.NewReader(f)
	data := []HamData{}
	for  {
		line, _, err := in.ReadLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Println("Error reading ham.json:", err)
		}
		var hamData HamData
		err = json.Unmarshal(line, &hamData)
		if err != nil {
			log.Println("Error Unmarshal ham.json:", err)
		}
		data = append(data, hamData)
	}
	// Convert into time series.
	type entry struct {
		Time int64 `json:"x"`
		Value float32 `json:"y"`
	}
	type series struct {
		Data []entry `json:"data"`
	}
	temps := series{[]entry{}}
	humids := series{[]entry{}}
	lights := series{[]entry{}}
		
	for _,v := range data {
		t,err := time.Parse(time.RFC3339, v.Time)
		if err != nil {
			log.Println("Error:", err)
		}
		temp := entry{t.Unix(),v.Temp}
		humid := entry{t.Unix(),v.Humid}
		light := entry{t.Unix(),v.Light}
		temps.Data = append(temps.Data, temp)
		humids.Data = append(humids.Data, humid)
		lights.Data = append(lights.Data, light)
	}

	
	allData := []series{temps, humids, lights}
	
	enc := json.NewEncoder(w)
	err = enc.Encode(allData)
	if err != nil {
		log.Println("Encoding error:", err)
	}
}

func main() {
	logchan := setupLogger("ham.json")
	http.HandleFunc("/ham", func(w http.ResponseWriter, r *http.Request) { getHamData(logchan,w,r) })
	http.HandleFunc("/historic", getAllHamData)
	http.Handle("/", http.FileServer(http.Dir(".")))
	for {
		http.ListenAndServe(":8000", nil)
	}
}


