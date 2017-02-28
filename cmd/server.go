// Copyright 2017 NDP Systèmes. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"go/build"
	"os"
	"os/exec"
	"path"
	"text/template"

	"github.com/gin-gonic/gin"
	"github.com/npiganeau/yep/yep/actions"
	"github.com/npiganeau/yep/yep/controllers"
	"github.com/npiganeau/yep/yep/models"
	"github.com/npiganeau/yep/yep/server"
	"github.com/npiganeau/yep/yep/tools/generate"
	"github.com/npiganeau/yep/yep/tools/logging"
	"github.com/npiganeau/yep/yep/views"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const startFilePrefix = "start"

var serverCmd = &cobra.Command{
	Use:   "server [projectDir]",
	Short: "Start the YEP server",
	Long: `Start the YEP server of the project in 'projectDir'.
If projectDir is omitted, defaults to the current directory.`,
	Run: func(cmd *cobra.Command, args []string) {
		projectDir := "."
		if len(args) > 0 {
			projectDir = args[0]
		}
		generateAndRunStartupFile(projectDir)
	},
}

// generateAndRunStartupFile creates the startup file of the project and runs it.
func generateAndRunStartupFile(projectDir string) {
	projectPack, err := build.ImportDir(path.Join(projectDir, "config"), 0)
	if err != nil && !generateTestModule {
		panic(fmt.Errorf("Error while importing project path: %s", err))
	}

	tmplData := struct {
		Imports []string
		Config  string
	}{
		Imports: projectPack.Imports,
		Config:  fmt.Sprintf("%#v", viper.AllSettings()),
	}
	startFileName := path.Join(projectDir, "start.go")
	generate.CreateFileFromTemplate(startFileName, startFileTemplate, tmplData)
	cmd := exec.Command("go", "run", startFileName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

// StartServer starts the YEP server. It is meant to be called from
// a project start file which imports all the project's module.
func StartServer(config map[string]interface{}) {
	for key, value := range config {
		viper.Set(key, value)
	}
	if !viper.GetBool("Server.Debug") {
		gin.SetMode(gin.ReleaseMode)
	}
	logging.Initialize()
	log := logging.GetLogger("init")
	connectString := fmt.Sprintf("dbname=%s sslmode=disable", viper.GetString("Server.DBName"))
	if viper.GetString("Server.DBUser") != "" {
		connectString += fmt.Sprintf(" user=%s", viper.GetString("Server.DBUser"))
	}
	if viper.GetString("Server.DBPassword") != "" {
		connectString += fmt.Sprintf(" password=%s", viper.GetString("Server.DBPassword"))
	}
	if viper.GetString("Server.DBHost") != "" {
		connectString += fmt.Sprintf(" host=%s", viper.GetString("Server.DBHost"))
	}
	if viper.GetString("Server.DBPort") != "5432" {
		connectString += fmt.Sprintf(" port=%s", viper.GetString("Server.DBPort"))
	}
	models.DBConnect(viper.GetString("Server.DBDriver"), connectString)
	models.BootStrap()
	server.LoadInternalResources()
	views.BootStrap()
	actions.BootStrap()
	controllers.BootStrap()
	server.PostInit()
	srv := server.GetServer()
	log.Info("YEP is up and running")
	srv.Run()
}

func initServer() {
	YEPCmd.AddCommand(serverCmd)

	serverCmd.PersistentFlags().String("db-driver", "postgres", "Database driver to use")
	viper.BindPFlag("Server.DBDriver", serverCmd.PersistentFlags().Lookup("db-driver"))
	serverCmd.PersistentFlags().String("db-host", "", "Database hostname or IP. Leave empty to connect through socket.")
	viper.BindPFlag("Server.DBHost", serverCmd.PersistentFlags().Lookup("db-host"))
	serverCmd.PersistentFlags().String("db-port", "5432", "Database port. Value is ignored if db-host is not set.")
	viper.BindPFlag("Server.DBPort", serverCmd.PersistentFlags().Lookup("db-port"))
	serverCmd.PersistentFlags().String("db-user", "", "Database user. Defaults to current user")
	viper.BindPFlag("Server.DBUser", serverCmd.PersistentFlags().Lookup("db-user"))
	serverCmd.PersistentFlags().String("db-password", "", "Database password. Leave empty when connecting through socket.")
	viper.BindPFlag("Server.DBPassword", serverCmd.PersistentFlags().Lookup("db-password"))
	serverCmd.PersistentFlags().String("db-name", "yep", "Database name. Defaults to 'yep'")
	viper.BindPFlag("Server.DBName", serverCmd.PersistentFlags().Lookup("db-name"))
	serverCmd.PersistentFlags().Bool("debug", false, "Enable server debug mode for development")
	viper.BindPFlag("Server.Debug", serverCmd.PersistentFlags().Lookup("debug"))
}

var startFileTemplate = template.Must(template.New("").Parse(`
// This file is autogenerated by yep-server
// DO NOT MODIFY THIS FILE - ANY CHANGES WILL BE OVERWRITTEN

package main

import (
	"github.com/npiganeau/yep/cmd"
{{ range .Imports }}	_ "{{ . }}"
{{ end }}
)

func main() {
	cmd.StartServer({{ .Config }})
}
`))
