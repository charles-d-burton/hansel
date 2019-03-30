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
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	ssh "golang.org/x/crypto/ssh"
	yaml "gopkg.in/yaml.v2"
)

const (
	authorizedFile = "/.hansel/authorized_users"
	pendingFile    = "/.hansel/pending_users"
	configDir      = "/.hansel/states/"
)

var (
	Port     string
	CFLocker *ConfigFileLocker
	maxFile  = (1024 * 1024)
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
		listenAndServe(privateKey)
	},
}

func init() {
	cfgFiles, err := setupConfigFiles(authorizedFile, pendingFile)
	if err != nil {
		log.Fatal(err)
	}
	if cfgFiles == nil {
		log.Fatal(errors.New("Unable to initialize config files"))
	}
	CFLocker = cfgFiles
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
	channel, requests, err := newChannel.Accept()
	defer channel.Close()
	if err != nil {
		log.Printf("could not accept channel (%s)", err)
		return
	}
	chanType := newChannel.ChannelType()
	extraData := newChannel.ExtraData()

	log.Printf("open channel [%s] '%s'", chanType, extraData)
	go readFromRemote(channel)
	//requests must be serviced
	go ssh.DiscardRequests(requests)
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	home, err := homedir.Dir()
	if err != nil {
		log.Println(err)
		return
	}
	configs, err := marshalConfigs(home + configDir)
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
	select {}
}

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

func setupConfigFiles(configs ...string) (*ConfigFileLocker, error) {
	var cfFlocker ConfigFileLocker
	home, err := homedir.Dir()
	if err != nil {
		log.Println(err)
		return nil, err
	}
	for _, configFile := range configs {
		cfgFile := home + configFile
		err = validateConfigFileExists(cfgFile)
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
