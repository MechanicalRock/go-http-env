package main

import (
	"context"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"
)

func main() {
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server, err := setup()
	if err != nil {
		panic(err)
	}

	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			panic(err)
		}
	}()

	<-sigc

	timeout, timeoutCancel := context.WithTimeout(ctx, 10*time.Second)
	defer timeoutCancel()

	if err := server.Shutdown(timeout); err != nil {
		panic(err)
	}
}

func setup() (*http.Server, error) {
	addr := ":" + os.Getenv("APP_PORT")
	if addr == ":" {
		addr += "8080"
	}

	mux := http.NewServeMux()

	mux.Handle("/env", &envHandler{})
	mux.Handle("/secrets", &secretHandler{})
	mux.Handle("/", &defaultHandler{})

	server := &http.Server{Addr: addr, Handler: mux}
	return server, nil
}

type server struct{}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {

}

type defaultHandler struct{}

func (dh *defaultHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(os.Stdout, "Received request for %s\n", r.URL)
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}

type envHandler struct{}

type entry struct {
	Key   string
	Value string
}

type envTable struct {
	List []entry
}

func (eh *envHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(os.Stdout, "Received request for %s\n", r.URL)
	envVars := os.Environ()
	list := make([]entry, 0, len(envVars))
	for _, envVar := range envVars {
		pair := strings.SplitN(envVar, "=", 2)
		list = append(list, entry{pair[0], pair[1]})
	}

	table := envTable{list}

	t := template.Must(template.ParseFiles("templates/env.tmpl"))
	err := t.Execute(w, table)
	if err != nil {
		panic(err)
	}
}

type secretHandler struct{}

type secretTable struct {
	Title string
	List  []secret
}

type secret struct {
	Key   string
	Value string
}

func (sh *secretHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(os.Stdout, "Received request for %s\n", r.URL)
	dir := "/etc/secrets"
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		w.Write([]byte(err.Error()))
		return
	}

	list := make([]secret, 0, len(files))
	for _, file := range files {
		if !file.IsDir() {
			continue
		}
		filepath := path.Join(dir, file.Name())
		contents, err := ioutil.ReadFile(filepath)
		if err != nil {
			continue
		}
		list = append(list, secret{file.Name(), string(contents)})

		table := secretTable{"Secrets", list}
		t := template.Must(template.ParseFiles("templates/secrets.tmpl"))
		err = t.Execute(w, table)
		if err != nil {
			panic(err)
		}
	}
}
