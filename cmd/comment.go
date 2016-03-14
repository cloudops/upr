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
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	"golang.org/x/oauth2"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/google/go-github/github"
	"github.com/ncw/swift"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/swill/upr/assets"
)

const (
	S3    string = "s3"
	SWIFT string = "swift"
)

var (
	templates *template.Template
)

type Upload struct {
	Name string
	Path string
	Obj  string
	URL  string
}

type CommentBody struct {
	CommitID      string
	Title         string
	Summary       string
	Uploads       map[string][]Upload
	UploadsExpire *time.Time // pointers can be nil, for template conditional
}

// commentCmd represents the comment command
var commentCmd = &cobra.Command{
	Use:   "comment",
	Short: "Add a comment to a pull request on Github.",
	Long: `Add a comment to a pull request on Github.

This command allows an arbitrary CI implementation to
post a comment to a pull request issue thread.

Optionally, files can be made public by uploading them to
an object store using either the Swift or S3 API.`,
}

func init() {
	RootCmd.AddCommand(commentCmd)

	commentCmd.Run = comment
	commentCmd.Flags().IntP("pr_num", "n", 0, "required unless 'commit' isset: pull request number on which to comment on")
	commentCmd.Flags().StringP("comment_file", "f", "", "required: file which includes the comment text")
	commentCmd.Flags().StringP("title", "t", "", "the title of the comment")
	commentCmd.Flags().StringP("uploads", "u", "", "comma separated list of files or directories to be recusively uploaded")
	commentCmd.Flags().String("uploads_api", "", fmt.Sprintf(
		"required if 'uploads' isset: api to use to upload to an object store (%s | %s)", S3, SWIFT))
	commentCmd.Flags().String("uploads_endpoint", "", "required if 'uploads' isset: object store url endpoint")
	commentCmd.Flags().String("uploads_region", "", fmt.Sprintf(
		"upload region when using the '%s' api", S3))
	commentCmd.Flags().String("uploads_identity", "", fmt.Sprintf(`%s: use the '~/.aws/credentials' file or a 'AWS_ACCESS_KEY_ID' env var
                                  %s: keystone identity as 'tenant:username'`, S3, SWIFT))
	commentCmd.Flags().String("uploads_secret", "", fmt.Sprintf(`%s: use the '~/.aws/credentials' file or a 'AWS_SECRET_ACCESS_KEY' env var
                                  %s: keystone password`, S3, SWIFT))
	commentCmd.Flags().StringP("uploads_bucket", "b", "", "required if 'uploads' isset: bucket to upload the files to (will be made public)")
	commentCmd.Flags().IntP("uploads_expire", "e", 0, "optional number of days to keep the uploaded files before they are removed")
	commentCmd.Flags().Int("uploads_concurrency", 4, "number of files to be uploaded concurrently")
	viper.BindPFlag("pr_num", commentCmd.Flags().Lookup("pr_num"))
	viper.BindPFlag("file", commentCmd.Flags().Lookup("comment_file"))
	viper.BindPFlag("title", commentCmd.Flags().Lookup("title"))
	viper.BindPFlag("uploads", commentCmd.Flags().Lookup("uploads"))
	viper.BindPFlag("uploads_api", commentCmd.Flags().Lookup("uploads_api"))
	viper.BindPFlag("uploads_endpoint", commentCmd.Flags().Lookup("uploads_endpoint"))
	viper.BindPFlag("uploads_region", commentCmd.Flags().Lookup("uploads_region"))
	viper.BindPFlag("uploads_identity", commentCmd.Flags().Lookup("uploads_identity"))
	viper.BindPFlag("uploads_secret", commentCmd.Flags().Lookup("uploads_secret"))
	viper.BindPFlag("uploads_bucket", commentCmd.Flags().Lookup("uploads_bucket"))
	viper.BindPFlag("uploads_expire", commentCmd.Flags().Lookup("uploads_expire"))
	viper.BindPFlag("uploads_concurrency", commentCmd.Flags().Lookup("uploads_concurrency"))
}

