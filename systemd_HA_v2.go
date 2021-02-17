package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"
)

type input struct {
	message              message
	neighborIP, listenIP *net.IPAddr
	password             string
}

type message struct {
	priority int          `json:"priority"`
	instance int          `json:"instance"`
	services serviceArray `json:"services"`
}
type finalMessage struct {
	checksum string `json:"checksum"`
	message  []byte `json:"message"`
}

type serviceArray []string

func (i *serviceArray) String() string {
	return "my string representation"
}
func (i *serviceArray) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func main() {
	input, err := parseInput()
	if err != nil {
		errorHandler(err)
	}
	go sendMessage(input)

}

func receiveMessage(i input) {
	l, err := net.ListenIP("nil", i.listenIP)
	if err != nil {
		log.Fatal(err)
	}
	l.SetReadBuffer(1500)
	buffer := make([]byte, 1500)
	for {
		n, src, err := l.ReadFromIP(buffer)
		if err != nil {
			log.Fatal(err)
		}
		m := &finalMessage{}
		json.Unmarshal(buffer[:n], &m)
		if receivedMessage, ok := integrityCheck(m, i, src); ok {
			toggleServices(i.message, receivedMessage)
		}
	}
}

func toggleServices(self message, neighbor message) {
	if self.instance == neighbor.instance {
		log.Println("Two servers have the same instance id. Can't decide what to do.")
		return
	}
	if !sameStringSlice(self.services, neighbor.services) {
		log.Println("Two servers have different services to monitor for. Can't decide what to do")
		return
	}
	if self.priority < neighbor.priority {
		return
	}
}
func sameStringSlice(x, y []string) bool {
	if len(x) != len(y) {
		return false
	}
	diff := make(map[string]int, len(x))
	for _, _x := range x {
		diff[_x]++
	}
	for _, _y := range y {
		if _, ok := diff[_y]; !ok {
			return false
		}
		diff[_y]--
		if diff[_y] == 0 {
			delete(diff, _y)
		}
	}
	if len(diff) == 0 {
		return true
	}
	return false
}

func integrityCheck(m *finalMessage, i input, src *net.IPAddr) (message, bool) {
	var decryptedMessage []byte
	reconstructedMessage := &message{}
	if len(i.password) > 0 {
		decryptedMessage = decryption(i.password, m.message)
	} else {
		decryptedMessage = m.message
	}
	json.Unmarshal(decryptedMessage, &reconstructedMessage)
	h := hash(*reconstructedMessage)
	if h != m.checksum {
		log.Printf("Checksum from %s doesn't match", src.IP)
		return *reconstructedMessage, false
	}
	return *reconstructedMessage, true
}
func decryption(password string, message []byte) []byte {
	c, err := aes.NewCipher([]byte(password))
	if err != nil {
		log.Fatal(err)
	}
	gcm, err := cipher.NewGCM(c)
	if err != nil {
		log.Fatal(err)
	}

	nonceSize := gcm.NonceSize()
	if len(message) < nonceSize {
		log.Fatal(err)
	}
	nonce, ciphertext := message[:nonceSize], message[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		fmt.Println(err)
	}
	return plaintext

}

func sendMessage(i input) {
	var f finalMessage
	l, err := net.DialIP("udp", i.neighborIP, nil)
	if err != nil {
		log.Println(err)
	}
	f.checksum = hash(i.message)
	if len(i.password) > 0 {
		f.message, err = encryption(i.message, i.password)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		f.message = []byte(fmt.Sprintf("%v", i.message))
	}
	for {
		l.Write([]byte(fmt.Sprintf("%v", f)))
		time.Sleep(time.Second * 5)
	}
}

func hash(m message) string {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%v", m)))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func encryption(m message, p string) ([]byte, error) {
	var encryptedData []byte
	jsonData, err := json.Marshal(m)
	if err != nil {
		return encryptedData, err
	}
	c, err := aes.NewCipher([]byte(p))
	if err != nil {
		return encryptedData, err
	}
	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return encryptedData, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return encryptedData, err
	}
	encryptedData = gcm.Seal(nonce, nonce, jsonData, nil)
	return encryptedData, nil
}

func parseInput() (input, error) {
	var i input
	var services serviceArray
	if len(os.Args) < 2 {
		return i, fmt.Errorf("Missing arguments")
	}
	if os.Args[1] == "--help" || os.Args[1] == "help" || os.Args[1] == "-help" {
		fmt.Printf("The purpose of the program is to provide high availability between systemd services on 2 linux servers\n -n, ip address and port of the neigbor e.g. 192.168.10.2:9000\n -l, ip address and port to listen on\n -p, priority of this machine\n, -i, instance id. Note that the instance id must be the same between 2 servers\n, -pass, password for encryption and authenication\n")
		os.Exit(0)
	}
	neighborIP := flag.String("n", "", "")
	listenIP := flag.String("l", "", "")
	priority := flag.Int("p", -1, "")
	instanceID := flag.Int("i", -1, "")
	password := flag.String("pass", "", "")
	flag.Var(&services, "s", "")
	flag.Parse()
	if len(*neighborIP) == 0 || len(*listenIP) == 0 || *priority == -1 || *instanceID == -1 {
		return i, fmt.Errorf("Missing arguments")
	}
	if len(*password) == 0 {
		log.Println("Missing password. The communication would be in plain-text")
	}
	neightbor, err := net.ResolveIPAddr("udp", *neighborIP)
	if err != nil {
		return i, fmt.Errorf("Incorrect neighbor IP address")
	}
	self, err := net.ResolveIPAddr("udp", *listenIP)
	if err != nil {
		return i, fmt.Errorf("Incorrect self IP address")
	}
	m := message{priority: *priority, instance: *instanceID, services: services}
	i.message = m
	i.neighborIP = neightbor
	i.listenIP = self
	i.password = *password
	return i, nil
}

func errorHandler(e error) {
	fmt.Println(e)
	fmt.Printf("Try --help for more information")
}
