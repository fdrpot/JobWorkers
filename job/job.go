package job

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type Job struct {
	ID               uint          `json:"id" redis:"id"`
	AttemptsFinished uint          `json:"attempts_finished" redis:"attempts_finished"`
	Status           string        `json:"status" redis:"status"`
	MaxAttempts      uint          `json:"max_attempts" redis:"max_attempts"`
	Data             string        `json:"data" redis:"data"`
	Timeout          time.Duration `json:"timeout" redis:"timeout"`
	TimeAdded        time.Time     `json:"time_added" redis:"time_added"`
}

func (job *Job) SetDefaultValues() {
	job.ID = 0
	job.AttemptsFinished = 0
	job.Status = "pending"
	job.TimeAdded = time.Now().UTC()

	if job.MaxAttempts == 0 {
		job.MaxAttempts = 1
	}

	if job.Timeout == 0 {
		job.Timeout = 3 * time.Second
	}
}

func (job *Job) GetFromDB(rdb *redis.Client, ctx *context.Context, id uint) error {
	res, err := rdb.HGetAll(*ctx, fmt.Sprintf("job:%d", id)).Result()
	if err != nil {
		return fmt.Errorf("error while getting job from DB")
	}
	return job.MapFromMap(&res)
}

func (job *Job) SendToPendingQue(rdb *redis.Client, ctx *context.Context) error {
	return rdb.LPush(*ctx, "pending_jobs", job.ID).Err()
}

func (job *Job) GetFromPendingQue(rdb *redis.Client, ctx *context.Context) error {
	jobID, err := rdb.BLMove(*ctx, "pending_jobs", "processing_jobs", "RIGHT", "LEFT", 0).Result()

	if err != nil {
		return err
	}

	err = rdb.Set(*ctx, fmt.Sprintf("job_start:%s", jobID), time.Now(), 0).Err()

	if err != nil {
		return err
	}

	convertedJobID, err := strconv.Atoi(jobID)

	if err != nil {
		return nil
	}

	return job.GetFromDB(rdb, ctx, uint(convertedJobID))
}

func (job *Job) DeleteFromWorkingQue(rdb *redis.Client, ctx *context.Context) error {
	err := rdb.LRem(*ctx, "processing_jobs", 1, job.ID).Err()
	if err == nil {
		rdb.Del(*ctx, fmt.Sprintf("job_start:%d", job.ID))
	}
	return err
}

func (job *Job) SaveToDB(rdb *redis.Client, ctx *context.Context) error {
	return rdb.HSet(*ctx, fmt.Sprintf("job:%d", job.ID), job).Err()
}

func (job *Job) AddToDB(rdb *redis.Client, ctx *context.Context) error {
	job.SetDefaultValues()

	newID, err := rdb.Incr(*ctx, "job:id").Result()

	if err != nil {
		return fmt.Errorf("error while getting new ID for job")
	}

	job.ID = uint(newID)

	if job.SaveToDB(rdb, ctx) != nil {
		return fmt.Errorf("error while adding job to DB")
	}

	return nil
}

func getUintFromMap(m *map[string]string, key string) (uint, error) {
	if val, ok := (*m)[key]; ok {
		convertedVal, err := strconv.Atoi(val)
		if err != nil {
			return 0, err
		}
		return uint(convertedVal), nil
	}
	return 0, fmt.Errorf("value is not in map")
}

func (job *Job) MapFromMap(val *map[string]string) error {

	var errs []error

	id, err := getUintFromMap(val, "id")
	job.ID = id
	errs = append(errs, err)

	attempts, err := getUintFromMap(val, "attempts_finished")
	job.AttemptsFinished = attempts
	errs = append(errs, err)

	if status, ok := (*val)["status"]; ok {
		job.Status = status
	} else {
		errs = append(errs, fmt.Errorf("value is not in map"))
	}

	maxAttempts, err := getUintFromMap(val, "max_attempts")
	job.MaxAttempts = maxAttempts
	errs = append(errs, err)

	if data, ok := (*val)["data"]; ok {
		job.Data = data
	} else {
		errs = append(errs, fmt.Errorf("value is not in map"))
	}

	if timeout, ok := (*val)["timeout"]; ok {
		convertedTimeout, err := strconv.Atoi(timeout)
		if err != nil {
			errs = append(errs, err)
		}
		job.Timeout = time.Duration(convertedTimeout)
	} else {
		errs = append(errs, fmt.Errorf("value is not in map"))
	}

	if timeAdded, ok := (*val)["time_added"]; ok {
		convertedTime, err := time.Parse(time.RFC3339Nano, timeAdded)
		if err != nil {
			errs = append(errs, err)
		}
		job.TimeAdded = convertedTime
	} else {
		errs = append(errs, fmt.Errorf("value is not in map"))
	}

	return errors.Join(errs...)

}
