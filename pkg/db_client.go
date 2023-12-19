package main

import (
	"database/sql"
)

type DBClient interface {
	SaveJob(job Job) error
	GetJob(mrID int) (*Job, error)
	DeleteJob(mrID int) error
	GetAllJobs() ([]Job, error)
}

type sqliteDBClient struct {
	db *sql.DB
}

func NewDBClient(dbFilePath string) (DBClient, error) {
	db, err := sql.Open("sqlite3", dbFilePath)
	if err != nil {
		return nil, err
	}
	return &sqliteDBClient{db: db}, nil
}

func (c *sqliteDBClient) SaveJob(job Job) error {
	insertSQL := `INSERT OR REPLACE INTO jobs (mr_id, mr_iid, project_id, commit_sha, branch, update_attempt)
				  VALUES (?, ?, ?, ?, ?, ?)`
	_, err := c.db.Exec(insertSQL, job.MRID, job.MRIID, job.ProjectID, job.CommitSHA, job.Branch, job.UpdateAttempt)
	return err
}

func (c *sqliteDBClient) GetJob(mrID int) (*Job, error) {
	job := &Job{}
	row := c.db.QueryRow("SELECT * FROM jobs WHERE mr_id = ?", mrID)
	err := row.Scan(&job.MRID, &job.MRIID, &job.ProjectID, &job.CommitSHA, &job.Branch, &job.UpdateAttempt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}

func (c *sqliteDBClient) DeleteJob(mrID int) error {
	_, err := c.db.Exec("DELETE FROM jobs WHERE mr_id = ?", mrID)
	return err
}

func (c *sqliteDBClient) GetAllJobs() ([]Job, error) {
	rows, err := c.db.Query("SELECT * FROM jobs")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var job Job
		if err := rows.Scan(&job.MRID, &job.MRIID, &job.ProjectID, &job.CommitSHA, &job.Branch, &job.UpdateAttempt); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}
