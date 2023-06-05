package main

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

func ExecuteCommand(cmd string) string {
	args := strings.Split(cmd, " ")
	out, err := exec.Command(args[0], args[1:]...).Output()

	if err != nil {
		log.Error(err)
		return ""
	}

	return string(out)
}

func randomIndex(length int) int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(length)
}

// baga6ea4seaqbl2h2mamvynevzq2ohvvjjwg4hhtjaxp6w7mcfv5u2uc77etycfq.car
func GenerateCarFileName(carDestination string, pieceCid string) string {
	return carDestination + pieceCid + ".car"
}

// Resolves an issue in Go where redirected http requests will not have headers re-applied to them
func PersistentHeaderHttpClient(originalRequest *http.Request) *http.Client {
	client := http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			for key, values := range originalRequest.Header {
				for _, value := range values {
					req.Header.Add(key, value)
				}
			}
			return nil
		}}
	return &client
}

func FileExists(filename string) bool {
	if _, err := os.Stat(filename); err == nil {
		return true
	} else {
		return false
	}
}

//   GoLang: os.Rename() give error "invalid cross-device link" for Docker container with Volumes.
//   MoveFile(source, destination) will work moving file between folders
//	 https://gist.github.com/bigHave/fe2375114a565bcac277c0e7d8eb4ab1
func MoveFile(sourcePath, destPath string) error {
	log.Debugf("moving file to longterm storage: %s -> %s", sourcePath, destPath)
	inputFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("couldn't open source file: %s", err)
	}
	outputFile, err := os.Create(destPath)
	if err != nil {
		inputFile.Close()
		return fmt.Errorf("couldn't open dest file: %s", err)
	}
	defer outputFile.Close()
	_, err = io.Copy(outputFile, inputFile)
	inputFile.Close()
	if err != nil {
		return fmt.Errorf("writing to output file failed: %s", err)
	}
	// The copy was successful, so now delete the original file
	err = os.Remove(sourcePath)
	if err != nil {
		return fmt.Errorf("failed removing original file: %s", err)
	}
	return nil
}
