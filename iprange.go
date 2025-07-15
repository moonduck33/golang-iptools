package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

func main() {
	fmt.Print("[+] Your IP List : ")
	var fileName string
	fmt.Scanln(&fileName)

	inputFile, err := os.Open(fileName)
	if err != nil {
		fmt.Println("Error opening input file:", err)
		return
	}
	defer inputFile.Close()

	outputFile, err := os.OpenFile("IP Range.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error creating output file:", err)
		return
	}
	defer outputFile.Close()

	seen := make(map[string]struct{})
	scanner := bufio.NewScanner(inputFile)
	writer := bufio.NewWriter(outputFile)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// De-duplicate
		if _, exists := seen[line]; exists || line == "" {
			continue
		}
		seen[line] = struct{}{}

		if !isValidIP(line) {
			fmt.Printf("Invalid IP: %s\n", line)
			continue
		}

		prefix := getPrefix(line)
		if prefix == "" {
			fmt.Printf("Skipped: %s (prefix error)\n", line)
			continue
		}

		for i := 0; i <= 255; i++ {
			writer.WriteString(fmt.Sprintf("%s.%d\n", prefix, i))
		}

		fmt.Printf("Completed: %s\n", line)
	}

	writer.Flush()
	fmt.Println("✅ All IPs ranged successfully.")
}

func getPrefix(ip string) string {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return ""
	}
	return strings.Join(parts[:3], ".")
}

func isValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}