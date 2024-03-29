package main

// Libsearch queries Overdrive and other online library services for books and checks if they are available to borrow.
// It provides a HTTP server that returns JSON responses for book search queries.

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"time"

	log "github.com/sirupsen/logrus"
)

func init() {
	// Log as JSON instead of the default ASCII formatter.
	// log.SetFormatter(&log.JSONFormatter{})

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	log.SetOutput(os.Stdout)

	// Only log the warning severity or above.
	log.SetLevel(log.InfoLevel)
}

// Setup the HTTP server
func main() {

	log.Info("Starting server")

	// Determine port for HTTP service.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.WithFields(log.Fields{
		"port": port,
	}).Info("HTTP server listening on port")

	http.HandleFunc("/", search)

	http.ListenAndServe(":"+port, nil)

}

func RequestAndParseOverdrive(url string, domain string, ch chan interface{}) {

	// This function makes the HTTP request, parses out the JSON and sends the results to the channel.
	startTime := time.Now()

	// Make a GET request to the Overdrive search URL with the query as a search param
	log.WithFields(log.Fields{
		"url": url,
	}).Info("Making GET request")

	// make the GET request with a browser user agent
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)

	if err != nil {
		fmt.Println(err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/76.0.3809.132 Safari/537.36")

	resp, err := client.Do(req)

	if err != nil {
		fmt.Println(err)
	}

	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
	}

	// Get the HTML response into a string
	html := string(body)

	// Search the HTML using regex for the JSON content
	re := regexp.MustCompile(`window.OverDrive.mediaItems = (.*);`)
	match := re.FindStringSubmatch(html)

	// Search the HTML for the library ID
	re_id := regexp.MustCompile(`window.OverDrive.tenant = (.*);`)
	match_id := re_id.FindStringSubmatch(html)

	log.Info(match_id)

	// If there is a match, decode the JSON and return the results
	if len(match) > 0 {

		log.Info("Found Overdrive results")

		var data map[string]interface{}
		json.Unmarshal([]byte(match[1]), &data)

		out := map[string]interface{}{
			"library": match_id[1],
			"data":    data,
		}

		ch <- out

	} else {

		log.Info("No Overdrive results found")
		ch <- nil

	}

	log.WithFields(log.Fields{
		"duration": time.Since(startTime),
	}).Info("Request completed")

}

// Search function
func search(w http.ResponseWriter, r *http.Request) {

	startTime := time.Now()

	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Get the 'search' query parameter
	query := r.URL.Query().Get("query")

	// Reject request if the query param is not found or if the length is zero
	if query == "" || len(query) == 0 {
		log.Error("Query parameter not found")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Initialize "service" struct with fields name, url and domains. Domains is a slice with any number of strings.
	type service struct {
		Name    string
		Url     string
		Domains []string
	}

	// Dictionary of library services, their respective search URLs and available domains (e.g erl, nypl, etc)
	services := map[string]service{
		"overdrive": service{
			Name:    "Overdrive",
			Url:     "https://%s.overdrive.com/search",
			Domains: []string{"lapl", "erl", "portphillip", "boroondara", "baysidelibrary"},
		},
		"cloudlibrary": service{
			Name:    "Cloud Library",
			Url:     "https://ebook.yourcloudlibrary.com/uisvc/%s/Search/CatalogSearch?media=all&src=lib",
			Domains: []string{"melbourne", "hobsonsbay", "yarra"},
		},
	}

	type searchresponse struct {
		Overdrive []interface{}
		// cloudlibrary []interface{}
	}

	// Create a new searchResponse struct and export it as JSON
	searchResponse := searchresponse{
		Overdrive: []interface{}{},
	}

	log.WithFields(log.Fields{
		"query": query,
	}).Info("Search query received")

	// Make the channel that the concurrent
	// goroutines will output into
	channel := make(chan interface{})

	// Do Overdrive processing
	for _, domain := range services["overdrive"].Domains {

		log.WithFields(log.Fields{
			"domain": domain,
			"query":  query,
		}).Info("Searching Overdrive")

		// Add query to the end of the URL and encode it
		url := fmt.Sprintf(services["overdrive"].Url, domain) + "?query=" + url.QueryEscape(query)

		go RequestAndParseOverdrive(url, domain, channel)

	}

	// Get all the results from the channel
	for i := 0; i < len(services["overdrive"].Domains); i++ {

		// Assign channel output to a variable
		result := <-channel

		// If the result is nil, skip it
		if result == nil {
			continue
		}

		// Append the results to the searchResponse struct
		searchResponse.Overdrive = append(searchResponse.Overdrive, result)

	}

	log.Info("Returning searchResponse")

	// Send the JSON to the client
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(searchResponse)

	log.WithFields(log.Fields{
		"duration": time.Since(startTime),
	}).Info("Search completed")

}
