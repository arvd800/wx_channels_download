package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/lcandy2/wx_channels_download/internal/downloader"
	"github.com/lcandy2/wx_channels_download/internal/proxy"
)

const (
	version     = "1.0.0"
	defaultPort = 8080
)

func main() {
	var (
		port    int
		verbose bool
		outDir  string
		showVer bool
	)

	flag.IntVar(&port, "port", defaultPort, "proxy server listening port")
	flag.BoolVar(&verbose, "verbose", false, "enable verbose logging")
	// Changed default output dir to ~/Downloads/wx_channels for better organization
	flag.StringVar(&outDir, "out", "./wx_channels", "output directory for downloaded videos")
	flag.BoolVar(&showVer, "version", false, "show version information")
	flag.Parse()

	if showVer {
		fmt.Printf("wx_channels_download v%s\n", version)
		os.Exit(0)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outDir, 0755); err != nil {
		log.Fatalf("failed to create output directory: %v", err)
	}

	if verbose {
		log.Printf("wx_channels_download v%s starting...\n", version)
		log.Printf("Proxy port: %d", port)
		log.Printf("Output directory: %s", outDir)
	}

	// Initialize the downloader
	dl := downloader.New(outDir, verbose)

	// Initialize and start the proxy server
	p := proxy.New(port, dl, verbose)

	fmt.Printf("wx_channels_download v%s\n", version)
	fmt.Printf("Proxy server listening on port %d\n", port)
	fmt.Println("Configure your browser/WeChat to use this proxy to intercept video URLs.")
	fmt.Println("Press Ctrl+C to stop.")

	if err := p.Start(); err != nil {
		log.Fatalf("proxy server error: %v", err)
	}
}
