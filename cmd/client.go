// Copyright © 2019 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"bytes"
	"encoding/gob"
	"fmt"

	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/charles-d-burton/hansel/datums"
	"github.com/charles-d-burton/hansel/keys"
	"github.com/spf13/cobra"
	ssh "golang.org/x/crypto/ssh"
)

var (
	clientHost string
	clientPort string
)

type Server struct {
	sync.RWMutex
	Host      *string
	Port      *string
	Closed    bool
	SSHConfig *ssh.ClientConfig
	Channel   ssh.Channel
}

// clientCmd represents the client command
var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("client called")
		err := connect(privateKey)
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(clientCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// clientCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// clientCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	clientCmd.Flags().StringVarP(&clientHost, "host", "h", "", "The remote host to connect to")
	clientCmd.Flags().StringVarP(&clientPort, "port", "p", "62621", "Port of remote host")

}

func connect(privateKey string) error {
	name, err := os.Hostname()
	if err != nil {
		return err
	}
	pubkey, err := keys.PublicKeyFile(privateKey)
	if err != nil {
		return err
	}
	sshConfig := &ssh.ClientConfig{
		User: name,
		Auth: []ssh.AuthMethod{
			pubkey,
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
		Timeout: time.Second * 30,
	}
	//Setup the Server
	server := &Server{
		Host:      &clientHost,
		Port:      &clientPort,
		SSHConfig: sshConfig,
	}
	server.Connect()
	return err
}

func (server *Server) Connect() {
	operation := func() error {
		log.Println("Attempting to connect")

		client, err := ssh.Dial("tcp", *server.Host+":"+*server.Port, server.SSHConfig)
		if err != nil {
			log.Println(err)
			return err
		}
		channel, _, err := client.Conn.OpenChannel("session", make([]byte, 1024))
		if err != nil {
			return err
		}
		server.Channel = channel
		server.Closed = false
		err = server.ProcessReqs()
		if err != nil {
			return err
		}
		return nil
	}
	bof := backoff.NewExponentialBackOff()
	err := backoff.Retry(operation, bof)
	if err != nil {
		log.Fatal(err)
	}
}

//ProcessReqs TODO: Ensure this works like I think it does.  I believe this should just run forever and attempt reconnect on failures
func (server *Server) ProcessReqs() error {
	go server.sendStatus()
	log.Println("Reading channel")
	var message datums.ServerMessage
	dec := gob.NewDecoder(server.Channel)
	for {
		err := dec.Decode(&message)
		if err != nil {
			server.Closed = true
			server.Channel.Close()
			return err
		}
		log.Println(&message)
		result, err := message.Execute()
		if err != nil {
			server.Closed = true
			server.Channel.Close()
			return err
		}
		server.sendReturn(result)
	}
}

//Send a message back to the server
func (server *Server) sendReturn(message []string) error {
	server.Lock()
	defer server.Unlock()
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	status := datums.ClientResult{Name: clientHost, Results: message}
	err := enc.Encode(&status)
	if err != nil {
		return err
	}
	_, err = server.Channel.Write(buf.Bytes())
	if err != nil {
		return err
	}
	return nil
}

//TODO: Update the ClientStatus with a lot more system info
func (server *Server) sendStatus() {
	defer server.Unlock()
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	ticker := time.NewTicker(2 * time.Second)
	for t := range ticker.C {
		log.Println(t)
		status := datums.ClientStatus{
			Name:    clientHost,
			Message: "keepalive",
		}
		err := enc.Encode(&status)
		if err != nil {
			log.Println(err)
			continue
		}
		if !server.Closed {
			server.Lock()
			_, err = server.Channel.Write(buf.Bytes())
			server.Unlock()
			if err != nil {
				return
			}
			buf.Reset()
			continue
		}
		return
	}
}
