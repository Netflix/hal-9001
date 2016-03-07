package hal

// Broker is an instance of a broker that can send/receive events.
type Broker interface {
	Name() string
	Send(Evt)
	Stream(out chan *Evt)
}

// BrokerConfig is used to create named instances of brokers using NewBroker()
type BrokerConfig interface {
	NewBroker(name string) Broker
}