func commentCheckUsage() {
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
	if !viper.IsSet("commit") && !viper.IsSet("pr_num") {
		missing = append(missing, "(commit || pr_num)")
	}
	if !viper.IsSet("file") {
		missing = append(missing, "comment_file")
	}

	if viper.IsSet("uploads") {
		if !viper.IsSet("uploads_api") {
			missing = append(missing, "uploads_api")
		}
		if !viper.IsSet("uploads_endpoint") {
			missing = append(missing, "uploads_endpoint")
		}
		if !viper.IsSet("uploads_bucket") {
			missing = append(missing, "uploads_bucket")
		}

		api := strings.ToLower(viper.GetString("uploads_api"))
		apis := []string{S3, SWIFT}
		if !in(apis, api) {
			invalid += fmt.Sprintf("ERROR: The 'uploads_api' flag must be one of: %s\n", strings.Join(apis, ", "))
		}
		if api == SWIFT && !viper.IsSet("uploads_identity") {
			missing = append(missing, "uploads_identity")
		}
		if api == SWIFT && !viper.IsSet("uploads_secret") {
			missing = append(missing, "uploads_secret")
		}
		if api == SWIFT && viper.IsSet("uploads_identity") {
			if !strings.Contains(viper.GetString("uploads_identity"), ":") {
				invalid += fmt.Sprintf("ERROR: The 'uploads_identity' flag for '%s' is formatted as 'tenant:username'\n", SWIFT)
			}
		}
		if api == S3 && !viper.IsSet("uploads_region") {
			missing = append(missing, "uploads_region")
			invalid += fmt.Sprintf("ERROR: The 'uploads_region' flag is required when using the '%s' api for 'uploads'\n", S3)
		}
	}

	if len(missing) > 0 {
		usage += fmt.Sprintf("MISSING REQUIRED FLAGS: %s\n", strings.Join(missing, ", "))
	}

	usage += invalid
	if usage != "" {
		fmt.Printf("\n%s\n", usage)
		commentCmd.Help()
		os.Exit(-1)
	}
}

// Post a comment in a Github pull request
func comment(cmd *cobra.Command, args []string) {
	// check if an int is in a list
	in := func(list []int, a int) bool {
		for _, b := range list {
			if b == a {
				return true
			}
		}
		return false
	}

	commentCheckUsage()
	prs := []int{}
	found_pr := false
	token := viper.GetString("token")
	owner := viper.GetString("owner")
	repo := viper.GetString("repo")
	commit := viper.GetString("commit")
	pr_num := viper.GetInt("pr_num")
	title := viper.GetString("title")
	comment_file := viper.GetString("file")
	api := strings.ToLower(viper.GetString("uploads_api"))

	// load the templates to be used later
	local := false // default to false
	if viper.IsSet("custom_template") {
		local = viper.GetBool("custom_template")
	}
	templates = template.Must(template.New("").Parse(
		assets.FSMustString(local, fmt.Sprintf("%sstatic%stemplates.tpl", string(os.PathSeparator), string(os.PathSeparator))),
	))

	// setup authentication via a github token and create connection
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	gh := github.NewClient(tc)

	// populate the 'prs' with the different prs to post to
	if viper.IsSet("pr_num") {
		prs = append(prs, pr_num)
	}

	// if commit is set, check which prs include this commit
	if viper.IsSet("commit") {
		// list all pull requests and try to match one to our commit id
		opts := &github.PullRequestListOptions{}
		all_prs, _, err := gh.PullRequests.List(owner, repo, opts)
		if err != nil {
			log.Printf("ERROR getting commit PRs: %s\n", err.Error())
			os.Exit(-1)
		}
		if len(all_prs) > 0 {
			for _, pr := range all_prs {
				pr_commits, _, err := gh.PullRequests.ListCommits(owner, repo, *pr.Number, nil)
				if err != nil {
					log.Printf("ERROR getting Commits for PR '%d': %s\n", *pr.Number, err.Error())
					os.Exit(-1)
				}
				for _, pr_commit := range pr_commits {
					if *pr_commit.SHA == commit {
						if !in(prs, *pr.Number) {
							prs = append(prs, *pr.Number)
						}
					}
				}
			}
		}
	}

	// have at least one PR to post to, create the comment and upload files (if needed)
	var comment *github.IssueComment
	if len(prs) > 0 {
		// get comment text
		comment_text, err := ioutil.ReadFile(comment_file)
		if err != nil {
			log.Printf("ERROR reading comment_file '%s': %s\n", comment_file, err.Error())
			os.Exit(-1)
		}

		// populate the CommentBody object to be passed into the template
		comment_body := &CommentBody{
			Summary: string(comment_text),
		}

		if viper.IsSet("title") {
			comment_body.Title = title
		}

		if viper.IsSet("commit") {
			comment_body.CommitID = commit
		}

		if viper.IsSet("uploads") {
			comment_body.PopulateUploads()

			if api == SWIFT {
				comment_body.UploadToSwift()
			}
			if api == S3 {
				comment_body.UploadToS3()
			}
		}

		var buf bytes.Buffer
		err = templates.ExecuteTemplate(&buf, "pr_comment", comment_body)
		if err != nil {
			log.Printf("ERROR executing template: %s\n", err.Error())
			os.Exit(-1)
		}
		body := buf.String()
		_body := &body

		// create the issue comment on github
		comment = &github.IssueComment{
			Body: _body,
		}
	}

	// loop through all the PRs to comment on and make the comment
	for _, pr_int := range prs {
		found_pr = true
		// Proceed commenting on all relevant PRs
		log.Printf("Updating PR '%d' with details.\n", pr_int)

		_, _, err := gh.Issues.CreateComment(owner, repo, pr_int, comment)
		if err != nil {
			log.Printf("ERROR: %s\n", err.Error())
			os.Exit(-1)
		}
	}
	if found_pr {
		log.Printf("Finished commenting on pull request(s)!\n\n")
	} else {
		log.Printf("NOTICE: No PRs were found matching your query, nothing done...\n\n")
	}

}

