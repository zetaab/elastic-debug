package main

import (
    "bytes"
    "crypto/tls"
    "encoding/json"
    "github.com/gorilla/mux"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "io/ioutil"
    "log"
    "net/http"
    "time"
    "os"
    "strconv"
)

var reqsuccess = prometheus.NewCounter(
    prometheus.CounterOpts{
        Name: "es_requests_success",
        Help: "Current number of elasticsearch success request",
    },
)

var reqfailed = prometheus.NewCounter(
    prometheus.CounterOpts{
        Name: "es_requests_failed",
        Help: "Current number of elasticsearch failed request",
    },
)

func init() {
    prometheus.MustRegister(reqsuccess)
    prometheus.MustRegister(reqfailed)
}

var TOKEN string
func main() {
    token, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
    if err != nil {
        panic(err)
    }
    TOKEN = string(token)
    threads, err := strconv.Atoi(os.Getenv("THREADS"))
    if err != nil {
        log.Printf(err.Error())
        os.Exit(2)
    }

    for i := 0; i < threads; i++ {
        go fetch()
    }
    router := mux.NewRouter()
    router.HandleFunc("/", frontpage).Methods("GET")
    router.Handle("/metrics", promhttp.Handler())
    log.Fatal(http.ListenAndServe(":8000", router))
}

func fetch() {
    nextTime := time.Now().Truncate(time.Second)
    nextTime = nextTime.Add(time.Second)
    time.Sleep(time.Until(nextTime))
    go call_elastic()
    fetch() 
}

type match struct {
    Hostname string `json:"hostname,omitempty"`
}

type must struct {
    Match match `json:"match"`
}
type bools struct {
    Must []must `json:"must"`
}

type query struct {
    Bool bools `json:"bool"`
}

type esReq struct {
    Query query `json:"query"`
}

func frontpage(w http.ResponseWriter, r *http.Request) {}

func call_elastic() {
    req := esReq{
        Query: query{
            Bool: bools{
                Must: []must{},
            },
        },
    }
    req.Query.Bool.Must = append(req.Query.Bool.Must, must{Match: match{Hostname: "ignorethis"}})
    jsonValue, _ := json.Marshal(req)

    url := os.Getenv("URL")
    tr := &http.Transport{
        TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
    }
    request, _ := http.NewRequest("GET", url, bytes.NewBuffer(jsonValue))
    request.Header.Set("Authorization", "Bearer "+TOKEN)
    request.Header.Set("X-Proxy-Remote-User", "sa")
    request.Header.Set("X-Forwarded-For", "127.0.0.1")
    client := &http.Client{Transport: tr}
    resp, err := client.Do(request)
    if err != nil {
        log.Printf("error: %s", err.Error())
    }
    if resp != nil {
        if resp.StatusCode == 200 {
            reqsuccess.Inc()
        } else if resp.StatusCode == 401 {
            reqfailed.Inc()
            log.Printf("http code 401")
        } else {
            log.Printf("got unexpected code %d", resp.StatusCode)
        }
    }
}