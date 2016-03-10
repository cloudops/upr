// Copyright © 2016 Will Stevens <wstevens@cloudops.com>
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
	"fmt"
	"log"
	"os"
	"strings"

	"golang.org/x/oauth2"

	"github.com/google/go-github/github"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Add or update a pull request status on Github.",
	Long: `Add or update  the status of a pull request on Github.

This command allows an arbitrary CI implementation to
post back the status of its run to the pull request
related to the commit the CI was run against.`,
}

func init() {
	RootCmd.AddCommand(statusCmd)

	statusCmd.Run = status
	statusCmd.Flags().StringP("state", "s", "", "required: pull request state (pending | success | failure | error)")
	statusCmd.Flags().StringP("desc", "d", "", "a short description of the environment context")
	statusCmd.Flags().StringP("context", "x", "", "required: the contextual identifier for this status")
	statusCmd.Flags().StringP("url", "u", "", "a reference url for more information about this status")
	viper.BindPFlag("state", statusCmd.Flags().Lookup("state"))
	viper.BindPFlag("desc", statusCmd.Flags().Lookup("desc"))
	viper.BindPFlag("context", statusCmd.Flags().Lookup("context"))
	viper.BindPFlag("url", statusCmd.Flags().Lookup("url"))
}

func statusCheckUsage() {
	// check if a string is in a list
	in := func(list []string, a string) bool {
		for _, b := range list {
			if b == a {
				return true
			}
		}
		return false
	}
	missing := []string{}
	usage := ""
	invalid := ""

	if !viper.IsSet("token") {
		missing = append(missing, "token")
	}
	if !viper.IsSet("owner") {
		missing = append(missing, "owner")
	}
	if !viper.IsSet("repo") {
		missing = append(missing, "repo")
	}
	if !viper.IsSet("commit") {
		missing = append(missing, "commit")
	}
	if !viper.IsSet("state") {
		missing = append(missing, "state")
	}
	if !viper.IsSet("context") {
		missing = append(missing, "context")
	}

	if viper.IsSet("state") {
		state := strings.ToLower(viper.GetString("state"))
		states := []string{"pending", "success", "failure", "error"}
		if !in(states, state) {
			invalid += fmt.Sprintf("ERROR: The 'state' flag must be one of: %s\n", strings.Join(states, ", "))
		}
	}

	if len(missing) > 0 {
		usage += fmt.Sprintf("MISSING REQUIRED FLAGS: %s\n", strings.Join(missing, ", "))
	}

	usage += invalid
	if usage != "" {
		fmt.Printf("\n%s\n", usage)
		statusCmd.Help()
		os.Exit(-1)
	}
}

func status(cmd *cobra.Command, args []string) {
	statusCheckUsage()
	token := viper.GetString("token")
	owner := viper.GetString("owner")
	repo := viper.GetString("repo")
	commit := viper.GetString("commit")
	state := strings.ToLower(viper.GetString("state"))
	desc := viper.GetString("desc")
	context := viper.GetString("context")
	url := viper.GetString("url")

	// setup authentication via a github token and create connection
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	gh := github.NewClient(tc)

	// RepoStatus takes type *string
	_state := &state
	_context := &context
	_desc := &desc
	_url := &url
	repo_status := &github.RepoStatus{
		State:       _state,
		Description: _desc,
		Context:     _context,
		TargetURL:   _url,
	}
	_, _, err := gh.Repositories.CreateStatus(owner, repo, commit, repo_status)
	if err != nil {
		log.Printf("ERROR: %s\n", err.Error())
		os.Exit(-1)
	}
	log.Println("Successfully updated the status!")
}