// Populates the Uploads section of the CommentBody struct
func (c *CommentBody) PopulateUploads() {
	uploads := viper.GetString("uploads")
	c.Uploads = make(map[string][]Upload)

	// local code reuse for populating the 'Uploads' field
	populate_upload := func(path string) {
		dir := filepath.Dir(path)
		name := filepath.Base(path)
		obj := strings.Replace(path, "..", "up", -1) // replace '..' in the obj path
		obj = strings.TrimPrefix(obj, string(os.PathSeparator))
		obj = filepath.ToSlash(obj) // fix windows paths

		if _, exists := c.Uploads[dir]; exists {
			c.Uploads[dir] = append(c.Uploads[dir], Upload{
				Name: name,
				Path: path,
				Obj:  obj,
			})
		} else {
			c.Uploads[dir] = []Upload{
				{
					Name: name,
					Path: path,
					Obj:  obj,
				},
			}
		}
	}

	items := strings.Split(uploads, ",")
	for _, item := range items {
		clean := filepath.Clean(strings.TrimSpace(item))
		f, err := os.Open(clean)
		if err != nil {
			log.Printf("ERROR: Failed to open upload file '%s'.\n", clean)
			continue
		}
		defer f.Close()
		fi, err := f.Stat()
		if err != nil {
			log.Printf("ERROR: Failed to stat upload file '%s'.\n", clean)
			continue
		}
		switch mode := fi.Mode(); {
		// Process a directory
		case mode.IsDir():
			err = filepath.Walk(clean, func(path string, info os.FileInfo, _ error) (err error) {
				if info.Mode().IsRegular() {
					sub_clean := filepath.Clean(strings.TrimSpace(path))
					populate_upload(sub_clean)
				}
				return nil
			})
			if err != nil {
				log.Printf("ERROR: Walking upload directory '%s'.\n", clean)
			}
		// Process a regular file
		case mode.IsRegular():
			populate_upload(clean)
		}
	}
}

