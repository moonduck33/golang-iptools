package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	ipMap     = make(map[string]struct{})
	ipMapLock sync.Mutex
	counter   int
)

func main() {
	fmt.Print("Enter domain list file: ")
	var file string
	fmt.Scanln(&file)

	domains, err := readDomains(file)
	if err != nil {
		fmt.Println("Error reading file:", err)
		return
	}

	fmt.Print("Thread count (100–500 recommended): ")
	var threadCount int
	fmt.Scanln(&threadCount)
	if threadCount < 1 {
		threadCount = 100
	}

	ipMap = loadIPs("ips.txt")

	var wg sync.WaitGroup
	domainChan := make(chan string, threadCount*2)

	// Start workers
	for i := 0; i < threadCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for domain := range domainChan {
				ip := resolveWithTimeout(domain, 5*time.Second)
				if ip == "" {
					continue
				}

				// Check + write immediately
				ipMapLock.Lock()
				if _, exists := ipMap[ip]; !exists {
					ipMap[ip] = struct{}{}
					appendIP(ip)
					updateProgress()
				}
				ipMapLock.Unlock()
			}
		}()
	}

	// Feed domains
	for _, d := range domains {
		domainChan <- d
	}
	close(domainChan)

	wg.Wait()
	fmt.Println("✅ Done. All domains processed.")
}

func readDomains(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var list []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		line = strings.TrimPrefix(line, "http://")
		line = strings.TrimPrefix(line, "https://")
		if line != "" {
			list = append(list, line)
		}
	}
	return list, scanner.Err()
}

func loadIPs(path string) map[string]struct{} {
	m := make(map[string]struct{})
	file, err := os.Open(path)
	if err != nil {
		return m
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		ip := strings.TrimSpace(scanner.Text())
		if ip != "" {
			m[ip] = struct{}{}
		}
	}
	return m
}

func appendIP(ip string) {
	f, err := os.OpenFile("ips.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Write error:", err)
		return
	}
	defer f.Close()

	_, _ = f.WriteString(ip + "\n")
}

func updateProgress() {
	counter++
	if counter%500 == 0 {
		fmt.Printf("Resolved %d IPs so far...\n", counter)
	}
}

func resolveWithTimeout(domain string, timeout time.Duration) string {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resolver := &net.Resolver{}
	ips, err := resolver.LookupIP(ctx, "ip4", domain)
	if err != nil || len(ips) == 0 {
		return ""
	}
	return ips[0].String()
}
