package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

var (
	gitlabAPIURL = os.Getenv("GITLAB_API_URL")
	checkerURL   = os.Getenv("CHECKER_URL")
	gitlabToken  = os.Getenv("GITLAB_TOKEN")
	statusName   = "approval_checker"
	maxAttempt   = 5
	dbFile       = "./db/jobs.db"
)

type Job struct {
	MRID          int    `json:"mr_id"`
	MRIID         int    `json:"mr_iid"`
	ProjectID     int    `json:"project_id"`
	CommitSHA     string `json:"commit_sha"`
	Branch        string `json:"branch"`
	UpdateAttempt int    `json:"update_attempt"`
}

func checkApprovals(dbClient DBClient, gitLabClient GitLabClient) {
	for {
		jobs, err := dbClient.GetAllJobs()
		if err != nil {
			log.Println("Error fetching jobs:", err)
			continue
		}

		for _, job := range jobs {
			checkApproval(dbClient, gitLabClient, job)
		}
		time.Sleep(2 * time.Minute)
	}
}

func checkApproval(dbClient DBClient, gitLabClient GitLabClient, job Job) {
	log.Printf("Checking job %d\n", job.MRID)

	isApproved, err := gitLabClient.GetMergeRequestApprovals(job.ProjectID, job.MRIID)
	if err != nil {
		log.Println("Error getting merge request approvals:", err)
		job.UpdateAttempt++
		if job.UpdateAttempt > maxAttempt {
			dbClient.DeleteJob(job.MRID)
		} else {
			dbClient.SaveJob(job)
		}
		return
	}

	status := "pending"
	if isApproved {
		status = "success"
		log.Printf("Approving job %d\n", job.MRID)
		dbClient.DeleteJob(job.MRID)
	} else {
		dbClient.SaveJob(job)
	}

	err = gitLabClient.UpdateMergeRequestStatus(job.ProjectID, job.CommitSHA, job.Branch, status)
	if err != nil {
		log.Printf("Failed to update status for job %d\n", job.MRID)
	}
}

func webhookHandler(dbClient DBClient, gitLabClient GitLabClient, w http.ResponseWriter, r *http.Request) {
	var data struct {
		Project struct {
			ID int `json:"id"`
		} `json:"project"`
		ObjectAttributes struct {
			ID         int `json:"id"`
			IID        int `json:"iid"`
			LastCommit struct {
				ID string `json:"id"`
			} `json:"last_commit"`
			SourceBranch string `json:"source_branch"`
		} `json:"object_attributes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	job := Job{
		MRID:          data.ObjectAttributes.ID,
		MRIID:         data.ObjectAttributes.IID,
		ProjectID:     data.Project.ID,
		CommitSHA:     data.ObjectAttributes.LastCommit.ID,
		Branch:        data.ObjectAttributes.SourceBranch,
		UpdateAttempt: 0,
	}
	dbClient.SaveJob(job)

	// Immediately check for approval
	checkApproval(dbClient, gitLabClient, job)

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
}

func statusHandler(dbClient DBClient, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr, ok := vars["id"]
	if !ok {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID should be an int", http.StatusBadRequest)
		return
	}

	job, err := dbClient.GetJob(id)
	if err != nil {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(job)
}

func main() {
	dbClient, err := NewDBClient(dbFile)
	if err != nil {
		log.Fatal("Failed to create DB client:", err)
	}

	gitLabClient := NewGitLabClient(gitlabAPIURL, gitlabToken)

	go checkApprovals(dbClient, gitLabClient)

	fmt.Println("Starting server...")

	r := mux.NewRouter()
	r.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok")
	}).Methods("GET")
	r.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		webhookHandler(dbClient, gitLabClient, w, r)
	}).Methods("POST")
	r.HandleFunc("/status/{id:[0-9]+}", func(w http.ResponseWriter, r *http.Request) {
		statusHandler(dbClient, w, r)
	}).Methods("GET")

	http.ListenAndServe(":5000", r)
}
