package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	_ "github.com/lib/pq"
	"github.com/user/whois"
)

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/", homePage)
	r.Route("/api", func(r chi.Router) {
		r.Get("/history", getHistory)
		r.Route("/info", func(r chi.Router) {
			r.Get("/{url}", getInfo)
		})
	})
	err := http.ListenAndServe(":3333", r)
	if err != nil {
		fmt.Println("ListenAndServe:", err)
	}
	//handleRequests()
}

func handleRequests() {
	http.HandleFunc("/", homePage)
	http.HandleFunc("/info/", getInfo)
	log.Fatal(http.ListenAndServe(":9091", nil))
}

func homePage(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Welcome to the HomePage! xd")
	fmt.Println("Endpoint Hit: homePage xd")
}

func getInfo(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Endpoint Hit: getInfo xd")
	url := chi.URLParam(r, "url")

	response, err := http.Get("https://api.ssllabs.com/api/v3/analyze?host=" + string(url))

	if err != nil {
		fmt.Print(err.Error())
		os.Exit(1)
	}

	responseData, err := ioutil.ReadAll(response.Body)
	fmt.Println(string(responseData))
	if err != nil {
		log.Fatal(err)
	}
	insertHistory(string(url))
	out := makeResponse(responseData)
	fmt.Fprintf(w, out)
}

func getHistory(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Endpoint Hit: history xd")
	db, err := sql.Open("postgres", "postgresql://maxroach@localhost:26257/api2?sslmode=disable")
	if err != nil {
		log.Fatal("error connecting to the database: ", err)
	}
	rows, err := db.Query("SELECT name FROM history2")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	var itemsN []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			log.Fatal(err)
		}
		itemsN = append(itemsN, name+" info")
	}
	history := History{
		Items: itemsN,
	}
	out, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(w, string(out))
}

func insertJSON(js string) {
	db, err := sql.Open("postgres", "postgresql://maxroach@localhost:26257/api2?sslmode=disable")
	if err != nil {
		log.Fatal("error connecting to the database: ", err)
	}
	fmt.Println("---------------------------")
	fmt.Println("INSERT INTO jsontable (id,info) VALUES (DEFAULT,'" + js + "')")
	fmt.Println("---------------------------")
	if _, err := db.Exec(
		"INSERT INTO jsontable (id,info) VALUES (DEFAULT,'" + js + "')"); err != nil {
		log.Fatal(err)
	}
}

func insertHistory(name string) {
	db, err := sql.Open("postgres", "postgresql://maxroach@localhost:26257/api2?sslmode=disable")
	if err != nil {
		log.Fatal("error connecting to the database: ", err)
	}

	if _, err := db.Exec(
		"INSERT INTO history2 (id,name) VALUES (DEFAULT,'" + name + "')"); err != nil {
		log.Fatal(err)
	}
}

func insertRequest(url, numServers, ssl string) {
	db, err := sql.Open("postgres", "postgresql://maxroach@localhost:26257/api2?sslmode=disable")
	if err != nil {
		log.Fatal("error connecting to the database: ", err)
	}
	fmt.Println("numServers: " + numServers)
	query := "INSERT INTO full_history (id,url,numServers,sslGrade,time) VALUES (DEFAULT,'" + url + "'," + numServers + ",'" + ssl + "','" + time.Now().String() + "')"
	fmt.Println(query)
	if _, err := db.Exec(query); err != nil {
		log.Fatal(err)
	}
}

func getPast(url string) []PastRequest {
	db, err := sql.Open("postgres", "postgresql://maxroach@localhost:26257/api2?sslmode=disable")
	if err != nil {
		log.Fatal("error connecting to the database: ", err)
	}
	rows, err := db.Query("SELECT id,url,numServers,sslGrade,time FROM full_history WHERE url = '" + url + "'")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	var pastReqs []PastRequest
	var id int
	var urlX, numS, ssl, time string
	for rows.Next() {
		if err := rows.Scan(&id, &urlX, &numS, &ssl, &time); err != nil {
			log.Fatal(err)
		}
		num, err := strconv.Atoi(numS)
		if err == nil {
		}
		req := PastRequest{
			ID:         id,
			URL:        urlX,
			NumServers: num,
			SslGrade:   ssl,
			Time:       time,
		}
		fmt.Printf("%+v\n", req)
		pastReqs = append(pastReqs, req)
	}
	return pastReqs
}

