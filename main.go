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

package main

import (
	"log"
	"os"
	"os/user"

	"github.com/charles-d-burton/hansel/cmd"
	"github.com/fatih/color"
)

func main() {
	user, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}

	if user.Uid != "0" {
		color.Red("You must run as root.")
		os.Exit(0)
	}
	cmd.Execute()
}
