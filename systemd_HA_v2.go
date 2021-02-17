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
	"os/exec"
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
	ch := make(chan bool, 2)
	go func() {
		for {
			n, src, err := l.ReadFromIP(buffer)
			if err != nil {
				log.Fatal(err)
			}
			ch <- true
			m := &finalMessage{}
			json.Unmarshal(buffer[:n], &m)
			if receivedMessage, ok := integrityCheck(m, i, src); ok {
				preToggleServicesCheck(i.message, receivedMessage, false)
			}
			time.Sleep(time.Second * 5)
		}
	}()
	for {
		select {
		case <-ch:
			//clear channel
			for len(ch) > 0 {
				<-ch
			}

		case <-time.After(time.Second * 10):
			var dummyMessage message
			preToggleServicesCheck(i.message, dummyMessage, true)

		}
		time.Sleep(time.Second * 5)
	}
}

func preToggleServicesCheck(self message, neighbor message, force bool) {
	if force {
		go toggleServices(self.services, true)
		return
	}
	if self.instance != neighbor.instance {
		log.Println("Two servers have the different instance id. Stopping all services to prevent damages.")
		go toggleServices(self.services, false)

		return
	}
	if !sameStringSlice(self.services, neighbor.services) {
		log.Println("Two servers have different services to monitor for. Stopping all services to prevent damages")
		go toggleServices(self.services, false)

		return
	}
	if self.priority < neighbor.priority {
		go toggleServices(self.services, false)

		return
	}
	go toggleServices(self.services, true)
}

func toggleServices(s []string, on bool) {
	var action string
	if on {
		action = "start"
	} else {
		action = "stop"
	}
	for i := 0; i < len(s); i++ {
		cmd := exec.Command("systemctl", action, s[i])
		if err := cmd.Run(); err != nil {
			log.Println(err)
		}
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
		fmt.Printf("The purpose of the program is to provide high availability between systemd services on 2 linux servers\n\n -n, ip address and port of the neigbor e.g. 192.168.10.2:9000\n\n -l, ip address and port to listen on\n\n -p, priority of this machine\n\n -i, instance id. Note that the instance id must be the same between 2 servers\n\n -pass, password for encryption and authenication. Note that if the password is empty, no encryption would be done!\n\n -s, systemd services to toggle (could be multiple)\n")
		os.Exit(0)
	}
	neighborIP := flag.String("n", "", "")
	listenIP := flag.String("l", "", "")
	priority := flag.Int("p", -1, "")
	instanceID := flag.Int("i", -1, "")
	password := flag.String("pass", "", "")
	flag.Var(&services, "s", "")
	flag.Parse()
	if len(*neighborIP) == 0 || len(*listenIP) == 0 || *priority == -1 || *instanceID == -1 || len(services) == 0 {
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
	fmt.Printf("Try --help for more information\n")
}
