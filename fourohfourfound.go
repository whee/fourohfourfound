// fourohfourfound is a fallback HTTP server that may redirect requests.
// It is primarily for creating redirections for web servers like nginx
// where you would otherwise have to edit the configuration and restart to
// modify redirections. Eventually, it will provide statistics for tracking
// if you are, for example, placing these redirected urls on physical ads.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
)

// The host to listen on.
var host *string = flag.String("host", "localhost", "listen host")

// The port to listen on.
var port *int = flag.Int("port", 4404, "listen port")

// The location of a JSON configuration file specifying the redirections.
var configFile *string = flag.String("config", "config.json", "configuration file")

// Configuration file format:
//
// 	{
//    "redirections": {
//	    "source":"destination",
//		"another source":"another destination",
//		...
//	  }
//	}

// The redirection code to send to clients.
var redirectionCode *int = flag.Int("code", 302, "redirection code")

// The configuration for the handlers includes the redirection code (e.g., 301) and
// a mapping of /source to /destination redirections.
type Redirector struct {
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

// Load the config file and create a redirections map.
func redirectionsFrom(config string) (redirections map[string]string, err error) {
	bytes, err := ioutil.ReadFile(config)
	if err != nil {
		return
	}
	var data interface{}
	err = json.Unmarshal(bytes, &data)
	if err != nil {
		return
	}

	m := data.(map[string]interface{})

	// See if any redirections are specified in the configuration.
	if m["redirections"] == nil {
		return
	}

	// Populate redirections from the configuration.
	redirections = make(map[string]string)
	for k, v := range m["redirections"].(map[string]interface{}) {
		redirections[k] = v.(string)
	}
	return
}

// Get will redirect the client if the path is found in the redirections map.
// Otherwise, a 404 is returned.
func (redir *Redirector) Get(w http.ResponseWriter, req *http.Request) {
	if destination, ok := redir.redirections[req.URL.Path]; ok {
		log.Println(realAddr(req), "redirected from", req.URL.Path, "to", destination)
		http.Redirect(w, req, destination, redir.code)
	} else {
		log.Println(realAddr(req), "sent 404 for", req.URL.Path)
		http.NotFound(w, req)
	}
}

// Put will add a redirection from the PUT path to the path specified in the
// request's data.
func (redir *Redirector) Put(w http.ResponseWriter, req *http.Request) {
	// TODO: Require authorization to change redirections
	buf := new(bytes.Buffer)
	io.Copy(buf, req.Body)
	destination := buf.String()

	redir.redirections[req.URL.Path] = destination
	log.Println(realAddr(req), "adding redirection from", req.URL.Path, "to", destination)
}

// Delete removes the redirection at the specified path.
func (redir *Redirector) Delete(w http.ResponseWriter, req *http.Request) {
	// TODO: Require authorization to delete redirections
	delete(redir.redirections, req.URL.Path)
	log.Println(realAddr(req), "removed redirection for", req.URL.Path)
}

func (redir *Redirector) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET":
		redir.Get(w, req)
	case "PUT":
		redir.Put(w, req)
	case "DELETE":
		redir.Delete(w, req)
	}
}

func main() {
	flag.Parse()
	redirections, err := redirectionsFrom(*configFile)
	if err != nil {
		log.Fatal("redirectionsFrom: ", err)
	}
	log.Printf("%s: %d redirections loaded\n", *configFile, len(redirections))

	addr := *host + ":" + strconv.Itoa(*port)
	redirector := &Redirector{code: *redirectionCode, redirections: redirections}

	http.Handle("/", redirector)
	err = http.ListenAndServe(addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