func checkTimeStamps(thenStr string) bool {
	fmt.Println(time.Parse(time.RFC3339, thenStr))

	then, _ := time.Parse("2006-01-02-150405", thenStr)
	//fmt.Println("then:")
	//fmt.Println(then)
	now := time.Now()
	diff := now.Sub(then)
	if diff.Hours() <= 1 {
		return true
	}
	return false
}

func makeResponse(data []byte) string {
	var site Site
	json.Unmarshal([]byte(data), &site)

	var servers []Server
	if site.Status == "ERROR" {
		response := Response{
			Servers:          servers,
			ServersChanged:   false,
			SslGrade:         "",
			PreviousSslGrade: "",
			Logo:             "",
			Title:            "",
			IsDown:           true,
		}
		out, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			log.Fatal(err)
		}
		return string(out)

	}
	var endpoints = site.Endpoints
	for a := 0; a < len(endpoints); a++ {
		country, owner := whois.GetCountryAndOwner(endpoints[a].IPAddress)
		server := Server{
			Address:  endpoints[a].IPAddress,
			SslGrade: endpoints[a].Grade,
			Country:  country,
			Owner:    owner,
		}
		servers = append(servers, server)
	}
	serversChanged, previousSSL := checkPast(site.Host, len(servers))
	sslGrade := checkSSLGrade(servers)
	title := getTitle(site.Host)
	logo := getIcons(site.Host)
	response := Response{
		Servers:          servers,
		ServersChanged:   serversChanged,
		SslGrade:         sslGrade,
		PreviousSslGrade: previousSSL,
		Logo:             logo,
		Title:            title,
		IsDown:           false,
	}
	insertRequest(site.Host, strconv.Itoa(len(response.Servers)), response.SslGrade)
	out, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	insertJSON(string(out))
	return string(out)
}

func getTitle(url string) string {
	if !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}
	response, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer response.Body.Close()

	// Get the response body as a string
	dataInBytes, _ := ioutil.ReadAll(response.Body)
	pageContent := string(dataInBytes)

	// Find a substr
	titleStartIndex := strings.Index(pageContent, "<title>")
	if titleStartIndex == -1 {
		fmt.Println("No title element found")
		os.Exit(0)
	}
	// The start index of the title is the index of the first
	// character, the < symbol. We don't want to include
	// <title> as part of the final value, so let's offset
	// the index by the number of characers in <title>
	titleStartIndex += 7

	// Find the index of the closing tag
	titleEndIndex := strings.Index(pageContent, "</title>")
	if titleEndIndex == -1 {
		fmt.Println("No closing tag for title found.")
		os.Exit(0)
	}

	// (Optional)
	// Copy the substring in to a separate variable so the
	// variables with the full document data can be garbage collected
	pageTitle := []byte(pageContent[titleStartIndex:titleEndIndex])

	////////////////

	response, err = http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer response.Body.Close()

	// Create a goquery document from the HTTP response
	document, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		log.Fatal("Error loading HTTP response body. ", err)
	}

	// Find and print image URLs
	var imgs []string
	document.Find("img").Each(func(index int, element *goquery.Selection) {
		imgSrc, exists := element.Attr("src")
		if exists {
			imgs = append(imgs, imgSrc)
		}
	})

	return string(pageTitle)
}

func checkPast(url string, servers int) (bool, string) {
	requests := getPast(url)
	var changed bool
	var ssl string
	//ssl = requests[len(requests)-1].SslGrade
	//fmt.Println("prev ssl: " + ssl)
	if len(requests) > 0 {
		if checkTimeStamps(requests[len(requests)-1].Time) {
			ssl = requests[len(requests)-1].SslGrade
			fmt.Println("prev ssl: " + ssl)
			if requests[len(requests)-1].NumServers == servers {
				changed = false
			} else {
				changed = true
			}
		}
	}
	return changed, ssl
}

