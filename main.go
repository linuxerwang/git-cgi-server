package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	ApplicationName = "git-cgi-server"

	defaultAddr     = ":8080"
	shutdownTimeout = time.Second * 5
)

type Args struct {
	ProjectRoot    string
	ExportAll      bool
	BackendCGI     string
	BasicAuthFile  string
	DigestAuthFile string
	AuthRealm      string
	URIPrefix      string
	Addr           string
	CertFile       string
	KeyFile        string
	PID            string
}

func parseArgs() (*Args, error) {
	var args Args

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(),
			"%s v%s\n\nUsage: %s [REPOS_DIR]\n", ApplicationName, Version, os.Args[0])
		flag.PrintDefaults()
	}

	flag.BoolVar(&args.ExportAll, "export-all", false, "export all repositories")
	flag.StringVar(&args.BackendCGI, "backend-cgi", "", "path to the CGI (git-http-backend)")
	flag.StringVar(&args.BasicAuthFile, "basic-auth-file", "", "path to the basic auth file (htpasswd)")
	flag.StringVar(&args.DigestAuthFile, "digest-auth-file", "", "path to the digest auth file (htdigest)")
	flag.StringVar(&args.AuthRealm, "auth-realm", "Git", "realm name for the auth")
	flag.StringVar(&args.URIPrefix, "uri-prefix", "/", "URI prefix")
	flag.StringVar(&args.Addr, "addr", defaultAddr, "server address")
	flag.StringVar(&args.CertFile, "cert-file", "", "TLS Certificate")
	flag.StringVar(&args.KeyFile, "key-file", "", "TLS Certificate Key")
	flag.StringVar(&args.PID, "pid", "", "PID file")
	flag.Parse()

	if args.CertFile != "" && args.KeyFile == "" {
		fmt.Println("-key-file is required when -cert-file specified")
		os.Exit(1)
	}

	projectRoot, err := getProjectRoot(flag.Args())
	if err != nil {
		return nil, err
	}
	args.ProjectRoot = projectRoot

	return &args, nil
}

func main() {
	args, err := parseArgs()
	if err != nil {
		log.Fatalln(err)
	}

	server := &GitCGIServer{
		ProjectRoot:     args.ProjectRoot,
		ExportAll:       args.ExportAll,
		BackendCGI:      args.BackendCGI,
		BasicAuthFile:   args.BasicAuthFile,
		DigestAuthFile:  args.DigestAuthFile,
		AuthRealm:       args.AuthRealm,
		URIPrefix:       args.URIPrefix,
		Addr:            args.Addr,
		CertFile:        args.CertFile,
		KeyFile:         args.KeyFile,
		ShutdownTimeout: shutdownTimeout,
	}

	errCh := make(chan error)
	go func() {
		if err := server.Serve(); err != nil {
			errCh <- err
		}
		close(errCh)
	}()
	log.Printf("Starting HTTP server on %s (PID=%d)\n", args.Addr, os.Getpid())
	if args.PID != "" {
		if err := writePIDFile(args.PID); err != nil {
			log.Fatalln(err)
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	select {
	case err, ok := <-errCh:
		if ok {
			log.Println("HTTP server error:", err)
		}

	case sig := <-sigCh:
		log.Printf("Signal %s received\n", sig)
		if err := server.Shutdown(); err != nil {
			log.Println("Failed to shutdown HTTP server:", err)
		}
		log.Println("HTTP server shutdown")
	}

	if args.PID != "" {
		removePIDFile(args.PID)
	}
}

func getProjectRoot(args []string) (string, error) {
	if len(args) > 0 && args[0] != "" {
		return args[0], nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", nil
	}
	return cwd, nil
}
