package main

import (
	"context"
	"fmt"
	"job_workers/job"
	"log"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"
)

var rdb *redis.Client
var ctx = context.Background()

func init() {
	rdb = redis.NewClient(&redis.Options{
		Addr:     "redis_server:6379",
		Password: "",
		DB:       0,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal("Unable to ping redis db")
	}
}

func main() {
	for {
		job := &job.Job{}

		if job.GetFromPendingQue(rdb, &ctx) == nil {
			fmt.Printf("Job #%d was popped from pending que and sent to processing que.\n", job.ID)
		} else {
			continue
		}

		Process(job)
	}
}

func Process(job *job.Job) {
	processChan := make(chan int)

	go func() {
		fmt.Printf("Job #%d was started. Data: %s.\n", job.ID, job.Data)

		job.Status = "processing"
		if job.SaveToDB(rdb, &ctx) != nil {
			processChan <- 1
			return
		} else {
			fmt.Printf("Job #%d's state was saved to DB.\n", job.ID)
		}

		<-time.After(time.Duration(rand.Int63n(int64(3 * time.Second))))

		if rand.Intn(4) == 3 {
			processChan <- 1
			return
		}
		processChan <- 0
		// 0 - done, 1 - failed

	}()

	go func() {
		time.Sleep(job.Timeout)
		processChan <- -1
	}()

	result := <-processChan
	job.AttemptsFinished += 1

	switch result {
	case -1:
		fmt.Printf("Job #%d was timeouted.\n", job.ID)
	case 0:
		fmt.Printf("Job #%d was done.\n", job.ID)
	case 1:
		fmt.Printf("Job #%d was failed.\n", job.ID)
	}

	switch result {
	case -1, 1:
		if job.MaxAttempts <= job.AttemptsFinished {
			if result == -1 {
				job.Status = "timeout"
			} else {
				job.Status = "failed"
			}
		} else {
			job.Status = "pending"
		}
	case 0:
		job.Status = "done"
	}

	if job.SaveToDB(rdb, &ctx) != nil {
		if job.MaxAttempts >= job.AttemptsFinished {
			job.Status = "failed"
		} else {
			job.Status = "pending"
		}
	} else {
		fmt.Printf("Job #%d's state was saved to DB.\n", job.ID)
	}

	if job.Status == "pending" {

		if job.SendToPendingQue(rdb, &ctx) == nil {
			fmt.Printf("Job #%d was sent to pending que.\n", job.ID)

			if job.DeleteFromWorkingQue(rdb, &ctx) == nil {
				fmt.Printf("Job #%d was deleted from working que.\n", job.ID)
			}
		}

	} else {
		if job.DeleteFromWorkingQue(rdb, &ctx) == nil {
			fmt.Printf("Job #%d was deleted from working que.\n", job.ID)
		}
	}
}
