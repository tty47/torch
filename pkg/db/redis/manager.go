package redis

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
)

// SetNodeId stores the values in redis.
func SetNodeId(
	podName string,
	r *RedisClient,
	ctx context.Context,
	output string,
) error {
	// try to get the value from redis
	// if the value is empty, then we add it
	nodeName, err := CheckIfNodeExistsInDB(r, ctx, podName)
	if err != nil {
		return err
	}

	// if the node is not in the db, then we add it
	if nodeName == "" {
		log.Info("Node ", "["+podName+"]"+" not found in Redis, let's add it")
		err := r.SetKey(ctx, podName, output, 1000*time.Hour)
		if err != nil {
			log.Error("Error adding the node to redis: ", err)
			return err
		}
	} else {
		log.Info("Node ", "["+podName+"]"+" found in Redis")
	}

	return nil
}

// CheckIfNodeExistsInDB checks if node is in the DB and return it.
func CheckIfNodeExistsInDB(
	r *RedisClient,
	ctx context.Context,
	nodeName string,
) (string, error) {
	nodeName, err := r.GetKey(ctx, nodeName)
	if err != nil {
		log.Error("Error: ", err)
		return "", err
	}

	return nodeName, err
}
