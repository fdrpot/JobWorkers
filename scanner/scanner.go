package main

import (
	"context"
	"errors"
	"fmt"
	"log"
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
		fmt.Println("Scan for old jobs...")
		n, err := checkLostJobs()
		if err != nil {
			fmt.Println(err)
		}
		fmt.Printf("Number of old jobs, found during this scan: %d.\n", n)
		time.Sleep(time.Second * 30)
	}
}

func checkLostJobs() (uint, error) {
	processingIDs, err := rdb.LRange(ctx, "processing_jobs", 0, -1).Result()

	if err != nil {
		return 0, err
	}

	counter := uint(0)

	for _, id := range processingIDs {
		startTime, err1 := rdb.Get(ctx, fmt.Sprintf("job_start:%s", id)).Result()

		parsedTime, err2 := time.Parse(time.RFC3339Nano, startTime)

		if time.Since(parsedTime) > time.Second*10 || err1 != nil || err2 != nil {
			err = transferJobToPending(id)
			if err != nil {
				return counter, err
			}
			counter += 1
		}
	}

	return counter, nil
}

func transferJobToPending(id string) error {
	fmt.Printf("Old job #%s was found in processing que. Transfer to pending que...\n", id)
	return errors.Join(rdb.LPush(ctx, "pending_jobs", id).Err(), rdb.LRem(ctx, "processing_jobs", 1, id).Err())
}
