package alice

import (
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog/log"
)

// A RabbitBroker implements the Broker interface
type RabbitBroker struct {
	config       *ConnectionConfig // The config for the connection
	consumerConn *connection       // Dedicated connection for consumers
	producerConn *connection       // Dedicated connection for producers
}

/*
CreateBroker creates a broker

	config: *ConnectionConfig, the connection configuration that should be used to connect to the broker
	Returns Broker and a possible error
*/
func CreateBroker(config *ConnectionConfig) (Broker, error) {
	broker := RabbitBroker{
		config: config,
	}

	log.Info().Msg("creating broker")

	attemptNumber := 1

	// Check if reconnect is turned on
	if config.autoReconnect {
		// Create a ticker with the reconnect delay
		ticker := time.NewTicker(config.reconnectDelay)

		// Create done channel (for when the connection is successfully established)
		done := make(chan bool, 1)

		log.Info().Msg("attempting RabbitMQ connection")

		// Attempt to connect to the broker
		conn, err := broker.connect()

		// If there is no error
		if err == nil {
			// Close the connection for now
			conn.conn.Close()

			// Stop the ticker
			ticker.Stop()

			// Signal that we are done
			done <- true
		} else {
			// There is an error
			log.Error().Err(err).Int("attempt", attemptNumber).Msg("error while connecting to RabbitMQ")
		}

		for {
			select {
			case <-ticker.C:
				log.Info().Msg("attempting RabbitMQ connection")

				// Attempt to connect to the broker
				conn, err := broker.connect()

				// If there is no error
				if err == nil {
					// Close the connection for now
					conn.conn.Close()

					// Stop the ticker
					ticker.Stop()

					// Signal that we are done
					done <- true
				}

				attemptNumber++
			case <-done:
				log.Info().Int("attempt", attemptNumber).Msg("successfully connected to RabbitMQ")
				return &broker, nil
			}
		}
	} else {
		// Test connection
		conn, err := broker.connect()
		if err != nil {
			return nil, err
		}

		// Close connection as it is not needed right now
		conn.conn.Close()
		return &broker, nil
	}
}

/*
CreateConsumer creates a consumer

	queue: *Queue, the queue this consumer should bind to
	bindingKey: string, the key with which this consumer binds to the queue
	errorHandler: func(error), the function to handle possible consumer errors
	Returns: Consumer and a possible error
*/
func (b *RabbitBroker) CreateConsumer(queue *Queue, bindingKey string, consumerTag string) (Consumer, error) {
	if b.consumerConn == nil {
		b.consumerConn, _ = b.connect()
		go b.consumerConn.reconnect("consumer", b.consumerConn.conn.NotifyClose(make(chan *amqp.Error)))
	}

	return b.consumerConn.createConsumer(queue, bindingKey, consumerTag)
}

/*
CreateProducer creates a producer

	exchange: *Exchange, the exchange this producer will produce to
	errorHandler: func(ProducerError), the errorhandler for this producer
	Returns: Producer and a possible error
*/
func (b *RabbitBroker) CreateProducer(exchange *Exchange) (Producer, error) {
	if b.producerConn == nil {
		b.producerConn, _ = b.connect()
		go b.producerConn.reconnect("producer", b.producerConn.conn.NotifyClose(make(chan *amqp.Error)))
	}

	return b.producerConn.createProducer(exchange)
}

// Connect attempts to make a connection to the broker using the broker connection config
func (b *RabbitBroker) connect() (*connection, error) {
	var err error

	// Get the connection config from the broker
	config := *b.config

	// Create a connection struct
	connection := &connection{
		conn:   nil,
		config: config,
	}

	// Form the RabbitMQ connection URI
	amqpURI := "amqp://" + config.user + ":" + config.password + "@" + config.host + ":" + fmt.Sprint(config.port)

	// Create a buffered done channel to fill when connection is established
	done := make(chan error, 1)

	// Go create a connection
	go func() {
		// Attempt to dial up RabbitMQ
		connection.conn, err = amqp.Dial(amqpURI)

		// Once AMQP dial has completed, pass a possible error into the done channel
		done <- err
	}()
	err = <-done
	if err != nil {
		return nil, err
	}

	return connection, nil
}