// Upload the files via the Swift API
func (c *CommentBody) UploadToSwift() {
	var tenant, username string
	bucket := viper.GetString("uploads_bucket")
	expires := viper.GetInt("uploads_expire")
	var expire_time time.Time
	if expires != 0 {
		// Making the Swift format match the required S3 format of '2014-04-08T00:00:00.000Z', truncated to midnight on GTM time
		expire_time = time.Now().Truncate(time.Duration(24) * time.Hour).Add(time.Duration(expires+1) * 24 * time.Hour)
		c.UploadsExpire = &expire_time
	}

	// get the details about the identity (tenant and user)
	parts := strings.Split(viper.GetString("uploads_identity"), ":")
	if len(parts) > 1 {
		tenant = parts[0]
		username = parts[1]
	} else {
		log.Printf("ERROR: The 'uploads_identity' flag for '%s' is formatted as 'tenant:username'\n", SWIFT)
		commentCmd.Help()
		os.Exit(-1)
	}

	// make a swift connection
	conn := swift.Connection{
		Tenant:   tenant,
		UserName: username,
		ApiKey:   viper.GetString("uploads_secret"),
		AuthUrl:  viper.GetString("uploads_endpoint"),
	}

	// authenticate swift user
	err := conn.Authenticate()
	if err != nil {
		log.Println("ERROR: Swift authentication failed.  Validate your credentials are correct.")
		os.Exit(-1)
	}

	// create the container if it does not already exist
	err = conn.ContainerCreate(bucket, nil)
	if err != nil {
		log.Printf("ERROR: Problem creating bucket '%s'\n", bucket)
		log.Println(err)
		os.Exit(-1)
	}

	// update container headers
	metadata := make(swift.Metadata, 0)
	headers := metadata.ContainerHeaders()
	headers["X-Container-Read"] = ".r:*,.rlistings" // make the container public
	err = conn.ContainerUpdate(bucket, headers)
	if err != nil {
		log.Printf("ERROR: Problem updating headers to make bucket '%s' public\n", bucket)
		log.Println(err)
		os.Exit(-1)
	}

	log.Printf("Using bucket: %s\n", bucket)
	log.Println("Starting upload...  This can take a while, go get a coffee.  :)")

	// do the actual upload
	process_upload := func(u *Upload) error {
		if len(u.Obj) > 0 {
			log.Printf("  started: %s\n", u.Obj)
			f, err := os.Open(u.Path)
			if err != nil {
				log.Printf("ERROR: Problem opening file '%s'\n", u.Path)
				log.Println(err)
				return err
			}
			defer f.Close()
			obj_metadata := make(swift.Metadata, 0)
			obj_headers := obj_metadata.ObjectHeaders()
			if expires != 0 {
				obj_headers["X-Delete-At"] = fmt.Sprintf("%d", expire_time.Unix())
			}
			// currently NOT validating the hash of the upload since I expect large files
			_, err = conn.ObjectPut(bucket, u.Obj, f, false, "", "", obj_headers)
			if err != nil {
				log.Printf("ERROR: Problem uploading object '%s'\n", u.Obj)
				log.Println(err)
				return err
			}
			log.Printf(" uploaded: %s\n", u.Obj)
			u.URL = fmt.Sprintf("%s/%s/%s", strings.TrimRight(conn.StorageUrl, "/"), bucket, u.Obj)
		}
		return nil
	}

	// setup 'process_upload' concurrency controls
	uploadc := make(chan *Upload)
	var wg sync.WaitGroup
	// setup the number of concurrent goroutine workers
	for i := 0; i < viper.GetInt("uploads_concurrency"); i++ {
		wg.Add(1)
		go func() {
			for u := range uploadc {
				process_upload(u)
			}
			wg.Done()
		}()
	}
	// feed the uploads into the concurrent goroutines to be uploaded
	for dir, uploads := range c.Uploads { // loop through the map
		for i, _ := range uploads { // loop through each dir list
			uploadc <- &c.Uploads[dir][i] // point to the object so we can modify it inline
		}
	}
	close(uploadc)
	wg.Wait()
}

