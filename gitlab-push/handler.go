package function

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/openfaas/openfaas-cloud/sdk"
)

// Handle a serverless request
func Handle(req []byte) string {
	event := os.Getenv("Http_X_Gitlab_Event")
	if event != "Push Hook" {
		return fmt.Sprintf("Your request is: `%s` and event we support is `Push Event`", event)
	}

	gitlabPushEvent := GitLabPushEvent{}
	json.Unmarshal(req, &gitlabPushEvent)
	pushEvent := sdk.PushEvent{
		Ref: gitlabPushEvent.Ref,
		Repository: sdk.Repository{
			Name:     gitlabPushEvent.Project.Name,
			FullName: gitlabPushEvent.Project.PathWithNamespace,
			CloneURL: gitlabPushEvent.GitLabRepository.CloneURL,
			Owner: sdk.Owner{
				Login: gitlabPushEvent.UserUsername,
				Email: gitlabPushEvent.UserEmail,
			},
		},
		AfterCommitID: gitlabPushEvent.AfterCommitID,
	}

	serviceValue := fmt.Sprintf("%s-%s", pushEvent.Repository.Owner.Login, pushEvent.Repository.Name)
	eventInfo := sdk.BuildEventFromPushEvent(pushEvent)
	status := sdk.BuildStatus(eventInfo, sdk.EmptyAuthToken)
	status.AddStatus(sdk.StatusPending, fmt.Sprintf("%s stack deploy is in progress", serviceValue), sdk.StackContext)
	reportStatus(status)

	statusCode, postErr, url := postEvent(pushEvent)
	if postErr != nil {
		status.AddStatus(sdk.StatusFailure, postErr.Error(), sdk.StackContext)
		reportStatus(status)
		if url == "" {
			return postErr.Error() + "NO URL BRAH"
		}
		return postErr.Error() + url
	}
	return fmt.Sprintf("Push - %v, git-tar status: %d\n", pushEvent, statusCode)
}

func postEvent(pushEvent sdk.PushEvent) (int, error, string) {
	gatewayURL := os.Getenv("gateway_url")

	body, _ := json.Marshal(pushEvent)

	c := http.Client{}
	bodyReader := bytes.NewBuffer(body)
	httpReq, _ := http.NewRequest(http.MethodPost, gatewayURL+"async-function/git-tar", bodyReader)
	res, reqErr := c.Do(httpReq)

	if reqErr != nil {
		return http.StatusServiceUnavailable, reqErr, gatewayURL
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	return res.StatusCode, nil, gatewayURL
}

func reportStatus(status *sdk.Status) {

	if !enableStatusReporting() {
		return
	}

	gatewayURL := os.Getenv("gateway_url")

	_, reportErr := status.Report(gatewayURL)
	if reportErr != nil {
		log.Printf("failed to report status, error: %s", reportErr.Error())
	}
}

func enableStatusReporting() bool {
	return os.Getenv("report_status") == "true"
}

type GitLabPushEvent struct {
	Ref              string           `json:"ref"`
	UserUsername     string           `json:"user_username"`
	UserEmail        string           `json:"user_email"`
	Project          Project          `json:"project"`
	GitLabRepository GitLabRepository `json:"repository"`
	AfterCommitID    string           `json:"after"`
}

type Project struct {
	Name              string `json:"name"`
	PathWithNamespace string `json:"path_with_namespace"` //would be repo full name
}

type GitLabRepository struct {
	CloneURL string `json:"git_http_url"`
}
