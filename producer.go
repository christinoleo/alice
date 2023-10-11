package alice

import (
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog/log"
)

// RabbitProducer models a RabbitMQ producer
type RabbitProducer struct {
	channel  *amqp.Channel // The channel this producer uses to communicate with the broker
	exchange *Exchange     // The exchange this producer produces to
	conn     *connection   // Pointer to broker connection
}

// CreateProducer creates and returns a producer attached to the given exchange.
// The errorHandler can be the DefaultProducerErrorHandler or a custom handler.
func (c *connection) createProducer(exchange *Exchange) (*RabbitProducer, error) {

	var err error

	// Create producer object
	p := &RabbitProducer{
		channel:  nil,
		exchange: exchange,
		conn:     c,
	}

	// Open channel to broker
	err = p.openChannel(p.conn)
	if err != nil {
		// Throw error that channel could not be opened
		return nil, err
	}

	// Declare the exchange
	err = p.declareExchange(p.exchange)
	if err != nil {
		// Throw error that the exchange could not be declared
		return nil, err
	}

	// Listen for channel or connection close message
	p.listenForClose()

	// Listen for overflow messages from broker
	// p.listenForFlow()

	// Listen for returned messages from the broker
	// p.listenForReturnedMessages()

	log.Info().Str("exchange", p.exchange.name).Msg("created producer")

	return p, nil
}

// Open channel to broker
func (p *RabbitProducer) openChannel(c *connection) error {
	var err error
	log.Info().Str("type", "producer").Str("exchange", p.exchange.name).Msg("attempting to open channel")
	p.channel, err = c.conn.Channel()
	if err != nil {
		log.Error().AnErr("err", err).Str("type", "producer").Str("exchange", p.exchange.name).Msg("failed to open channel")
	}
	return err
}

// Declare exchange this producer will produce to
func (p *RabbitProducer) declareExchange(exchange *Exchange) error {
	err := p.channel.ExchangeDeclare(
		exchange.name,
		exchange.exchangeType.String(),
		exchange.durable,
		exchange.autoDelete,
		exchange.internal,
		exchange.noWait,
		exchange.args,
	)
	return err
}

// Subscribe to channel close events and make sure to respond to them
func (p *RabbitProducer) listenForClose() {
	closeChan := p.channel.NotifyClose(make(chan *amqp.Error))
	go func() {
		closeErr := <-closeChan
		log.Error().Str("type", "producer").AnErr("err", closeErr).Str("exchange", p.exchange.name).Msg("connection was closed")
	}()
}

// Listen for flow messages from the broker
// If a flow message comes in we need to do some rate limiting
func (p *RabbitProducer) listenForFlow() {
	flowChan := p.channel.NotifyFlow(make(chan bool))
	go func() {
		for {
			<-flowChan
			log.Error().Str("type", "producer").Str("exchange", p.exchange.name).Msg("too many messages being produced")
		}
	}()
}

// Listen for a returned message from the broker
// Will only be called if the mandatory flag is set and there is no queue bound, or the immediate flag is set and there is no free consumer
func (p *RabbitProducer) listenForReturnedMessages() {
	returnedMessageChan := p.channel.NotifyReturn(make(chan amqp.Return))
	go func() {
		for {
			returnedMsg := <-returnedMessageChan
			log.Error().Str("type", "producer").Str("exchange", p.exchange.name).Interface("msg", returnedMsg).Msg("message was returned")
		}
	}()
}

/*
PublishMessageJsonHeaders publishes a message with the specified routing key

	body: *[]byte, the message to send, most likely a marshalled JSON object
	routingKey: string, the key by which this message will be routed on the exchange
	headers: *map[string]interface{}, the headers for this message, containing things like userID and sessionID
*/
func (p *RabbitProducer) PublishMessageJsonHeaders(body *[]byte, routingKey string, headers *map[string]interface{}) {
	h := amqp.Table{}

	for k, v := range *headers {
		h[k] = v
	}

	p.PublishMessage(*body, &routingKey, &h)
}

// PublishMessage publishes a message with the given routing key
func (p *RabbitProducer) PublishMessage(msg []byte, key *string, headers *amqp.Table) {

	log.Trace().Str("type", "producer").Str("routingKey", *key).Str("exchange", p.exchange.name).Int("msgSize", len(msg)).Msg("producing message")

	err := p.channel.Publish(
		p.exchange.name,
		*key,
		false,
		false,
		amqp.Publishing{
			DeliveryMode: amqp.Transient,
			ContentType:  "plaintext",
			Body:         msg,
			Timestamp:    time.Now(),
			Headers:      *headers,
		},
	)
	if err != nil {
		p.ReconnectChannel()
		err := p.channel.Publish(
			p.exchange.name,
			*key,
			false,
			false,
			amqp.Publishing{
				DeliveryMode: amqp.Transient,
				ContentType:  "plaintext",
				Body:         msg,
				Timestamp:    time.Now(),
				Headers:      *headers,
			},
		)

		if err != nil {
			log.Error().Str("type", "producer").AnErr("err", err).Str("routingKey", *key).Str("exchange", p.exchange.name).Msg("error during message production")
		}
	}
}

// ReconnectChannel tries to re-open this producer's channel
func (p *RabbitProducer) ReconnectChannel() {
	p.openChannel(p.conn)
}

// Shutdown closes this producer's channel
func (p *RabbitProducer) Shutdown() error {
	log.Info().Str("type", "producer").Str("exchange", p.exchange.name).Msg("shutting down")
	return p.channel.Close()
}
