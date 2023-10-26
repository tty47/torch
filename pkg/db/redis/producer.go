package redis

import (
	"github.com/adjust/rmq/v5"
	log "github.com/sirupsen/logrus"
)

// Producer add data into the queue.
func Producer(data, queueName string) error {
	log.Info("Adding STS [", data, "] node to the queue: [", queueName, "]")
	data += "-0"
	log.Info("Getting the pod from the STS [", data, "]")

	connection, err := rmq.OpenConnection(
		"producer",
		"tcp",
		GetRedisFullURL(),
		2,
		nil,
	)
	if err != nil {
		log.Error("Error: ", err)
		return err
	}

	queue, err := connection.OpenQueue(queueName)
	if err != nil {
		log.Error("Error: ", err)
		return err
	}

	if err := queue.Publish(data); err != nil {
		log.Error("Error, failed to publish: ", err)
		return err
	}

	return nil
}
