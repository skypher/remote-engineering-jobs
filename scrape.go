package main

import (
	"bytes"
	"fmt"
	"log"
	//"time"
	"github.com/ungerik/go-rss"
	"html/template"
	"net/url"
	"strings"
	//"github.com/gocolly/colly" // scraping
)

type Job struct {
	Title string
	Date  rss.Date
	Url   *url.URL
}

func getViaRSS(href string) []Job {
	var jobs []Job

	channel, err := rss.Read(href)
	if err != nil {
		log.Println(err)
	}
	log.Println("Processing", channel.Title)

	for _, item := range channel.Item {
		url, err := url.Parse(item.Link)
		if err != nil {
			log.Println("*** WARNING: couldn't parse url", item.Link)
			continue
		}
		jobs = append(jobs, Job{
			Title: strings.TrimSpace(item.Title),
			Date:  item.PubDate,
			Url:   url,
		})
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
	log.Println("*** WARNING: support for golangprojects.com not implemented yet")
	return []Job{}
}

func main() {
	var jobs []Job
	jobs = append(jobs, getWeWorkRemotely("remote-programming-jobs")...)
	jobs = append(jobs, getWeWorkRemotely("remote-devops-sysadmin-jobs")...)
	jobs = append(jobs, getRemoteOK()...)
	jobs = append(jobs, getGolangProjects()...)

	// Debugging output to stderr
	//for _, job := range jobs {
	//	log.Println(job)
	//}

	// HTML output to stdout
	tmpl := template.Must(template.ParseFiles("remote-jobs.tmpl.html"))
	buf := new(bytes.Buffer)
	tmpl.Execute(buf, jobs)
	html := buf.String()

	fmt.Println(html)
}
