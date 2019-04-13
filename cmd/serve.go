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
	"bufio"
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/charles-d-burton/hansel/datums"
	"github.com/charles-d-burton/hansel/keys"
	"github.com/spf13/cobra"
	ssh "golang.org/x/crypto/ssh"
	yaml "gopkg.in/yaml.v2"
)

const (
	authorizedFile = "/etc/hansel/authorized_users"
	pendingFile    = "/etc/hansel/pending_users"
	configDir      = "/var/lib/hansel/"
)

var (
	Port     string
	CFLocker *ConfigFileLocker
	maxFile  = (1024 * 1024)
	clients  = make([]*Client, 100)
)

//RemoteHost represents a Host Object with send and receive channels
type Client struct {
	sync.RWMutex
	Name     string
	IP       net.Addr
	KeySha   string
	Channel  ssh.Channel
	Controls struct {
		Timer int
	}
	Stop chan bool
	Send chan datums.ServerMessage
}

type ConfigFileLocker struct {
	AuthorizedUsers struct {
		sync.RWMutex
		ConfigFile string
	}
	PendingUsers struct {
		sync.RWMutex
		ConfigFile string
	}
}

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the server",
	Long:  `Serve the SSH system to listen for incoming connections`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("serve called")
		cfgFiles, err := setupConfigFiles(authorizedFile, pendingFile)
		if err != nil {
			log.Fatal(err)
		}
		if cfgFiles == nil {
			log.Fatal(errors.New("Unable to initialize config files"))
		}
		CFLocker = cfgFiles
		listenAndServe(privateKey)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().StringVarP(&Port, "port", "p", "62621", "Set the port to listen for connections")
	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// serveCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// serveCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func listenAndServe(privateKeyFile string) {
	signer, err := keys.PrivateKeySigner(privateKeyFile)
	if err != nil {
		log.Println(err)
	}
	config := &ssh.ServerConfig{
		NoClientAuth:      false,
		PublicKeyCallback: validatePubKey,
	}
	config.AddHostKey(*signer)
	listener, err := net.Listen("tcp", ":"+Port)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Listening on ", Port)
	for {
		tcpConn, err := listener.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		sshConn, chans, reqs, err := ssh.NewServerConn(tcpConn, config)
		if err != nil {
			log.Println("Failed to handshake ", err)
			continue
		}
		log.Printf("New SSH connection from %s (%s)", sshConn.RemoteAddr(), sshConn.ClientVersion())
		client := &Client{
			IP: sshConn.RemoteAddr(),
		}
		log.Println(client)
		go ssh.DiscardRequests(reqs)
		go handleChannels(chans)
	}
}

func handleChannels(chans <-chan ssh.NewChannel) {
	for newChannel := range chans {
		go handleChannel(newChannel)
	}
}

func handleChannel(newChannel ssh.NewChannel) {
	var client Client
	client.Lock()
	defer client.Close()
	defer client.Unlock()

	channel, requests, err := newChannel.Accept()
	if err != nil {
		log.Printf("could not accept channel (%s)", err)
		return
	}
	chanType := newChannel.ChannelType()
	extraData := newChannel.ExtraData()

	log.Printf("open channel [%s] '%s'", chanType, extraData)
	//Setup the client
	client.Channel = channel
	clients = append(clients, &client)
	client.Stop = make(chan bool, 1)
	client.Send = make(chan datums.ServerMessage, 100)
	client.Unlock()

	go readFromRemote(channel)
	//requests must be serviced
	go ssh.DiscardRequests(requests)
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	//watch for messages or a stop
	select {
	case message := <-client.Send:
		log.Println("Got message to publish: ", message)
		err := enc.Encode(&message)
		if err != nil {
			log.Println(err)
		}
		_, err = channel.Write(buf.Bytes())
		if err != nil {
			log.Println(err)
			return
		}
		buf.Reset()
	case <-client.Stop:
		return
	}

	configs, err := marshalConfigs(configDir)
	if err != nil {
		log.Println(err)
		return
	}
	for _, config := range configs {
		log.Println("Got configs to send")
		log.Println(config)
		err := enc.Encode(&config)
		if err != nil {
			log.Println(err)
		}
		channel.Write(buf.Bytes())
		buf.Reset()
	}
}

//Validate that the provided user and key are valid
func validatePubKey(connMeta ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	valid, err := isUserValid(connMeta.User(), ssh.FingerprintSHA256(key))
	if err != nil {
		log.Fatal(err)
	}
	if valid {
		return nil, nil
	}
	err = markUserPending(connMeta.User(), ssh.FingerprintSHA256(key))
	if err != nil {
		log.Fatal(err)
	}
	return nil, errors.New("User is not valid")
}

