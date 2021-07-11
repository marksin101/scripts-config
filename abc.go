package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v2"
)

type result struct {
	data [][]string
	m    sync.Mutex
}

type Input struct {
	Version     string `yaml:"version"`
	Credentials struct {
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	}
	Commands []struct {
		Command string `yaml:"command"`
	}
	Hosts []struct {
		Host string `yaml:"host"`
	}
}

func main() {
	var wg sync.WaitGroup
	var r result

	i := parseInput()
	for _, target := range i.Hosts {
		wg.Add(1)
		go worker(i, &r, target.Host, &wg)
	}
	wg.Wait()
	csvFile, err := os.Create("output.csv")
	if err != nil {
		log.Fatal(err.Error())
	}
	csvwriter := csv.NewWriter(csvFile)
	csvwriter.Write([]string{"Hostname", "Model Number", "Software Version"})
	for _, empRow := range r.data {
		if err := csvwriter.Write(empRow); err != nil {
			log.Fatal(err.Error())
		}
	}
	csvwriter.Flush()
}

func authenticate(host, username, password string) (ssh.Client, error) {

	// Create client config
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Second * 5,
	}

	// Connect to the remote server and perform the SSH handshake.
	client, err := ssh.Dial("tcp", host+":22", config)

	return *client, err
}

func extractItem(output []string, pattern *regexp.Regexp) string {
	template := []byte("$name")
	result := []byte{}
	var target string
	for _, v := range output {
		content := []byte(v)
		for _, submatches := range pattern.FindAllSubmatchIndex(content, -1) {
			result = pattern.Expand(result, template, content, submatches)
		}

	}
	target = string(result)
	target = strings.Replace(target, "\n", "", -1)
	return target
}

func parseInput() Input {
	var i Input
	if len(os.Args) < 2 {
		log.Fatal("input.yml is not specified. Quitting....")

	}
	inputFile := os.Args[1]
	b, err := ioutil.ReadFile(inputFile)
	if err != nil {
		log.Fatal("Unable to read file %s. Quitting..." + os.Args[1])
	}
	if err = yaml.Unmarshal(b, &i); err != nil {
		log.Fatal("Unable to parse file" + os.Args[1])
	}
	return i
}

func worker(input Input, r *result, target string, wg *sync.WaitGroup) {
	client, err := authenticate(target, input.Credentials.Username, input.Credentials.Password)
	if err != nil {
		log.Fatalf("unable to connect: %v", err)
	}
	defer client.Close()
	// Create a session
	session, err := client.NewSession()
	if err != nil {
		log.Fatal("Failed to create session: ", err)
	}
	defer session.Close()
	stdin, err := session.StdinPipe()
	if err != nil {
		fmt.Println(err.Error())
	}

	session.Stderr = os.Stderr
	reader, err := session.StdoutPipe()
	if err != nil {
		fmt.Println(err.Error())
	}
	b := bufio.NewReader(reader)
	go func() {
		output := []string{}
		for {
			text, _, err := b.ReadLine()
			if err == io.EOF {
				break
			}
			output = append(output, string(text))
		}
		hostnameRegExp := regexp.MustCompile(`hostname\s+(?P<name>[a-zA-Z0-9._-]+)`)
		hostname := extractItem(output, hostnameRegExp)
		modelNumberRegExp := regexp.MustCompile(`Model\snumber\s+:\s(?P<name>[a-zA-Z0-9._-]+)`)
		modelNumber := extractItem(output, modelNumberRegExp)
		//Get the software version of the master switch only
		softwareVersionRegExp := regexp.MustCompile(`\*\s+\d\s\d+\s+[a-zA-Z0-9._-]+\s+(?P<name>[^\s]+)\s+[^\s]+`)
		softwareVersion := extractItem(output, softwareVersionRegExp)
		r.m.Lock()
		r.data = append(r.data, []string{hostname, modelNumber, softwareVersion})
		r.m.Unlock()
		wg.Done()
	}()
	if err := session.Shell(); err != nil {
		log.Fatal(err)
	}
	stdin.Write([]byte("terminal length 0\n"))
	for _, c := range input.Commands {
		stdin.Write([]byte(c.Command + "\n"))
	}
	stdin.Write([]byte("exit\n"))
	session.Wait()
}

// //input.yml
// version: 1
// credentials:
//   username: 
//   password: 
// commands:
//   - command: 
//   - command: 
// hosts:
//   - host: 
//   - host: 
