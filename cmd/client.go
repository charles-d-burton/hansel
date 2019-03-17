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
	"encoding/gob"
	"fmt"
	"hansel/datums"
	"hansel/keys"
	"log"
	"net"
	"os"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/spf13/cobra"
	ssh "golang.org/x/crypto/ssh"
)

var (
	clientHost string
	clientPort string
)

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

	operation := func() error {
		log.Println("Attempting to connect")
		client, err := ssh.Dial("tcp", clientHost+":"+clientPort, sshConfig)
		if err != nil {
			log.Println(err)
			return err
		}
		channel, _, err := client.Conn.OpenChannel("session", make([]byte, 1024))
		if err != nil {
			return err
		}

		log.Println("Reading channel")
		gob.Register(datums.Message{})
		var message datums.Message
		dec := gob.NewDecoder(channel)
		for {
			err := dec.Decode(&message)
			if err != nil {
				log.Println(err)
			}
			log.Println(&message)
		}
	}
	bof := backoff.NewExponentialBackOff()
	err = backoff.Retry(operation, bof)
	return err
}
