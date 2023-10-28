package nodes

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/adjust/rmq/v5"
	log "github.com/sirupsen/logrus"

	"github.com/jrmanes/torch/config"
	"github.com/jrmanes/torch/pkg/db/redis"
)

const (
	consumerName            = "torch-consumer" // consumerName name used in the tag to identify the consumer.
	prefetchLimit           = 10               // prefetchLimit
	pollDuration            = 10 * time.Second // pollDuration how often is Torch going to pull data from the queue.
	timeoutDurationConsumer = 60 * time.Second // timeoutDurationConsumer timeout for the consumer.
)

// ConsumerInit initialize the process to check the queues in Redis.
func ConsumerInit(queueName string) {
	errChan := make(chan error, 10)
	go logErrors(errChan)

	red := redis.InitRedisConfig()
	// Create a new context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeoutDurationConsumer)

	// Make sure to call the cancel function to release resources when you're done
	defer cancel()

	connection, err := rmq.OpenConnection(
		"consumer",
		"tcp",
		redis.GetRedisFullURL(),
		2,
		errChan,
	)
	if err != nil {
		log.Error("Error: ", err)
	}

	queue, err := connection.OpenQueue(queueName)
	if err != nil {
		log.Error("Error: ", err)
	}

	if err := queue.StartConsuming(prefetchLimit, pollDuration); err != nil {
		log.Error("Error: ", err)
	}

	_, err = queue.AddConsumerFunc(consumerName, func(delivery rmq.Delivery) {
		log.Info("Performing task: ", delivery.Payload())
		peer := config.Peer{
			NodeName:      delivery.Payload(),
			NodeType:      "da",
			ContainerName: "da",
		}

		// here we wil send the node to generate the id
		err := CheckNodesInDBOrCreateThem(peer, red, ctx)
		if err != nil {
			log.Error("Error checking the nodes: CheckNodesInDBOrCreateThem - ", err)
		}

		if err := delivery.Ack(); err != nil {
			log.Error("Error: ", err)
		}
	})
	if err != nil {
		log.Error("Error: ", err)
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT)
	defer signal.Stop(signals)

	<-signals // wait for signal
	go func() {
		<-signals // hard exit on second signal (in case shutdown gets stuck)
		os.Exit(1)
	}()

	<-connection.StopAllConsuming() // wait for all Consume() calls to finish
}

func logErrors(errChan <-chan error) {
	for err := range errChan {
		switch err := err.(type) {
		case *rmq.HeartbeatError:
			if err.Count == rmq.HeartbeatErrorLimit {
				log.Print("heartbeat error (limit): ", err)
			} else {
				log.Print("heartbeat error: ", err)
			}
		case *rmq.ConsumeError:
			log.Print("consume error: ", err)
		case *rmq.DeliveryError:
			log.Print("delivery error: ", err.Delivery, err)
		default:
			log.Print("other error: ", err)
		}
	}
}