//Setup the configs
func setupConfigFiles(configs ...string) (*ConfigFileLocker, error) {
	var cfFlocker ConfigFileLocker
	for _, configFile := range configs {
		err := validateConfigFileExists(configFile)
		if err != nil {
			return nil, err
		}
		if configFile == authorizedFile {
			cfFlocker.AuthorizedUsers.ConfigFile = cfgFile
		}
		if configFile == pendingFile {
			cfFlocker.PendingUsers.ConfigFile = cfgFile
		}
	}
	return &cfFlocker, nil
}

//TODO: implement some kind of EOM marker and publish the message to a queue that prints them in sequence with client info
//Read data returned from the client
func readFromRemote(channel ssh.Channel) {
	var clientMessage datums.ClientMessage
	log.Println("Reading channel")
	dec := gob.NewDecoder(channel)
	for {
		err := dec.Decode(&clientMessage)
		if err != nil {
			log.Println("Failed reading from channel", err)
			//return
		}
		log.Println("Received status from: ", clientMessage.GetClientInfo().Name)
		for _, result := range clientMessage.GetResults() {
			log.Println(result)
		}

	}
}

//Check if a user is in the authorized keys file
func isUserValid(user, sha string) (bool, error) {
	CFLocker.AuthorizedUsers.Lock()
	defer CFLocker.AuthorizedUsers.Unlock()
	file, err := os.OpenFile(CFLocker.AuthorizedUsers.ConfigFile, os.O_APPEND|os.O_RDWR, 0600)
	defer file.Close()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Checking for validity: ")
	return lookForUser(file, user, sha)
}

//Create a new user and mark it as pending TODO: move with user management and also increase uniqueness(client side?)
func markUserPending(user, sha string) error {
	CFLocker.PendingUsers.Lock()
	defer CFLocker.PendingUsers.Unlock()
	file, err := os.OpenFile(CFLocker.PendingUsers.ConfigFile, os.O_APPEND|os.O_RDWR, 0600)
	defer file.Close()
	if err != nil {
		log.Fatal(err)
	}
	userPending, err := lookForUser(file, user, sha)
	if err != nil {
		log.Fatal(err)
	}
	if !userPending {
		log.Println("User not found, marking pending")
		if _, err := file.WriteString(user + "=" + sha + "\n"); err != nil {
			log.Fatal(err)
		}
	}
	return nil
}

//Search the provided file for a user TODO: User management should be a separate package
func lookForUser(file *os.File, user, sha string) (bool, error) {
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Println(user)
		userAndKey := strings.Split(line, "=")
		if strings.Compare(strings.TrimSpace(userAndKey[0]), strings.TrimSpace(user)) == 0 {
			log.Println("User matches, checking sha")
			if strings.Compare(strings.TrimSpace(userAndKey[1]), strings.TrimSpace(sha)) == 0 {
				return true, nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	return false, nil
}

//Validate that a config exists in the requested directory
func validateConfigFileExists(filePath string) error {

	if _, err := os.Stat(filePath); err == nil {
		log.Println("Config file exists: ", filePath)
		return nil
	} else if os.IsNotExist(err) {
		log.Println("No config file found, createing: ", filePath)
		emptyFile, err := os.Create(filePath)
		defer emptyFile.Close()
		if err != nil {
			log.Fatal(err)
		}
		return nil
	} else {
		log.Fatal("Something went wrong while creating authorization file")
	}
	return nil
}

//Marshal the config files from the config directory TODO: Move this somewhere
func marshalConfigs(configDir string) ([]*datums.CommandRunner, error) {
	d, err := os.Open(configDir)
	if err != nil {
		return nil, err
	}
	files, err := d.Readdir(-1)
	if err != nil {
		return nil, err
	}
	var messages []*datums.CommandRunner
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if file.Mode().IsRegular() && file.Size() <= int64(maxFile) {

			if filepath.Ext(file.Name()) == ".yml" {
				buffer := make([]byte, file.Size())
				f, err := os.Open(filepath.Join(configDir + file.Name()))
				if err != nil {
					log.Println(err)
					continue
				}
				_, err = f.Read(buffer)
				if err != nil {
					log.Println(err)
					continue
				}
				var message datums.CommandRunner
				err = yaml.Unmarshal(buffer, &message)
				if err != nil {
					log.Println(err)
					continue
				}
				messages = append(messages, &message)
			}
		}
	}
	return messages, nil
}

//Close client connection
func (client *Client) Close() {
	client.Channel.Close()
}
