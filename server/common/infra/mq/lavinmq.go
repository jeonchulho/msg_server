package mq

import amqp "github.com/rabbitmq/amqp091-go"

func NewConnection(url string) (*amqp.Connection, error) {
	return amqp.Dial(url)
}
