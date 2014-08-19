package logworker

import (
	"bytes"
	"errors"
	"fmt"
	linuxproc "github.com/c9s/goprocinfo/linux"
	uuid "code.google.com/p/go-uuid/uuid"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var Debug int = 1

func GetMonthAsIntString(m string) string {

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

func GetLogfileName() string {
	y, m, d := time.Now().Date()
	return strconv.Itoa(y) + "-" + GetMonthAsIntString(m.String()) + "-" + strconv.Itoa(d) + "-" + strconv.Itoa(time.Now().Hour()) + "00.txt"
}

func LoadConfig(filename string, conf *logworker.Config) error {

	valid := map[string]int{
		"debug": 1, "logger_address": 1, "log_directory": 1, "num_workers": 1, "generate_udid": 1,
		"buffer_capacity": 1, "enable_ssl": 1, "enable_stats": 1, "stats_address": 1,
		"cookie_domain": 1, "dump_to_graphite": 1, "graphite_host": 1, "graphite_port": 1, "force_fsync": 1,
	}

	buf := bytes.NewBuffer(nil)
	f, err := os.Open(filename) // Error handling elided for brevity.
	if err != nil {
		return errors.New("Invalid or missing config!")
	}

	io.Copy(buf, f) // Error handling elided for brevity.
	f.Close()
	s := string(buf.Bytes())

	for _, l := range strings.Split(strings.Trim(s, " "), "\n") {
		// Ignore line that begins with #
		if l == "" || string(l[0]) == "#" {
			continue
		}
		parts := strings.SplitN(strings.Trim(l, " "), "=", 2)
		if _, ok := valid[parts[0]]; ok {
			if parts[0] == "debug" {
				v, _ := strconv.Atoi(parts[1])
				if v < 0 || v > 1 {
					fmt.Println(DateStampAsString(), "Config ERROR: debug can only be 0 or 1")
					os.Exit(1)
				}
				conf.Debug = v
			} else if parts[0] == "logger_address" {
				conf.LoggerAddress = parts[1]
			} else if parts[0] == "log_directory" {
				conf.LogDir = parts[1]
			} else if parts[0] == "num_workers" {
				v, _ := strconv.Atoi(parts[1])
				if v < 4 {
					fmt.Println(DateStampAsString(), "Config ERROR: num_workers must be >= 4")
					os.Exit(1)
				} 
				conf.NumWorkers = v
			} else if parts[0] == "buffer_capacity" {
				v, _ := strconv.Atoi(parts[1])
				if v < 256 {
					fmt.Println(DateStampAsString(), "Config ERROR: buffer_capacity must be >= 256")
					os.Exit(1)
				} 
				conf.ByteBufferCapacity = v
			} else if parts[0] == "enable_ssl" {
				v, _ := strconv.Atoi(parts[1])
				if v != 0 && v != 1 {
					fmt.Println(DateStampAsString(), "Config ERROR: enable_ssl must be 0 or 1")
					os.Exit(1)
				} 
				conf.EnableSSL = v
			} else if parts[0] == "enable_stats" {
				v, _ := strconv.Atoi(parts[1])
				if v != 0 && v != 1 {
					fmt.Println(DateStampAsString(), "Config ERROR: enable_stats must be 0 or 1")
					os.Exit(1)
				} 
				conf.EnableStats = v
			} else if parts[0] == "stats_address" {
				conf.StatsAddress = parts[1]
			} else if parts[0] == "cookie_domain" {
				conf.CookieDomain = parts[1]
			} else if parts[0] == "generate_udid" {
				v, _ := strconv.Atoi(parts[1])
				if v != 0 && v != 1 {
					fmt.Println(DateStampAsString(), "Config ERROR: generate_udid must be 0 or 1")
					os.Exit(1)
				} 
				conf.GenerateUDID = v
			} else if parts[0] == "force_fsync" {
				v, _ := strconv.Atoi(parts[1])
				if v != 0 && v != 1 {
					fmt.Println(DateStampAsString(), "Config ERROR: generate_udid must be 0 or 1")
					os.Exit(1)
				} 
				conf.ForceFsync = v
			}
		}
	}
	return nil
}

func GetUDID() string {
	/*
	f, _ := os.Open("/dev/urandom")
	b := make([]byte, 16)
	f.Read(b)
	f.Close()
	uuid := fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
	return uuid
	*/
	return uuid.NewUUID().String()
}

func GetUuidCookie(r *http.Request) string {
	var uuid string
	cookie := r.Header.Get("Cookie")
	if cookie != "" {
		cookies := strings.Split(cookie, "; ")
		for i := 0; i < len(cookies); i++ {
			parts := strings.Split(cookies[i], "=")
			if parts[0] == "udid" {
				uuid = parts[1]
				break
			}
		}
		// If the cookie isn't found, then generate a udid and then send the cookie
	}
	return uuid
}

func UpdateCpuUsageStats(stats *logworker.Stats) {

	var prev_cpu_total uint64
	var prev_cpu_idle	uint64
	var diff_idle uint64
	var diff_total uint64
	var diff_usage float32

	for {
		stat, err := linuxproc.ReadStat("/proc/stat")
		if err != nil {
			if Debug == 1 {
				fmt.Println("LINUXPROC ERROR: stat read fail")
			}
			time.Sleep(5 * time.Second)
			continue
		}
		//s := stat.CPUStats

		/*
		The meanings of the columns are as follows, from left to right:

		- user: normal processes executing in user mode
		- nice: niced processes executing in user mode
		- system: processes executing in kernel mode
		- idle: twiddling thumbs
		- iowait: waiting for I/O to complete
		- irq: servicing interrupts
		- softirq: servicing softirqs
		- steal: involuntary wait
		- guest: running a normal guest
		- guest_nice: running a niced gues

			Calculating CPU usage:
			CPU_Percentage = ( (Total-PrevTotal) - (Idle-PrevIdle) ) / (Total-PrevTotal)
		*/

		prev_cpu_idle = stat.CPUStatAll.Idle
		prev_cpu_total = stat.CPUStatAll.User + stat.CPUStatAll.Nice +  stat.CPUStatAll.System + stat.CPUStatAll.Idle + stat.CPUStatAll.IOWait + stat.CPUStatAll.IRQ + stat.CPUStatAll.SoftIRQ + stat.CPUStatAll.Steal 

		time.Sleep(1 * time.Second)

		stat, err = linuxproc.ReadStat("/proc/stat")
		diff_idle = stat.CPUStatAll.Idle - prev_cpu_idle
		diff_total = (stat.CPUStatAll.User + stat.CPUStatAll.Nice +  stat.CPUStatAll.System + stat.CPUStatAll.Idle + stat.CPUStatAll.IOWait + stat.CPUStatAll.IRQ + stat.CPUStatAll.SoftIRQ + stat.CPUStatAll.Steal) - prev_cpu_total
		diff_usage =  float32(100 * (diff_total - diff_idle) / diff_total)


		//stats.CpuUsagePercentage = float32(( (curr_cpu_total-prev_cpu_total) - (stat.CPUStatAll.Idle - prev_cpu_idle) ) / (curr_cpu_total-prev_cpu_total))
		stats.CpuUsagePercentage = diff_usage
	}
}

func YmdToString() string {
	t := time.Now()
	y,m,d := t.Date()
	return strconv.Itoa(y)+fmt.Sprintf("%02d", m)+fmt.Sprintf("%02d",d)
}
func DateStampAsString() string{
	t := time.Now()
	return "[" + YmdToString() + " " + fmt.Sprintf("%02d", t.Hour()) + ":" + fmt.Sprintf("%02d", t.Minute()) + ":" + fmt.Sprintf("%02d", t.Second()) + "]"
}