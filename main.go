package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"strings"
	"time"
)

var Index int

type task struct {
	Description string
	Assigned    string `json:"-"`
	Completed   bool   `json:"-"`
	ApiId       string `json:"-"`
}

type week struct {
	StartDate string
	EndDate   string

	Zones []zone
}

type zone struct {
	Name string

	Users []string
	Tasks []task
}

type configuration struct {
	Zones []zone
}

func getWeekRange() (start, end time.Time) {
	start = time.Now()
	for {
		if start.Weekday() == time.Monday {
			break
		}
		start = start.AddDate(0, 0, -1)
	}

	end = start.AddDate(0, 0, 7)

	return start, end
}

func distributeTasks(startIndex int, users []string, tasks []task) {
	if startIndex < 0 {
		log.Fatal("Mate, the range is out of whack: ", startIndex)
	}

	for i := range tasks {
		tasks[i].Assigned = "Anyone"
		tasks[i].Completed = false
	}

	if len(tasks) > 1 {
		for i, u := range users {
			tasks[(startIndex+i)%len(tasks)].Assigned = u
		}
		return
	}

	tasks[0].Assigned = users[startIndex%len(users)]

}

func randomHex(n int) string {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		log.Fatal(err)
	}
	return hex.EncodeToString(bytes)
}

func main() {

	configPath := flag.String("config", "/usr/local/share/chores/config.json", "Configuration file mapping out zones")
	webroot := flag.String("root", "/usr/local/share/chores/web", "Path where web resources are found")
	state := flag.String("index_path", "/var/chores/index.int", "The index of current person for current task")

	flag.Parse()

	index, err := ioutil.ReadFile(*state)
	if err != nil {
		log.Fatal(err)
	}

	_, err = fmt.Sscan(string(index), &Index)
	if err != nil {
		log.Fatal(err)
	}

	file, err := ioutil.ReadFile(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	var config configuration
	err = json.Unmarshal(file, &config)
	if err != nil {
		log.Fatal(err)
	}

	apiIDMap := make(map[string]*task)
	for j := range config.Zones {
		zone := &config.Zones[j]

		log.Println("Zone \"", zone.Name, "\"")
		log.Println("\tUsers: ", zone.Users)
		log.Println("\tTasks: ", zone.Tasks)

		for i := range zone.Tasks {
			task := &zone.Tasks[i]
			task.ApiId = randomHex(16)
			apiIDMap[task.ApiId] = task
		}

		distributeTasks(Index, zone.Users, zone.Tasks)
	}

	start, end := getWeekRange()

	fs := http.FileServer(http.Dir(*webroot + "/static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	sigs := make(chan os.Signal, 1)

	// `signal.Notify` registers the given channel to
	// receive notifications of the specified signals.
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	// This goroutine executes a blocking receive for
	// signals. When it gets one it'll print it out
	// and then notify the program that it can finish.
	go func() {
		<-sigs

		err = ioutil.WriteFile(*state, []byte(strconv.Itoa(Index)), 0644)
		if err != nil {
			log.Fatal(err)
		}

		os.Exit(0)
	}()

	go func() {
		for {
			<-time.After(end.Sub(time.Now()))

			Index++
			err = ioutil.WriteFile(*state, []byte(strconv.Itoa(Index)), 0644)
			if err != nil {
				log.Fatal(err)
			}

			for j := range config.Zones {
				zone := &config.Zones[j]
				distributeTasks(Index, zone.Users, zone.Tasks)
			}
			start, end = getWeekRange()

		}
	}()

	tmpl := template.Must(template.ParseFiles(*webroot + "/index.html"))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		log.Println(r)

		tw := week{
			StartDate: start.Format("Jan-02-06"),
			EndDate:   end.Format("Jan-02-06"),
			Zones:     config.Zones,
		}

		tmpl.Execute(w, tw)

	})

	http.HandleFunc("/toggle/", func(w http.ResponseWriter, r *http.Request) {
		log.Println(r)

		id := strings.TrimPrefix(r.URL.Path, "/toggle/")
		if task, ok := apiIDMap[id]; ok {
			task.Completed = !task.Completed

			w.WriteHeader(http.StatusNoContent)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	})

	http.HandleFunc("/rotate", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Rotating roster")

		Index++
		err = ioutil.WriteFile(*state, []byte(strconv.Itoa(Index)), 0644)
		if err != nil {
			log.Fatal(err)
		}

		for j := range config.Zones {
			zone := &config.Zones[j]
			distributeTasks(Index, zone.Users, zone.Tasks)
		}
		start, end = getWeekRange()

	})

	err = http.ListenAndServe("127.0.0.1:8080", nil)
	if err != nil {
		log.Fatal(err)
	}
}