// Upload the files via the S3 API
func (c *CommentBody) UploadToS3() {
	bucket := viper.GetString("uploads_bucket")
	endpoint := viper.GetString("uploads_endpoint")
	region := viper.GetString("uploads_region")
	expires := viper.GetInt("uploads_expire")
	var expire_time time.Time
	if expires != 0 {
		// S3 requires the format to be '2014-04-08T00:00:00.000Z', truncated to midnight on GTM time
		expire_time = time.Now().Truncate(time.Duration(24) * time.Hour).Add(time.Duration(expires+1) * 24 * time.Hour)
		c.UploadsExpire = &expire_time
	}

	conn := s3.New(session.New(), &aws.Config{
		Endpoint: aws.String(endpoint),
		Region:   aws.String(region),
	})

	// does bucket exist?
	head_bucket_params := &s3.HeadBucketInput{
		Bucket: aws.String(bucket), // Required
	}
	_, err := conn.HeadBucket(head_bucket_params)
	if err != nil { // bucket did not exist, create it...
		// create a bucket
		create_bucket_params := &s3.CreateBucketInput{
			Bucket: aws.String(bucket), // Required
		}
		_, err = conn.CreateBucket(create_bucket_params)
		if err != nil {
			log.Printf("ERROR: Problem creating bucket '%s'\n", bucket)
			log.Println(err)
			os.Exit(-1)
		}
	}

	// update the acls for the bucket
	acl_bucket_params := &s3.PutBucketAclInput{
		Bucket: aws.String(bucket),
		ACL:    aws.String(s3.BucketCannedACLPublicRead),
	}
	_, err = conn.PutBucketAcl(acl_bucket_params)
	if err != nil {
		log.Printf("ERROR: Problem updating ACLs to make bucket '%s' public\n", bucket)
		log.Println(err)
		os.Exit(-1)
	}

	// set the expire time for the bucket
	if expires != 0 {
		bucket_lifecycle_params := &s3.PutBucketLifecycleConfigurationInput{
			Bucket: aws.String(bucket), // Required
			LifecycleConfiguration: &s3.BucketLifecycleConfiguration{
				Rules: []*s3.LifecycleRule{ // Required
					{ // Required
						Prefix: aws.String("upload-expires"),           // Required
						Status: aws.String(s3.ExpirationStatusEnabled), // Required
						Expiration: &s3.LifecycleExpiration{
							Date: aws.Time(expire_time),
						},
					},
				},
			},
		}
		_, err = conn.PutBucketLifecycleConfiguration(bucket_lifecycle_params)
		if err != nil {
			log.Printf("ERROR: Problem updating lifecycle to automatically expire objects in bucket '%s'\n", bucket)
			log.Println(err)
			os.Exit(-1)
		}
	}

	log.Printf("Using bucket: %s\n", bucket)
	log.Println("Starting upload...  This can take a while, go get a coffee.  :)")

	// do the actual upload
	process_upload := func(u *Upload) error {
		if len(u.Obj) > 0 {
			if expires != 0 {
				u.Obj = fmt.Sprintf("upload-expires/%s", u.Obj)
			}
			log.Printf("  started: %s\n", u.Obj)
			f, err := os.Open(u.Path)
			if err != nil {
				log.Printf("ERROR: Problem opening file '%s'\n", u.Path)
				log.Println(err)
				return err
			}
			defer f.Close()
			put_obj_params := &s3.PutObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(u.Obj),
				Body:   f,
			}
			if expires != 0 {
				put_obj_params.Expires = aws.Time(expire_time)
			}
			_, err = conn.PutObject(put_obj_params)
			if err != nil {
				log.Printf("ERROR: Problem uploading object '%s'\n", u.Obj)
				log.Println(err)
				return err
			}
			// update the acls for the object
			acl_object_params := &s3.PutObjectAclInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(u.Obj),
				ACL:    aws.String(s3.ObjectCannedACLPublicRead),
			}
			_, err = conn.PutObjectAcl(acl_object_params)
			if err != nil {
				log.Printf("ERROR: Problem updating ACLs to make object '%s' public\n", u.Obj)
				log.Println(err)
				return err
			}
			log.Printf(" uploaded: %s\n", u.Obj)
			u.URL = fmt.Sprintf("%s/%s/%s", strings.TrimRight(endpoint, "/"), bucket, u.Obj)
		}
		return nil
	}

	// setup 'process_upload' concurrency controls
	uploadc := make(chan *Upload)
	var wg sync.WaitGroup
	// setup the number of concurrent goroutine workers
	for i := 0; i < viper.GetInt("uploads_concurrency"); i++ {
		wg.Add(1)
		go func() {
			for u := range uploadc {
				process_upload(u)
			}
			wg.Done()
		}()
	}
	// feed the uploads into the concurrent goroutines to be uploaded
	for dir, uploads := range c.Uploads { // loop through the map
		for i, _ := range uploads { // loop through each dir list
			uploadc <- &c.Uploads[dir][i] // point to the object so we can modify it inline
		}
	}
	close(uploadc)
	wg.Wait()
}
