// Purpose of this script is to synchronize systemd service between 2 linux instances so that if the target systemd service fail on the master linux instance, the slave would take over just like in other HA protocol like vrrp but this is for systemd service only.

// In order to acheive this purpose, you need to create a seperate systemd service using the binary compiled from this script to work.

// Sample Systemd service

// [Unit]
// Description=systemd service Availability checking tool.

// [Service]
// Type=simple
// Restart=on-failure
// RestartSec=3
// ExecStart=/etc/scripts/systemd-services-HA -D 10.77.0.2:9000 -L 0.0.0.0:8000 -P 100 -SERVICE [Whatever systemd service to target for] -I eth0
// #ExecStop=pkill -f systemd-services-HA
// [Install]
// WantedBy=multi-user.target

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"time"
)

type sendMessage struct {
	Priority int      `json:"priority"`
	Instance int      `json:"instance"`
	Services []string `json:"services"`
}

type recieveMessage struct {
	ipAddr *net.UDPAddr
	Body   sendMessage
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

	var services serviceArray
	sendIPAddr := flag.String("D", "", "Destination Ip addrss and port number")
	listenIPAddr := flag.String("L", "", "Listen Ip address and port numeber")
	priority := flag.Int("P", 100, "Priority of this server")
	instance := flag.Int("ID", 10, "Instance ID of this connection")
	netInterface := flag.String("I", "", "Network Interface to listen for udp traffic")
	flag.Var(&services, "SERVICE", "Systemctl service to toggle")
	flag.Parse()
	s := make(chan recieveMessage, 1)
	if len(services) == 0 || *sendIPAddr == "" || *listenIPAddr == "" {
		log.Println("services,listening address and destination address must not be empty")
		os.Exit(1)
	}
	previousStatus := false
	messageToSend := sendMessage{Priority: *priority, Instance: *instance, Services: services}
	timeOutCounter := 0
	//    stausCounter:=0
	resolvedSendAddr, err := net.ResolveUDPAddr("udp", *sendIPAddr)
	resolvedListenAddr, err := net.ResolveUDPAddr("udp", *listenIPAddr)
	ief, err := net.InterfaceByName(*netInterface)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	log.Println("sending to", resolvedSendAddr)
	go receiveMsg(s, resolvedListenAddr, ief)
	logAlready := false
	firstRun := true
	for {
		sendMsg(resolvedSendAddr, messageToSend)

		select {
		case data := <-s: // msg recieved

			status, shutdown := checkStatus(messageToSend, data)
			if !logAlready {
				logAlready = true
				if timeOutCounter%5 == 0 {
					logAlready = false
				}
			}
			if shutdown == true {
				toggleService(messageToSend, false)
			} else {
				if status != previousStatus && status == true { //switch from inactive to active
					log.Println("switch from inactive to active")
					toggleService(messageToSend, true)

					previousStatus = status
				} else if status != previousStatus && status == false { //switch from active to inactive
					//stop service
					fmt.Println("switch from active to inactive")
					toggleService(messageToSend, false)

					previousStatus = status
				}
			}
			if firstRun && !status {
				toggleService(messageToSend, false)
				firstRun = false
			}

			timeOutCounter = 0

		case <-time.After(time.Millisecond * 500): // wait for peer timeout
			status := true
			timeOutCounter++
			if status != previousStatus && timeOutCounter >= 5 { //Timeout. switch from inactive to active
				//start service
				fmt.Println("Wait for peer Timeout. Switch from inactive to active")
				toggleService(messageToSend, true)
				previousStatus = status
			}
			if timeOutCounter >= 1000000 {
				timeOutCounter = 6
			}
		}
		time.Sleep(time.Second * 2)
	}

}

func sendMsg(sendAddr *net.UDPAddr, msg sendMessage) {

	l, err := net.DialUDP("udp", nil, sendAddr)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	var jsonData []byte
	jsonData, err = json.Marshal(msg)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	l.Write(jsonData)

}

func receiveMsg(c1 chan recieveMessage, addr *net.UDPAddr, ief *net.Interface) {
	l, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	} else {
		log.Println("listening at", addr)
	}
	l.SetReadBuffer(1500)
	b := make([]byte, 1500)
	for {
		n, src, err := l.ReadFromUDP(b)
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}
		jsonMessage := &recieveMessage{ipAddr: src}
		json.Unmarshal(b[:n], &jsonMessage.Body)
		c1 <- *jsonMessage
		time.Sleep(time.Millisecond * 500)
	}

}

func checkStatus(self sendMessage, peer recieveMessage) (bool, bool) {
	//check whether received message is valid
	if peer.Body.Instance == 0 || peer.Body.Priority == 0 || peer.Body.Services == nil {
		log.Println("message recieved from peer addr", *peer.ipAddr, "but neccesary parameters are missing")
		return false, true
	}
	if peer.Body.Instance == self.Instance {
		if peer.Body.Priority < self.Priority {
			if len(peer.Body.Services) == len(self.Services) {
				return true, false
			}
		} else if peer.Body.Priority == self.Priority {
			return false, true
		}
	}
	return false, false
}

func toggleService(self sendMessage, toggle bool) {
	if toggle == false { //stop services
		for i := 0; i < len(self.Services); i++ {
			cmd := exec.Command("systemctl", "stop", self.Services[i])
			_, err := cmd.CombinedOutput()
			if err != nil {
				log.Println(err)
			}
		}
	} else {
		for i := 0; i < len(self.Services); i++ {
			cmd := exec.Command("systemctl", "start", self.Services[i])
			out, err := cmd.CombinedOutput()
			if err != nil {
				log.Fatal(err)
			}
			log.Println(out)
		}
	}

}
