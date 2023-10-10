package alice

import (
	amqp "github.com/rabbitmq/amqp091-go"
)

// A MockProducer implements the Producer interface
type MockProducer struct {
	exchange *Exchange
	broker   *MockBroker
}

// PublishMessageJsonHeaders publishes a message with JSON headers
func (p *MockProducer) PublishMessageJsonHeaders(body *[]byte, routingKey string, headers *map[string]interface{}) {
	h := amqp.Table{}

	for k, v := range *headers {
		h[k] = v
	}

	p.PublishMessage(*body, &routingKey, &h)
}

// PublishMessage publishes a message
func (p *MockProducer) PublishMessage(msg []byte, key *string, headers *amqp.Table) {
	// Find the queues this message was meant for
	var queuesToSendTo []*Queue = make([]*Queue, 0, 10)
	for _, q := range p.broker.exchanges[p.exchange] {
		if q.bindingKey == *key {
			queuesToSendTo = append(queuesToSendTo, q)
		}
	}

	delivery := amqp.Delivery{
		Headers:         *headers,
		ContentType:     "",
		ContentEncoding: "",
		Body:            msg,
	}

	// Send message to the queues
	for _, q := range queuesToSendTo {
		p.broker.Messages[q] <- delivery
	}
}

// Shutdown shuts this producer down
func (p *MockProducer) Shutdown() error {
	return nil
}
