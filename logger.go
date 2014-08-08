package main

import (
    "flag"
    "fmt"
    "net/http"
    "os"
    "time"
    "log"
    "strconv"
    "strings"
    "runtime"
    "sync"
    "bytes"
    "errors"
    "io"
    "encoding/json"
    "encoding/base64"
    "github.com/facebookgo/grace/gracehttp"
)

const (
    PNGPX_B64     string = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR4nGP6zwAAAgcBApocMXEAAAAASUVORK5CYII="
)

type LogWorker struct {
  fileRoot string
  buffer []byte
  position int
  currentLogFile string
  currentLogFileHandle *os.File
  requestsHandled int64
}


type Config struct {
    LoggerAddress string
    StatsAddress string
    Debug int
    LogDir string
    NumWorkers int
    ByteBufferCapacity int
    EnableSSL int
    EnableStats int
}

var conf = NewConfig()

var (
    now      = time.Now()
    channel = make(chan []byte, 6144) // 6144-1 number of log events can be in the channel before it blocks
    address = flag.String("a", "0.0.0.0:80", "Address to listen on for logging")
    addressStats = flag.String("r", "0.0.0.0:88", "Address to listen on for stats")
)

var debug, buffer_capacity, num_workers, enable_stats, enable_ssl int
var logdir, configFile string
var workers []*LogWorker
var mutexWrite, mutexCreate *sync.Mutex


func init() {

}


func NewConfig() (c *Config) {
    return &Config{}
}

func Log(event []byte) {
  select {
    case channel <- event:
    case <- time.After(5 * time.Second):
      // throw away the message, so sad
  }
}

func NewLogWorker(id int) (w *LogWorker) {

    fh, err := os.OpenFile(strings.TrimRight(logdir, "/") + "/" + getLogfileName(), os.O_RDWR|os.O_APPEND, 0660)
    if err != nil {                 
       if debug == 1 { fmt.Println("\tCould not open file to append data, attempting to create file..") }
       fh,err = os.Create(strings.TrimRight(logdir, "/") + "/" + getLogfileName())      
       if err != nil {
            fmt.Println("Worker could not open log file! :")
            panic(err)
       }         
    
    }

  return &LogWorker{
    fileRoot: logdir + "/" + strconv.Itoa(id) + "_",
    buffer: make([]byte, buffer_capacity),
    currentLogFile: getLogfileName(),
    currentLogFileHandle: fh,
  }
}

func (w *LogWorker) ListenForLogEvent(channel chan []byte) {
  for {
    event := <- channel
    length := len(event)
    // we run with nginx's client_max_body_size set to 2K which makes this
    // unlikely to happen, but, just in case...
    if length > buffer_capacity {
      log.Println("message received was too large")
      continue
    }

    if debug == 1 {
        fmt.Println("Msg length: ", length, ", Position: ", w.position, ", Capacity: ", buffer_capacity)
    }

    if (length + w.position) > buffer_capacity {
        if debug == 1 {
            fmt.Println("Dumping buffer to file to file!")
        }
        w.Save()
    }
    copy(w.buffer[w.position:], event)
    w.position += length
  }
}


func (w *LogWorker) Save() {

    if w.position == 0 { return }

    
    if getLogfileName() != w.currentLogFile {
                   
            defer w.currentLogFileHandle.Close()
            mutexCreate.Lock()
            if debug == 1 { fmt.Println("\tCould not open file to append data, attempting to create file..") }
            fh, err := os.Create(strings.TrimRight(logdir, "/") + "/" + getLogfileName())      
            if err != nil {
                fmt.Println("ERROR: Worker could not open new log file!")
                panic(err)
            }         
            w.currentLogFileHandle = fh
            defer w.currentLogFileHandle.Close()
            mutexCreate.Unlock()

    }


    // close fo on exit and check for its returned error
    mutexWrite.Lock()
    w.currentLogFileHandle.Write(w.buffer[0:w.position])
    w.currentLogFileHandle.Sync()
    mutexWrite.Unlock()

    w.position = 0
    runtime.Gosched()
}


/************************************************************/

func getMonthAsIntString(m string) string {

        switch m {
        case "January":
                return "01"
        case "Februrary":
                return "02"
        case "March":
                return "03"
        case "April":
                return "04"
        case "May":
                return "05"
        case "June":
                return "06"
        case "July":
                return "07"
        case "August":
                return "08"
        case "September":
                return "09"
        case "October":
                return "10"
        case "November":
                return "11"
        case "December":
                return "12"
        }
        return "01"
}

func getLogfileName() string {
    y, m, d := time.Now().Date()
    return strconv.Itoa(y) + "-" + getMonthAsIntString(m.String()) + "-" + strconv.Itoa(d) + "-" + strconv.Itoa(time.Now().Hour()) + "00.txt"
}



