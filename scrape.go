package main

import (
	"github.com/adam-hanna/arrayOperations"
	"github.com/badoux/checkmail"
	 "github.com/microcosm-cc/bluemonday"
	"fmt"
	"regexp"
	"io/ioutil"
	"syscall"
	"sort"
	"github.com/ungerik/go-rss"
	"html/template"
	"log"
	"net/url"
	"net/http"
	"strings"
	"time"
	"os"
	"github.com/gin-gonic/gin"
	//"github.com/gocolly/colly" // scraping
)

// TODO extract all constants across the file (e.g. paths) for easier tweaking. 

type Job struct {
	Title string
	Date  time.Time
	Url   *url.URL
	Description string
}

var blueMondayPolicy *bluemonday.Policy = bluemonday.StrictPolicy()
var shortDescriptionCutoff int = 350
// How often we should refresh data.
var updateInterval time.Duration = 4 * time.Hour
// The file size limit to be applied with syscall.Setrlimit.
var fslimit uint64 = 1024 * 1024 * 200 // 200 MiB

func (job *Job) ShortDescription() string {
	// TODO get more sophisticated here.
	desc := job.Description + job.Title
	desc = blueMondayPolicy.Sanitize(desc)
	n := len(desc)
	if n > shortDescriptionCutoff {
		n = shortDescriptionCutoff
	}
	return desc[:n]
}

func (job *Job) ExtractKeywords() []string {
	desc := job.Description
	bytes, err := ioutil.ReadFile("config/keywords.txt")
	if err != nil {
		fmt.Println("WARN: couldn't get keywords")
		return nil
	}
	keywordsRaw := string(bytes)
	keywords := strings.Split(keywordsRaw, "\n")
	desc = strings.TrimSpace(desc)
	descWords := regexp.MustCompile(`[\W]+`).Split(desc, -1)
	fmt.Printf("%#v\n", keywords)
	fmt.Printf("%#v\n", descWords)
	matches, _ := arrayOperations.Intersect(keywords, descWords)
	matchesSlice, _ := matches.Interface().([]string)

	sort.Strings(matchesSlice)

	return matchesSlice
}

func getViaRSS(href string) []Job {
	var jobs []Job

	channel, err := rss.Read(href)
	if err != nil {
		log.Println("ERROR:", err)
	}
	log.Println("INFO: Processing", channel.Title)

	for _, item := range channel.Item {
		url, err := url.Parse(item.Link)
		if err != nil {
			log.Println("WARN: couldn't parse url", item.Link)
			continue
		}

		var date time.Time
		err = nil
		date, err = item.PubDate.Parse()
		if err != nil {
			log.Println("WARN: couldn't parse date", item.PubDate)
			continue
		}

		job := Job{
			Title: strings.TrimSpace(item.Title),
			Date:  date,
			Url:   url,
			Description: item.Description,
		}
		//log.Println("INFO: Job: ", job)
		jobs = append(jobs, job)
	}
	return jobs
}

func getWeWorkRemotely(category string) []Job {
	return getViaRSS(fmt.Sprintf("http://weworkremotely.com/categories/%s.rss", category))
}

func getRemoteOK() []Job {
	return getViaRSS("https://remoteok.io/remote-jobs.rss")
}

func getGolangProjects() []Job {
	log.Println("WARN: support for golangprojects.com not implemented yet")
	return []Job{}
}

func getJobs() []Job{
	// TODO: angel.co, HN who is hiring (and what else?)
	var jobs []Job
	jobs = append(jobs, getWeWorkRemotely("remote-programming-jobs")...)
	jobs = append(jobs, getWeWorkRemotely("remote-devops-sysadmin-jobs")...)
	jobs = append(jobs, getRemoteOK()...)
	//jobs = append(jobs, getGolangProjects()...)
	sort.Slice(jobs, func(i, j int) bool {
		// Sort by display time, as otherwise the days might not match up later
		// due to timezone differences.
		fmt := "20060102"
		return jobs[j].Date.Format(fmt) < jobs[i].Date.Format(fmt)
	})
	return jobs
}

func updateSite() {
	_ = os.Mkdir("./static", 0755)
	tmpfile := "static/index.html.tmp"
	for {
		jobs := getJobs()
		// HTML output to stdout
		log.Println("INFO: Updating site...")
		tmpl := template.Must(template.ParseFiles("tmpl/remote-jobs.tmpl.html"))
		f, err := os.OpenFile(tmpfile, os.O_WRONLY | os.O_CREATE | os.O_TRUNC, 0644)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		defer os.Remove(tmpfile)
		err = tmpl.Execute(f, jobs)
		if err != nil {
			panic(err)
		}
		os.Rename("static/index.html.tmp", "static/index.html")
		log.Println("INFO: Finished generating site.")
		time.Sleep(4 * time.Hour)
	}
}

func main() {
	syscall.Setrlimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{Cur: fslimit, Max: fslimit})

	go updateSite()

	r := gin.Default()
	r.Static("/css", "./css")
	r.StaticFile("/", "./static/index.html")
	r.GET("/newsletter-subscribe", func(c *gin.Context) {
		email := c.Query("email")
		honeypot := c.Query("honeypot-captcha")

		if honeypot != "" {
			c.String(http.StatusBadRequest, "drowned in the honeypot")
			return
		}

		err := checkmail.ValidateFormat(email)
		if err != nil {
			c.String(http.StatusBadRequest, "invalid email: %s", err)
			return
		}

		_ = os.Mkdir("./data", 0755)
		f, err := os.OpenFile("data/emails.txt", os.O_WRONLY | os.O_APPEND | os.O_CREATE, 0644)
		if err != nil {
			c.String(http.StatusInternalServerError, "couldn't open email database: %s", err)
			return
		}
		defer f.Close()
		_, err = fmt.Fprintln(f, email)
		if err != nil {
			c.String(http.StatusInternalServerError, "couldn't add email to database: %s", err)
			return
		}

		c.String(http.StatusOK, "subscribed <%s>", email)
	})

	r.Run(":8081")
}
