//domain_extractor.go
// Simple domain extractor in Go.
// Features:
//  - Reads from files (supports directories with -r) or stdin
//  - Extracts only domains (scheme + host) from URLs, ignoring paths
//  - Removes duplicates and sorts output
//  - Automatically writes results to domains.txt
//
// Usage examples:
//  go build -o domain_extractor domain_extractor.go
//  ./domain_extractor file1.txt file2.txt
//  ./domain_extractor -r ./input_folder
//  cat input.txt | ./domain_extractor -   # read stdin (use - as filename)

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

var (
	reURL = regexp.MustCompile(`https?://[^\s"'<>]+`)
)

func main() {
	recursive := flag.Bool("r", false, "recurse into directories")
	workers := flag.Int("t", 8, "number of worker goroutines")
	flag.Parse()

	paths := flag.Args()
	if len(paths) == 0 {
		// read stdin
		extractFromReader(os.Stdin, "domains.txt")
		return
	}

	files := make([]string, 0, len(paths))
	for _, p := range paths {
		if p == "-" {
			files = append(files, "-")
			continue
		}
		info, err := os.Stat(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: can't access %s: %v\n", p, err)
			continue
		}
		if info.IsDir() {
			if *recursive {
				filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
					if err != nil {
						fmt.Fprintf(os.Stderr, "walk error: %v\n", err)
						return nil
					}
					if d.IsDir() {
						return nil
					}
					files = append(files, path)
					return nil
				})
			} else {
				fmt.Fprintf(os.Stderr, "warning: %s is a directory (use -r to recurse)\n", p)
			}
		} else {
			files = append(files, p)
		}
	}

	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "no input files found")
		return
	}

	taskCh := make(chan io.ReadCloser)
	resCh := make(chan string)
	wg := &sync.WaitGroup{}

	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for rc := range taskCh {
				if rc == nil {
					continue
				}
				extractFromStream(rc, resCh)
				rc.Close()
			}
		}()
	}

	go func() {
		for _, f := range files {
			if f == "-" {
				extractFromStream(os.Stdin, resCh)
				continue
			}
			fh, err := os.Open(f)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: can't open %s: %v\n", f, err)
				continue
			}
			taskCh <- fh
		}
		close(taskCh)
	}()

	go func() {
		wg.Wait()
		close(resCh)
	}()

	domSet := make(map[string]struct{})
	for d := range resCh {
		domSet[d] = struct{}{}
	}

	domains := make([]string, 0, len(domSet))
	for d := range domSet {
		domains = append(domains, d)
	}
	sort.Strings(domains)

	f, err := os.Create("domains.txt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "can't create output file: %v\n", err)
		return
	}
	defer f.Close()

	bw := bufio.NewWriter(f)
	for _, d := range domains {
		bw.WriteString(d)
		bw.WriteByte('\n')
	}
	bw.Flush()
}

func extractFromReader(r io.Reader, outFile string) {
	resCh := make(chan string)
	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		extractFromStream(io.NopCloser(r), resCh)
		close(resCh)
	}()

	domSet := make(map[string]struct{})
	for d := range resCh {
		domSet[d] = struct{}{}
	}
	domains := make([]string, 0, len(domSet))
	for d := range domSet {
		domains = append(domains, d)
	}
	sort.Strings(domains)

	f, err := os.Create(outFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "can't create output file: %v\n", err)
		return
	}
	defer f.Close()

	bw := bufio.NewWriter(f)
	for _, d := range domains {
		bw.WriteString(d)
		bw.WriteByte('\n')
	}
	bw.Flush()
	wg.Wait()
}

func extractFromStream(rc io.ReadCloser, out chan<- string) {
	scanner := bufio.NewScanner(rc)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		for _, m := range reURL.FindAllString(line, -1) {
			if d := baseDomain(m); d != "" {
				out <- d
			}
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "scan error: %v\n", err)
	}
}

func baseDomain(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	h := u.Host
	if i := strings.IndexByte(h, ':'); i >= 0 {
		h = h[:i]
	}
	if h == "" {
		return ""
	}
	return u.Scheme + "://" + strings.ToLower(h)
}