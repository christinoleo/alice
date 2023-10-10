package alice

import (
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog/log"
)

// A MockConsumer implements the Consumer interface
type MockConsumer struct {
	queue            *Queue
	broker           *MockBroker
	ReceivedMessages []amqp.Delivery
}

// ConsumeMessages consumes messages sent to the consumer
func (c *MockConsumer) ConsumeMessages(args amqp.Table, autoAck bool, messageHandler func(amqp.Delivery)) {
	for msg := range c.broker.Messages[c.queue] {
		c.ReceivedMessages = append(c.ReceivedMessages, msg)

		go func(msg amqp.Delivery) {
			// Intercept any errors propagating up the stack
			defer func() {
				if err := recover(); err != nil {
					log.Err(err.(error)).Msg("error occurred")
				}
			}()

			// Call the message handler
			messageHandler(msg)
		}(msg)
	}
}

// Shutdown shuts down the consumer
func (c *MockConsumer) Shutdown() error {
	return nil
}
