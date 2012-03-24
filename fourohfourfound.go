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
var configFile *string = flag.String("config", "config.json", "configuration file")

// The redirection code to send to clients.
var redirectionCode *int = flag.Int("code", 302, "redirection code")

// The configuration for the handlers includes the redirection code (e.g., 301) and
// a mapping of /source to /destination redirections.
type handlerConfig struct {
	code         int
	redirections map[string]string
}

// The remote address is either the client's address or X-Real-Ip, if set.
// X-Real-Ip must be sent by the forwarding server to us.
func realAddr(req *http.Request) (addr string) {
	if headerAddr := req.Header["X-Real-Ip"]; len(headerAddr) > 0 {
		addr = headerAddr[0]
	} else {
		addr = req.RemoteAddr
	}
	return
}

// redirectHandler will redirect the client if the path is found in the
// redirections map. Otherwise, a 404 is returned.
func redirectHandler(w http.ResponseWriter, req *http.Request, cfg handlerConfig) {
	if destination, ok := cfg.redirections[req.URL.Path]; ok {
		log.Println(realAddr(req), "redirected from", req.URL.Path, "to", destination)
		http.Redirect(w, req, destination, cfg.code)
	} else {
		log.Println(realAddr(req), "sent 404 for", req.URL.Path)
		http.NotFound(w, req)
	}
}

func makeHandler(fn func(http.ResponseWriter, *http.Request, handlerConfig),
	cfg handlerConfig) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		fn(w, r, cfg)
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
	redirections, err := redirectionsFrom(*configFile)
	if err != nil {
		log.Fatal("redirectionsFrom: ", err)
	}
	log.Printf("%s: %d redirections loaded\n", *configFile, len(redirections))

	addr := ":" + strconv.Itoa(*port)
	handlerConfig := handlerConfig{code: *redirectionCode, redirections: redirections}

	http.HandleFunc("/", makeHandler(redirectHandler, handlerConfig))
	err = http.ListenAndServe(addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
