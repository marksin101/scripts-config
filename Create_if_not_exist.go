//Create if Not Exist (cine)
// This is a simple program that would create directory recursively if it doesn't exist when you create a file
// Usage: compile it and put it into /usr/bin
// run the program before you use the text editor
// e.g. cine nano /tmp/132/abc.txt  <-- This will create /tmp/132 directory if it doesn't exist
// e.g. cine code /tmp/jjj <- create /tmp/jjj and open code at that directory
// e.g. cine /tmp/lll <- if no text editor is present, a prompt would appear where you can choose one of the three text editors (vscode, nano and vim) 
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {

	path, err := checkinput()
	if err != nil {
		log.Fatal(err)
	}
	mkdir(path)
	if len(os.Args) == 2 {
		var i int
	loop:
		for {
			fmt.Print("\033[H\033[2J")
			fmt.Printf("Select text editor\n [0] vscode\n [1] nano\n [2] vim\n")
			fmt.Scanln(&i)
			if i == 0 || i == 1 || i == 2 {
				break loop
			}
		}
		switch i {
		case 0:
			os.Args = append([]string{"code"}, os.Args[1:]...)
		case 1:
			os.Args = append([]string{"nano"}, os.Args[1:]...)
		case 2:
			os.Args = append([]string{"vim"}, os.Args[1:]...)

		}
		os.Args = append([]string{"dummy"}, os.Args...)
		if i == 1 || i == 2 {
			if !strings.Contains(os.Args[len(os.Args)-1], ".") {
				var s string
				fmt.Printf("File to be created:\n")
				fmt.Scanln(&s)
				os.Args[len(os.Args)-1] += "/" + s
			}
		}
	}
	cmd := exec.Command(os.Args[1], os.Args[2:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}

}

func checkinput() (string, error) {
	var path string
	var e error = nil
	regFilePath := "^(/[^/ ]*)+/?$"
	reg, err := regexp.Compile(regFilePath)
	if err != nil {
		log.Fatal(err)
	}
	if len(os.Args) == 1 {
		e = fmt.Errorf("Missing argument")
		return path, e
	}
	for i := len(os.Args) - 1; 0 < i; i-- {
		if reg.MatchString(os.Args[i]) {
			if strings.Contains(os.Args[i], ".") {
				path = filepath.Dir(os.Args[i])
			} else {
				path = os.Args[i]
			}
			break
		}
	}
	return path, e
}

func mkdir(path string) {
	_, err := os.Stat(path)
	if err == nil {
		fmt.Printf("path %s already exist", path)
	} else {
		err = os.MkdirAll(path, 0755)
		if err != nil {
			log.Fatal(err)
		}
	}
}
