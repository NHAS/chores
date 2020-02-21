package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

type Task struct {
	Description string
	Assigned    string `json:"-"`
	Completed   bool   `json:"-"`
}

type Week struct {
	StartDate string
	EndDate   string

	Tasks map[string]Task
}

type Configuration struct {
	Users []string
	Tasks []Task
}

func getWeekRange() (start, end time.Time) {
	current := time.Now()
	for {
		if current.Weekday() == time.Monday {
			break
		}
		current = current.AddDate(0, 0, -1)
	}

	start = current
	end = current.AddDate(0, 0, 6)

	return start, end
}

func buildMap(startIndex int, users []string, tasks []Task) (idToTask map[string]Task) {
	idToTask = make(map[string]Task)

	if startIndex < 0 {
		log.Fatal("Mate, the range is out of whack: ", startIndex)
	}

	for i, _ := range tasks {
		tasks[i].Assigned = "Anyone"
	}

	for i, u := range users {
		tasks[(startIndex+i)%len(tasks)].Assigned = u
	}

	for _, t := range tasks {
		hasher := sha1.New()
		hasher.Write([]byte(t.Description))
		id := string(hex.EncodeToString(hasher.Sum(nil)))

		idToTask[id] = t
	}

	return idToTask
}

func main() {

	file, err := ioutil.ReadFile("config.json")
	if err != nil {
		log.Fatal(err)
	}

	var config Configuration
	err = json.Unmarshal(file, &config)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Users: ", config.Users)
	log.Println("Tasks: ", config.Tasks)

	s, e := getWeekRange()

	startTask := 0
	weekTasks := buildMap(startTask, config.Users, config.Tasks)

	tmpl := template.Must(template.ParseFiles("index.html"))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		log.Println(r)

		if time.Now().After(e) {
			startTask++
			s, e = getWeekRange()
			weekTasks = buildMap(startTask, config.Users, config.Tasks)
		}

		tw := Week{
			StartDate: s.Format("Jan-02-06"),
			EndDate:   e.Format("Jan-02-06"),
			Tasks:     weekTasks,
		}

		tmpl.Execute(w, tw)

	})

	http.HandleFunc("/complete/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/complete/")
		if _, ok := weekTasks[id]; ok {
			completed := weekTasks[id]
			completed.Completed = true
			weekTasks[id] = completed
		}
		log.Println(r)
		w.WriteHeader(http.StatusNoContent)
	})

	http.HandleFunc("/uncomplete/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/uncomplete/")
		if _, ok := weekTasks[id]; ok {
			uncompleted := weekTasks[id]
			uncompleted.Completed = false
			weekTasks[id] = uncompleted
		}
		log.Println(r)
		w.WriteHeader(http.StatusNoContent)
	})

	log.Println("Started up successfully!")
	http.ListenAndServe("127.0.0.1:8080", nil)
}
