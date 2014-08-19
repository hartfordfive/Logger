package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/facebookgo/grace/gracehttp"
	"io"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	//"sync/atomic"
	"time"
	"github.com/mindgeekoss/logger/lib/logworker"
)

const (
	PNGPX_B64 string = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR4nGP6zwAAAgcBApocMXEAAAAASUVORK5CYII="
)

var conf = NewConfig()
var stats = NewStats()

var (
	now          = time.Now()
	//channel      = make(chan []byte, 10000) // 6144-1 number of log events can be in the channel before it blocks
	//pending_write_channel      = make(chan logworker.LogEntry, 10000) // 10000-1 number of pending write events can be in the channel before it blocks
	address      = flag.String("a", "0.0.0.0:80", "Address to listen on for logging")
	addressStats = flag.String("r", "0.0.0.0:88", "Address to listen on for stats")
)

var debug, buffer_capacity, num_workers, enable_stats, enable_ssl int
var logdir, configFile string
var workers []*LogWorker
var mutexWrite, mutexCreate, mutexIncr, mutexRPS *sync.Mutex

func init() {

}


/****************************************************************/

func main() {

	runtime.GOMAXPROCS(runtime.NumCPU())

	flag.StringVar(&configFile, "c", "logger.conf", "Load configs from specified config")
	flag.IntVar(&debug, "v", 0, "Start in fmt.Println mode (debug)")
	flag.StringVar(&logdir, "d", "logs", "Directory to dump log files")
	flag.IntVar(&num_workers, "w", 4, "Number of logging workers")
	flag.IntVar(&buffer_capacity, "n", 4096, "Event buffer size")
	flag.IntVar(&enable_ssl, "s", 0, "Enable SSL")
	flag.IntVar(&enable_stats, "es", 1, "Enable status module")
	flag.Parse()


	// Read the config file
	if configFile != "" {
		fmt.Println(utils.DateStampAsString(), "Loading config!")
		err := utils.LoadConfig(configFile, conf)
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
			fmt.Println(utils.DateStampAsString(), "ERROR: Could not created directory: ", conf.LogDir)
			os.Exit(0)
		}
	}

	fmt.Println(utils.ateStampAsString(), "Starting Logger on ", conf.LoggerAddress)

	mutexWrite = &sync.Mutex{}
	mutexCreate = &sync.Mutex{}
	mutexIncr = &sync.Mutex{}
	mutexRPS = &sync.Mutex{}

	// Start the thread to collect the CPU stats every 5 seconds
	//go updateCpuUsageStats()

	
	fh, err := os.OpenFile(strings.TrimRight(conf.LogDir, "/")+"/"+utils.GetLogfileName(), os.O_RDWR|os.O_APPEND, 0660)
	if err != nil {
		if conf.Debug == 1 {
			fmt.Println("\tCould not open file to append data, attempting to create file..")
		}
		fh, err = os.Create(strings.TrimRight(conf.LogDir, "/") + "/" + utils.GetLogfileName())
		if err != nil {
			fmt.Println(utils.DateStampAsString(), "Worker could not open log file! :")
			panic(err)
		}

	}
	conf.CurrentLogFileHandle = fh
	defer conf.CurrentLogFileHandle.Close()
	conf.CurrentLogFile = utils.GetLogfileName()

	workers = make([]*LogWorker, conf.NumWorkers)
	

	go logworker.FileWritter(pending_write_channel)

	for i := 0; i < conf.NumWorkers; i++ {

		if conf.Debug == 1 {
			fmt.Println(utils.DateStampAsString(), "Spawning log worker ", i)
		}
		workers[i] = NewLogWorker(i, conf.LogDir, conf.ByteBufferCapacity)

		go workers[i].ListenForLogEvent(channel, pending_write_channel)
		//go FileWritter(pending_write_channel)
		go workers[i].UpdateRPS()
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
		//uid := r.URL.Query().Get("uid")
		suid := r.URL.Query().Get("suid")
		value := r.URL.Query().Get("value")
		referer := r.URL.Query().Get("referer")
		log_ua := r.URL.Query().Get("log_ua")
		data := r.URL.Query().Get("data")
		requestIP := r.RemoteAddr

		// If the _golog_uuid cookie is not set, then create the uuid and set it
		var udid string
		if conf.GenerateUDID == 1 {
			udid := utils.GetUuidCookie(r)
			if udid == "" {
				y, m, d := time.Now().Date()
				expiryTime := time.Date(y, m, d+365, 0, 0, 0, 0, time.UTC)
				w.Header().Set("Set-Cookie", "udid="+utils.GetUDID()+"; Domain="+conf.CookieDomain+"; Path=/; Expires="+expiryTime.Format(time.RFC1123))
			}
		}

		ts := int(time.Now().Unix())
		str := fmt.Sprintf("%d ~ %s ~ %s ~ %s ~ %s ~ %s ~ %s ~ %s ~ %s ~ %s ~ %s ~ %s ~ %s\n", ts, app_id, category, action, label, clientIP, requestIP, udid, suid, value, referer, log_ua, data)
		Log([]byte(str))

		// Finally, return the tracking pixel and exit the request.
		w.Header().Set("Cache-control", "public, max-age=0")
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Server", "Logger/0.1")
		output, _ := base64.StdEncoding.DecodeString(PNGPX_B64)
		io.WriteString(w, string(output))

	})
	return mux
}

func statsHandler(name string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {

		/*
		stats := map[string]interface{}{
			"status":                 "OK",
			"total_requests_served":  stats.TotalRequestsServed,
			"current_workers":        conf.NumWorkers,
			"current_uptime_seconds": time.Now().Unix() - stats.ProcessStartTime,
			"memory_usage":           0,
			"current_logfile":        getLogfileName(),
			"current_rps":            stats.RPS,
			"current_cpu_usage":		stats.CpuUsagePercentage,
			"curr_max_request_size":  stats.CurrMaxRequestSize,
			"curr_min_request_size":  stats.CurrMinRequestSize,
		}
		*/

		stats.CurrentProcessUptime = time.Now().Unix() - stats.ProcessStartTime
		stats.NumWorkers = conf.NumWorkers
		data, err := json.Marshal(stats)

		w.Header().Set("Cache-control", "public, max-age=0")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Server", "Logger/0.1")

		if err == nil {
			fmt.Fprintf(w, string(data))
		} else {
			fmt.Fprintf(w, "{\"status\": \"ERROR\"}")
		}

	})
	return mux
}
