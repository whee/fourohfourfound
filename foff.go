// fourohfourfound is a fallback HTTP server that may redirect requests.
// It is primarily for creating redirections for web servers like nginx
// where you would otherwise have to edit the configuration and restart to
// modify redirections. Eventually, it will provide statistics for tracking
// if you are, for example, placing these redirected urls on physical ads.
package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
)

// The port to listen on.
var port *int = flag.Int("port", 4404, "listen port")

// The location of a JSON configuration file specifying the redirections.
// The format:
//
// 	{
//		"source":"destination",
//		"another source":"another destination",
//		...
//	}
var redirectConfig *string = flag.String("config", "config.json", "redirection configuration file")

// The redirection code to send to clients.
var redirectionCode *int = flag.Int("code", 302, "redirection code")

// redirectHandler will redirect the client if the path is found in the
// redirections map. Otherwise, a 404 is returned.
func redirectHandler(w http.ResponseWriter, req *http.Request, redirections map[string]string) {
	remoteAddr := "-"
	if headerAddr := req.Header["X-Real-Ip"]; len(headerAddr) > 0 {
		remoteAddr = headerAddr[0]
	}

	if destination, ok := redirections[req.URL.Path]; ok {
		log.Println(remoteAddr, "redirected from", req.URL.Path, "to", destination)
		http.Redirect(w, req, destination, *redirectionCode)
	} else {
		log.Println(remoteAddr, "sent 404 for", req.URL.Path)
		http.NotFound(w, req)
	}
}

func makeHandler(fn func(http.ResponseWriter, *http.Request, map[string]string),
	redirections map[string]string) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		fn(w, r, redirections)
	}
}

// Load the config file and create a redirections map.
func redirectionsFrom(config string) (redirections map[string]string, err error) {
	b, err := ioutil.ReadFile(config)
	if err != nil {
		return
	}
	err = json.Unmarshal(b, &redirections)
	if err != nil {
		return
	}

	return
}

func main() {
	flag.Parse()
	redirections, err := redirectionsFrom(*redirectConfig)
	if err != nil {
		log.Fatal("redirectionsFrom: ", err)
	}
	log.Printf("%s: %d redirections loaded\n", *redirectConfig, len(redirections))

	addr := ":" + strconv.Itoa(*port)

	http.HandleFunc("/", makeHandler(redirectHandler, redirections))
	err = http.ListenAndServe(addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