func checkSSLGrade(servers []Server) string {
	var ssls []int
	for a := 0; a < len(servers); a++ {
		if servers[a].SslGrade == "A+" {
			ssls = append(ssls, 7)
		} else if servers[a].SslGrade == "A" {
			ssls = append(ssls, 6)
		} else if servers[a].SslGrade == "B" {
			ssls = append(ssls, 5)
		} else if servers[a].SslGrade == "C" {
			ssls = append(ssls, 4)
		} else if servers[a].SslGrade == "D" {
			ssls = append(ssls, 3)
		} else if servers[a].SslGrade == "E" {
			ssls = append(ssls, 2)
		} else if servers[a].SslGrade == "F" {
			ssls = append(ssls, 1)
		}
	}

	min := ssls[0]
	for a := 0; a < len(ssls); a++ {
		if ssls[a] <= min {
			min = ssls[a]
		}
	}

	var out string

	if min == 7 {
		out = "A+"
	} else if min == 6 {
		out = "A"
	} else if min == 5 {
		out = "B"
	} else if min == 4 {
		out = "C"
	} else if min == 3 {
		out = "D"
	} else if min == 2 {
		out = "E"
	} else if min == 1 {
		out = "F"
	}

	return out
}

func getIcons(url string) string {
	response, err := http.Get("https://besticon-demo.herokuapp.com/allicons.json?url=" + url)

	if err != nil {
		fmt.Print(err.Error())
		os.Exit(1)
	}

	responseData, err := ioutil.ReadAll(response.Body)
	//fmt.Println(string(responseData))
	if err != nil {
		log.Fatal(err)
	}

	var iconsResp IconsResponse
	json.Unmarshal([]byte(responseData), &iconsResp)
	icons := iconsResp.Icons
	var iconsURLs []string
	for a := 0; a < len(icons); a++ {
		iconsURLs = append(iconsURLs, icons[a].URL)
	}
	if len(iconsURLs) > 0 {
		return iconsURLs[0]
	}
	return ""
}

//History Objeto de historial
type History struct {
	Items []string `json:"items"`
}

//Server Objeto de servidor
type Server struct {
	Address  string `json:"address"`
	SslGrade string `json:"ssl_grade"`
	Country  string `json:"country"`
	Owner    string `json:"owner"`
}

//Response Objeto de respuesta
type Response struct {
	Servers          []Server `json:"servers"`
	ServersChanged   bool     `json:"servers_changed"`
	SslGrade         string   `json:"ssl_grade"`
	PreviousSslGrade string   `json:"previous_ssl_grade"`
	Logo             string   `json:"logo"`
	Title            string   `json:"title"`
	IsDown           bool     `json:"is_down"`
}

//PastRequest Objeto de past-request
type PastRequest struct {
	ID         int    `json:"id"`
	URL        string `json:"url"`
	NumServers int    `json:"numServers"`
	SslGrade   string `json:"sslGrade"`
	Time       string `json:"time"`
}

//Site Objeto de sitio
type Site struct {
	Host            string     `json:"host"`
	Port            int        `json:"port"`
	Protocol        string     `json:"protocol"`
	IsPublic        bool       `json:"isPublic"`
	Status          string     `json:"status"`
	StartTime       int        `json:"startTime"`
	TestTime        int        `json:"testTime"`
	EngineVersion   string     `json:"engineVersion"`
	CriteriaVersion string     `json:"criteriaVersion"`
	Endpoints       []Endpoint `json:"endpoints"`
}

//Endpoint Objeto de endpoint
type Endpoint struct {
	IPAddress         string `json:"ipAddress"`
	ServerName        string `json:"serverName"`
	StatusMessage     string `json:"statusMessage"`
	Grade             string `json:"grade"`
	GradeTrustIgnored string `json:"gradeTrustIgnored"`
	HasWasrnings      bool   `json:"hasWarnings"`
	IsExceptional     bool   `json:"isExceptional"`
	Progress          int    `json:"progress"`
	Duration          int    `json:"duration"`
	Delegation        int    `json:"delegation"`
}

//IconsResponse Objeto de iconsResponse
type IconsResponse struct {
	URL   string `json:"url"`
	Icons []Icon `json:"icons"`
}

//Icon Objeto de icon
type Icon struct {
	URL     string `json:"url"`
	Width   string `json:"width"`
	Height  string `json:"height"`
	Format  string `json:"format"`
	Bytes   string `json:"bytes"`
	Error   string `json:"error"`
	Sha1Sum string `json:"sha1sum"`
}
