package server

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/bakape/meguca/auth"
	"github.com/bakape/meguca/util"
	"github.com/go-playground/log"
)

var (
	reproducibleMap = map[string]string{
		"on":  "true",
		"off": "false",
	}
)

type JiraPost struct {
	Fields JiraFields `json:"fields"`
}

type JiraFields struct {
	Project     JiraProject `json:"project"`
	Summary     string      `json:"summary"`
	Description string      `json:"description"`
	Issuetype   JiraIssue   `json:"issuetype"`
}

type JiraProject struct {
	Key string `json:"key"`
}

type JiraIssue struct {
	ID string `json:"id"`
}

type JiraResponse struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Self string `json:"self"`
}

// Bug Report
func reportBug(w http.ResponseWriter, r *http.Request) {

	var (
		jiraResponse JiraResponse
	)

	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		log.Errorf(err.Error())
		http.Error(w, "Failed to parse multipart form data", http.StatusBadRequest)
		return
	}

	wallet, _ := util.ConnectedWalletAddress(r)
	ip, _ := auth.GetIP(r)

	jiraResponse, err = postBugToJira(w, r, ip, wallet)
	if err != nil {
		log.Errorf("Failed to post bug to jira")
		log.Errorf(err.Error())
	}

	if len(r.MultipartForm.File["screenshot"]) > 0 {
		err = attachImageToIssue(w, r, jiraResponse)
		if err != nil {
			log.Errorf("Failed to attach image to issue")
			log.Errorf(err.Error())
		}
	}

	w.WriteHeader(http.StatusOK)
	return
}

func postBugToJira(w http.ResponseWriter, r *http.Request, ip string, wallet string) (jiraResponse JiraResponse, err error) {

	// browser + contact

	var (
		url          = "https://remilia.atlassian.net/rest/api/2/issue"
		reproducible = reproducibleMap[r.FormValue("reproducible")]
		agent        = r.UserAgent()
		info         = r.FormValue("os")
		contact      = r.FormValue("contact")
		summary      = "AUTO: " + r.FormValue("title")
		description  = "REPORTED AT " + time.Now().Format(time.RFC850) + "\nREPORTER:\n" + wallet + " @ " + ip + "\nBrowser: " + agent + "\nReported OS/Browser: " + info + "\nContact: " + contact + "\nDESCRIPTION:\n" +
			r.FormValue("description") + "\nREPRODUCIBLE: " + reproducible
		jiraPostData = JiraPost{
			Fields: JiraFields{
				Project: JiraProject{
					Key: "MBR",
				},
				Issuetype: JiraIssue{
					ID: "10139",
				},
				Summary:     summary,
				Description: description,
			},
		}
	)

	payload, err := json.Marshal(jiraPostData)
	if err != nil {
		return
	}

	// hardcoded auth value ;p
	// do not give out src until this is in environment plz
	req, _ := http.NewRequest("POST", url, strings.NewReader(string(payload)))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Basic bmVpbHNlbWFpbEBtZS5jb206QVRBVFQzeEZmR0YwV0JMM0cxZmt2ZTNVOFh4aUZMMWVuMWRtTmpzVHNLMDY3YUdYSFR2ZjFfRHJFZ09wRWd6VEJnd0h5a0lnR25LNmVJWWQ0RFY2Q0dGMDZya2xEZ0JlOXpfTGFvLUVYZi1YWHRHVmhINVhWZGp4ekktbXpuZjZsaDZlQTNFdXF4ZW8xSElhaVBXRnVBTloyMmtpVC1rQ3RjY2pXdzRYOWcweTZvQXAzTUxwMmg0PUE1ODA4RTA1")
	res, _ := http.DefaultClient.Do(req)
	defer res.Body.Close()

	err = json.NewDecoder(res.Body).Decode(&jiraResponse)
	if err != nil {
		log.Errorf(err.Error())
		return
	}

	return

}

// update to support multiple file upload
func attachImageToIssue(w http.ResponseWriter, r *http.Request, jiraResponse JiraResponse) (err error) {
	url := "https://remilia.atlassian.net/rest/api/2/issue/" + jiraResponse.Key + "/attachments"

	f, err := r.MultipartForm.File["screenshot"][0].Open()
	if err != nil {
		err = nil
		return
	}

	defer f.Close()

	buf := new(bytes.Buffer)
	writer := multipart.NewWriter(buf)

	part, err := writer.CreateFormFile("file", r.MultipartForm.File["screenshot"][0].Filename)

	b, err := ioutil.ReadAll(f)

	if err != nil {
		return
	}

	part.Write(b)
	writer.Close()

	req, _ := http.NewRequest("POST", url, buf)
	req.Header.Add("X-Atlassian-Token", "nocheck")
	req.Header.Add("Authorization", "Basic bmVpbHNlbWFpbEBtZS5jb206QVRBVFQzeEZmR0YwV0JMM0cxZmt2ZTNVOFh4aUZMMWVuMWRtTmpzVHNLMDY3YUdYSFR2ZjFfRHJFZ09wRWd6VEJnd0h5a0lnR25LNmVJWWQ0RFY2Q0dGMDZya2xEZ0JlOXpfTGFvLUVYZi1YWHRHVmhINVhWZGp4ekktbXpuZjZsaDZlQTNFdXF4ZW8xSElhaVBXRnVBTloyMmtpVC1rQ3RjY2pXdzRYOWcweTZvQXAzTUxwMmg0PUE1ODA4RTA1")
	req.Header.Add("Content-Type", writer.FormDataContentType())

	res, err := http.DefaultClient.Do(req)

	defer res.Body.Close()

	return
}
