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
	"os"
	"strings"

	"golang.org/x/oauth2"

	"github.com/google/go-github/github"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	commit  string
	state   string
	desc    string
	context string
	url     string
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Add or update a pull request status on Github.",
	Long: `Add or update a pull request status on Github.

This command allows an arbitrary CI implementation to
post back the status of its run to the pull request
related to the commit the CI was run against.`,
}

func init() {
	RootCmd.AddCommand(statusCmd)

	statusCmd.Run = status
	statusCmd.Flags().StringVarP(&commit, "commit", "c", "", "commit to associate the status with")
	statusCmd.Flags().StringVarP(&state, "state", "s", "", "pull request state (pending | success | failure | error)")
	statusCmd.Flags().StringVarP(&desc, "desc", "d", "", "a short description of the status")
	statusCmd.Flags().StringVarP(&context, "context", "x", "", "the contextual identifier for this status")
	statusCmd.Flags().StringVarP(&url, "url", "u", "", "a reference url for more information about this status")
	viper.BindPFlag("commit", statusCmd.Flags().Lookup("commit"))
	viper.BindPFlag("state", statusCmd.Flags().Lookup("state"))
	viper.BindPFlag("desc", statusCmd.Flags().Lookup("desc"))
	viper.BindPFlag("context", statusCmd.Flags().Lookup("context"))
	viper.BindPFlag("url", statusCmd.Flags().Lookup("url"))
}

func check_usage() {
	// check if a value is in a list
	in := func(a string, list []string) bool {
		for _, b := range list {
			if b == a {
				return true
			}
		}
		return false
	}
	missing := []string{}
	usage := ""

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

	if len(missing) > 0 {
		usage += fmt.Sprintf("MISSING REQUIRED FLAGS: %s\n", strings.Join(missing, ", "))
	}

	state = strings.ToLower(state)
	states := []string{"pending", "success", "failure", "error"}
	if !in(state, states) {
		usage += fmt.Sprintf("ERROR: The 'state' flag must be one of: %s\n", strings.Join(states, ", "))
	}

	if usage != "" {
		fmt.Printf("\n%s\n", usage)
		statusCmd.Help()
		os.Exit(-1)
	}
}

func status(cmd *cobra.Command, args []string) {
	check_usage()
	token := viper.GetString("token")
	owner := viper.GetString("owner")
	repo := viper.GetString("repo")

	// setup authentication via a github token and create connection
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	gh := github.NewClient(tc)

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
		fmt.Printf("\nERROR: %s\n", err.Error())
		os.Exit(2)
	}
	fmt.Println("\nSuccessfully updated the status!\n")
}
