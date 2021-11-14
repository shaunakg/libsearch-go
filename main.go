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

	http.HandleFunc("/", search)
	http.ListenAndServe(":8080", nil)
}

// Search function
func search(w http.ResponseWriter, r *http.Request) {

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
			Domains: []string{"lapl", "nypl", "erl", "portphillip", "boroondara", "baysidelibrary"},
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

	// Get the 'search' query parameter
	query := r.URL.Query().Get("query")

	log.WithFields(log.Fields{
		"query": query,
	}).Info("Search query received")

	// Do Overdrive processing
	for _, domain := range services["overdrive"].Domains {

		log.WithFields(log.Fields{
			"domain": domain,
			"query":  query,
		}).Info("Searching Overdrive")

		// Add query to the end of the URL and encode it
		url := fmt.Sprintf(services["overdrive"].Url, domain) + "?query=" + url.QueryEscape(query)

		// Make a GET request to the Overdrive search URL with the query as a search param
		log.WithFields(log.Fields{
			"url": url,
		}).Info("Making GET request")

		resp, err := http.Get(url)
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

		// If there is a match, decode the JSON and return the results
		if len(match) > 0 {

			log.Info("Found Overdrive results")

			var data map[string]interface{}
			json.Unmarshal([]byte(match[1]), &data)

			// Add the results to the searchResponse struct
			searchResponse.Overdrive = append(searchResponse.Overdrive, data)

		}

	}

	log.Info("Returning searchResponse")

	// Send the JSON to the client
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(searchResponse)

}