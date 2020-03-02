package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

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
	Index int
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

func setupTasks(startIndex int, users []string, tasks []task) {
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

	flag.Parse()

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

		setupTasks(config.Index, zone.Users, zone.Tasks)
	}

	s, e := getWeekRange()

	fs := http.FileServer(http.Dir(*webroot + "/static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	tmpl := template.Must(template.ParseFiles(*webroot + "/index.html"))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		log.Println(r)

		if time.Now().After(e) {
			config.Index++
			for j := range config.Zones {
				zone := &config.Zones[j]
				setupTasks(config.Index, zone.Users, zone.Tasks)
			}
			s, e = getWeekRange()
		}

		tw := week{
			StartDate: s.Format("Jan-02-06"),
			EndDate:   e.Format("Jan-02-06"),
			Zones:     config.Zones,
		}

		tmpl.Execute(w, tw)

	})

	http.HandleFunc("/complete/", func(w http.ResponseWriter, r *http.Request) {
		log.Println(r)

		id := strings.TrimPrefix(r.URL.Path, "/complete/")
		if task, ok := apiIDMap[id]; ok {
			task.Completed = true

			w.WriteHeader(http.StatusNoContent)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	})

	http.HandleFunc("/uncomplete/", func(w http.ResponseWriter, r *http.Request) {
		log.Println(r)

		id := strings.TrimPrefix(r.URL.Path, "/uncomplete/")
		if task, ok := apiIDMap[id]; ok {
			task.Completed = false

			w.WriteHeader(http.StatusNoContent)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	})

	err = http.ListenAndServe("127.0.0.1:8080", nil)
	if err != nil {
		log.Fatal(err)
	}
}
