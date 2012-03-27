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
	"strings"
	"sync"
)

// The host to listen on.
var host *string = flag.String("host", "localhost", "listen host")

// The port to listen on.
var port *int = flag.Int("port", 4404, "listen port")

// The location of a JSON configuration file specifying the redirections.
var configFile *string = flag.String("config", "config.json", "configuration file")

// Configuration file format:
//
// {
//   "redirections": {
//     "source":"destination",
//      "another source":"another destination",
//      ...
//   }
// }

// The redirection code to send to clients.
var redirectionCode *int = flag.Int("code", 302, "redirection code")

// The configuration for the handlers includes the redirection code (e.g., 301) and
// a mapping of /source to /destination redirections.
type Redirector struct {
	code         int
	mu           sync.RWMutex
	Redirections map[string]string `json:"redirections"`
}

// Create a new Redirector with a default code of StatusFound (302) and an empty redirections map.
func NewRedirector() *Redirector {
	return &Redirector{code: http.StatusFound, Redirections: make(map[string]string)}
}

// The remote address is either the client's address or X-Real-Ip, if set.
// X-Real-Ip must be sent by the forwarding server to us.
func realAddr(req *http.Request) (addr string) {
	if headerAddr := req.Header.Get("X-Real-Ip"); headerAddr != "" {
		return headerAddr
	}
	return req.RemoteAddr
}

// A handler wrapped with onlyLocal will return http.StatusUnauthorized if the client
// is not localhost. The upstream server must send X-Real-Ip to work properly.
func onlyLocal(w http.ResponseWriter, req *http.Request, fn func()) {
	addr := strings.SplitN(realAddr(req), ":", 2)[0]
	switch addr {
	case "localhost", "127.0.0.1":
		fn()
	default:
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}
}

// Get will redirect the client if the path is found in the redirections map.
// Otherwise, a 404 is returned.
func (redir *Redirector) Get(w http.ResponseWriter, req *http.Request) {
	redir.mu.RLock()
	defer redir.mu.RUnlock()
	if destination, ok := redir.Redirections[req.URL.Path]; ok {
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
	redir.mu.Lock()
	defer redir.mu.Unlock()
	// TODO: Require authorization to change redirections
	buf := new(bytes.Buffer)
	io.Copy(buf, req.Body)
	destination := buf.String()

	redir.Redirections[req.URL.Path] = destination
	log.Println(realAddr(req), "added redirection from", req.URL.Path, "to", destination)
}

// Delete removes the redirection at the specified path.
func (redir *Redirector) Delete(w http.ResponseWriter, req *http.Request) {
	redir.mu.Lock()
	defer redir.mu.Unlock()
	// TODO: Require authorization to delete redirections
	delete(redir.Redirections, req.URL.Path)
	log.Println(realAddr(req), "removed redirection for", req.URL.Path)
}

func (redir *Redirector) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET":
		redir.Get(w, req)
	case "PUT":
		onlyLocal(w, req, func() { redir.Put(w, req) })
	case "DELETE":
		onlyLocal(w, req, func() { redir.Delete(w, req) })
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// Use the specified JSON configuration to configure the Redirector.
func (redir *Redirector) LoadConfig(config []byte) (err error) {
	redir.mu.Lock()
	defer redir.mu.Unlock()
	err = json.Unmarshal(config, redir)
	log.Printf("%d redirections loaded\n", len(redir.Redirections))
	return
}

// Read the JSON configuration from a file to configure the Redirector.
func (redir *Redirector) LoadConfigFile(config string) (err error) {
	bytes, err := ioutil.ReadFile(config)
	if err != nil {
		return
	}
	err = redir.LoadConfig(bytes)
	return
}

// GETting the config supplies the client with a JSON formatted configuration
// suitable for storing as the configuration file.
func (redir *Redirector) GetConfig(w http.ResponseWriter, req *http.Request) {
	redir.mu.RLock()
	defer redir.mu.RUnlock()
	jsonConfig, err := json.MarshalIndent(redir, "", "  ")
	if err != nil {
		http.Error(w, "Error encoding JSON config", http.StatusInternalServerError)
		return
	}
	io.WriteString(w, string(jsonConfig))
}

// Set the Redirector configuration from the JSON supplied in the PUT
// request's data.
func (redir *Redirector) SetConfig(w http.ResponseWriter, req *http.Request) {
	buf := new(bytes.Buffer)
	io.Copy(buf, req.Body)
	err := redir.LoadConfig(buf.Bytes())
	if err != nil {
		http.Error(w, "Error decoding JSON config", http.StatusInternalServerError)
		return
	}
	io.WriteString(w, "Configuration successfully loaded.\n")
}

// When deleted, the Redirector configuration is emptied.
func (redir *Redirector) DeleteConfig(w http.ResponseWriter, req *http.Request) {
	redir.mu.Lock()
	defer redir.mu.Unlock()
	redir.Redirections = make(map[string]string)
}

// The ConfigHandler handles retrieving the Redirector configuration (GET) and
// setting it (PUT) through the configuration path.
func (redir *Redirector) ConfigHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		log.Println(realAddr(req), req.Method, req.URL.Path)
		onlyLocal(w, req,
			func() {
				switch req.Method {
				case "GET":
					redir.GetConfig(w, req)
				case "PUT":
					redir.SetConfig(w, req)
				case "DELETE":
					redir.DeleteConfig(w, req)
				default:
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				}
			})
	}
}

func main() {
	flag.Parse()
	addr := *host + ":" + strconv.Itoa(*port)

	redirector := NewRedirector()
	redirector.code = *redirectionCode

	err := redirector.LoadConfigFile(*configFile)
	if err != nil {
		log.Fatal("LoadConfigFile: ", err)
	}

	http.Handle("/", redirector)
	http.HandleFunc("/_config", redirector.ConfigHandler())
	err = http.ListenAndServe(addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
