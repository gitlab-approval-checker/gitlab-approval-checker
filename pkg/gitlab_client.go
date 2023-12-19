package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type GitLabClient interface {
	GetMergeRequestApprovals(projectID, mrIID int) (bool, error)
	UpdateMergeRequestStatus(projectID int, commitSHA, branch, status string) error
}

type gitLabClientImpl struct {
	apiURL     string
	token      string
	httpClient *http.Client
}

func NewGitLabClient(apiURL, token string) GitLabClient {
	return &gitLabClientImpl{
		apiURL:     apiURL,
		token:      token,
		httpClient: &http.Client{},
	}
}

func (c *gitLabClientImpl) GetMergeRequestApprovals(projectID, mrIID int) (bool, error) {
	url := fmt.Sprintf("%s/projects/%d/merge_requests/%d/approvals", c.apiURL, projectID, mrIID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, err
	}
	req.Header.Add("PRIVATE-TOKEN", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("non-OK HTTP status: %d", resp.StatusCode)
	}

	var data struct {
		ApprovedBy []struct{} `json:"approved_by"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return false, err
	}

	return len(data.ApprovedBy) > 0, nil
}

func (c *gitLabClientImpl) UpdateMergeRequestStatus(projectID int, commitSHA, branch, status string) error {
	url := fmt.Sprintf("%s/projects/%d/statuses/%s?ref=%s", c.apiURL, projectID, commitSHA, branch)
	statusPayload := struct {
		State     string `json:"state"`
		Name      string `json:"name"`
		TargetURL string `json:"target_url"`
	}{
		State:     status,
		Name:      statusName,
		TargetURL: fmt.Sprintf("%s/status/%d", checkerURL, projectID), // Assumed projectID is used here
	}
	payloadBytes, err := json.Marshal(statusPayload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return err
	}
	req.Header.Add("PRIVATE-TOKEN", c.token)
	req.Header.Add("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	// Handle specific non-OK response
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		var respBody struct {
			Message string `json:"message"`
		}
		json.Unmarshal(bodyBytes, &respBody)

		if respBody.Message == "Cannot transition status via :enqueue from :pending (Reason(s): Status cannot transition via \"enqueue\")" {
			return nil
		}

		return fmt.Errorf("failed to update status: HTTP %d, Message: %s", resp.StatusCode, respBody.Message)
	}

	return nil
}
