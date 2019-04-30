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

	"github.com/charles-d-burton/hansel/datums"
	"github.com/spf13/cobra"
)

var (
	hostPattern string
)

// controllerCmd represents the controller command
var controlCmd = &cobra.Command{
	Use:   "control",
	Short: "Control machines or the runner",
	Long:  `Used to issue commands against remotes or schedule commands`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("controller called")
		err := doControl()
		if err != nil {
			fmt.Println(err)
		}

	},
}

func init() {
	rootCmd.AddCommand(controlCmd)
	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// controllerCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	controlCmd.Flags().StringVarP(&hostPattern, "hosts", "h", ".*", "PCRE host lookup (required)")
	controlCmd.MarkFlagRequired("hosts")
}

func doControl() error {
	var controller datums.ControllerReq
	if hostPattern != "" {
		controller.Pattern = hostPattern
	}
	c, err := net.Dial("unix", domainSocketAddr)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err = enc.Encode(controller)
	if err != nil {
		log.Println(err)
	}
	_, err = c.Write(buf.Bytes())
	err = listenForResult(c)
	c.Close()
	if err != nil {
		return err
	}
	return nil
}

func listenForResult(c net.Conn) error {
	log.Println("Reading Control Stream")
	var message datums.ClientResult
	for {
		dec := gob.NewDecoder(c)
		err := dec.Decode(&message)
		if err != nil {
			if err.Error() != "EOF" {
				log.Println(err)
				return err
			}
			return nil
		}
		fmt.Printf("%+v\n", message)
	}
}