func loadConfig(filename string, conf *Config) error {

    valid := map[string]int{
        "debug": 1, "logger_address": 1, "log_directory": 1, "num_workers": 1, 
        "buffer_capacity": 1, "enable_ssl": 1, "enable_stats": 1, "stats_address": 1,
    }


    buf := bytes.NewBuffer(nil)
    f, err := os.Open(filename) // Error handling elided for brevity.
    if err != nil {
        return errors.New("Invalid or missing config!")
    }
    io.Copy(buf, f)           // Error handling elided for brevity.
    f.Close()
    s := string(buf.Bytes())
    for _,l:= range strings.Split(strings.Trim(s, " "), "\n") {
        // Ignore line that begins with #
        if string(l[0]) == "#" { continue }
        parts := strings.SplitN(strings.Trim(l," "), "=", 2)
        fmt.Println(parts)
        if _,ok := valid[parts[0]]; ok {
            if parts[0] == "debug" {
                 v,_ := strconv.Atoi(parts[1])
                conf.Debug = v
            } else if parts[0] == "logger_address" {
                conf.LoggerAddress = parts[1]
            } else if parts[0] == "log_directory" {
                conf.LogDir = parts[1]
            } else if parts[0] == "num_workers" {
                v,_ := strconv.Atoi(parts[1])
                conf.NumWorkers = v
            } else if parts[0] == "buffer_capacity" {
                v,_ := strconv.Atoi(parts[1])
                conf.ByteBufferCapacity = v
            } else if parts[0] == "enable_ssl" {
                v,_ := strconv.Atoi(parts[1])
                conf.EnableSSL = v
            } else if parts[0] == "enable_stats" {
                v,_ := strconv.Atoi(parts[1])
                conf.EnableStats = v
            } else if parts[0] == "stats_address" {
                conf.StatsAddress = parts[1]
            }
        }
    }
    return nil
}

/****************************************************************/

func main() {

    runtime.GOMAXPROCS(runtime.NumCPU())

    flag.StringVar(&configFile, "c", "logger.conf", "Load configs from specified config")
    flag.IntVar(&debug, "v", 0, "Start in verbose mode (debug)")
    flag.StringVar(&logdir, "d", "logs", "Directory to dump log files")
    flag.IntVar(&num_workers, "w", 4, "Number of logging workers")
    flag.IntVar(&buffer_capacity, "n", 4096, "Event buffer size")
    flag.IntVar(&enable_ssl, "s", 0, "Enable SSL")
    flag.IntVar(&enable_stats, "es", 1, "Enable status module")
    flag.Parse()


    // Read the config file
    if configFile != "" {
        fmt.Println("Loading config!")
        err := loadConfig(configFile, conf)
        if err != nil {
            panic("ERROR: Please verify that configuration file is valid")
        }
        
    } else {
        conf.LoggerAddress = *address
        conf.Debug = debug
        conf.LogDir = logdir
        conf.NumWorkers = num_workers
        conf.ByteBufferCapacity = buffer_capacity
        conf.EnableSSL = enable_ssl
        conf.EnableStats = enable_stats
        conf.StatsAddress = *addressStats
    }

    // Ensure that the log directory 
    if _, err := os.Stat(conf.LogDir); err != nil {
        if os.IsNotExist(err) {
            err = os.Mkdir(conf.LogDir, 0755)
        }
        if err != nil {
            fmt.Println("ERROR: Could not created directory: ", conf.LogDir)
            os.Exit(0)
        }
    }

    fmt.Println("Starting Logger on ", conf.LoggerAddress)

    mutexWrite = &sync.Mutex{}
    mutexCreate = &sync.Mutex{}

    workers = make([]*LogWorker, num_workers)
    for i := 0; i < num_workers; i++ {
        if debug == 1 {
            fmt.Printf("Spawning log worker %d \n", i)
        }
        workers[i] = NewLogWorker(i)
        defer workers[i].currentLogFileHandle.Close()
        go workers[i].ListenForLogEvent(channel)
    }

    gracehttp.Serve(
        &http.Server{Addr: conf.LoggerAddress, Handler: newHandler("logging_handler")},
        &http.Server{Addr: conf.StatsAddress, Handler: statsHandler("stats_handler")},
    )
}


func newHandler(name string) http.Handler {
    mux := http.NewServeMux()
    mux.HandleFunc("/etahub-web/track", func(w http.ResponseWriter, r *http.Request) {
        
        app_id := r.URL.Query().Get("app_id")
        category := r.URL.Query().Get("category")
        action := r.URL.Query().Get("action")
        label := r.URL.Query().Get("label")
        clientIP := r.URL.Query().Get("clientIP")
        data := r.URL.Query().Get("data")
        uid := r.URL.Query().Get("uid")
        suid := r.URL.Query().Get("suid")
        value := r.URL.Query().Get("value")
        referer := r.URL.Query().Get("referer")
        log_ua := r.URL.Query().Get("log_ua")

        ts := int(time.Now().Unix())
        str := fmt.Sprintf("%d ~ %s ~ %s ~ %s ~ %s ~ %s ~ %s ~ %s ~ %s ~ %s ~ %s ~ %s\n", ts, app_id, category, action, label, clientIP, data, uid, suid, value, referer, log_ua)
        Log([]byte(str))

        // Finally, return the tracking pixel and exit the request.
        w.Header().Set("Cache-control", "public, max-age=0")
        w.Header().Set("Content-Type", "image/png")   
        w.Header().Set("Server","Logger/0.1")
        output,_ := base64.StdEncoding.DecodeString(PNGPX_B64)
        io.WriteString(w, string(output))


    })
    return mux
}

func statsHandler(name string) http.Handler {
    mux := http.NewServeMux()
    mux.HandleFunc("/etahub-web/stats", func(w http.ResponseWriter, r *http.Request) {
        
        stats := map[string]interface{}{"status": "OK", "total_requests_serverd": 1, "current_workers": 1}
        data,err := json.Marshal(stats)

        w.Header().Set("Cache-control", "public, max-age=0")
        w.Header().Set("Content-Type", "application/json")
        w.Header().Set("Server","Logger/0.1")

        if err == nil {
            fmt.Fprintf(w, string(data))
        } else {
            fmt.Fprintf(w, "{\"status\": \"ERROR\"}")
        }

    })
    return mux
}