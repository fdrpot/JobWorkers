package main

import (
	"context"
	"encoding/json"
	"fmt"
	"job_workers/job"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
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
	r := chi.NewRouter()
	r.Post("/jobs", addJobHandler)
	r.Get("/jobs/{job_id}", getJobHandler)
	log.Fatal(http.ListenAndServe(":3333", r))
}

func addJobHandler(w http.ResponseWriter, r *http.Request) {
	var job job.Job

	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		http.Error(w, "JSON encoding error", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if err := job.AddToDB(rdb, &ctx); err != nil {
		http.Error(w, fmt.Sprintf("DB error: %s", err), http.StatusInternalServerError)
		return
	}

	if job.SendToPendingQue(rdb, &ctx) != nil {
		rdb.Del(ctx, fmt.Sprintf("job:%d", job.ID))
		http.Error(w, "error while adding to que", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(job); err != nil {
		http.Error(w, fmt.Sprintf("error while job encoding: %s", err), http.StatusInternalServerError)
	}
}

func getJobHandler(w http.ResponseWriter, r *http.Request) {
	requestedID := chi.URLParam(r, "job_id")

	convertedID, err := strconv.Atoi(requestedID)

	if err != nil {
		http.Error(w, "invalid job_id", http.StatusBadRequest)
		return
	}

	job := job.Job{}
	if err := job.GetFromDB(rdb, &ctx, uint(convertedID)); err != nil {
		http.Error(w, "Job with provided id wasn't found.", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(job); err != nil {
		http.Error(w, fmt.Sprintf("error while job encoding: %s", err), http.StatusInternalServerError)
	}
}
